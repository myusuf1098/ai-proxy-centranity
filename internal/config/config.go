package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int           `json:"port"`
	MetricsPort     int           `json:"metrics_port"`
	Env             string        `json:"env"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	URL             string        `json:"-"` // secret, never serialize in logs
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string `json:"-"` // secret
	Password string `json:"-"` // secret
	DB       int    `json:"db"`
}

// NineRouterConfig holds 9Router upstream adapter configuration
type NineRouterConfig struct {
	BaseURL string        `json:"base_url"`
	APIKey  string        `json:"-"` // secret
	Timeout time.Duration `json:"timeout"`
}

// AdminConfig holds Management API credentials & permissions
type AdminConfig struct {
	ManagementToken string   `json:"-"` // secret
	AllowedOrigins  []string `json:"allowed_origins"`
}

// Config represents the complete ProxyGateway configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Database   DatabaseConfig   `json:"database"`
	Redis      RedisConfig      `json:"redis"`
	NineRouter NineRouterConfig `json:"ninerouter"`
	Admin      AdminConfig      `json:"admin"`
}

// Load reads configuration from environment variables with safe defaults
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvInt("PG_SERVER_PORT", 8088),
			MetricsPort:     getEnvInt("PG_METRICS_PORT", 9099),
			Env:             getEnvString("PG_ENV", "development"),
			ReadTimeout:     getEnvDuration("PG_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("PG_WRITE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getEnvDuration("PG_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			URL:             getEnvString("PG_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/proxygateway?sslmode=disable"),
			MaxOpenConns:    getEnvInt("PG_DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("PG_DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvDuration("PG_DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL:      getEnvString("PG_REDIS_URL", "redis://localhost:6379/0"),
			Password: getEnvString("PG_REDIS_PASSWORD", ""),
			DB:       getEnvInt("PG_REDIS_DB", 0),
		},
		NineRouter: NineRouterConfig{
			BaseURL: getEnvString("PG_NINEROUTER_URL", "http://127.0.0.1:20128"),
			APIKey:  getEnvString("PG_NINEROUTER_API_KEY", ""),
			Timeout: getEnvDuration("PG_NINEROUTER_TIMEOUT", 60*time.Second),
		},
		Admin: AdminConfig{
			ManagementToken: getEnvString("PG_ADMIN_TOKEN", ""),
			AllowedOrigins:  []string{"*"},
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks configuration values for validity
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d (must be 1-65535)", c.Server.Port)
	}
	if c.Server.MetricsPort <= 0 || c.Server.MetricsPort > 65535 {
		return fmt.Errorf("invalid metrics port: %d (must be 1-65535)", c.Server.MetricsPort)
	}
	if c.Server.Port == c.Server.MetricsPort {
		return fmt.Errorf("server port and metrics port cannot be identical (%d)", c.Server.Port)
	}
	if c.NineRouter.BaseURL == "" {
		return fmt.Errorf("ninerouter base URL cannot be empty")
	}
	return nil
}

func getEnvString(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return fallback
}
