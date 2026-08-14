package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
)

func TestHashKey(t *testing.T) {
	key := "sk-pg-test-key-12345"
	hash1 := auth.HashKey(key)
	hash2 := auth.HashKey(key)

	if hash1 != hash2 {
		t.Errorf("expected deterministic hash output")
	}
	if len(hash1) != 64 { // SHA-256 hex length
		t.Errorf("expected 64 character hex hash, got %d", len(hash1))
	}
}

func TestGenerateAPIKey(t *testing.T) {
	rawKey, keyModel, err := auth.GenerateAPIKey("Test Key")
	if err != nil {
		t.Fatalf("expected key generation to succeed, got: %v", err)
	}

	if rawKey == "" || keyModel.Hash == "" || keyModel.Prefix == "" {
		t.Fatalf("invalid generated key properties")
	}

	if auth.HashKey(rawKey) != keyModel.Hash {
		t.Errorf("hash of generated raw key does not match key model hash")
	}
}

func TestMemoryKeyStore(t *testing.T) {
	store := auth.NewMemoryKeyStore()
	rawKey, keyModel, _ := auth.GenerateAPIKey("Production Client")

	ctx := context.Background()
	if err := store.Create(ctx, keyModel); err != nil {
		t.Fatalf("failed to store key: %v", err)
	}

	retrieved, err := store.GetByHash(ctx, auth.HashKey(rawKey))
	if err != nil {
		t.Fatalf("failed to retrieve key by hash: %v", err)
	}

	if retrieved.ID != keyModel.ID || retrieved.Name != "Production Client" {
		t.Errorf("mismatched retrieved key data: %+v", retrieved)
	}
}

func TestAuthMiddleware(t *testing.T) {
	store := auth.NewMemoryKeyStore()
	rawKey, keyModel, _ := auth.GenerateAPIKey("Dev Client")
	_ = store.Create(context.Background(), keyModel)

	handler := auth.AuthMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := auth.GetAPIKey(r.Context())
		if key == nil {
			t.Errorf("expected API key in context, got nil")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Missing Authorization header
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	wNoAuth := httptest.NewRecorder()
	handler.ServeHTTP(wNoAuth, reqNoAuth)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on missing auth header, got %d", wNoAuth.Code)
	}

	// 2. Valid Authorization header
	reqValid := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqValid.Header.Set("Authorization", "Bearer "+rawKey)
	wValid := httptest.NewRecorder()
	handler.ServeHTTP(wValid, reqValid)
	if wValid.Code != http.StatusOK {
		t.Errorf("expected 200 on valid auth header, got %d", wValid.Code)
	}

	// 3. Expired API Key
	expiredTime := time.Now().Add(-1 * time.Hour)
	_, expiredKeyModel, _ := auth.GenerateAPIKey("Expired Client")
	expiredKeyModel.ExpiresAt = &expiredTime
	_ = store.Create(context.Background(), expiredKeyModel)

	reqExpired := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqExpired.Header.Set("Authorization", "Bearer "+expiredKeyModel.ID)
	wExpired := httptest.NewRecorder()
	handler.ServeHTTP(wExpired, reqExpired)
	if wExpired.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on expired key, got %d", wExpired.Code)
	}
}
