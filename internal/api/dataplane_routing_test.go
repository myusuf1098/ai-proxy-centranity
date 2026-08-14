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
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

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
