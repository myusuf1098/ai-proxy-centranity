package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

// auditAdapterStub is a minimal NineRouterPort implementation for audit wiring tests.
type auditAdapterStub struct{}

func (auditAdapterStub) CheckHealth(ctx context.Context) error { return nil }

func (auditAdapterStub) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return []ninerouter.ModelInfo{}, nil
}

func (auditAdapterStub) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(io.LimitReader(nil, 0)),
	}, nil
}

func TestAuditAuthFailureEmitted(t *testing.T) {
	auditStore := audit.NewMemoryStore()
	keyStore := auth.NewMemoryKeyStore()
	adapter := auditAdapterStub{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	dp := NewDataPlaneHandlerWithRouting(
		adapter, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(),
		routing.NewEngine(nil), logger,
	)
	dp.SetAuditStore(auditStore)

	cfg, _ := config.Load()
	router := NewRouterWithManagement(cfg, health.NewHandler(), dp, nil, logger)

	// No auth header -> 401 at AuthMiddleware -> AUTH_FAILURE at router level
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	events, _ := auditStore.List(context.Background())
	found := false
	for _, e := range events {
		if e.EventType == audit.EventAuthFailure {
			found = true
		}
	}
	if !found {
		t.Error("no AUTH_FAILURE audit event emitted")
	}
}

func TestManagementAuthFailureAudited(t *testing.T) {
	auditStore := audit.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mgmt := NewManagementHandler(nil, auth.NewMemoryKeyStore(), routing.NewEngine(nil), nil, logger)
	mgmt.SetAuditStore(auditStore)

	cfg, _ := config.Load()
	cfg.Admin.ManagementToken = "test-admin-token"
	router := NewRouterWithManagement(cfg, health.NewHandler(), nil, mgmt, logger)

	// No admin token -> 401 at AdminAuthMiddleware -> AUTH_FAILURE at router level
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	events, _ := auditStore.List(context.Background())
	found := false
	for _, e := range events {
		if e.EventType == audit.EventAuthFailure && e.Target == "/api/v1/system" && e.Actor == "unknown" {
			found = true
		}
	}
	if !found {
		t.Error("no AUTH_FAILURE audit event emitted for management route")
	}
}
