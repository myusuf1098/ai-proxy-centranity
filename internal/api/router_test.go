package api_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
)

func TestRouterRoutes(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouter(cfg, healthHandler, logger)

	// Test liveness route
	reqLive := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	wLive := httptest.NewRecorder()
	router.ServeHTTP(wLive, reqLive)

	if wLive.Code != http.StatusOK {
		t.Errorf("expected status 200 on /health/live, got %d", wLive.Code)
	}

	// Test readiness route
	reqReady := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	wReady := httptest.NewRecorder()
	router.ServeHTTP(wReady, reqReady)

	if wReady.Code != http.StatusOK {
		t.Errorf("expected status 200 on /health/ready, got %d", wReady.Code)
	}
}
