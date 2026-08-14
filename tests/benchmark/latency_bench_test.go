package benchmark_test

import (
	"context"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

func BenchmarkPolicyEngine_Evaluate(b *testing.B) {
	engine := policy.NewEngine()
	key := &auth.APIKey{
		AllowedModels: []string{"cc-haiku", "cc-sonnet", "gemini-flash"},
		DeniedModels:  []string{"cc-opus"},
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decision := engine.EvaluateModel(ctx, key, "cc-sonnet")
		if !decision.Allowed {
			b.Fatalf("expected allowed")
		}
	}
}

func BenchmarkRoutingEngine_Resolve(b *testing.B) {
	cb := routing.NewCircuitBreaker(routing.CircuitBreakerConfig{})
	engine := routing.NewEngine(cb)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decision, err := engine.Resolve(ctx, "coding")
		if err != nil || decision.TargetModel != "cc-sonnet" {
			b.Fatalf("unexpected routing result")
		}
	}
}

func BenchmarkLimiter_Allow(b *testing.B) {
	l := limiter.NewMemoryLimiter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = l.Allow(ctx, "bench_key", 10000000)
	}
}
