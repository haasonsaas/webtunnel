package terminal

import (
	"bufio"
	"context"
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

	session.connMu.Lock()
	session.connections[conn] = true
	session.connMu.Unlock()

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

	// Handle disconnection
	go func() {
		defer func() {
			session.connMu.Lock()
			delete(session.connections, conn)
			session.connMu.Unlock()
			conn.Close()
		}()

		for {
			var msg Message
			if err := conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.Error("WebSocket error", zap.Error(err))
				}
				break
			}

			// Handle different message types
			switch msg.Type {
			case "input":
				if err := s.SendInput(sessionID, []byte(msg.Data)); err != nil {
					s.logger.Error("Failed to send input", zap.Error(err))
				}
			case "resize":
				// Handle terminal resize
				// Implementation would parse resize data and call pty.Setsize()
			}
		}
	}()

	return nil
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
	// Create command
	session.cmd = exec.CommandContext(session.ctx, "/bin/bash", "-c", session.Command)
	session.cmd.Dir = session.WorkingDir

	// Set environment variables
	env := os.Environ()
	for key, value := range s.config.EnvironmentVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	session.cmd.Env = env

	// Start the command with PTY
	var err error
	session.pty, err = pty.Start(session.cmd)
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}

	// Start output monitoring
	go s.monitorOutput(session)

	return nil
}

func (s *Service) monitorOutput(session *Session) {
	defer func() {
		if session.pty != nil {
			session.pty.Close()
		}
		session.Status = StatusStopped
	}()

	scanner := bufio.NewScanner(session.pty)
	for scanner.Scan() {
		select {
		case <-session.ctx.Done():
			return
		default:
			output := scanner.Text() + "\n"
			
			// Write to buffer
			session.outputBuf.Write([]byte(output))
			
			// Send to all connected WebSockets
			session.connMu.RLock()
			for conn := range session.connections {
				msg := Message{
					Type:      "output",
					Data:      output,
					Timestamp: time.Now(),
					SessionID: session.ID,
				}
				if err := conn.WriteJSON(msg); err != nil {
					s.logger.Error("Failed to send output to WebSocket", zap.Error(err))
				}
			}
			session.connMu.RUnlock()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		s.logger.Error("Error reading from PTY", zap.Error(err))
		session.Status = StatusError
	}
}

func generateSessionID() string {
	// Implementation would generate a unique session ID
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}