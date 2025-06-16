package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/webtunnel/internal/config"
	"github.com/yourusername/webtunnel/internal/database"
	"github.com/yourusername/webtunnel/internal/middleware"
	"github.com/yourusername/webtunnel/internal/services/auth"
	"github.com/yourusername/webtunnel/internal/services/session"
	"github.com/yourusername/webtunnel/internal/services/terminal"
	"github.com/yourusername/webtunnel/internal/handlers"
	"go.uber.org/zap"
)

type Server struct {
	config       *config.Config
	logger       *zap.Logger
	httpServer   *http.Server
	db           *database.DB
	authService  *auth.Service
	termService  *terminal.Service
	sessService  *session.Service
}

func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	// Initialize database
	db, err := database.New(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize services
	authService := auth.New(cfg.Auth, db, logger)
	termService := terminal.New(cfg.Session, logger)
	sessService := session.New(cfg.Redis, logger)

	server := &Server{
		config:      cfg,
		logger:      logger,
		db:          db,
		authService: authService,
		termService: termService,
		sessService: sessService,
	}

	// Setup HTTP server
	server.setupHTTPServer()

	return server, nil
}

func (s *Server) setupHTTPServer() {
	// Set Gin mode
	if s.config.Server.TLS {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	
	// Global middleware
	router.Use(middleware.Logger(s.logger))
	router.Use(middleware.Recovery(s.logger))
	router.Use(middleware.CORS(s.config.Server.AllowOrigins))
	router.Use(middleware.RateLimit(s.config.Auth.RateLimit))

	// Health check endpoint
	router.GET("/health", handlers.Health)

	// API routes
	api := router.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			authHandler := handlers.NewAuth(s.authService, s.logger)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.POST("/refresh", authHandler.Refresh)
		}

		// Protected routes
		protected := api.Group("")
		protected.Use(middleware.JWTAuth(s.authService))
		{
			// Session management
			sessions := protected.Group("/sessions")
			{
				sessHandler := handlers.NewSession(s.termService, s.sessService, s.logger)
				sessions.GET("", sessHandler.List)
				sessions.POST("", sessHandler.Create)
				sessions.GET("/:id", sessHandler.Get)
				sessions.DELETE("/:id", sessHandler.Delete)
				sessions.POST("/:id/input", sessHandler.SendInput)
				sessions.GET("/:id/stream", sessHandler.Stream)
				sessions.GET("/:id/share", sessHandler.Share)
			}

			// File operations
			files := protected.Group("/files")
			{
				fileHandler := handlers.NewFile(s.logger)
				files.GET("/browse", fileHandler.Browse)
				files.POST("/upload", fileHandler.Upload)
				files.GET("/download", fileHandler.Download)
			}

			// User management
			users := protected.Group("/users")
			{
				userHandler := handlers.NewUser(s.authService, s.logger)
				users.GET("/profile", userHandler.GetProfile)
				users.PUT("/profile", userHandler.UpdateProfile)
			}
		}
	}

	// Serve static files (React app)
	router.Static("/static", s.config.Server.StaticDir)
	router.StaticFile("/", s.config.Server.StaticDir+"/index.html")
	router.NoRoute(func(c *gin.Context) {
		c.File(s.config.Server.StaticDir + "/index.html")
	})

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Configure TLS if enabled
	if s.config.Server.TLS {
		if s.config.Server.CertFile != "" && s.config.Server.KeyFile != "" {
			// Use provided certificates
			s.httpServer.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		} else {
			// Generate self-signed certificates
			s.logger.Info("Generating self-signed TLS certificates")
			// Implementation would generate certs here
		}
	}
}

func (s *Server) Run(ctx context.Context) error {
	// Start cleanup routines
	go s.startCleanupRoutines(ctx)

	// Start HTTP server
	errChan := make(chan error, 1)
	
	go func() {
		s.logger.Info("Starting HTTP server",
			zap.String("addr", s.httpServer.Addr),
			zap.Bool("tls", s.config.Server.TLS),
		)

		var err error
		if s.config.Server.TLS {
			if s.config.Server.CertFile != "" && s.config.Server.KeyFile != "" {
				err = s.httpServer.ListenAndServeTLS(s.config.Server.CertFile, s.config.Server.KeyFile)
			} else {
				// Would use auto-generated certs
				err = s.httpServer.ListenAndServe()
			}
		} else {
			err = s.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-ctx.Done():
		s.logger.Info("Shutdown signal received, gracefully shutting down...")
		return s.shutdown()
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	}
}

func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("Error shutting down HTTP server", zap.Error(err))
	}

	// Close terminal sessions
	s.termService.Shutdown()

	// Close database connections
	s.db.Close()

	s.logger.Info("Server shutdown complete")
	return nil
}

func (s *Server) startCleanupRoutines(ctx context.Context) {
	// Terminal session cleanup
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.termService.CleanupStaleSessions()
		}
	}
}