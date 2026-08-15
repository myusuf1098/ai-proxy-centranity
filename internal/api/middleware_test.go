package api_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := api.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := api.GetRequestID(r.Context())
		if reqID == "" {
			t.Errorf("expected request ID in context, got empty")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	respID := w.Header().Get("X-Request-ID")
	if respID == "" {
		t.Errorf("expected X-Request-ID header in response")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	panicHandler := api.RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected memory violation")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	panicHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 on panic recovery, got %d", w.Code)
	}
}

func TestCORSAllowedOrigins(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	h := api.CORSMiddleware(allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed origin
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("allowed origin: got %q", got)
	}

	// Disallowed origin — no ACAO header
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://evil.example.com")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin leaked: %q", got)
	}
}

func TestManagementRoutesRequireAdminToken(t *testing.T) {
	cfg := &config.Config{Admin: config.AdminConfig{ManagementToken: "tok"}}
	h := health.NewHandler()
	router := api.NewRouterWithManagement(cfg, h, nil, &api.ManagementHandler{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/system"},
		{http.MethodGet, "/api/v1/overview"},
		{http.MethodGet, "/api/v1/proxies"},
		{http.MethodPost, "/api/v1/proxies"},
		{http.MethodPut, "/api/v1/proxies/p1"},
		{http.MethodDelete, "/api/v1/proxies/p1"},
	}
	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: got %d, want 401", rt.method, rt.path, rec.Code)
		}
	}
}
