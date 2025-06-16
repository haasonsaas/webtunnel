package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yourusername/webtunnel/internal/services/auth"
	"github.com/yourusername/webtunnel/internal/services/session"
	"github.com/yourusername/webtunnel/internal/services/terminal"
	"go.uber.org/zap"
)

// Health check handler
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"timestamp": gin.H{
			"unix": c.Request.Context().Value("timestamp"),
		},
	})
}

// Auth handlers
type AuthHandler struct {
	authService *auth.Service
	logger      *zap.Logger
}

func NewAuth(authService *auth.Service, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.AuthenticateUser(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	userID := c.GetString("user_id")
	
	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// Session handlers
type SessionHandler struct {
	termService *terminal.Service
	sessService *session.Service
	logger      *zap.Logger
}

func NewSession(termService *terminal.Service, sessService *session.Service, logger *zap.Logger) *SessionHandler {
	return &SessionHandler{
		termService: termService,
		sessService: sessService,
		logger:      logger,
	}
}

func (h *SessionHandler) List(c *gin.Context) {
	userID := c.GetString("user_id")
	sessions := h.termService.ListSessions(userID)
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (h *SessionHandler) Create(c *gin.Context) {
	userID := c.GetString("user_id")
	
	var req struct {
		Command    string `json:"command" binding:"required"`
		WorkingDir string `json:"working_dir"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.termService.CreateSession(userID, req.Command, req.WorkingDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

func (h *SessionHandler) Get(c *gin.Context) {
	sessionID := c.Param("id")
	
	session, exists := h.termService.GetSession(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) Delete(c *gin.Context) {
	sessionID := c.Param("id")
	
	if err := h.termService.KillSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

func (h *SessionHandler) SendInput(c *gin.Context) {
	sessionID := c.Param("id")
	
	var req struct {
		Input string `json:"input" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.termService.SendInput(sessionID, []byte(req.Input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Input sent"})
}

func (h *SessionHandler) Stream(c *gin.Context) {
	sessionID := c.Param("id")
	
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade to WebSocket", zap.Error(err))
		return
	}

	if err := h.termService.AttachWebSocket(sessionID, conn); err != nil {
		h.logger.Error("Failed to attach WebSocket", zap.Error(err))
		conn.Close()
		return
	}
}

func (h *SessionHandler) Share(c *gin.Context) {
	sessionID := c.Param("id")
	
	// Generate shareable URL
	shareURL := "https://" + c.Request.Host + "/shared/" + sessionID
	
	c.JSON(http.StatusOK, gin.H{
		"share_url": shareURL,
		"expires_at": "24h", // Demo value
	})
}

// File handlers
type FileHandler struct {
	logger *zap.Logger
}

func NewFile(logger *zap.Logger) *FileHandler {
	return &FileHandler{
		logger: logger,
	}
}

func (h *FileHandler) Browse(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = "/tmp"
	}

	// Simple file browser implementation
	c.JSON(http.StatusOK, gin.H{
		"path": path,
		"files": []gin.H{
			{"name": "example.txt", "type": "file", "size": 1024},
			{"name": "folder", "type": "directory", "size": 0},
		},
	})
}

func (h *FileHandler) Upload(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Upload not implemented yet"})
}

func (h *FileHandler) Download(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Download not implemented yet"})
}

// User handlers
type UserHandler struct {
	authService *auth.Service
	logger      *zap.Logger
}

func NewUser(authService *auth.Service, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		authService: authService,
		logger:      logger,
	}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	
	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update profile not implemented yet"})
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in demo
	},
}