package api_test

import (
	"context"
	"encoding/json"
	"errors"
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

// transportErrAdapter: first target returns 503, subsequent targets
// transport-error — exercises the mixed-failure path where a stale 5xx
// status must not override the final transport-error code.
type transportErrAdapter struct {
	calls int
}

func (t *transportErrAdapter) CheckHealth(ctx context.Context) error { return nil }
func (t *transportErrAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (t *transportErrAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	t.calls++
	if t.calls == 1 {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("up")), Header: http.Header{}}, nil
	}
	return nil, errors.New("connection refused")
}

// forbiddenMetricsAdapter forwards a 403 from upstream — in the forwarded-4xx
// class that the handler owns for duration (not 5xx/401/429).
type forbiddenMetricsAdapter struct{}

func (forbiddenMetricsAdapter) CheckHealth(ctx context.Context) error { return nil }
func (forbiddenMetricsAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (forbiddenMetricsAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"denied"}}`)),
	}, nil
}

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

// tokenLabel pulls the `type="..."` label off a series line.
func tokenLabel(line, name string) string {
	needle := name + "=\""
	i := strings.Index(line, needle)
	if i < 0 {
		return ""
	}
	rest := line[i+len(needle):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func TestChatTokensLabeledInputNotOutput(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	dp := api.NewDataPlaneHandlerWithRouting(
		&okMetricsAdapter{}, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), routing.NewEngine(nil), testLogger(),
	)
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	var tokenLine string
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "pg_tokens_total{") {
			tokenLine = line
			break
		}
	}
	if tokenLine == "" {
		t.Fatal("token counter series not emitted")
	}
	// Request-side token estimate must be labeled "input", not "output".
	if got := tokenLabel(tokenLine, "type"); got != "input" {
		t.Fatalf("token type label = %q, want \"input\":\n%s", got, tokenLine)
	}
}

func TestMetricsUpstreamErrorsCountedOnFallback(t *testing.T) {
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

	// The 503 absorbed by fallback is an upstream error and must be counted.
	want := `pg_upstream_errors_total{code="UPSTREAM_UNAVAILABLE",provider="9router"} 1`
	if !strings.Contains(out, want) {
		t.Fatalf("absorbed 5xx not counted:\nwant series %q\nout:\n%s", want, out)
	}
}

func TestMetricsUpstreamErrorsOnTransportError(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	adapter := &transportErrAdapter{}
	engine := routing.NewEngine(nil)
	engine.SetAlias("fast", []string{"cc-haiku", "gemini-flash"})

	dp := api.NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	body := `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502 (all targets transport-error)", w.Code)
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	// Transport error must map to UPSTREAM_UNREACHABLE, never the stale
	// UPSTREAM_UNAVAILABLE of a prior 5xx attempt.
	if errResp.Error.Code != ninerouter.ErrUpstreamUnreach {
		t.Fatalf("error code = %q, want %q (transport errors must not inherit prior 5xx status)", errResp.Error.Code, ninerouter.ErrUpstreamUnreach)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()
	// The 503 attempt and the transport-error attempt each count once.
	want := `pg_upstream_errors_total{code="UPSTREAM_UNREACHABLE",provider="9router"} 1`
	if !strings.Contains(out, want) {
		t.Fatalf("transport-error series missing:\nwant %q\nout:\n%s", want, out)
	}
	wantUnavail := `pg_upstream_errors_total{code="UPSTREAM_UNAVAILABLE",provider="9router"} 1`
	if !strings.Contains(out, wantUnavail) {
		t.Fatalf("prior 5xx attempt not counted:\nwant %q\nout:\n%s", wantUnavail, out)
	}
}

func TestMetricsAll5xxFailCountedOncePerTarget(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	adapter := &always503Adapter{calls: map[string]int{}}
	engine := routing.NewEngine(nil)
	engine.SetAlias("fast", []string{"cc-haiku", "gemini-flash"})

	dp := api.NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	body := `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502 (all targets 503)", w.Code)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()

	// Two targets each 503: exactly N=2 per-attempt counts, no summary re-count.
	want := `pg_upstream_errors_total{code="UPSTREAM_UNAVAILABLE",provider="9router"} 2`
	if !strings.Contains(out, want) {
		t.Fatalf("all-5xx fail count wrong:\nwant %q\nout:\n%s", want, out)
	}
	wantOne := `pg_upstream_errors_total{code="UPSTREAM_UNAVAILABLE",provider="9router"} 1`
	if strings.Contains(out, wantOne) {
		t.Fatalf("all-5xx fail double-counted (summary re-increment):\n%s", out)
	}
}

func TestRejectedChatRequestStillHasDurationSample(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	keyStore := auth.NewMemoryKeyStore()
	rateLimiter := limiter.NewMemoryLimiter()
	dp := api.NewDataPlaneHandlerWithRouting(
		&okMetricsAdapter{}, keyStore, policy.NewEngine(), rateLimiter, routing.NewEngine(nil), testLogger(),
	)
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	// RPMLimit=1, second request rejected 429 before any upstream forward.
	raw, key, _ := auth.GenerateAPIKey("limited")
	key.RPMLimit = 1
	_ = keyStore.Create(context.Background(), key)

	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+raw)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if i == 1 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("got %d, want 429 (rate-limit rejection)", w.Code)
		}
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()

	// A rejected chat request must still produce a duration sample (middleware
	// observes failures), even though the handler's success observe never runs.
	want := `pg_request_duration_seconds_count{model="",path="/v1/chat/completions"} 1`
	if !strings.Contains(out, want) {
		t.Fatalf("rejected chat request has no duration sample:\nwant %q\nout:\n%s", want, out)
	}
}

func TestForwardedUpstream403DurationSingleCounted(t *testing.T) {
	cfg, _ := config.Load()
	healthHandler := health.NewHandler()

	metrics := telemetry.NewMetrics()
	dp := api.NewDataPlaneHandlerWithRouting(
		&forbiddenMetricsAdapter{}, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), routing.NewEngine(nil), testLogger(),
	)
	router := api.NewRouterWithTelemetry(cfg, healthHandler, dp, nil, metrics, testLogger())

	// Upstream returns 403; the handler forwards it (not in the 5xx/401/429
	// failure class), so the model-labeled handler observe is the only sample.
	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d, want 403 (forwarded upstream)", w.Code)
	}

	rec := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out := rec.Body.String()

	// Exactly ONE duration sample total: the handler's model-labeled one.
	// The middleware must not add a blank-model sample for forwarded 4xx.
	wantModel := `pg_request_duration_seconds_count{model="cc-haiku",path="/v1/chat/completions"} 1`
	if !strings.Contains(out, wantModel) {
		t.Fatalf("forwarded 403 missing handler duration sample:\nwant %q\nout:\n%s", wantModel, out)
	}
	wantBlank := `pg_request_duration_seconds_count{model="",path="/v1/chat/completions"}`
	if strings.Contains(out, wantBlank) {
		t.Fatalf("forwarded 403 double-counted by middleware (blank-model sample present):\n%s", out)
	}
}
