package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type failFirstAdapter struct {
	models []ninerouter.ModelInfo
	calls  map[string]int // model -> call count
}

func (f *failFirstAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return f.models, nil
}
func (f *failFirstAdapter) CheckHealth(ctx context.Context) error { return nil }
func (f *failFirstAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(body).Decode(&payload)
	f.calls[payload.Model]++
	if f.calls[payload.Model] > 1 {
		return nil, errors.New("should not retry same target")
	}
	// Primary (cc-sonnet) fails 503, fallback (cc-haiku) succeeds
	if payload.Model == "cc-sonnet" {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("up")), Header: http.Header{}}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
}

type always503Adapter struct {
	calls map[string]int // model -> call count
}

func (a *always503Adapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (a *always503Adapter) CheckHealth(ctx context.Context) error { return nil }
func (a *always503Adapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	var payload struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(body).Decode(&payload)
	a.calls[payload.Model]++
	return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("up")), Header: http.Header{}}, nil
}

func TestChatFallbackExecutesOn5xx(t *testing.T) {
	adapter := &failFirstAdapter{models: []ninerouter.ModelInfo{}, calls: map[string]int{}}
	engine := routing.NewEngine(nil)
	engine.SetAlias("coding", []string{"cc-sonnet", "cc-haiku"})

	dp := api.NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())

	body := `{"model":"coding","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	dp.ChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (fallback should succeed)", rec.Code)
	}
	if adapter.calls["cc-sonnet"] != 1 {
		t.Errorf("primary cc-sonnet called %d times, want 1", adapter.calls["cc-sonnet"])
	}
	if adapter.calls["cc-haiku"] != 1 {
		t.Errorf("fallback cc-haiku called %d times, want 1", adapter.calls["cc-haiku"])
	}
}

func TestChatAllTargetsFailReturns502(t *testing.T) {
	adapter := &always503Adapter{calls: map[string]int{}}
	engine := routing.NewEngine(nil)
	engine.SetAlias("fast", []string{"cc-haiku", "gemini-flash"})

	dp := api.NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())

	body := `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	dp.ChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", rec.Code)
	}
	if adapter.calls["cc-haiku"] != 1 || adapter.calls["gemini-flash"] != 1 {
		t.Errorf("each target should be tried once: %v", adapter.calls)
	}
}

type capturingAdapter struct {
	lastForwardedModel string
}

func (c *capturingAdapter) CheckHealth(ctx context.Context) error { return nil }
func (c *capturingAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (c *capturingAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	bodyBytes, _ := io.ReadAll(body)
	var req struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(bodyBytes, &req)
	c.lastForwardedModel = req.Model

	respBody := `{"choices":[{"message":{"content":"resolved"}}]}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}, nil
}

func TestDataPlane_AliasResolutionForwarding(t *testing.T) {
	capturing := &capturingAdapter{}
	routeEngine := routing.NewEngine(nil)

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	dpHandler := api.NewDataPlaneHandlerWithRouting(
		capturing,
		nil,
		policy.NewEngine(),
		limiter.NewMemoryLimiter(),
		routeEngine,
		logger,
	)
	router := api.NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)

	// Send request with alias "coding"
	reqBody := `{"model":"coding","messages":[{"role":"user","content":"write code"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify that the forwarded model was rewritten from "coding" to "cc-sonnet"!
	if capturing.lastForwardedModel != "cc-sonnet" {
		t.Errorf("expected upstream model cc-sonnet, got %s", capturing.lastForwardedModel)
	}
}
