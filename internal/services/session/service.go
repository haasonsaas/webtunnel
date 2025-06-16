package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yourusername/webtunnel/internal/config"
	"go.uber.org/zap"
)

type Service struct {
	redis  *redis.Client
	logger *zap.Logger
}

type SessionData struct {
	UserID    string            `json:"user_id"`
	SessionID string            `json:"session_id"`
	Data      map[string]string `json:"data"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at"`
}

func New(cfg config.RedisConfig, logger *zap.Logger) *Service {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.URL[8:], // Remove redis:// prefix
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &Service{
		redis:  rdb,
		logger: logger,
	}
}

func (s *Service) StoreSession(ctx context.Context, userID, sessionID string, data map[string]string, ttl time.Duration) error {
	sessionData := SessionData{
		UserID:    userID,
		SessionID: sessionID,
		Data:      data,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	bytes, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	key := fmt.Sprintf("session:%s", sessionID)
	return s.redis.Set(ctx, key, bytes, ttl).Err()
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (*SessionData, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	bytes, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var sessionData SessionData
	if err := json.Unmarshal(bytes, &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &sessionData, nil
}

func (s *Service) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("session:%s", sessionID)
	return s.redis.Del(ctx, key).Err()
}

func (s *Service) PublishMessage(ctx context.Context, channel string, message interface{}) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return s.redis.Publish(ctx, channel, bytes).Err()
}

func (s *Service) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return s.redis.Subscribe(ctx, channel)
}