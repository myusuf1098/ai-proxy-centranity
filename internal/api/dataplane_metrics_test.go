package api_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	"github.com/myusuf1098/ai-proxy-centranity/internal/telemetry"
)

type okMetricsAdapter struct{}

func (okMetricsAdapter) CheckHealth(ctx context.Context) error { return nil }
func (okMetricsAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (okMetricsAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}, nil
}

func TestMetricsEmittedOnRequest(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	keyStore := auth.NewMemoryKeyStore()
	dp := api.NewDataPlaneHandlerWithRouting(
		&okMetricsAdapter{}, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(), routing.NewEngine(nil), testLogger(),
	)
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	raw, key, _ := auth.GenerateAPIKey("tester")
	_ = keyStore.Create(context.Background(), key)

	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()

	// Series (not just # TYPE) proves the counter was actually touched.
	if !strings.Contains(out, "pg_http_requests_total{") {
		t.Fatal("request counter series not emitted")
	}
	if !strings.Contains(out, "pg_tokens_total{") {
		t.Fatal("token counter series not emitted")
	}
	if !strings.Contains(out, `model="cc-haiku"`) {
		t.Fatalf("metric missing non-empty model label:\n%s", out)
	}
}
