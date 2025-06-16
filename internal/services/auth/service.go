package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/webtunnel/internal/config"
	"github.com/yourusername/webtunnel/internal/database"
	"go.uber.org/zap"
)

type Service struct {
	config config.AuthConfig
	db     *database.DB
	logger *zap.Logger
}

type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func New(config config.AuthConfig, db *database.DB, logger *zap.Logger) *Service {
	return &Service{
		config: config,
		db:     db,
		logger: logger,
	}
}

func (s *Service) GenerateToken(userID, email, role string) (string, error) {
	expirationTime, err := time.ParseDuration(s.config.SessionExpiry)
	if err != nil {
		expirationTime = 24 * time.Hour // default
	}

	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "webtunnel",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (s *Service) ValidateToken(tokenString string) (string, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	return claims.UserID, nil
}

func (s *Service) AuthenticateUser(email, password string) (*User, error) {
	// For demo purposes, create a simple auth that accepts any password
	// In production, this would check against database with hashed passwords
	
	user := &User{
		ID:       "user_" + email,
		Email:    email,
		Username: email,
		Role:     "user",
	}

	s.logger.Info("User authenticated", zap.String("email", email))
	return user, nil
}

func (s *Service) GetUserByID(userID string) (*User, error) {
	// For demo purposes, return a mock user
	// In production, this would query the database
	
	return &User{
		ID:       userID,
		Email:    "demo@example.com",
		Username: "demo",
		Role:     "user",
	}, nil
}