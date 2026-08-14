package proxy_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/proxy"
)

func TestProfileCredentialRedaction(t *testing.T) {
	p := &proxy.Profile{
		ID:        "proxy_01",
		Name:      "Secure SOCKS5",
		Type:      proxy.TypeSOCKS5,
		Host:      "proxy.example.com",
		Port:      1080,
		Username:  "secret_user",
		Password:  "super_secret_password",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	bytes, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal proxy profile: %v", err)
	}

	jsonStr := string(bytes)

	// Verify that neither username nor password appears in JSON output
	if strings.Contains(jsonStr, "secret_user") {
		t.Errorf("username leaked in JSON serialization: %s", jsonStr)
	}
	if strings.Contains(jsonStr, "super_secret_password") {
		t.Errorf("password leaked in JSON serialization: %s", jsonStr)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	s := proxy.NewMemoryStore()
	p := &proxy.Profile{ID: "p1", Name: "HTTP-01", Type: proxy.TypeHTTP, Host: "h", Port: 8080}
	if err := s.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(context.Background(), "p1"); err == nil {
		t.Fatal("expected not-found after delete")
	}
}

func TestProfileRegistry(t *testing.T) {
	store := proxy.NewMemoryStore()
	ctx := context.Background()

	p := &proxy.Profile{
		ID:      "proxy_http_1",
		Name:    "US Proxy",
		Type:    proxy.TypeHTTP,
		Host:    "127.0.0.1",
		Port:    8080,
		Enabled: true,
	}

	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}

	retrieved, err := store.Get(ctx, "proxy_http_1")
	if err != nil {
		t.Fatalf("failed to retrieve profile: %v", err)
	}

	if retrieved.Name != "US Proxy" || retrieved.Port != 8080 {
		t.Errorf("mismatched profile data: %+v", retrieved)
	}
}

func TestBuildTransport_Direct(t *testing.T) {
	p := &proxy.Profile{
		Type: proxy.TypeDirect,
	}

	transport, err := proxy.BuildTransport(p)
	if err != nil {
		t.Fatalf("failed to build direct transport: %v", err)
	}

	if transport == nil {
		t.Fatalf("expected non-nil transport")
	}
}

func TestBuildTransport_HTTPProxy(t *testing.T) {
	p := &proxy.Profile{
		Type:     proxy.TypeHTTP,
		Host:     "127.0.0.1",
		Port:     8080,
		Username: "user",
		Password: "password",
	}

	transport, err := proxy.BuildTransport(p)
	if err != nil {
		t.Fatalf("failed to build HTTP proxy transport: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("failed to resolve proxy URL: %v", err)
	}

	if proxyURL == nil || proxyURL.Host != "127.0.0.1:8080" {
		t.Errorf("unexpected proxy URL: %v", proxyURL)
	}
}

func TestBuildTransportSOCKS5UsesRealDialer(t *testing.T) {
	profile := &proxy.Profile{
		ID:   "socks-test",
		Name: "SOCKS5-01",
		Type: proxy.TypeSOCKS5,
		Host: "127.0.0.1",
		Port: 1080,
	}
	tr, err := proxy.BuildTransport(profile)
	if err != nil {
		t.Fatalf("BuildTransport: %v", err)
	}
	if tr.DialContext == nil {
		t.Fatal("SOCKS5 transport has no custom DialContext")
	}
	// stdlib http.ProxyURL leaves DialContext nil; a real SOCKS5 dialer sets it
}

func TestCheckHealth_Direct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &proxy.Profile{Type: proxy.TypeDirect}
	latency, err := proxy.CheckHealth(context.Background(), p, server.URL)
	if err != nil {
		t.Fatalf("expected health check to pass, got: %v", err)
	}
	if latency <= 0 {
		t.Errorf("expected positive latency, got %v", latency)
	}
}
