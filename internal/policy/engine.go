package policy

import (
	"context"
	"strings"
	"sync"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
)

// Decision represents the policy evaluation outcome
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

type globalDeny struct {
	models    []string
	providers []string
}

// Engine evaluates security, model, and provider policies
type Engine struct {
	mu         sync.RWMutex
	globalDeny globalDeny
}

// NewEngine creates a new PolicyEngine
func NewEngine() *Engine {
	return &Engine{}
}

// SetGlobalDeny configures the global denylist, evaluated before per-key policy
func (e *Engine) SetGlobalDeny(models, providers []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalDeny = globalDeny{models: models, providers: providers}
}

// EvaluateModel checks if the authenticated key is authorized to access the requested model
func (e *Engine) EvaluateModel(ctx context.Context, key *auth.APIKey, modelID string) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if key == nil {
		return Decision{Allowed: false, Reason: "UNAUTHENTICATED"}
	}

	// 0. Global deny always wins
	for _, denied := range e.globalDeny.models {
		if strings.EqualFold(denied, modelID) || denied == "*" {
			return Decision{Allowed: false, Reason: "GLOBAL_MODEL_DENIED"}
		}
	}

	// 1. Check DeniedModels (Deny always overrides allow)
	for _, denied := range key.DeniedModels {
		if strings.EqualFold(denied, modelID) || denied == "*" {
			return Decision{Allowed: false, Reason: "MODEL_DENIED"}
		}
	}

	// 2. Check AllowedModels
	if len(key.AllowedModels) > 0 {
		allowed := false
		for _, allowPattern := range key.AllowedModels {
			if strings.EqualFold(allowPattern, modelID) || allowPattern == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return Decision{Allowed: false, Reason: "MODEL_NOT_ALLOWED"}
		}
	}

	return Decision{Allowed: true, Reason: "ALLOWED"}
}

// EvaluateProvider checks if the key is authorized to access the upstream provider
func (e *Engine) EvaluateProvider(ctx context.Context, key *auth.APIKey, providerID string) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if key == nil {
		return Decision{Allowed: false, Reason: "UNAUTHENTICATED"}
	}

	// 0. Global deny always wins
	for _, denied := range e.globalDeny.providers {
		if strings.EqualFold(denied, providerID) || denied == "*" {
			return Decision{Allowed: false, Reason: "GLOBAL_PROVIDER_DENIED"}
		}
	}

	// 1. Check DeniedProviders
	for _, denied := range key.DeniedProviders {
		if strings.EqualFold(denied, providerID) || denied == "*" {
			return Decision{Allowed: false, Reason: "PROVIDER_DENIED"}
		}
	}

	// 2. Check AllowedProviders
	if len(key.AllowedProviders) > 0 {
		allowed := false
		for _, allowPattern := range key.AllowedProviders {
			if strings.EqualFold(allowPattern, providerID) || allowPattern == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return Decision{Allowed: false, Reason: "PROVIDER_NOT_ALLOWED"}
		}
	}

	return Decision{Allowed: true, Reason: "ALLOWED"}
}
