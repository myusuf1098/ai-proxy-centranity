package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
)

// PostgresStore manages the PostgreSQL connection pool
type PostgresStore struct {
	db *sql.DB
}

// NewPostgresStore creates and configures a new PostgreSQL store
func NewPostgresStore(cfg config.DatabaseConfig) (*PostgresStore, error) {
	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &PostgresStore{db: db}, nil
}

// DB returns the underlying *sql.DB instance
func (s *PostgresStore) DB() *sql.DB {
	return s.db
}

// Name returns the health check identifier
func (s *PostgresStore) Name() string {
	return "database"
}

// Check verifies database liveness/ping
func (s *PostgresStore) Check(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	return s.db.PingContext(ctx)
}

// Close gracefully closes the database connection pool
func (s *PostgresStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
