package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/yourusername/webtunnel/internal/config"
)

type DB struct {
	*sql.DB
}

func New(cfg config.DatabaseConfig) (*DB, error) {
	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	
	if cfg.ConnMaxLifetime != "" {
		duration, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			return nil, fmt.Errorf("invalid conn_max_lifetime: %w", err)
		}
		db.SetConnMaxLifetime(duration)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}