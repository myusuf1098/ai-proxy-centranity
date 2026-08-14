package security_test

import (
	"context"
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
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

type dummyAdapter struct{}

func (d *dummyAdapter) CheckHealth(ctx context.Context) error { return nil }
func (d *dummyAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return []ninerouter.ModelInfo{{ID: "cc-haiku"}}, nil
}
func (d *dummyAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"safe"}}]}`)),
	}, nil
}

func TestSecurity_MalformedAndInjectedAuthHeaders(t *testing.T) {
	keyStore := auth.NewMemoryKeyStore()
	policyEngine := policy.NewEngine()
	rateLimiter := limiter.NewMemoryLimiter()
	routeEngine := routing.NewEngine(nil)

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	dpHandler := api.NewDataPlaneHandlerWithRouting(&dummyAdapter{}, keyStore, policyEngine, rateLimiter, routeEngine, logger)
	router := api.NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)

	malformedTokens := []string{
		"Bearer ' OR '1'='1",
		"Bearer \r\nInjected: evil",
		"Bearer <script>alert(1)</script>",
		"Bearer ; DROP TABLE api_keys; --",
		"Bearer ",
	}

	for _, token := range malformedTokens {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"cc-haiku","messages":[]}`))
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for malformed token '%s', got %d", token, w.Code)
		}
	}
}

func TestSecurity_AliasSpoofingToBypassModelPolicy(t *testing.T) {
	keyStore := auth.NewMemoryKeyStore()
	policyEngine := policy.NewEngine()
	rateLimiter := limiter.NewMemoryLimiter()
	routeEngine := routing.NewEngine(nil)

	// Register alias "custom-coding" -> ["cc-opus"]
	routeEngine.SetAlias("custom-coding", []string{"cc-opus"})

	// Create key that DENIES cc-opus
	rawKey, keyModel, _ := auth.GenerateAPIKey("Restricted Client")
	keyModel.DeniedModels = []string{"cc-opus"}
	_ = keyStore.Create(context.Background(), keyModel)

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	dpHandler := api.NewDataPlaneHandlerWithRouting(&dummyAdapter{}, keyStore, policyEngine, rateLimiter, routeEngine, logger)
	router := api.NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)

	// Attempt to request alias "custom-coding" which resolves to denied "cc-opus"
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"custom-coding","messages":[{"role":"user","content":"bypass"}]}`))
	req.Header.Set("Authorization", "Bearer "+rawKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when alias resolves to denied target model, got %d", w.Code)
	}
}
