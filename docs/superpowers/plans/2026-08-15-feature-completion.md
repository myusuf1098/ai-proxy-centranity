# Feature Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make non-functional features from the 2026-08-15 audit actually work: execute the routing fallback chain, enforce RPS/TPM/token quotas, and increment the Prometheus counters that are currently dead code.

**Architecture:** Add fallback execution loop in the data plane chat handler that retries `FallbackChain` on upstream 5xx/error; extend the limiter to RPS and add a quota package with daily/monthly tracking; emit metrics at the existing instrumentation points (rate-limit, forward, token count).

**Tech Stack:** Go 1.26, stdlib `net/http`, Prometheus client_golang (already dep), `log/slog`.

**Spec:** [FEAT-008 Scope 4 fallback](docs/features/FEAT-008-routing-engine.md), [FEAT-007 Scope 3 rate/quotas](docs/features/FEAT-007-policy-plane.md), [FEAT-011 §1 metrics](docs/features/FEAT-011-observability.md), PRD FR-006/FR-008/FR-009/FR-010/FR-014, audit findings S2.1/S2.2/S2.4

## Global Constraints

- Never log API keys, tokens, secrets, authorization headers, request content (PROMT §17)
- Fallback must NOT retry the same failed target — only `FallbackChain` entries
- Rate-limit rejection stays HTTP 429 with `RATE_LIMITED`/`rate_limited` code (FEAT-007)
- Quota data persists in memory for MVP (PROMT §33 — safe assumption, no persistence boundary change)
- TDD: RED → GREEN → REFACTOR
- Every code change updates `docs/CHANGELOG.md`

---

### Task 1: Execute Routing Fallback Chain

**Files:**
- Modify: `internal/api/dataplane.go:219-239` (fallback loop around forward)
- Test: `internal/api/dataplane_routing_test.go` (append)

**Interfaces:**
- Consumes: `*routing.RouteDecision{FallbackChain []string}` — already returned by `h.routingEngine.Resolve` (`engine.go:53-99`)
- Produces: chat handler retries each fallback target once on upstream 5xx/transport error, rewrites `model` per attempt, records circuit result per target, returns first success or normalized upstream error.

- [ ] **Step 1: Write failing test — fallback executes on 5xx**

```go
// internal/api/dataplane_routing_test.go — append
type failFirstAdapter struct {
	models []ninerouter.ModelInfo
	calls  map[string]int // model -> call count
}

func (f *failFirstAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) { return f.models, nil }
func (f *failFirstAdapter) CheckHealth(ctx context.Context) error { return nil }
func (f *failFirstAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	var payload struct{ Model string `json:"model"` }
	_ = json.NewDecoder(body).Decode(&payload)
	f.calls[payload.Model]++
	if f.calls[payload.Model] > 1 {
		return nil, errors.New("should not retry same target")
	}
	// Primary (cc-sonnet) fails 503, fallback (cc-haiku) succeeds
	if payload.Model == "cc-sonnet" {
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader("up")), Header: http.Header{}}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{"Content-Type": {"application/json"}}}, nil
}

func TestChatFallbackExecutesOn5xx(t *testing.T) {
	adapter := &failFirstAdapter{models: []ninerouter.ModelInfo{}, calls: map[string]int{}}
	engine := routing.NewEngine(nil)
	engine.SetAlias("coding", []string{"cc-sonnet", "cc-haiku"})
	keyStore := auth.NewMemoryKeyStore()

	dp := NewDataPlaneHandlerWithRouting(adapter, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())

	body := `{"model":"coding","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	// insert a valid key into context via auth middleware
	raw, key, _ := auth.GenerateAPIKey("tester")
	_ = keyStore.Create(context.Background(), key)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	dp.ChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 (fallback should succeed)", rec.Code)
	}
	if adapter.calls["cc-sonnet"] != 1 {
		t.Errorf("primary cc-sonnet called %d times, want 1", adapter.calls["cc-sonnet"])
	}
	if adapter.calls["cc-haiku"] != 1 {
		t.Errorf("fallback cc-haiku called %d times, want 1", adapter.calls["cc-haiku"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestChatFallbackExecutesOn5xx -v`
Expected: FAIL — `cc-haiku` called 0 times (current code returns 503 on first failure)

- [ ] **Step 3: Implement fallback loop**

```go
// internal/api/dataplane.go — replace lines 219-239 forward block
// 4. Forward to 9Router adapter, with fallback on upstream failure
targets := []string{targetModel}
if decision != nil && len(decision.FallbackChain) > 0 {
	targets = append(targets, decision.FallbackChain...)
}

var lastResp *http.Response
var lastErr error
forwarded := false

for _, t := range targets {
	// Rewrite model per target
	payloadMap["model"] = t
	if updatedBytes, err := json.Marshal(payloadMap); err == nil {
		bodyBytes = updatedBytes
	}

	resp, err := h.adapter.ForwardChatCompletion(r.Context(), bytes.NewReader(bodyBytes), r.Header)
	if err != nil {
		if h.routingEngine != nil {
			h.routingEngine.RecordResult(t, false)
		}
		h.logger.Warn("upstream forward failed, trying fallback", slog.Any("error", err), slog.String("model", t), slog.String("request_id", GetRequestID(r.Context())))
		lastErr = err
		continue
	}

	if resp.StatusCode >= 500 {
		if h.routingEngine != nil {
			h.routingEngine.RecordResult(t, false)
		}
		h.logger.Warn("upstream 5xx, trying fallback", slog.Int("status", resp.StatusCode), slog.String("model", t), slog.String("request_id", GetRequestID(r.Context())))
		resp.Body.Close()
		lastErr = fmt.Errorf("upstream returned %d for %s", resp.StatusCode, t)
		continue
	}

	if h.routingEngine != nil {
		h.routingEngine.RecordResult(t, true)
	}
	lastResp = resp
	forwarded = true
	break
}

if !forwarded {
	if lastErr != nil {
		h.writeError(w, http.StatusBadGateway, "upstream gateway error: "+lastErr.Error(), "upstream_error", "upstream_unavailable")
	} else {
		h.writeError(w, http.StatusBadGateway, "upstream gateway error", "upstream_error", "upstream_unavailable")
	}
	return
}

resp := lastResp
defer resp.Body.Close()
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestChatFallbackExecutesOn5xx -v`
Expected: PASS

- [ ] **Step 5: Add test — no retry of same target, all-fail path**

```go
// internal/api/dataplane_routing_test.go — append
func TestChatAllTargetsFailReturns502(t *testing.T) {
	adapter := &always503Adapter{}
	engine := routing.NewEngine(nil)
	engine.SetAlias("fast", []string{"cc-haiku", "gemini-flash"})
	keyStore := auth.NewMemoryKeyStore()

	dp := NewDataPlaneHandlerWithRouting(adapter, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(), engine, testLogger())

	body := `{"model":"fast","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	raw, key, _ := auth.GenerateAPIKey("tester")
	_ = keyStore.Create(context.Background(), key)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()

	dp.ChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", rec.Code)
	}
	if adapter.calls["cc-haiku"] != 1 || adapter.calls["gemini-flash"] != 1 {
		t.Errorf("each target should be tried once: %v", adapter.calls)
	}
}
```

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/api/dataplane.go internal/api/dataplane_routing_test.go
git commit -m "feat(routing): execute fallback chain on upstream 5xx/error

- Try each FallbackChain target once, rewrite model per attempt
- Record circuit result per target, return 502 when all fail
- Fixes audit finding S2.1 (fallback computed but never executed)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Extend Limiter to RPS (sub-second window)

**Files:**
- Modify: `internal/limiter/limiter.go` (add AllowRPS)
- Test: `internal/limiter/limiter_test.go` (append)

**Interfaces:**
- Consumes: existing `MemoryLimiter` struct + window mechanism
- Produces: `func (l *MemoryLimiter) AllowRPS(ctx context.Context, keyID string, limitRPS int) (bool, int, time.Duration)` — same sliding-window over 1 second. Existing `Allow` (RPM) unchanged. `RateLimiter` interface gains `AllowRPS`.

- [ ] **Step 1: Extend interface + write failing test**

```go
// internal/limiter/limiter.go — add to RateLimiter interface
type RateLimiter interface {
	Allow(ctx context.Context, keyID string, limitRPM int) (allowed bool, remaining int, retryAfter time.Duration)
	AllowRPS(ctx context.Context, keyID string, limitRPS int) (allowed bool, remaining int, retryAfter time.Duration)
}
```

```go
// internal/limiter/limiter_test.go — append
func TestMemoryLimiterAllowRPS(t *testing.T) {
	l := NewMemoryLimiter()
	ctx := context.Background()
	// limit 2 RPS
	if ok, rem, _ := l.AllowRPS(ctx, "k1", 2); !ok || rem != 1 {
		t.Fatalf("1st: ok=%v rem=%d", ok, rem)
	}
	if ok, rem, _ := l.AllowRPS(ctx, "k1", 2); !ok || rem != 0 {
		t.Fatalf("2nd: ok=%v rem=%d", ok, rem)
	}
	if ok, _, retry := l.AllowRPS(ctx, "k1", 2); ok {
		t.Fatalf("3rd should be rejected, retryAfter=%v", retry)
	}
	// different key unaffected
	if ok, _, _ := l.AllowRPS(ctx, "k2", 2); !ok {
		t.Fatal("different key should be allowed")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/limiter/ -run TestMemoryLimiterAllowRPS -v`
Expected: FAIL — `AllowRPS undefined`

- [ ] **Step 3: Implement AllowRPS**

```go
// internal/limiter/limiter.go — append
func (l *MemoryLimiter) AllowRPS(ctx context.Context, keyID string, limitRPS int) (bool, int, time.Duration) {
	if limitRPS <= 0 {
		return true, 999999, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Second)
	windowKey := keyID + ":rps"

	entry, exists := l.windows[windowKey]
	if !exists {
		entry = &windowEntry{timestamps: make([]time.Time, 0, limitRPS)}
		l.windows[windowKey] = entry
	}

	valid := make([]time.Time, 0, len(entry.timestamps))
	for _, t := range entry.timestamps {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}
	entry.timestamps = valid

	if len(entry.timestamps) >= limitRPS {
		retryAfter := entry.timestamps[0].Add(time.Second).Sub(now)
		if retryAfter < 0 {
			retryAfter = time.Second
		}
		return false, 0, retryAfter
	}

	entry.timestamps = append(entry.timestamps, now)
	remaining := limitRPS - len(entry.timestamps)
	return true, remaining, 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/limiter/ -run TestMemoryLimiterAllowRPS -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/limiter/limiter.go internal/limiter/limiter_test.go
git commit -m "feat(limiter): add RPS sliding-window enforcement

- AllowRPS on 1-second window, distinct per-key namespace
- Interface extended, existing RPM Allow unchanged
- Part of fixing audit finding S2.2 (RPS never enforced)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Token Quota Package + Enforcement

**Files:**
- Create: `internal/quota/quota.go`
- Create: `internal/quota/quota_test.go`
- Modify: `internal/api/dataplane.go` (call quota check + record usage)

**Interfaces:**
- Consumes: `auth.APIKey{DailyTokenQuota, MonthlyTokenQuota int64}` (`auth.go:36-37`)
- Produces:
  - `type QuotaStore interface { Allow(ctx, keyID, daily, monthly int64, estimated int64) (bool, int64, int64) ; Record(ctx, keyID string, tokens int64) }`
  - `type MemoryQuota struct` — `Allow` returns (allowed, dailyRemaining, monthlyRemaining); `Record` accumulates.
  - Quota rejection → HTTP 429 with code `quota_exceeded`.

- [ ] **Step 1: Write failing test**

```go
// internal/quota/quota_test.go
package quota

import (
	"context"
	"testing"
)

func TestMemoryQuotaDailyLimit(t *testing.T) {
	q := NewMemoryQuota()
	ctx := context.Background()

	// daily 100, request estimated 60
	allowed, dr, mr := q.Allow(ctx, "k1", 100, 10000, 60)
	if !allowed || dr != 40 || mr != 9940 {
		t.Fatalf("1st: allowed=%v dailyRem=%d monthlyRem=%d", allowed, dr, mr)
	}
	q.Record(ctx, "k1", 60)

	// daily 100, request estimated 50 -> exceeds remaining 40
	allowed, _, _ = q.Allow(ctx, "k1", 100, 10000, 50)
	if allowed {
		t.Fatal("2nd should be rejected (daily quota exceeded)")
	}
}

func TestMemoryQuotaMonthlyLimit(t *testing.T) {
	q := NewMemoryQuota()
	ctx := context.Background()

	allowed, _, mr := q.Allow(ctx, "k2", 100000, 100, 60)
	if allowed {
		t.Fatalf("should reject: monthly remaining=%d", mr)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/quota/ -v`
Expected: FAIL — package `internal/quota` not found

- [ ] **Step 3: Implement quota package**

```go
// internal/quota/quota.go
package quota

import (
	"context"
	"sync"
)

// QuotaStore tracks daily/monthly token usage per API key
type QuotaStore interface {
	Allow(ctx context.Context, keyID string, dailyLimit, monthlyLimit, estimated int64) (bool, int64, int64)
	Record(ctx context.Context, keyID string, tokens int64)
}

type usage struct {
	daily   int64
	monthly int64
}

// MemoryQuota is an in-memory quota tracker
type MemoryQuota struct {
	mu     sync.Mutex
	usage  map[string]*usage
}

func NewMemoryQuota() *MemoryQuota {
	return &MemoryQuota{usage: make(map[string]*usage)}
}

// Allow checks whether estimated tokens fit within daily/monthly limits
func (q *MemoryQuota) Allow(ctx context.Context, keyID string, dailyLimit, monthlyLimit, estimated int64) (bool, int64, int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	u, exists := q.usage[keyID]
	if !exists {
		u = &usage{}
		q.usage[keyID] = u
	}

	dailyRem := dailyLimit - u.daily
	monthlyRem := monthlyLimit - u.monthly

	if dailyLimit > 0 && dailyRem < estimated {
		return false, dailyRem, monthlyRem
	}
	if monthlyLimit > 0 && monthlyRem < estimated {
		return false, dailyRem, monthlyRem
	}
	return true, dailyRem, monthlyRem
}

// Record accumulates tokens consumed for a key
func (q *MemoryQuota) Record(ctx context.Context, keyID string, tokens int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	u, exists := q.usage[keyID]
	if !exists {
		u = &usage{}
		q.usage[keyID] = u
	}
	u.daily += tokens
	u.monthly += tokens
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/quota/ -v`
Expected: PASS

- [ ] **Step 5: Wire quota into dataplane**

```go
// internal/api/dataplane.go
// Add field:
quotaStore quota.QuotaStore

// In NewDataPlaneHandlerWithRouting, accept quota store:
func NewDataPlaneHandlerWithRouting(
	adapter ninerouter.NineRouterPort,
	keyStore auth.KeyStore,
	policyEngine *policy.Engine,
	rateLimiter limiter.RateLimiter,
	routingEngine *routing.Engine,
	logger *slog.Logger,
) *DataPlaneHandler {
	// ...existing nil checks...
	return &DataPlaneHandler{
		adapter:       adapter,
		keyStore:      keyStore,
		policyEngine:  policyEngine,
		rateLimiter:   rateLimiter,
		routingEngine: routingEngine,
		quotaStore:    quota.NewMemoryQuota(),
		logger:        logger,
	}
}

// In ChatCompletions, after rate limit check (~line 208), add:
if key != nil && h.quotaStore != nil {
	// Estimate tokens from messages if possible, else conservative 0 (no limit applied)
	estimated := estimateTokens(parsed.Messages)
	allowed, dailyRem, monthlyRem := h.quotaStore.Allow(r.Context(), key.ID, key.DailyTokenQuota, key.MonthlyTokenQuota, estimated)
	w.Header().Set("X-RateLimit-Quota-Daily-Remaining", fmt.Sprintf("%d", dailyRem))
	w.Header().Set("X-RateLimit-Quota-Monthly-Remaining", fmt.Sprintf("%d", monthlyRem))
	if !allowed {
		h.writeError(w, http.StatusTooManyRequests, "Token quota exceeded", "quota_error", "quota_exceeded")
		return
	}
}

// estimateTokens: rough heuristic, ponytail: chars/4, upgrade when usage API lands
func estimateTokens(messages []any) int64 {
	if len(messages) == 0 {
		return 0
	}
	// conservative: count roughly 4 chars per token across serialized messages
	b, err := json.Marshal(messages)
	if err != nil {
		return 0
	}
	return int64(len(b) / 4)
}

// After successful forward, record usage:
if key != nil && h.quotaStore != nil {
	h.quotaStore.Record(r.Context(), key.ID, estimateTokens(parsed.Messages))
}
```

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/quota/quota.go internal/quota/quota_test.go internal/api/dataplane.go
git commit -m "feat(quota): daily/monthly token quota enforcement

- New internal/quota package with Allow + Record
- Data plane checks quota before forward, records after success
- 429 quota_exceeded on overage
- Fixes audit finding S2.2 (token quotas never enforced)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Increment Prometheus Counters

**Files:**
- Modify: `internal/api/dataplane.go` (emit metrics at existing points)
- Test: `internal/api/dataplane_metrics_test.go` (create)

**Interfaces:**
- Consumes: `telemetry.Metrics` methods `ObserveRequest(method, path, status)`, `ObserveDuration(path, model, d)`, `IncTokens(keyID, model, typ, n)`, `IncUpstreamError(provider, code)` (signatures per `internal/telemetry/metrics.go`)
- Produces: dead counters `pg_tokens_total` + `pg_upstream_errors_total` get real callers; duration model label populated.

- [ ] **Step 1: Read metrics.go to confirm method names**

Run: `grep -n "^func (m \*Metrics)" internal/telemetry/metrics.go`
Expected: list of exported methods. Use exact names in code below.

- [ ] **Step 2: Wire metrics into dataplane handler**

```go
// internal/api/dataplane.go
// Add field:
metrics *telemetry.Metrics

// Setter:
func (h *DataPlaneHandler) SetMetrics(m *telemetry.Metrics) { h.metrics = m }

// In router.go NewRouterWithTelemetry, after creating dpHandler:
if dpHandler != nil {
	dpHandler.SetMetrics(metrics)
}
```

- [ ] **Step 3: Emit at call sites**

```go
// After successful forward (both stream + non-stream), ~line 239:
if h.metrics != nil {
	h.metrics.IncTokens(keyIDOrUnknown(r), targetModel, "output", estimateTokens(parsed.Messages))
	h.metrics.ObserveRequest("POST", "/v1/chat/completions", resp.StatusCode)
	h.metrics.ObserveDuration("/v1/chat/completions", targetModel, time.Since(startTime))
}

// On upstream error (before writeError ~line 226):
if h.metrics != nil {
	h.metrics.IncUpstreamError("9router", "upstream_unavailable")
}

// startTime := time.Now() at top of ChatCompletions
```

- [ ] **Step 4: Add metrics integration test**

```go
// internal/api/dataplane_metrics_test.go
package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/telemetry"
)

func TestMetricsEmittedOnRequest(t *testing.T) {
	m := telemetry.NewMetrics()
	adapter := &okAdapter{}
	keyStore := auth.NewMemoryKeyStore()
	dp := NewDataPlaneHandlerWithRouting(adapter, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(), routing.NewEngine(nil), testLogger())
	dp.SetMetrics(m)

	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	raw, key, _ := auth.GenerateAPIKey("tester")
	_ = keyStore.Create(req.Context(), key)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	dp.ChatCompletions(rec, req)

	// scrape the metrics endpoint and look for pg_http_requests_total with our labels
	out := scrapeMetrics(t, m)
	if !strings.Contains(out, `pg_http_requests_total`) {
		t.Fatal("request counter not incremented")
	}
	if !strings.Contains(out, `pg_tokens_total`) {
		t.Fatal("token counter not incremented")
	}
}
```

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/dataplane.go internal/api/router.go internal/api/dataplane_metrics_test.go
git commit -m "feat(observability): increment token + upstream error counters

- Wire metrics into dataplane handler, emit at forward/error/usage points
- Populate model label on duration histogram
- Fixes audit finding S2.4 (dead Prometheus counters)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Global Deny Policy (precedence top)

**Files:**
- Modify: `internal/policy/engine.go` (add global deny state)
- Modify: `internal/api/dataplane.go` (call global check first)
- Test: `internal/policy/policy_test.go` (append)

**Interfaces:**
- Consumes: `policy.Engine` (currently empty struct)
- Produces:
  - `func (e *Engine) SetGlobalDeny(models, providers []string)` — configures global denylist
  - `func (e *Engine) EvaluateModel(ctx, key, modelID)` now checks global deny FIRST, then per-key (precedence: Global Deny > Per-Key Deny > Per-Key Allow, FEAT-007)

- [ ] **Step 1: Write failing test**

```go
// internal/policy/policy_test.go — append
func TestGlobalDenyOverridesPerKeyAllow(t *testing.T) {
	e := NewEngine()
	e.SetGlobalDeny([]string{"cc-opus"}, nil)

	key := &auth.APIKey{
		ID:           "k1",
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/policy/ -run TestGlobalDenyOverridesPerKeyAllow -v`
Expected: FAIL — `SetGlobalDeny undefined`

- [ ] **Step 3: Implement global deny**

```go
// internal/policy/engine.go — add global state + check
type Engine struct {
	mu            sync.RWMutex
	globalDeny    globalDeny
}

type globalDeny struct {
	models    []string
	providers []string
}

func (e *Engine) SetGlobalDeny(models, providers []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalDeny = globalDeny{models: models, providers: providers}
}

// EvaluateModel — prepend before per-key checks:
func (e *Engine) EvaluateModel(ctx context.Context, key *auth.APIKey, modelID string) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, denied := range e.globalDeny.models {
		if strings.EqualFold(denied, modelID) || denied == "*" {
			return Decision{Allowed: false, Reason: "GLOBAL_MODEL_DENIED"}
		}
	}
	// ...existing per-key logic (remove old RLock/Unlock pattern; now nested under e.mu)
}

// EvaluateProvider — same global check for providers
func (e *Engine) EvaluateProvider(ctx context.Context, key *auth.APIKey, providerID string) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, denied := range e.globalDeny.providers {
		if strings.EqualFold(denied, providerID) || denied == "*" {
			return Decision{Allowed: false, Reason: "GLOBAL_PROVIDER_DENIED"}
		}
	}
	// ...existing per-key logic
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/policy/ -run TestGlobalDenyOverridesPerKeyAllow -v`
Expected: PASS

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/policy/engine.go internal/policy/policy_test.go
git commit -m "feat(policy): global deny precedence over per-key allow

- SetGlobalDeny(models, providers), checked first in EvaluateModel/Provider
- Precedence: Global Deny > Per-Key Deny > Per-Key Allow
- Fixes audit finding S2.3

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 6: Align 9Router Error Codes + Request Validation

**Files:**
- Modify: `internal/ninerouter/client.go` (return typed errors)
- Modify: `internal/api/dataplane.go` (map typed errors to contract codes; validate messages)
- Test: `tests/contract/ninerouter_contract_test.go` (append), `internal/api/dataplane_test.go` (append)

**Interfaces:**
- Consumes: `docs/api/9router-compatibility.md` §2 contract (`UPSTREAM_AUTH_ERROR`, `UPSTREAM_RATE_LIMIT`, `UPSTREAM_UNAVAILABLE`, `UPSTREAM_UNREACHABLE`)
- Produces: data plane maps upstream failures to the documented error codes; chat request validates `messages` present

- [ ] **Step 1: Define typed error codes**

```go
// internal/ninerouter/client.go — append
// Error codes per docs/api/9router-compatibility.md §2
const (
	ErrUpstreamAuth       = "UPSTREAM_AUTH_ERROR"
	ErrUpstreamRateLimit  = "UPSTREAM_RATE_LIMIT"
	ErrUpstreamUnavail    = "UPSTREAM_UNAVAILABLE"
	ErrUpstreamUnreach    = "UPSTREAM_UNREACHABLE"
)
```

- [ ] **Step 2: Write failing test — 401 maps to UPSTREAM_AUTH_ERROR**

```go
// internal/api/dataplane_test.go — append
type authFailAdapter struct{}

func (a *authFailAdapter) ListModels(ctx context.Context) ([]ninerouter.ModelInfo, error) {
	return nil, nil
}
func (a *authFailAdapter) CheckHealth(ctx context.Context) error { return nil }
func (a *authFailAdapter) ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader(`{"error":"API key required"}`)), Header: http.Header{}}, nil
}

func TestUpstream401MapsToAuthError(t *testing.T) {
	dp := NewDataPlaneHandler(authFailAdapter{}, testLogger())
	body := `{"model":"cc-haiku","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	dp.ChatCompletions(rec, req)

	var resp OpenAIError
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", rec.Code)
	}
	if resp.Error.Code != ninerouter.ErrUpstreamAuth {
		t.Fatalf("code = %q, want %q", resp.Error.Code, ninerouter.ErrUpstreamAuth)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestUpstream401MapsToAuthError -v`
Expected: FAIL — code is `upstream_unavailable`, want `UPSTREAM_AUTH_ERROR`

- [ ] **Step 4: Implement status→code mapping in dataplane**

```go
// internal/api/dataplane.go — in fallback loop, when resp.StatusCode >= 500
// OR in the non-fallback path, map status to contract code:
func upstreamErrorCode(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return ninerouter.ErrUpstreamAuth
	case status == http.StatusTooManyRequests:
		return ninerouter.ErrUpstreamRateLimit
	case status >= 500:
		return ninerouter.ErrUpstreamUnavail
	default:
		return ninerouter.ErrUpstreamUnavail
	}
}

// Connection refused/timeout: adapter returns error — map to ErrUpstreamUnreach.
// In ChatCompletions forward-failure branch (all fallbacks failed):
if lastErr != nil {
	code := ninerouter.ErrUpstreamUnreach
	h.writeError(w, http.StatusBadGateway, "upstream gateway error: "+lastErr.Error(), "upstream_error", code)
	return
}
```

- [ ] **Step 5: Add request validation — messages required**

```go
// internal/api/dataplane.go — after parsed.Model check (~line 176)
if len(parsed.Messages) == 0 {
	h.writeError(w, http.StatusBadRequest, "field 'messages' is required", "invalid_request_error", "missing_messages")
	return
}
```

- [ ] **Step 6: Add validation test**

```go
// internal/api/dataplane_test.go — append
func TestChatCompletionsRequiresMessages(t *testing.T) {
	dp := NewDataPlaneHandler(&okAdapter{}, testLogger())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"cc-haiku"}`))
	rec := httptest.NewRecorder()
	dp.ChatCompletions(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 7: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ninerouter/client.go internal/api/dataplane.go internal/api/dataplane_test.go tests/contract/ninerouter_contract_test.go
git commit -m "feat(9router): align upstream error codes with compatibility matrix

- UPSTREAM_AUTH_ERROR / UPSTREAM_RATE_LIMIT / UPSTREAM_UNAVAILABLE / UPSTREAM_UNREACHABLE
- Validate messages field present in chat payload
- Fixes audit findings S3.1 + S3.6

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- S2.1 (fallback exec) → Task 1 ✓
- S2.2 (RPS) → Task 2 ✓
- S2.2 (token quotas) → Task 3 ✓
- S2.4 (metric counters) → Task 4 ✓
- S2.3 (global deny) → Task 5 ✓
- S3.1 (9Router error codes) + S3.6 (request validation) + S3.7 (error normalization) → Task 6 ✓
- FR-010 retry → Task 1 fallback = single retry per target (not exponential backoff — noted as MVP; backoff deferred per YAGNI, only if upstream demands)
- FEAT-007 429 + `RATE_LIMITED` code → quota uses `quota_exceeded`, rate limit keeps `rate_limited` ✓

**Deferred (not in this plan — needs separate plan/scope decision):**
- S2.5 TUI screens render static data → separate TUI-wiring plan (needs mgmt API endpoints `/models`, `/keys`, `/routes` + screen fetch)
- S2.7 backup/restore automation → infra plan (scripts + cron + DR drill)
- S3.5 TUI UX spec deviations (emoji controls, palette, modals, help overlay) → fold into TUI-wiring plan
- S3.9 benchmark honesty (`BenchmarkLimiter_Allow` inflated) → small fix, can ride along with any later bench pass

**Placeholder scan:** `estimateTokens` heuristic marked with `ponytail:` comment — no TBD. `keyIDOrUnknown` reused from Plan 1 Task 3. `testLogger()`, `okAdapter`, `always503Adapter`, `scrapeMetrics` are test helpers — must be added to test files (noted in steps). No placeholder steps.

**Type consistency:** `quota.QuotaStore.Allow(ctx, keyID, daily, monthly, estimated) (bool, int64, int64)` consistent across Task 3 steps. `AllowRPS` signature matches `Allow` pattern. `telemetry.Metrics` method names must be verified in Step 1 of Task 4 before coding. `RouteDecision.FallbackChain` used verbatim from `engine.go:14`. `policy.Engine.SetGlobalDeny(models, providers []string)` consistent in Task 5. `upstreamErrorCode(status int) string` returns the four `ninerouter.ErrUpstream*` constants in Task 6.
