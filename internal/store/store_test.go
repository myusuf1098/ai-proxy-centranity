package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/store"
)

func TestStoreHealthCheckerInterface(t *testing.T) {
	// Verify that PostgresStore and RedisStore satisfy health.HealthChecker interface
	var _ health.HealthChecker = (*store.PostgresStore)(nil)
	var _ health.HealthChecker = (*store.RedisStore)(nil)
}

func TestPostgresStoreCheck_Closed(t *testing.T) {
	pgCfg := config.DatabaseConfig{
		URL:             "postgres://user:pass@127.0.0.1:9999/dummy?sslmode=disable",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	pgStore, err := store.NewPostgresStore(pgCfg)
	if err != nil {
		t.Fatalf("expected store initialization, got: %v", err)
	}
	defer pgStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := pgStore.Check(ctx); err == nil {
		t.Errorf("expected error pinging unreachable db, got nil")
	}

	if pgStore.Name() != "database" {
		t.Errorf("expected name 'database', got %s", pgStore.Name())
	}
}

func TestRedisStoreCheck_Closed(t *testing.T) {
	rCfg := config.RedisConfig{
		URL: "redis://127.0.0.1:9999/0",
	}

	rStore, err := store.NewRedisStore(rCfg)
	if err != nil {
		t.Fatalf("expected store initialization, got: %v", err)
	}
	defer rStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := rStore.Check(ctx); err == nil {
		t.Errorf("expected error pinging unreachable redis, got nil")
	}

	if rStore.Name() != "redis" {
		t.Errorf("expected name 'redis', got %s", rStore.Name())
	}
}
