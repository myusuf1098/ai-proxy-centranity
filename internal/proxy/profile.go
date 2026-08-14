package proxy

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Type represents supported proxy protocols
type Type string

const (
	TypeDirect Type = "DIRECT"
	TypeHTTP   Type = "HTTP"
	TypeHTTPS  Type = "HTTPS"
	TypeSOCKS5 Type = "SOCKS5"
)

// Profile represents an outbound proxy configuration with secret isolation
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      Type      `json:"type"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"-"` // Secret: never serialize in JSON/logs
	Password  string    `json:"-"` // Secret: never serialize in JSON/logs
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store interface for managing proxy profiles
type Store interface {
	Get(ctx context.Context, id string) (*Profile, error)
	Save(ctx context.Context, profile *Profile) error
	List(ctx context.Context) ([]*Profile, error)
}

// MemoryStore provides thread-safe in-memory proxy profile storage
type MemoryStore struct {
	mu       sync.RWMutex
	profiles map[string]*Profile
}

// NewMemoryStore creates a new MemoryStore
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		profiles: make(map[string]*Profile),
	}
}

// Get retrieves a proxy profile by ID
func (s *MemoryStore) Get(ctx context.Context, id string) (*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, exists := s.profiles[id]
	if !exists {
		return nil, errors.New("proxy profile not found")
	}
	return p, nil
}

// Save stores or updates a proxy profile
func (s *MemoryStore) Save(ctx context.Context, profile *Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.profiles[profile.ID] = profile
	return nil
}

// List returns all registered proxy profiles
func (s *MemoryStore) List(ctx context.Context) ([]*Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Profile, 0, len(s.profiles))
	for _, p := range s.profiles {
		list = append(list, p)
	}
	return list, nil
}
