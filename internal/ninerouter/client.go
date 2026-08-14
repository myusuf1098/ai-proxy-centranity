package ninerouter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ModelInfo represents a model discovered from 9Router
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// Config holds connection parameters for the 9Router adapter
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

// NineRouterPort defines the decoupled interface for 9Router operations
type NineRouterPort interface {
	CheckHealth(ctx context.Context) error
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error)
}

// HTTPAdapter implements NineRouterPort over HTTP/REST
type HTTPAdapter struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewHTTPAdapter creates a new 9Router HTTP adapter
func NewHTTPAdapter(cfg Config) *HTTPAdapter {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &HTTPAdapter{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name implements health.HealthChecker interface
func (a *HTTPAdapter) Name() string {
	return "ninerouter"
}

// Check implements health.HealthChecker interface
func (a *HTTPAdapter) Check(ctx context.Context) error {
	return a.CheckHealth(ctx)
}

// CheckHealth verifies 9Router liveness via /api/health
func (a *HTTPAdapter) CheckHealth(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/health", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("9router unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("9router returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

// ListModels retrieves the registered models from 9Router via GET /v1/models
func (a *HTTPAdapter) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/v1/models", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create models request: %w", err)
	}

	if a.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute models request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("9router authentication failed (status %d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("9router returned error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Object string      `json:"object"`
		Data   []ModelInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode models response: %w", err)
	}

	return response.Data, nil
}

// ForwardChatCompletion forwards a chat completion request to 9Router
func (a *HTTPAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", a.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create forward request: %w", err)
	}

	// Copy appropriate headers
	for key, values := range headers {
		if strings.EqualFold(key, "Authorization") {
			continue // Do not forward client auth header directly
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// Inject 9Router upstream API key
	if a.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	}

	// Use transport directly without client timeout to preserve streaming longevity
	transport := a.httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return transport.RoundTrip(req)
}
