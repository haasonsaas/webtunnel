package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	authService AuthServiceInterface
	logger      *zap.Logger
}

// AuthServiceInterface defines the contract for authentication services
type AuthServiceInterface interface {
	GenerateToken(userID, email, role string) (string, error)
	ValidateToken(token string) (string, error)
	AuthenticateUser(email, password string) (*auth.User, error)
	GetUserByID(userID string) (*auth.User, error)
}

func NewAuth(authService AuthServiceInterface, logger *zap.Logger) *AuthHandler {
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

	// Security check - prevent directory traversal
	if strings.Contains(path, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path"})
		return
	}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory"})
		return
	}

	var files []gin.H
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := "file"
		if entry.IsDir() {
			fileType = "directory"
		}

		files = append(files, gin.H{
			"name": entry.Name(),
			"type": fileType,
			"size": info.Size(),
			"modified": info.ModTime().Format(time.RFC3339),
			"permissions": info.Mode().String(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"path": path,
		"files": files,
	})
}

func (h *FileHandler) Upload(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID required"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
		return
	}
	defer file.Close()

	targetPath := c.PostForm("path")
	if targetPath == "" {
		targetPath = "/tmp/" + header.Filename
	}

	// Create target file
	dst, err := os.Create(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file"})
		return
	}
	defer dst.Close()

	// Copy file content
	written, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"path": targetPath,
		"size": written,
	})
}

func (h *FileHandler) Download(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File path required"})
		return
	}

	// Security check - prevent directory traversal
	if strings.Contains(filePath, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file path"})
		return
	}

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access file"})
		}
		return
	}

	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot download directory"})
		return
	}

	// Set appropriate headers
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Send file
	c.File(filePath)
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