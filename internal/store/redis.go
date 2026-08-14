package store

import (
	"context"
	"fmt"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/redis/go-redis/v9"
)

// RedisStore manages the Redis client connection
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates and configures a new Redis store
func NewRedisStore(cfg config.RedisConfig) (*RedisStore, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	if cfg.Password != "" {
		opts.Password = cfg.Password
	}
	if cfg.DB != 0 {
		opts.DB = cfg.DB
	}

	client := redis.NewClient(opts)
	return &RedisStore{client: client}, nil
}

// Client returns the underlying *redis.Client instance
func (s *RedisStore) Client() *redis.Client {
	return s.client
}

// Name returns the health check identifier
func (s *RedisStore) Name() string {
	return "redis"
}

// Check verifies redis connectivity via ping
func (s *RedisStore) Check(ctx context.Context) error {
	if s.client == nil {
		return fmt.Errorf("redis client is not initialized")
	}
	return s.client.Ping(ctx).Err()
}

// Close gracefully closes the redis client
func (s *RedisStore) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}
