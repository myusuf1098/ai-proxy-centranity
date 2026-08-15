package api_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	cfg.Admin.ManagementToken = "test-admin-token"
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	mgmtHandler := api.NewManagementHandler(nil, auth.NewMemoryKeyStore(), routing.NewEngine(nil), proxy.NewMemoryStore(), logger)
	router := api.NewRouterWithManagement(cfg, healthHandler, nil, mgmtHandler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
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
	cfg.Admin.ManagementToken = "test-admin-token"
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	routeEngine := routing.NewEngine(nil)
	proxyStore := proxy.NewMemoryStore()
	keyStore := auth.NewMemoryKeyStore()

	mgmtHandler := api.NewManagementHandler(nil, keyStore, routeEngine, proxyStore, logger)
	router := api.NewRouterWithManagement(cfg, healthHandler, nil, mgmtHandler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
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

func TestProxyCRUD(t *testing.T) {
	store := proxy.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := api.NewManagementHandler(nil, auth.NewMemoryKeyStore(), routing.NewEngine(nil), store, logger)
	router := http.NewServeMux()
	router.Handle("POST /api/v1/proxies", http.HandlerFunc(h.CreateProxy))
	router.Handle("GET /api/v1/proxies/{id}", http.HandlerFunc(h.GetProxy))
	router.Handle("PUT /api/v1/proxies/{id}", http.HandlerFunc(h.UpdateProxy))
	router.Handle("DELETE /api/v1/proxies/{id}", http.HandlerFunc(h.DeleteProxy))

	// Create
	createBody := `{"name":"HTTP-01","type":"HTTP","host":"proxy.example","port":8080,"username":"secret-user-1","password":"secret-pass-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxies", strings.NewReader(createBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rec.Code)
	}
	// response must NOT contain credentials
	body := rec.Body.String()
	if strings.Contains(body, `"username"`) || strings.Contains(body, `"password"`) || strings.Contains(body, "secret-user-1") || strings.Contains(body, "secret-pass-1") {
		t.Fatalf("credentials leaked in create response: %s", body)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("create response missing id")
	}

	// Single GET
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxies/"+created.ID, nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", rec2.Code)
	}
	if strings.Contains(rec2.Body.String(), `"username"`) {
		t.Fatal("username/secret leaked in get response")
	}

	// Update (PUT)
	updateBody := `{"name":"HTTP-02","port":9090}`
	req4 := httptest.NewRequest(http.MethodPut, "/api/v1/proxies/"+created.ID, strings.NewReader(updateBody))
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("update: got %d, want 200", rec4.Code)
	}
	if strings.Contains(rec4.Body.String(), "secret-user-1") || strings.Contains(rec4.Body.String(), "secret-pass-1") {
		t.Fatal("credentials leaked in update response")
	}
	var updated proxy.Profile
	if err := json.Unmarshal(rec4.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.Name != "HTTP-02" || updated.Port != 9090 {
		t.Fatalf("update not applied: %+v", updated)
	}
	if updated.Host != "proxy.example" {
		t.Fatalf("update clobbered untouched field host: %+v", updated)
	}
	if !updated.Enabled {
		t.Fatalf("update zero-filled Enabled: %+v", updated)
	}

	// Delete
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/proxies/"+created.ID, nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", rec3.Code)
	}

	// Delete again should 404
	req5 := httptest.NewRequest(http.MethodDelete, "/api/v1/proxies/"+created.ID, nil)
	rec5 := httptest.NewRecorder()
	router.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusNotFound {
		t.Fatalf("delete missing: got %d, want 404", rec5.Code)
	}
}
