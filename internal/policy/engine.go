package policy

import (
	"context"
	"strings"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
)

// Decision represents the policy evaluation outcome
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
}

// Engine evaluates security, model, and provider policies
type Engine struct{}

// NewEngine creates a new PolicyEngine
func NewEngine() *Engine {
	return &Engine{}
}

// EvaluateModel checks if the authenticated key is authorized to access the requested model
func (e *Engine) EvaluateModel(ctx context.Context, key *auth.APIKey, modelID string) Decision {
	if key == nil {
		return Decision{Allowed: false, Reason: "UNAUTHENTICATED"}
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
	if key == nil {
		return Decision{Allowed: false, Reason: "UNAUTHENTICATED"}
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
