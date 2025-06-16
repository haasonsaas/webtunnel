package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Session  SessionConfig  `mapstructure:"session"`
}

type ServerConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	TLS          bool   `mapstructure:"tls"`
	CertFile     string `mapstructure:"cert_file"`
	KeyFile      string `mapstructure:"key_file"`
	StaticDir    string `mapstructure:"static_dir"`
	AllowOrigins []string `mapstructure:"allow_origins"`
}

type DatabaseConfig struct {
	URL             string `mapstructure:"url"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime string `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AuthConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	SessionExpiry string `mapstructure:"session_expiry"`
	RateLimit     int    `mapstructure:"rate_limit"`
}

type SessionConfig struct {
	MaxSessions        int    `mapstructure:"max_sessions"`
	MaxMemoryMB        int    `mapstructure:"max_memory_mb"`
	MaxCPUPercent      int    `mapstructure:"max_cpu_percent"`
	SessionTimeout     string `mapstructure:"session_timeout"`
	CleanupInterval    string `mapstructure:"cleanup_interval"`
	WorkingDirectory   string `mapstructure:"working_directory"`
	AllowedCommands    []string `mapstructure:"allowed_commands"`
	BlockedCommands    []string `mapstructure:"blocked_commands"`
	EnvironmentVars    map[string]string `mapstructure:"environment_vars"`
}

func Load(configFile string) (*Config, error) {
	v := viper.New()
	
	// Set defaults
	setDefaults(v)
	
	// Set config file
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		v.SetConfigName(".webtunnel")
		v.SetConfigType("yaml")
		v.AddConfigPath("$HOME")
		v.AddConfigPath(".")
	}

	// Environment variables
	v.SetEnvPrefix("WEBTUNNEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind command line flags
	v.BindPFlag("server.host", nil)
	v.BindPFlag("server.port", nil)
	v.BindPFlag("server.tls", nil)
	v.BindPFlag("database.url", nil)
	v.BindPFlag("redis.url", nil)

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8443)
	v.SetDefault("server.tls", true)
	v.SetDefault("server.static_dir", "./web/dist")
	v.SetDefault("server.allow_origins", []string{"*"})

	// Database defaults
	v.SetDefault("database.url", "postgres://localhost/webtunnel?sslmode=disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "15m")

	// Redis defaults
	v.SetDefault("redis.url", "redis://localhost:6379")
	v.SetDefault("redis.db", 0)

	// Auth defaults
	v.SetDefault("auth.jwt_secret", "your-secret-key-change-in-production")
	v.SetDefault("auth.session_expiry", "24h")
	v.SetDefault("auth.rate_limit", 100)

	// Session defaults
	v.SetDefault("session.max_sessions", 50)
	v.SetDefault("session.max_memory_mb", 512)
	v.SetDefault("session.max_cpu_percent", 80)
	v.SetDefault("session.session_timeout", "1h")
	v.SetDefault("session.cleanup_interval", "5m")
	v.SetDefault("session.working_directory", "/tmp/webtunnel")
	v.SetDefault("session.allowed_commands", []string{})
	v.SetDefault("session.blocked_commands", []string{"rm", "rmdir", "dd", "mkfs", "fdisk"})
	v.SetDefault("session.environment_vars", map[string]string{
		"TERM": "xterm-256color",
		"SHELL": "/bin/bash",
	})
}