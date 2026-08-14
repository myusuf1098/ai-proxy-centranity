package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// BuildTransport creates an *http.Transport configured with the proxy profile
func BuildTransport(profile *Profile) (*http.Transport, error) {
	if profile == nil || profile.Type == TypeDirect {
		return &http.Transport{
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}, nil
	}

	var proxyURL *url.URL
	var err error

	switch profile.Type {
	case TypeHTTP, TypeHTTPS:
		scheme := "http"
		if profile.Type == TypeHTTPS {
			scheme = "https"
		}

		rawURL := fmt.Sprintf("%s://%s:%d", scheme, profile.Host, profile.Port)
		if profile.Username != "" {
			rawURL = fmt.Sprintf("%s://%s:%s@%s:%d", scheme, url.QueryEscape(profile.Username), url.QueryEscape(profile.Password), profile.Host, profile.Port)
		}

		proxyURL, err = url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}

		return &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}, nil

	case TypeSOCKS5:
		rawURL := fmt.Sprintf("socks5://%s:%d", profile.Host, profile.Port)
		if profile.Username != "" {
			rawURL = fmt.Sprintf("socks5://%s:%s@%s:%d", url.QueryEscape(profile.Username), url.QueryEscape(profile.Password), profile.Host, profile.Port)
		}

		proxyURL, err = url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SOCKS5 URL: %w", err)
		}

		return &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			MaxIdleConns:        100,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", profile.Type)
	}
}

// CheckHealth probes connectivity through the proxy to a target URL
func CheckHealth(ctx context.Context, profile *Profile, probeURL string) (time.Duration, error) {
	transport, err := BuildTransport(profile)
	if err != nil {
		return 0, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create probe request: %w", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("proxy health probe failed: %w", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	if resp.StatusCode >= 500 {
		return duration, fmt.Errorf("probe returned server error (status %d)", resp.StatusCode)
	}

	return duration, nil
}
