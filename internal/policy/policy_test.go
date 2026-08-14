package policy_test

import (
	"context"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
)

func TestPolicyEngine_AllowAllByDefault(t *testing.T) {
	engine := policy.NewEngine()
	key := &auth.APIKey{
		AllowedModels: []string{},
		DeniedModels:  []string{},
	}

	decision := engine.EvaluateModel(context.Background(), key, "cc-haiku")
	if !decision.Allowed {
		t.Errorf("expected model to be allowed when no restrictions configured")
	}
}

func TestPolicyEngine_DeniedModel(t *testing.T) {
	engine := policy.NewEngine()
	key := &auth.APIKey{
		AllowedModels: []string{"cc-haiku", "cc-sonnet"},
		DeniedModels:  []string{"cc-sonnet"}, // Deny overrides allow
	}

	// 1. Explicitly denied
	decision1 := engine.EvaluateModel(context.Background(), key, "cc-sonnet")
	if decision1.Allowed {
		t.Errorf("expected cc-sonnet to be blocked by DeniedModels")
	}
	if decision1.Reason != "MODEL_DENIED" {
		t.Errorf("expected reason MODEL_DENIED, got %s", decision1.Reason)
	}

	// 2. Allowed model
	decision2 := engine.EvaluateModel(context.Background(), key, "cc-haiku")
	if !decision2.Allowed {
		t.Errorf("expected cc-haiku to be allowed")
	}

	// 3. Model not in allowlist
	decision3 := engine.EvaluateModel(context.Background(), key, "gpt-4o")
	if decision3.Allowed {
		t.Errorf("expected gpt-4o to be blocked when not in AllowedModels")
	}
}

func TestGlobalDenyOverridesPerKeyAllow(t *testing.T) {
	e := policy.NewEngine()
	e.SetGlobalDeny([]string{"cc-opus"}, nil)

	key := &auth.APIKey{
		ID:            "k1",
		AllowedModels: []string{"cc-opus", "cc-sonnet"},
		DeniedModels:  []string{},
	}
	// Per-key allow says cc-opus OK, but global deny must win
	d := e.EvaluateModel(context.Background(), key, "cc-opus")
	if d.Allowed {
		t.Fatalf("global deny should override per-key allow, got reason %s", d.Reason)
	}
	// cc-sonnet unaffected
	d2 := e.EvaluateModel(context.Background(), key, "cc-sonnet")
	if !d2.Allowed {
		t.Fatalf("cc-sonnet should be allowed, got %s", d2.Reason)
	}
}
