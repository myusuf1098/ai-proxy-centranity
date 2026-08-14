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
	// Assert the model label on the TOKEN series specifically, not just
	// anywhere in the output (the duration series could carry it instead).
	tokenLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "pg_tokens_total{") {
			tokenLine = line
			break
		}
	}
	if tokenLine == "" {
		t.Fatal("token counter series not emitted")
	}
	if !strings.Contains(tokenLine, `model="cc-haiku"`) {
		t.Fatalf("token series missing expected model label:\n%s", tokenLine)
	}
}

func TestMetricsModelLabelOnFallback(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	adapter := &failFirstAdapter{models: []ninerouter.ModelInfo{}, calls: map[string]int{}}
	engine := routing.NewEngine(nil)
	engine.SetAlias("coding", []string{"cc-sonnet", "cc-haiku"})

	dp := api.NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	body := `{"model":"coding","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (fallback should succeed)", w.Code)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()

	// Token series must carry the FALLBACK model (cc-haiku), not the primary (cc-sonnet).
	tokenLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "pg_tokens_total{") {
			tokenLine = line
			break
		}
	}
	if tokenLine == "" {
		t.Fatal("token counter series not emitted")
	}
	if !strings.Contains(tokenLine, `model="cc-haiku"`) {
		t.Fatalf("token series missing fallback model label:\n%s", tokenLine)
	}
	if strings.Contains(tokenLine, `model="cc-sonnet"`) {
		t.Fatalf("token series mislabeled with primary model:\n%s", tokenLine)
	}
}
