package routing_test

import (
	"context"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := routing.NewCircuitBreaker(routing.CircuitBreakerConfig{
		FailureThreshold: 3,
		CooldownDuration: 100 * time.Millisecond,
	})

	target := "provider_broken"

	// 1. Initial state is CLOSED
	if !cb.Allow(target) {
		t.Errorf("initial target should be allowed (CLOSED)")
	}

	// 2. Record 2 failures (below threshold) -> Still CLOSED
	cb.RecordFailure(target)
	cb.RecordFailure(target)
	if !cb.Allow(target) {
		t.Errorf("target should still be allowed after 2 failures")
	}

	// 3. 3rd failure reaches threshold -> State becomes OPEN
	cb.RecordFailure(target)
	if cb.Allow(target) {
		t.Errorf("target should be blocked (OPEN) after 3 failures")
	}

	// 4. Wait for cooldown -> State transitions to HALF_OPEN
	time.Sleep(120 * time.Millisecond)
	if !cb.Allow(target) {
		t.Errorf("target should be allowed for probe (HALF_OPEN) after cooldown")
	}

	// 5. Success in HALF_OPEN resets state back to CLOSED
	cb.RecordSuccess(target)
	if cb.GetState(target) != routing.CircuitClosed {
		t.Errorf("expected circuit to close after successful probe, got %v", cb.GetState(target))
	}
}

func TestRoutingEngine_AliasResolution(t *testing.T) {
	cb := routing.NewCircuitBreaker(routing.CircuitBreakerConfig{
		FailureThreshold: 3,
		CooldownDuration: time.Minute,
	})
	engine := routing.NewEngine(cb)

	ctx := context.Background()

	// 1. Resolve built-in alias "coding"
	decision, err := engine.Resolve(ctx, "coding")
	if err != nil {
		t.Fatalf("expected successful resolution for alias 'coding', got %v", err)
	}

	if !decision.IsAlias || decision.TargetModel != "cc-sonnet" {
		t.Errorf("unexpected coding target: %+v", decision)
	}
	if len(decision.FallbackChain) == 0 {
		t.Errorf("expected fallback chain for alias")
	}

	// 2. Direct model resolution
	directDecision, err := engine.Resolve(ctx, "gpt-4o")
	if err != nil {
		t.Fatalf("expected resolution for direct model, got %v", err)
	}
	if directDecision.IsAlias || directDecision.TargetModel != "gpt-4o" {
		t.Errorf("unexpected direct target: %+v", directDecision)
	}
}

func TestRoutingEngine_CircuitBypassesFailedTarget(t *testing.T) {
	cb := routing.NewCircuitBreaker(routing.CircuitBreakerConfig{
		FailureThreshold: 2,
		CooldownDuration: time.Minute,
	})
	engine := routing.NewEngine(cb)

	// Set custom alias: "smart" -> ["model-a", "model-b"]
	engine.SetAlias("smart", []string{"model-a", "model-b"})

	ctx := context.Background()

	// First resolution selects model-a
	d1, _ := engine.Resolve(ctx, "smart")
	if d1.TargetModel != "model-a" {
		t.Fatalf("expected initial target model-a, got %s", d1.TargetModel)
	}

	// Trip circuit for model-a
	cb.RecordFailure("model-a")
	cb.RecordFailure("model-a")

	// Next resolution automatically falls back to model-b!
	d2, err := engine.Resolve(ctx, "smart")
	if err != nil {
		t.Fatalf("expected fallback resolution, got error: %v", err)
	}
	if d2.TargetModel != "model-b" {
		t.Errorf("expected fallback target model-b, got %s", d2.TargetModel)
	}
}
