package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/webtunnel/internal/config"
	"github.com/yourusername/webtunnel/internal/handlers"
	"github.com/yourusername/webtunnel/internal/middleware"
	"github.com/yourusername/webtunnel/internal/services/auth"
	"github.com/yourusername/webtunnel/internal/services/terminal"
	"go.uber.org/zap"
)

func main() {
	fmt.Println("🌐 WebTunnel Local - Full Terminal Functionality")
	fmt.Println("Local mode with real terminal sessions (no database required)")

	// Create logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Create local config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:      "127.0.0.1",
			Port:      8081,
			TLS:       false,
			StaticDir: "./web/dist",
		},
		Auth: config.AuthConfig{
			JWTSecret:     "local-test-secret",
			SessionExpiry: "24h",
			RateLimit:     1000,
		},
		Session: config.SessionConfig{
			MaxSessions:      10,
			MaxMemoryMB:      512,
			MaxCPUPercent:    80,
			SessionTimeout:   "1h",
			WorkingDirectory: "/tmp/webtunnel-local",
			BlockedCommands:  []string{"rm", "sudo", "dd"},
			EnvironmentVars: map[string]string{
				"TERM":  "xterm-256color",
				"SHELL": "/bin/bash",
			},
		},
	}

	// Create services (no database required)
	authService := &MockAuthService{}
	termService := terminal.New(cfg.Session, logger)

	// Setup HTTP server
	router := gin.Default()

	// Middleware
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS([]string{"*"}))

	// Static files
	router.Static("/static", cfg.Server.StaticDir)
	router.StaticFile("/", cfg.Server.StaticDir+"/index.html")
	router.NoRoute(func(c *gin.Context) {
		c.File(cfg.Server.StaticDir + "/index.html")
	})

	// Health check
	router.GET("/health", handlers.Health)

	// API routes
	api := router.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			authHandler := handlers.NewAuth(authService, logger)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
		}

		// Protected routes (no real auth in local mode)
		protected := api.Group("")
		{
			// Session management with REAL terminal functionality
			sessions := protected.Group("/sessions")
			{
				sessHandler := handlers.NewSession(termService, nil, logger)
				sessions.GET("", sessHandler.List)
				sessions.POST("", sessHandler.Create)
				sessions.GET("/:id", sessHandler.Get)
				sessions.DELETE("/:id", sessHandler.Delete)
				sessions.POST("/:id/input", sessHandler.SendInput)
				sessions.GET("/:id/stream", sessHandler.Stream) // Real WebSocket streaming!
			}

			// File management routes
			files := protected.Group("/files")
			{
				fileHandler := handlers.NewFile(logger)
				files.GET("/browse", fileHandler.Browse)
				files.POST("/upload/:session_id", fileHandler.Upload)
				files.GET("/download", fileHandler.Download)
			}
		}
	}

	// Create and start server
	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 WebTunnel Local starting on http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
		fmt.Printf("📱 Open http://%s:%d in your browser\n", cfg.Server.Host, cfg.Server.Port)
		fmt.Printf("🔑 Use any email/password to login (local mode)\n")
		fmt.Printf("⚡ Real terminal sessions with PTY support!\n\n")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop terminal sessions
	termService.Shutdown()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	fmt.Println("✅ Server exited cleanly")
}

// MockAuthService provides authentication without database
type MockAuthService struct{}

func (m *MockAuthService) GenerateToken(userID, email, role string) (string, error) {
	return "local-test-token-" + userID, nil
}

func (m *MockAuthService) ValidateToken(token string) (string, error) {
	return "local-user", nil
}

func (m *MockAuthService) AuthenticateUser(email, password string) (*auth.User, error) {
	return &auth.User{
		ID:       "local-user",
		Email:    email,
		Username: email,
		Role:     "admin",
	}, nil
}

func (m *MockAuthService) GetUserByID(userID string) (*auth.User, error) {
	return &auth.User{
		ID:       userID,
		Email:    "local@webtunnel.dev",
		Username: "local",
		Role:     "admin",
	}, nil
}