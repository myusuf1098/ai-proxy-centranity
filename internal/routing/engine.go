package routing

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// RouteDecision represents the outcome of route resolution
type RouteDecision struct {
	TargetModel   string   `json:"target_model"`
	FallbackChain []string `json:"fallback_chain"`
	IsAlias       bool     `json:"is_alias"`
	Reason        string   `json:"reason"`
}

// Engine resolves model names and aliases to healthy upstream targets
type Engine struct {
	mu             sync.RWMutex
	circuitBreaker *CircuitBreaker
	aliases        map[string][]string
}

// NewEngine creates a new Routing Engine with default aliases
func NewEngine(cb *CircuitBreaker) *Engine {
	if cb == nil {
		cb = NewCircuitBreaker(CircuitBreakerConfig{})
	}

	e := &Engine{
		circuitBreaker: cb,
		aliases:        make(map[string][]string),
	}

	// Register standard built-in model aliases
	e.aliases["coding"] = []string{"cc-sonnet", "cc-haiku"}
	e.aliases["fast"] = []string{"cc-haiku", "gemini-flash"}
	e.aliases["reasoning"] = []string{"cc-opus", "cc-sonnet"}
	e.aliases["cheap"] = []string{"cc-haiku"}
	e.aliases["free"] = []string{"cc-haiku"}

	return e
}

// SetAlias registers or updates a model alias
func (e *Engine) SetAlias(alias string, targets []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aliases[strings.ToLower(alias)] = targets
}

// Resolve maps a requested model or alias to a healthy target
func (e *Engine) Resolve(ctx context.Context, requestedModel string) (*RouteDecision, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	aliasKey := strings.ToLower(requestedModel)
	targets, isAlias := e.aliases[aliasKey]

	if !isAlias {
		// Direct model specified
		if !e.circuitBreaker.Allow(requestedModel) {
			return nil, fmt.Errorf("circuit breaker OPEN for requested model '%s'", requestedModel)
		}
		return &RouteDecision{
			TargetModel:   requestedModel,
			FallbackChain: []string{},
			IsAlias:       false,
			Reason:        "DIRECT_MODEL",
		}, nil
	}

	// Iterate through alias targets to find the first healthy target
	var chosenTarget string
	var fallbackChain []string

	for _, target := range targets {
		if e.circuitBreaker.Allow(target) {
			if chosenTarget == "" {
				chosenTarget = target
			} else {
				fallbackChain = append(fallbackChain, target)
			}
		}
	}

	if chosenTarget == "" {
		// All targets in alias are currently circuit-broken
		return nil, fmt.Errorf("all upstream targets for alias '%s' are currently unavailable (circuit open)", requestedModel)
	}

	return &RouteDecision{
		TargetModel:   chosenTarget,
		FallbackChain: fallbackChain,
		IsAlias:       true,
		Reason:        fmt.Sprintf("RESOLVED_ALIAS_%s", strings.ToUpper(aliasKey)),
	}, nil
}

// RecordResult records success or failure for a model target
func (e *Engine) RecordResult(target string, success bool) {
	if success {
		e.circuitBreaker.RecordSuccess(target)
	} else {
		e.circuitBreaker.RecordFailure(target)
	}
}
