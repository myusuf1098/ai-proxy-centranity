package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
)

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Check(ctx context.Context) error {
	return m.err
}

func (m *mockChecker) Name() string {
	return m.name
}

func TestHealthLive(t *testing.T) {
	h := health.NewHandler()
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	h.Live(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status ok, got %v", res["status"])
	}
}

func TestHealthReady_AllHealthy(t *testing.T) {
	c1 := &mockChecker{name: "database", err: nil}
	c2 := &mockChecker{name: "redis", err: nil}
	h := health.NewHandler(c1, c2)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	h.Ready(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res health.ReadyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "ok" {
		t.Errorf("expected status ok, got %s", res.Status)
	}
	if res.Checks["database"] != "healthy" || res.Checks["redis"] != "healthy" {
		t.Errorf("unexpected checks state: %+v", res.Checks)
	}
}

func TestHealthReady_Degraded(t *testing.T) {
	c1 := &mockChecker{name: "database", err: nil}
	c2 := &mockChecker{name: "redis", err: errors.New("connection refused")}
	h := health.NewHandler(c1, c2)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	h.Ready(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", w.Code)
	}

	var res health.ReadyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "unhealthy" {
		t.Errorf("expected status unhealthy, got %s", res.Status)
	}
	if res.Checks["redis"] != "unhealthy: connection refused" {
		t.Errorf("unexpected redis check error: %s", res.Checks["redis"])
	}
}
