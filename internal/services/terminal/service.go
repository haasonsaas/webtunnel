package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/yourusername/webtunnel/internal/config"
	"go.uber.org/zap"
)

type Service struct {
	config   config.SessionConfig
	logger   *zap.Logger
	sessions map[string]*Session
	mu       sync.RWMutex
}

type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Command     string    `json:"command"`
	WorkingDir  string    `json:"working_dir"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	LastActive  time.Time `json:"last_active"`
	
	// Internal fields
	cmd         *exec.Cmd
	pty         *os.File
	ctx         context.Context
	cancel      context.CancelFunc
	connections map[*websocket.Conn]bool
	connMu      sync.RWMutex
	outputBuf   *CircularBuffer
}

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

type Message struct {
	Type      string    `json:"type"`
	Data      string    `json:"data,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
}

type CircularBuffer struct {
	data []byte
	size int
	pos  int
	full bool
	mu   sync.RWMutex
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		data: make([]byte, size),
		size: size,
	}
}

func (cb *CircularBuffer) Write(p []byte) (n int, err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	n = len(p)
	for _, b := range p {
		cb.data[cb.pos] = b
		cb.pos = (cb.pos + 1) % cb.size
		if cb.pos == 0 {
			cb.full = true
		}
	}
	return n, nil
}

func (cb *CircularBuffer) Read() []byte {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if !cb.full && cb.pos == 0 {
		return nil
	}

	var result []byte
	if cb.full {
		result = make([]byte, cb.size)
		copy(result, cb.data[cb.pos:])
		copy(result[cb.size-cb.pos:], cb.data[:cb.pos])
	} else {
		result = make([]byte, cb.pos)
		copy(result, cb.data[:cb.pos])
	}
	return result
}

func New(config config.SessionConfig, logger *zap.Logger) *Service {
	return &Service{
		config:   config,
		logger:   logger,
		sessions: make(map[string]*Session),
	}
}

func (s *Service) CreateSession(userID, command, workingDir string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check session limits
	userSessions := 0
	for _, sess := range s.sessions {
		if sess.UserID == userID && sess.Status == StatusRunning {
			userSessions++
		}
	}

	if userSessions >= s.config.MaxSessions {
		return nil, fmt.Errorf("user has reached maximum session limit (%d)", s.config.MaxSessions)
	}

	// Validate command if restrictions are configured
	if len(s.config.AllowedCommands) > 0 {
		allowed := false
		for _, allowedCmd := range s.config.AllowedCommands {
			if command == allowedCmd {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("command not allowed: %s", command)
		}
	}

	// Check blocked commands
	for _, blockedCmd := range s.config.BlockedCommands {
		if command == blockedCmd {
			return nil, fmt.Errorf("command is blocked: %s", command)
		}
	}

	// Generate session ID
	sessionID := generateSessionID()

	// Setup working directory
	if workingDir == "" {
		workingDir = s.config.WorkingDirectory
	}
	sessionWorkDir := filepath.Join(workingDir, "sessions", sessionID)
	if err := os.MkdirAll(sessionWorkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	// Create context for session
	ctx, cancel := context.WithCancel(context.Background())

	// Create session
	session := &Session{
		ID:          sessionID,
		UserID:      userID,
		Command:     command,
		WorkingDir:  sessionWorkDir,
		Status:      StatusRunning,
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[*websocket.Conn]bool),
		outputBuf:   NewCircularBuffer(1024 * 1024), // 1MB buffer
	}

	// Start the process
	if err := s.startProcess(session); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	s.sessions[sessionID] = session

	s.logger.Info("Created new terminal session",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID),
		zap.String("command", command),
	)

	return session, nil
}

func (s *Service) GetSession(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	session, exists := s.sessions[sessionID]
	return session, exists
}

func (s *Service) ListSessions(userID string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var userSessions []*Session
	for _, session := range s.sessions {
		if session.UserID == userID {
			userSessions = append(userSessions, session)
		}
	}
	return userSessions
}

func (s *Service) KillSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Cancel the session context
	session.cancel()
	
	// Close PTY
	if session.pty != nil {
		session.pty.Close()
	}

	// Kill the process
	if session.cmd != nil && session.cmd.Process != nil {
		session.cmd.Process.Kill()
	}

	session.Status = StatusStopped
	
	// Close all websocket connections
	session.connMu.Lock()
	for conn := range session.connections {
		conn.Close()
	}
	session.connMu.Unlock()

	delete(s.sessions, sessionID)

	s.logger.Info("Killed terminal session", zap.String("session_id", sessionID))
	return nil
}

func (s *Service) SendInput(sessionID string, input []byte) error {
	session, exists := s.GetSession(sessionID)
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusRunning {
		return fmt.Errorf("session is not running")
	}

	session.LastActive = time.Now()

	// Write input to PTY
	if session.pty != nil {
		_, err := session.pty.Write(input)
		return err
	}

	return fmt.Errorf("session PTY not available")
}

func (s *Service) AttachWebSocket(sessionID string, conn *websocket.Conn) error {
	session, exists := s.GetSession(sessionID)
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if session.Status != StatusRunning {
		return fmt.Errorf("session is not running")
	}

	session.connMu.Lock()
	session.connections[conn] = true
	session.connMu.Unlock()

	s.logger.Info("WebSocket attached to session", 
		zap.String("session_id", sessionID),
		zap.Int("total_connections", len(session.connections)))

	// Send welcome message
	welcomeMsg := Message{
		Type:      "output",
		Data:      fmt.Sprintf("\r\n🌐 WebTunnel connected to session %s\r\n", sessionID),
		Timestamp: time.Now(),
		SessionID: sessionID,
	}
	if err := conn.WriteJSON(welcomeMsg); err != nil {
		s.logger.Error("Failed to send welcome message", zap.Error(err))
	}

	// Send existing output buffer
	if buffer := session.outputBuf.Read(); len(buffer) > 0 {
		msg := Message{
			Type:      "output", 
			Data:      string(buffer),
			Timestamp: time.Now(),
			SessionID: sessionID,
		}
		if err := conn.WriteJSON(msg); err != nil {
			s.logger.Error("Failed to send buffer to WebSocket", zap.Error(err))
		}
	}

	// Handle WebSocket messages in goroutine
	go s.handleWebSocketMessages(session, conn)

	return nil
}

func (s *Service) handleWebSocketMessages(session *Session, conn *websocket.Conn) {
	defer func() {
		session.connMu.Lock()
		delete(session.connections, conn)
		session.connMu.Unlock()
		conn.Close()
		s.logger.Info("WebSocket disconnected from session", 
			zap.String("session_id", session.ID),
			zap.Int("remaining_connections", len(session.connections)))
	}()

	// Set connection limits
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket unexpected close", zap.Error(err))
			} else {
				s.logger.Debug("WebSocket connection closed", zap.Error(err))
			}
			break
		}

		// Reset read deadline on successful message
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Handle different message types
		switch msg.Type {
		case "input":
			if err := s.SendInput(session.ID, []byte(msg.Data)); err != nil {
				s.logger.Error("Failed to send input to session", 
					zap.Error(err), 
					zap.String("session_id", session.ID))
				
				// Send error back to client
				errorMsg := Message{
					Type:      "error",
					Data:      fmt.Sprintf("Failed to send input: %v", err),
					Timestamp: time.Now(),
					SessionID: session.ID,
				}
				conn.WriteJSON(errorMsg)
			}

		case "resize":
			// Handle terminal resize
			var resizeData struct {
				Cols int `json:"cols"`
				Rows int `json:"rows"`
			}
			if err := json.Unmarshal([]byte(msg.Data), &resizeData); err == nil {
				if session.pty != nil {
					if err := pty.Setsize(session.pty, &pty.Winsize{
						Rows: uint16(resizeData.Rows),
						Cols: uint16(resizeData.Cols),
					}); err != nil {
						s.logger.Error("Failed to resize PTY", zap.Error(err))
					} else {
						s.logger.Debug("PTY resized", 
							zap.Int("cols", resizeData.Cols),
							zap.Int("rows", resizeData.Rows))
					}
				}
			}

		case "ping":
			// Respond to ping with pong
			pongMsg := Message{
				Type:      "pong",
				Timestamp: time.Now(),
				SessionID: session.ID,
			}
			if err := conn.WriteJSON(pongMsg); err != nil {
				s.logger.Error("Failed to send pong", zap.Error(err))
			}

		default:
			s.logger.Warn("Unknown message type", 
				zap.String("type", msg.Type),
				zap.String("session_id", session.ID))
		}
	}
}

func (s *Service) CleanupStaleSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	timeout := 1 * time.Hour // Configure this
	now := time.Now()

	for sessionID, session := range s.sessions {
		if now.Sub(session.LastActive) > timeout {
			s.logger.Info("Cleaning up stale session", zap.String("session_id", sessionID))
			
			session.cancel()
			if session.pty != nil {
				session.pty.Close()
			}
			if session.cmd != nil && session.cmd.Process != nil {
				session.cmd.Process.Kill()
			}
			
			delete(s.sessions, sessionID)
		}
	}
}

func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, session := range s.sessions {
		session.cancel()
		if session.pty != nil {
			session.pty.Close()
		}
		if session.cmd != nil && session.cmd.Process != nil {
			session.cmd.Process.Kill()
		}
		
		s.logger.Info("Shutdown session", zap.String("session_id", sessionID))
	}
	
	s.sessions = make(map[string]*Session)
}

func (s *Service) startProcess(session *Session) error {
	// Determine the shell and command to run
	shell := "/bin/bash"
	if shellEnv := os.Getenv("SHELL"); shellEnv != "" {
		shell = shellEnv
	}

	var cmd *exec.Cmd
	if session.Command == "bash" || session.Command == "sh" || session.Command == "" {
		// Start interactive shell
		cmd = exec.CommandContext(session.ctx, shell)
	} else {
		// Run specific command in shell
		cmd = exec.CommandContext(session.ctx, shell, "-c", session.Command)
	}

	cmd.Dir = session.WorkingDir

	// Set environment variables
	env := os.Environ()
	for key, value := range s.config.EnvironmentVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	// Add session-specific environment
	env = append(env, fmt.Sprintf("WEBTUNNEL_SESSION_ID=%s", session.ID))
	env = append(env, fmt.Sprintf("WEBTUNNEL_USER_ID=%s", session.UserID))
	cmd.Env = env

	session.cmd = cmd

	// Start the command with PTY
	var err error
	session.pty, err = pty.Start(session.cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}

	// Set initial PTY size
	if err := pty.Setsize(session.pty, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		s.logger.Warn("Failed to set initial PTY size", zap.Error(err))
	}

	s.logger.Info("Started PTY session", 
		zap.String("session_id", session.ID),
		zap.String("command", session.Command),
		zap.String("shell", shell),
		zap.Int("pid", session.cmd.Process.Pid))

	// Start output monitoring in goroutine
	go s.monitorOutput(session)

	// Monitor process completion
	go func() {
		if err := session.cmd.Wait(); err != nil {
			s.logger.Info("Session process exited", 
				zap.String("session_id", session.ID),
				zap.Error(err))
		} else {
			s.logger.Info("Session process completed normally", 
				zap.String("session_id", session.ID))
		}
		session.Status = StatusStopped
	}()

	return nil
}

func (s *Service) monitorOutput(session *Session) {
	defer func() {
		if session.pty != nil {
			session.pty.Close()
		}
		session.Status = StatusStopped
		s.logger.Info("Session output monitoring stopped", zap.String("session_id", session.ID))
	}()

	// Use a buffer to read PTY output in chunks
	buffer := make([]byte, 4096)
	
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
			// Set read timeout to avoid blocking indefinitely
			session.pty.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			
			n, err := session.pty.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue // Timeout is expected, continue reading
				}
				if err == io.EOF {
					s.logger.Info("PTY EOF reached", zap.String("session_id", session.ID))
					return
				}
				s.logger.Error("Error reading from PTY", zap.Error(err), zap.String("session_id", session.ID))
				session.Status = StatusError
				return
			}
			
			if n > 0 {
				output := buffer[:n]
				
				// Write to buffer
				session.outputBuf.Write(output)
				
				// Send to all connected WebSockets
				session.connMu.RLock()
				for conn := range session.connections {
					msg := Message{
						Type:      "output",
						Data:      string(output),
						Timestamp: time.Now(),
						SessionID: session.ID,
					}
					if err := conn.WriteJSON(msg); err != nil {
						s.logger.Error("Failed to send output to WebSocket", zap.Error(err))
						// Remove failed connection
						delete(session.connections, conn)
						conn.Close()
					}
				}
				session.connMu.RUnlock()
				
				// Update last active time
				session.LastActive = time.Now()
			}
		}
	}
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", time.Now().Unix(), time.Now().UnixNano()%1000000)
}