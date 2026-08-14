package telemetry_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/telemetry"
)

func TestMetrics_HTTPMiddlewareAndEndpoint(t *testing.T) {
	m := telemetry.NewMetrics()

	handler := m.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	// Execute simulated request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Scrape /metrics endpoint
	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	m.Handler().ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected /metrics to return 200, got %d", metricsRec.Code)
	}

	metricsBody := metricsRec.Body.String()
	if !strings.Contains(metricsBody, "pg_http_requests_total") {
		t.Errorf("expected pg_http_requests_total metric in output: %s", metricsBody)
	}
}
