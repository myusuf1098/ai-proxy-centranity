package audit

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Event type constants (spec FEAT-011 §2)
const (
	EventAuthSuccess   = "AUTH_SUCCESS"
	EventAuthFailure   = "AUTH_FAILURE"
	EventPolicyDeny    = "POLICY_DENY"
	EventRateLimited   = "RATE_LIMITED"
	EventRouteResolved = "ROUTE_RESOLVED"
	EventConfigChanged = "CONFIG_CHANGED"
)

// Event represents a structured audit record
type Event struct {
	ID        string            `json:"id"`
	Timestamp time.Time         `json:"timestamp"`
	Actor     string            `json:"actor"`
	EventType string            `json:"event_type"`
	Target    string            `json:"target"`
	Status    string            `json:"status"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Store interface for recording and querying audit logs
type Store interface {
	Log(ctx context.Context, event Event) error
	List(ctx context.Context) ([]Event, error)
}

// MemoryStore provides in-memory thread-safe audit log storage with automatic redaction
type MemoryStore struct {
	mu     sync.RWMutex
	events []Event
}

// NewMemoryStore creates a new MemoryStore
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		events: make([]Event, 0, 1000),
	}
}

// Log records an audit event after sanitizing sensitive metadata
func (s *MemoryStore) Log(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Redact sensitive metadata keys/values
	sanitizedMetadata := make(map[string]string)
	for k, v := range event.Metadata {
		lowerK := strings.ToLower(k)
		if strings.Contains(lowerK, "key") || strings.Contains(lowerK, "password") || strings.Contains(lowerK, "token") || strings.Contains(lowerK, "secret") {
			sanitizedMetadata[k] = "[REDACTED]"
		} else {
			sanitizedMetadata[k] = v
		}
	}
	event.Metadata = sanitizedMetadata

	s.events = append(s.events, event)
	return nil
}

// List retrieves recent audit events
func (s *MemoryStore) List(ctx context.Context) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Event, len(s.events))
	copy(res, s.events)
	return res, nil
}
