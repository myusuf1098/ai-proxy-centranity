package limiter_test

import (
	"context"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
)

func TestMemoryLimiter_RPM(t *testing.T) {
	l := limiter.NewMemoryLimiter()
	ctx := context.Background()
	keyID := "key_test_123"
	limitRPM := 5

	// First 5 requests allowed
	for i := 0; i < 5; i++ {
		allowed, remaining, _ := l.Allow(ctx, keyID, limitRPM)
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
		expectedRemaining := limitRPM - (i + 1)
		if remaining != expectedRemaining {
			t.Errorf("expected remaining %d, got %d", expectedRemaining, remaining)
		}
	}

	// 6th request should be rejected (429)
	allowed, remaining, retryAfter := l.Allow(ctx, keyID, limitRPM)
	if allowed {
		t.Fatalf("6th request should be blocked by rate limit")
	}
	if remaining != 0 {
		t.Errorf("expected remaining 0, got %d", remaining)
	}
	if retryAfter <= 0 {
		t.Errorf("expected positive retry-after duration, got %v", retryAfter)
	}
}

func TestMemoryLimiter_DifferentKeysIsolated(t *testing.T) {
	l := limiter.NewMemoryLimiter()
	ctx := context.Background()

	// Fill key A
	for i := 0; i < 3; i++ {
		_, _, _ = l.Allow(ctx, "key_A", 3)
	}
	blockedA, _, _ := l.Allow(ctx, "key_A", 3)
	if blockedA {
		t.Errorf("key A should be blocked")
	}

	// Key B should still be allowed
	allowedB, _, _ := l.Allow(ctx, "key_B", 3)
	if !allowedB {
		t.Errorf("key B should be allowed")
	}
}
