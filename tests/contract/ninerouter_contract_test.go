package contract_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
)

func TestLiveNineRouterContract(t *testing.T) {
	liveURL := os.Getenv("PG_NINEROUTER_URL")
	if liveURL == "" {
		liveURL = "http://127.0.0.1:20128"
	}

	// Probe if live 9Router is reachable
	resp, err := http.Get(liveURL + "/api/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("Live 9Router not reachable on host, skipping live contract test")
		return
	}
	_ = resp.Body.Close()

	apiKey := os.Getenv("PG_NINEROUTER_API_KEY")
	if apiKey == "" {
		apiKey = "sk-b00a4ccd4484b040-3q9rhc-4e249c28" // Verified test key
	}

	adapter := ninerouter.NewHTTPAdapter(ninerouter.Config{
		BaseURL: liveURL,
		APIKey:  apiKey,
		Timeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Health check contract
	if err := adapter.CheckHealth(ctx); err != nil {
		t.Fatalf("live 9Router health check failed: %v", err)
	}

	// 2. Model listing contract
	models, err := adapter.ListModels(ctx)
	if err != nil {
		t.Fatalf("live 9Router list models failed: %v", err)
	}

	if len(models) == 0 {
		t.Errorf("expected at least 1 model from live 9Router, got 0")
	}

	t.Logf("Live 9Router contract verified successfully! Discovered %d models.", len(models))
}
