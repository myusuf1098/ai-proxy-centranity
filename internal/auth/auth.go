package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type contextKey string

const apiKeyContextKey contextKey = "authenticated_api_key"

// APIKey represents a client identity and its policy boundaries
type APIKey struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Prefix            string     `json:"prefix"`
	Hash              string     `json:"-"` // Never serialize hash
	Enabled           bool       `json:"enabled"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	AllowedModels     []string   `json:"allowed_models"`
	DeniedModels      []string   `json:"denied_models"`
	AllowedProviders  []string   `json:"allowed_providers"`
	DeniedProviders   []string   `json:"denied_providers"`
	RPMLimit          int        `json:"rpm_limit"`
	RPSLimit          int        `json:"rps_limit"`
	TPMLimit          int        `json:"tpm_limit"`
	DailyTokenQuota   int64      `json:"daily_token_quota"`
	MonthlyTokenQuota int64      `json:"monthly_token_quota"`
	BudgetLimit       float64    `json:"budget_limit"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// HashKey computes the SHA-256 hex digest of a raw API key
func HashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey creates a new cryptographic API key and its model representation
func GenerateAPIKey(name string) (string, *APIKey, error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	rawSuffix := hex.EncodeToString(randomBytes)
	rawKey := fmt.Sprintf("sk-pg-%s", rawSuffix)
	prefix := rawKey[:10]
	hash := HashKey(rawKey)
	now := time.Now().UTC()

	keyModel := &APIKey{
		ID:                fmt.Sprintf("key_%s", rawSuffix[:12]),
		Name:              name,
		Prefix:            prefix,
		Hash:              hash,
		Enabled:           true,
		AllowedModels:     []string{},
		DeniedModels:      []string{},
		AllowedProviders:  []string{},
		DeniedProviders:   []string{},
		RPMLimit:          60,
		RPSLimit:          10,
		TPMLimit:          100000,
		DailyTokenQuota:   1000000,
		MonthlyTokenQuota: 30000000,
		BudgetLimit:       0.0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	return rawKey, keyModel, nil
}

// KeyStore interface for retrieving and managing API keys
type KeyStore interface {
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	Create(ctx context.Context, key *APIKey) error
}

// MemoryKeyStore provides in-memory thread-safe storage for API keys
type MemoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKey
}

// NewMemoryKeyStore creates a new MemoryKeyStore
func NewMemoryKeyStore() *MemoryKeyStore {
	return &MemoryKeyStore{
		keys: make(map[string]*APIKey),
	}
}

// GetByHash retrieves a key by its SHA-256 hash
func (s *MemoryKeyStore) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, exists := s.keys[hash]
	if !exists {
		return nil, errors.New("api key not found")
	}
	return key, nil
}

// Create stores a new API key
func (s *MemoryKeyStore) Create(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.keys[key.Hash] = key
	return nil
}

// GetAPIKey extracts the authenticated APIKey from the request context
func GetAPIKey(ctx context.Context) *APIKey {
	if val, ok := ctx.Value(apiKeyContextKey).(*APIKey); ok {
		return val
	}
	return nil
}

// AuthMiddleware creates HTTP middleware to authenticate API keys
func AuthMiddleware(store KeyStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeAuthError(w, http.StatusUnauthorized, "Missing or malformed Authorization header", "auth_error", "missing_token")
				return
			}

			rawKey := strings.TrimPrefix(authHeader, "Bearer ")
			rawKey = strings.TrimSpace(rawKey)
			if rawKey == "" {
				writeAuthError(w, http.StatusUnauthorized, "Empty API key provided", "auth_error", "invalid_api_key")
				return
			}

			hash := HashKey(rawKey)
			apiKey, err := store.GetByHash(r.Context(), hash)
			if err != nil || apiKey == nil || !apiKey.Enabled {
				writeAuthError(w, http.StatusUnauthorized, "Invalid or disabled API key", "auth_error", "invalid_api_key")
				return
			}

			if apiKey.ExpiresAt != nil && time.Now().UTC().After(*apiKey.ExpiresAt) {
				writeAuthError(w, http.StatusUnauthorized, "API key has expired", "auth_error", "key_expired")
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyContextKey, apiKey)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, message, errType, errCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    errType,
			"code":    errCode,
		},
	})
}
