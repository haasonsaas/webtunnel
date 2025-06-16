package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/webtunnel/internal/config"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
		BlockedCommands:  []string{"rm", "sudo"},
	}
	logger := zap.NewNop()

	service := New(cfg, logger)

	assert.NotNil(t, service)
	assert.Equal(t, cfg, service.config)
	assert.NotNil(t, service.sessions)
	assert.NotNil(t, service.logger)
}

func TestCreateSession(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Test successful session creation
	session, err := service.CreateSession("user123", "echo", "/tmp")
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "user123", session.UserID)
	assert.Equal(t, "echo", session.Command)
	assert.NotNil(t, session.cmd)
	assert.NotNil(t, session.pty)

	// Clean up
	err = service.KillSession(session.ID)
	assert.NoError(t, err)
}

func TestBlockedCommands(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
		BlockedCommands:  []string{"rm", "sudo"},
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Test blocked command
	_, err := service.CreateSession("user123", "sudo", "/tmp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command is blocked")
}

func TestGetSession(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create a session
	session, err := service.CreateSession("user123", "echo", "/tmp")
	require.NoError(t, err)

	// Get existing session
	retrieved, exists := service.GetSession(session.ID)
	assert.True(t, exists)
	assert.Equal(t, session.ID, retrieved.ID)
	assert.Equal(t, session.UserID, retrieved.UserID)

	// Get non-existent session
	_, exists = service.GetSession("non-existent")
	assert.False(t, exists)

	// Clean up
	service.KillSession(session.ID)
}

func TestKillSession(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create a session
	session, err := service.CreateSession("user123", "sleep", "/tmp")
	require.NoError(t, err)

	// Kill the session
	err = service.KillSession(session.ID)
	assert.NoError(t, err)

	// Verify session is removed
	_, exists := service.GetSession(session.ID)
	assert.False(t, exists)

	// Try to kill non-existent session
	err = service.KillSession("non-existent")
	assert.Error(t, err)
}

func TestSendInput(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   "30m",
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create a session with bash
	session, err := service.CreateSession("user123", "bash", "/tmp")
	require.NoError(t, err)

	// Send input - should not error
	err = service.SendInput(session.ID, []byte("echo test\n"))
	assert.NoError(t, err)

	// Clean up
	service.KillSession(session.ID)
}