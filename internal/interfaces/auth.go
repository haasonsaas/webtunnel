package interfaces

import "github.com/yourusername/webtunnel/internal/services/auth"

// AuthServiceInterface defines the contract for authentication services
type AuthServiceInterface interface {
	GenerateToken(userID, email, role string) (string, error)
	ValidateToken(token string) (string, error)
	AuthenticateUser(email, password string) (*auth.User, error)
	GetUserByID(userID string) (*auth.User, error)
}