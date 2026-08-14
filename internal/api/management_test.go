package api_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/api"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/proxy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

func TestManagementAPI_System(t *testing.T) {
	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	mgmtHandler := api.NewManagementHandler(nil, auth.NewMemoryKeyStore(), routing.NewEngine(nil), proxy.NewMemoryStore(), logger)
	router := api.NewRouterWithManagement(cfg, healthHandler, nil, mgmtHandler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode system response: %v", err)
	}

	if resp.Name != "ProxyGateway Enterprise" || resp.Status != "operational" {
		t.Errorf("unexpected system payload: %+v", resp)
	}
}

func TestManagementAPI_Overview(t *testing.T) {
	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	routeEngine := routing.NewEngine(nil)
	proxyStore := proxy.NewMemoryStore()
	keyStore := auth.NewMemoryKeyStore()

	mgmtHandler := api.NewManagementHandler(nil, keyStore, routeEngine, proxyStore, logger)
	router := api.NewRouterWithManagement(cfg, healthHandler, nil, mgmtHandler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Status      string `json:"status"`
		RoutesCount int    `json:"routes_count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Status != "ok" || resp.RoutesCount == 0 {
		t.Errorf("unexpected overview data: %+v", resp)
	}
}
