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

func TestManagementRoutesRequireAdminToken(t *testing.T) {
	cfg := &config.Config{Admin: config.AdminConfig{ManagementToken: "tok"}}
	h := health.NewHandler()
	router := api.NewRouterWithManagement(cfg, h, nil, &api.ManagementHandler{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, path := range []string{"/api/v1/system", "/api/v1/overview", "/api/v1/proxies"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", path, rec.Code)
		}
	}
}
