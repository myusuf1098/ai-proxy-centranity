package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("PG_SERVER_PORT")
	os.Unsetenv("PG_METRICS_PORT")
	os.Unsetenv("PG_NINEROUTER_URL")
	os.Unsetenv("PG_DATABASE_URL")
	os.Unsetenv("PG_REDIS_URL")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading defaults, got: %v", err)
	}

	if cfg.Server.Port != 8088 {
		t.Errorf("expected default port 8088, got %d", cfg.Server.Port)
	}
	if cfg.Server.MetricsPort != 9099 {
		t.Errorf("expected default metrics port 9099, got %d", cfg.Server.MetricsPort)
	}
	if cfg.NineRouter.BaseURL != "http://127.0.0.1:20128" {
		t.Errorf("expected default 9Router URL http://127.0.0.1:20128, got %s", cfg.NineRouter.BaseURL)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("expected default read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PG_SERVER_PORT", "9000")
	os.Setenv("PG_METRICS_PORT", "9100")
	os.Setenv("PG_ENV", "production")
	os.Setenv("PG_NINEROUTER_URL", "http://ninerouter:20128")
	os.Setenv("PG_DATABASE_URL", "postgres://user:pass@localhost:5432/proxygateway?sslmode=disable")
	os.Setenv("PG_REDIS_URL", "redis://localhost:6379/0")
	os.Setenv("PG_ADMIN_TOKEN", "secret-admin-token")
	defer func() {
		os.Unsetenv("PG_SERVER_PORT")
		os.Unsetenv("PG_METRICS_PORT")
		os.Unsetenv("PG_ENV")
		os.Unsetenv("PG_NINEROUTER_URL")
		os.Unsetenv("PG_DATABASE_URL")
		os.Unsetenv("PG_REDIS_URL")
		os.Unsetenv("PG_ADMIN_TOKEN")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading from env, got: %v", err)
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
	if cfg.Server.MetricsPort != 9100 {
		t.Errorf("expected metrics port 9100, got %d", cfg.Server.MetricsPort)
	}
	if cfg.Server.Env != "production" {
		t.Errorf("expected env production, got %s", cfg.Server.Env)
	}
	if cfg.NineRouter.BaseURL != "http://ninerouter:20128" {
		t.Errorf("expected 9router url http://ninerouter:20128, got %s", cfg.NineRouter.BaseURL)
	}
	if cfg.Database.URL != "postgres://user:pass@localhost:5432/proxygateway?sslmode=disable" {
		t.Errorf("unexpected database URL: %s", cfg.Database.URL)
	}
	if cfg.Redis.URL != "redis://localhost:6379/0" {
		t.Errorf("unexpected redis URL: %s", cfg.Redis.URL)
	}
	if cfg.Admin.ManagementToken != "secret-admin-token" {
		t.Errorf("unexpected admin token: %s", cfg.Admin.ManagementToken)
	}
}

func TestLoadAllowedOrigins(t *testing.T) {
	os.Setenv("PG_ADMIN_ALLOWED_ORIGINS", "a.com,b.com")
	defer os.Unsetenv("PG_ADMIN_ALLOWED_ORIGINS")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected no error loading allowed origins, got: %v", err)
	}
	if len(cfg.Admin.AllowedOrigins) != 2 || cfg.Admin.AllowedOrigins[0] != "a.com" || cfg.Admin.AllowedOrigins[1] != "b.com" {
		t.Errorf("expected AllowedOrigins [a.com b.com], got %v", cfg.Admin.AllowedOrigins)
	}

	os.Unsetenv("PG_ADMIN_ALLOWED_ORIGINS")
	cfg, err = config.Load()
	if err != nil {
		t.Fatalf("expected no error loading default origins, got: %v", err)
	}
	if len(cfg.Admin.AllowedOrigins) != 1 || cfg.Admin.AllowedOrigins[0] != "*" {
		t.Errorf("expected default AllowedOrigins [*], got %v", cfg.Admin.AllowedOrigins)
	}
}

func TestValidationInvalidPort(t *testing.T) {
	os.Setenv("PG_SERVER_PORT", "999999")
	defer os.Unsetenv("PG_SERVER_PORT")

	_, err := config.Load()
	if err == nil {
		t.Errorf("expected error for invalid port 999999, got nil")
	}
}
