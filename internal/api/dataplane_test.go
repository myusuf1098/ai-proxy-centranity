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
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
)

type mockNineRouterAdapter struct {
	models          []ninerouter.ModelInfo
	listModelsErr   error
	forwardResponse *http.Response
	forwardErr      error
}

func (m *mockNineRouterAdapter) CheckHealth(ctx context.Context) error {
	return nil
}

func (m *mockNineRouterAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	if m.listModelsErr != nil {
		return nil, m.listModelsErr
	}
	return m.models, nil
}

func (m *mockNineRouterAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	if m.forwardErr != nil {
		return nil, m.forwardErr
	}
	return m.forwardResponse, nil
}

func TestDataPlaneListModels(t *testing.T) {
	mockAdapter := &mockNineRouterAdapter{
		models: []ninerouter.ModelInfo{
			{ID: "cc-haiku", Object: "model", OwnedBy: "combo"},
			{ID: "gemini-flash", Object: "model", OwnedBy: "google"},
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}

	if resp.Object != "list" || len(resp.Data) != 2 {
		t.Errorf("unexpected models payload: %+v", resp)
	}
	if resp.Data[0].ID != "cc-haiku" {
		t.Errorf("expected first model cc-haiku, got %s", resp.Data[0].ID)
	}
}

func TestDataPlaneChatCompletion_JSON(t *testing.T) {
	upstreamBody := `{"id":"chatcmpl-test","object":"chat.completion","created":1700000000,"model":"cc-haiku","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}]}`

	mockAdapter := &mockNineRouterAdapter{
		forwardResponse: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(upstreamBody)),
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	reqBody := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Hello!") {
		t.Errorf("expected response to contain 'Hello!', got: %s", w.Body.String())
	}
}

func TestDataPlaneChatCompletion_Streaming(t *testing.T) {
	sseStream := "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\ndata: [DONE]\n\n"

	mockAdapter := &mockNineRouterAdapter{
		forwardResponse: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(sseStream)),
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	reqBody := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}

	if !strings.Contains(w.Body.String(), "[DONE]") {
		t.Errorf("expected stream to contain [DONE], got %s", w.Body.String())
	}
}

func TestDataPlaneChatCompletion_UpstreamError(t *testing.T) {
	mockAdapter := &mockNineRouterAdapter{
		forwardErr: errors.New("upstream connection timeout"),
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	reqBody := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502 Bad Gateway on upstream failure, got %d", w.Code)
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ninerouter.ErrUpstreamUnreach {
		t.Errorf("expected error code %s, got %s", ninerouter.ErrUpstreamUnreach, errResp.Error.Code)
	}
}

func TestUpstream401MapsToAuthError(t *testing.T) {
	mockAdapter := &mockNineRouterAdapter{
		forwardResponse: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"error":"API key required"}`)),
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	reqBody := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502 Bad Gateway, got %d", w.Code)
	}

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != ninerouter.ErrUpstreamAuth {
		t.Errorf("expected error code %s, got %s", ninerouter.ErrUpstreamAuth, errResp.Error.Code)
	}
}

func TestChatCompletionsRequiresMessages(t *testing.T) {
	mockAdapter := &mockNineRouterAdapter{
		forwardResponse: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion","choices":[]}`)),
		},
	}

	cfg, _ := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	healthHandler := health.NewHandler()

	router := api.NewRouterWithAdapter(cfg, healthHandler, mockAdapter, logger)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"cc-haiku"}`))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", w.Code)
	}
}
