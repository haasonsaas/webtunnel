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
)

func main() {
	fmt.Println("🌐 WebTunnel Demo - Inspired by VibeTunnel")
	fmt.Println("Starting demo server (no database required)...")

	router := gin.Default()

	// Simple CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Serve static files
	router.Static("/static", "./web/dist")
	router.StaticFile("/", "./web/dist/index.html")
	router.NoRoute(func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"message": "WebTunnel Demo Server",
			"timestamp": time.Now(),
		})
	})

	// Mock API endpoints for demo
	api := router.Group("/api/v1")
	{
		// Mock auth
		api.POST("/auth/login", func(c *gin.Context) {
			var req struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			// Demo: accept any credentials
			c.JSON(http.StatusOK, gin.H{
				"token": "demo-jwt-token-12345",
				"user": gin.H{
					"id":       "demo-user-1",
					"email":    req.Email,
					"username": req.Email,
					"role":     "user",
				},
			})
		})

		// Protected routes (demo - no real auth)
		protected := api.Group("")
		{
			// Mock sessions
			protected.GET("/sessions", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"sessions": []gin.H{
						{
							"id":          "demo-session-1",
							"command":     "bash",
							"status":      "running",
							"working_dir": "/tmp",
							"created_at":  time.Now().Add(-10 * time.Minute),
						},
						{
							"id":          "demo-session-2", 
							"command":     "htop",
							"status":      "stopped",
							"working_dir": "/home",
							"created_at":  time.Now().Add(-1 * time.Hour),
						},
					},
				})
			})

			protected.POST("/sessions", func(c *gin.Context) {
				var req struct {
					Command    string `json:"command"`
					WorkingDir string `json:"working_dir"`
				}
				
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}

				sessionID := fmt.Sprintf("demo-session-%d", time.Now().Unix())
				
				c.JSON(http.StatusCreated, gin.H{
					"id":          sessionID,
					"command":     req.Command,
					"status":      "running",
					"working_dir": req.WorkingDir,
					"created_at":  time.Now(),
				})
			})

			protected.GET("/sessions/:id", func(c *gin.Context) {
				sessionID := c.Param("id")
				c.JSON(http.StatusOK, gin.H{
					"id":          sessionID,
					"command":     "bash",
					"status":      "running",
					"working_dir": "/tmp",
					"created_at":  time.Now().Add(-5 * time.Minute),
				})
			})

			protected.DELETE("/sessions/:id", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Session terminated (demo)",
				})
			})

			protected.POST("/sessions/:id/input", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Input sent (demo mode - no real terminal)",
				})
			})

			// Mock WebSocket endpoint (returns info message)
			protected.GET("/sessions/:id/stream", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "WebSocket streaming not available in demo mode",
					"note": "In full version, this would upgrade to WebSocket",
				})
			})
		}
	}

	// Create server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		fmt.Printf("🚀 Demo server starting on http://localhost:8080\n")
		fmt.Printf("📱 Open http://localhost:8080 in your browser\n")
		fmt.Printf("🔑 Use any email/password to login (demo mode)\n")
		fmt.Printf("⚡ This is a demo version inspired by VibeTunnel\n\n")
		
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

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	fmt.Println("✅ Server exited cleanly")
}