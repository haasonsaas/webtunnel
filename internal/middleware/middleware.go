package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logger.Info("HTTP Request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
		)
		return ""
	})
}

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		logger.Error("Panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}

func CORS(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Simple CORS implementation
		if len(allowOrigins) > 0 && allowOrigins[0] == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, allowedOrigin := range allowOrigins {
				if origin == allowedOrigin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), requestsPerMinute)
	
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// AuthServiceInterface defines the contract for authentication services  
type AuthServiceInterface interface {
	ValidateToken(token string) (string, error)
}

func JWTAuth(authService AuthServiceInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		userID, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}