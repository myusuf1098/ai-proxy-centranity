package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
)

func TestDataPlane_PolicyModelDenied(t *testing.T) {
	keyStore := auth.NewMemoryKeyStore()
	policyEngine := policy.NewEngine()
	rateLimiter := limiter.NewMemoryLimiter()

	rawKey, keyModel, _ := auth.GenerateAPIKey("Restricted Client")
	keyModel.DeniedModels = []string{"cc-opus"}
	_ = keyStore.Create(context.Background(), keyModel)

	mockAdapter := &mockNineRouterAdapter{
		models: []ninerouter.ModelInfo{
			{ID: "cc-haiku", Object: "model", OwnedBy: "combo"},
			{ID: "cc-opus", Object: "model", OwnedBy: "combo"},
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	dpHandler := api.NewDataPlaneHandlerWithPolicy(mockAdapter, keyStore, policyEngine, rateLimiter, logger)
	router := api.NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)

	// Attempt to access denied model cc-opus
	reqBody := `{"model":"cc-opus","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for denied model, got %d", w.Code)
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error.Code != "model_not_allowed" {
		t.Errorf("expected code model_not_allowed, got %s", errResp.Error.Code)
	}
}

func TestDataPlane_RateLimitEnforced(t *testing.T) {
	keyStore := auth.NewMemoryKeyStore()
	policyEngine := policy.NewEngine()
	rateLimiter := limiter.NewMemoryLimiter()

	rawKey, keyModel, _ := auth.GenerateAPIKey("Limited Client")
	keyModel.RPMLimit = 2
	_ = keyStore.Create(context.Background(), keyModel)

	mockAdapter := &mockNineRouterAdapter{
		forwardResponse: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	dpHandler := api.NewDataPlaneHandlerWithPolicy(mockAdapter, keyStore, policyEngine, rateLimiter, logger)
	router := api.NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)

	reqBody := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`

	// 2 Allowed requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
		req.Header.Set("Authorization", "Bearer "+rawKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should be 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be 429 Too Many Requests
	reqBlocked := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	reqBlocked.Header.Set("Authorization", "Bearer "+rawKey)
	wBlocked := httptest.NewRecorder()
	router.ServeHTTP(wBlocked, reqBlocked)

	if wBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 Too Many Requests, got %d", wBlocked.Code)
	}
}
