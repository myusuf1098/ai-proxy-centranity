package ninerouter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
)

func TestNineRouterCheckHealth_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: server.URL,
		Timeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.CheckHealth(ctx); err != nil {
		t.Fatalf("expected health check to pass, got: %v", err)
	}
}

func TestNineRouterCheckHealth_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: server.URL,
		Timeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := client.CheckHealth(ctx); err == nil {
		t.Fatalf("expected health check to fail on 503, got nil")
	}
}

func TestNineRouterListModels_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"API key required"}`))
			return
		}

		resp := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{"id": "cc-haiku", "object": "model", "owned_by": "combo"},
				{"id": "cc-sonnet", "object": "model", "owned_by": "combo"},
				{"id": "gemini-flash", "object": "model", "owned_by": "google"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: server.URL,
		APIKey:  "valid-token",
		Timeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("expected successful models list, got error: %v", err)
	}

	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(models))
	}

	if models[0].ID != "cc-haiku" || models[1].ID != "cc-sonnet" {
		t.Errorf("unexpected models returned: %+v", models)
	}
}

func TestNineRouterListModels_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"API key required"}`))
	}))
	defer server.Close()

	client := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: server.URL,
		APIKey:  "invalid-token",
		Timeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := client.ListModels(ctx)
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}
}

func TestNineRouterForwardChatCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}

		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-123","choices":[{"message":{"content":"Hello world"}}]}`))
	}))
	defer server.Close()

	client := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Timeout: 5 * time.Second,
	})

	body := strings.NewReader(`{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`)
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	resp, err := client.ForwardChatCompletion(context.Background(), body, headers)
	if err != nil {
		t.Fatalf("expected successful forward, got error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}
