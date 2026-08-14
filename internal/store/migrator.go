package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FindUpMigrations returns a sorted list of .up.sql migration files in the given directory
func FindUpMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	sort.Strings(files)
	return files, nil
}

// RunMigrations applies all pending up migrations to the database inside transactions
func RunMigrations(ctx context.Context, db *sql.DB, dir string) error {
	// Create schema_migrations tracker table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to initialize schema_migrations table: %w", err)
	}

	files, err := FindUpMigrations(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		version := filepath.Base(file)

		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`
		if err := db.QueryRowContext(ctx, query, version).Scan(&exists); err != nil {
			return fmt.Errorf("failed to query migration status for %s: %w", version, err)
		}

		if exists {
			continue // already applied
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", version, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", version, err)
		}

		insertSQL := `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`
		if _, err := tx.ExecContext(ctx, insertSQL, version, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", version, err)
		}
	}

	return nil
}
