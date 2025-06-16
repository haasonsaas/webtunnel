package terminal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/webtunnel/internal/config"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   30 * time.Minute,
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
		SessionTimeout:   30 * time.Minute,
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	t.Run("successful session creation", func(t *testing.T) {
		session, err := service.CreateSession("user123", "echo", "/tmp")
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)
		assert.Equal(t, "user123", session.UserID)
		assert.Equal(t, "echo", session.Command)
		assert.Equal(t, "/tmp", session.WorkingDir)
		assert.Equal(t, "active", session.Status)
		assert.NotNil(t, session.cmd)
		assert.NotNil(t, session.pty)

		// Clean up
		err = service.KillSession(session.ID)
		assert.NoError(t, err)
	})

	t.Run("blocked command", func(t *testing.T) {
		service.config.BlockedCommands = []string{"rm", "sudo"}
		_, err := service.CreateSession("user123", "sudo", "/tmp")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "command blocked")
	})

	t.Run("max sessions reached", func(t *testing.T) {
		service.config.MaxSessions = 1
		service.config.BlockedCommands = []string{}

		// Create first session
		session1, err := service.CreateSession("user123", "echo", "/tmp")
		require.NoError(t, err)

		// Try to create second session
		_, err = service.CreateSession("user123", "echo", "/tmp")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "max sessions reached")

		// Clean up
		err = service.KillSession(session1.ID)
		assert.NoError(t, err)
	})
}

func TestListSessions(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   30 * time.Minute,
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create sessions for different users
	session1, err := service.CreateSession("user1", "echo", "/tmp")
	require.NoError(t, err)

	session2, err := service.CreateSession("user2", "echo", "/tmp")
	require.NoError(t, err)

	session3, err := service.CreateSession("user1", "echo", "/tmp")
	require.NoError(t, err)

	// List sessions for user1
	user1Sessions := service.ListSessions("user1")
	assert.Len(t, user1Sessions, 2)

	// List sessions for user2
	user2Sessions := service.ListSessions("user2")
	assert.Len(t, user2Sessions, 1)

	// List all sessions
	allSessions := service.ListSessions("")
	assert.Len(t, allSessions, 3)

	// Clean up
	service.KillSession(session1.ID)
	service.KillSession(session2.ID)
	service.KillSession(session3.ID)
}

func TestGetSession(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   30 * time.Minute,
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
		SessionTimeout:   30 * time.Minute,
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
		SessionTimeout:   30 * time.Minute,
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create a session with bash
	session, err := service.CreateSession("user123", "bash", "/tmp")
	require.NoError(t, err)

	// Send input
	err = service.SendInput(session.ID, []byte("echo test\n"))
	assert.NoError(t, err)

	// Wait a bit for output
	time.Sleep(100 * time.Millisecond)

	// Check that output buffer has content
	service.mu.Lock()
	actualSession := service.sessions[session.ID]
	output := actualSession.outputBuf.String()
	service.mu.Unlock()

	assert.Contains(t, output, "test")

	// Clean up
	service.KillSession(session.ID)
}

func TestCleanupIdleSessions(t *testing.T) {
	cfg := config.SessionConfig{
		MaxSessions:      10,
		SessionTimeout:   100 * time.Millisecond, // Short timeout for testing
		WorkingDirectory: "/tmp",
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	// Create a session
	session, err := service.CreateSession("user123", "sleep", "/tmp")
	require.NoError(t, err)

	// Verify session exists
	_, exists := service.GetSession(session.ID)
	assert.True(t, exists)

	// Wait for timeout
	time.Sleep(200 * time.Millisecond)

	// Run cleanup
	service.cleanupIdleSessions()

	// Verify session is removed
	_, exists = service.GetSession(session.ID)
	assert.False(t, exists)
}

func TestIsCommandBlocked(t *testing.T) {
	cfg := config.SessionConfig{
		BlockedCommands: []string{"rm", "sudo", "dd"},
	}
	logger := zap.NewNop()
	service := New(cfg, logger)

	tests := []struct {
		command string
		blocked bool
	}{
		{"rm", true},
		{"sudo", true},
		{"dd", true},
		{"ls", false},
		{"echo", false},
		{"rm -rf /", true},       // Contains blocked command
		{"sudo apt update", true}, // Contains blocked command
		{"echo rm", false},        // rm is not the command itself
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			blocked := service.isCommandBlocked(tt.command)
			assert.Equal(t, tt.blocked, blocked)
		})
	}
}