# Security Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close 3 severity-1 security findings from the 2026-08-15 compliance audit: unauthenticated Management API, non-functional SOCKS5 dialing, and zero production audit callers — plus the CORS origin bypass.

**Architecture:** Add `AdminAuthMiddleware` (constant-time token verify, fail-closed) wrapping all `/api/v1/*` routes; wire `audit.MemoryStore` into the request pipeline with audit events at auth/policy/rate-limit/route points; replace fake SOCKS5 `http.ProxyURL` with a real `golang.org/x/net/proxy` SOCKS5 dialer; honor `AllowedOrigins` in CORS.

**Tech Stack:** Go 1.26, stdlib `net/http`, `crypto/subtle`, `golang.org/x/net/proxy` (new dep), `log/slog`.

**Spec:** [docs/specs/PROMT.md §17 Security Contract](docs/specs/PROMT.md), [FEAT-007 policy/auth](docs/features/FEAT-007-policy-plane.md), [FEAT-009 proxy creds](docs/features/FEAT-009-proxy-management.md), [FEAT-011 audit](docs/features/FEAT-011-observability.md), audit findings S1.1/S1.2/S1.3

## Global Constraints

- Never log: API keys, provider secrets, proxy passwords, authorization headers, bearer tokens, sensitive request content (PROMT §17)
- Admin token missing/empty → fail-closed (401) — security is release-blocking, no silent bypass
- Use constant-time comparison for token verify (anti timing-attack)
- New dependency allowed only with justification (PROMT §26) — `golang.org/x/net/proxy` justified by SOCKS5 requirement, stdlib cannot do it
- TDD: RED → GREEN → REFACTOR per task (AGENTS.md norm)
- Every code change must update `docs/CHANGELOG.md` (CLAUDE.md doc workflow)

---

### Task 1: Admin Auth Middleware (Management API protection)

**Files:**
- Create: `internal/api/adminauth.go`
- Create: `internal/api/adminauth_test.go`
- Modify: `internal/api/router.go:80-85` (wrap mgmt routes)
- Modify: `internal/api/router.go` (signature pass-through)

**Interfaces:**
- Consumes: `config.AdminConfig.ManagementToken` (`internal/config/config.go:44,85`)
- Produces: `func AdminAuthMiddleware(token string) func(http.Handler) http.Handler` — returns 401 JSON `{"error":{"message":"Unauthorized","type":"auth_error","code":"unauthorized"}}` when `Authorization: Bearer <token>` mismatches, else passes through.

- [ ] **Step 1: Write the failing test**

```go
// internal/api/adminauth_test.go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminAuthMiddleware(t *testing.T) {
	handler := AdminAuthMiddleware("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"missing header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"no bearer prefix", "secret123", http.StatusUnauthorized},
		{"correct token", "Bearer secret123", http.StatusOK},
		{"empty configured token", "Bearer anything", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestAdminAuthMiddleware -v`
Expected: FAIL — `undefined: AdminAuthMiddleware`

- [ ] **Step 3: Write minimal implementation**

```go
// internal/api/adminauth.go
package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// AdminAuthMiddleware protects Management API routes with a shared admin token.
// Fail-closed: empty or mismatched token -> 401.
func AdminAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validAdminToken(token, r.Header.Get("Authorization")) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"message": "Unauthorized",
						"type":    "auth_error",
						"code":    "unauthorized",
					},
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validAdminToken(configured, header string) bool {
	if configured == "" {
		return false // fail-closed
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	got := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(configured)) == 1
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestAdminAuthMiddleware -v`
Expected: PASS (all 5 cases)

- [ ] **Step 5: Wire into router**

```go
// internal/api/router.go — inside NewRouterWithTelemetry, mgmt block
if mgmtHandler != nil {
	adminAuth := AdminAuthMiddleware(cfg.Admin.ManagementToken)
	mux.Handle("GET /api/v1/system", adminAuth(http.HandlerFunc(mgmtHandler.GetSystem)))
	mux.Handle("GET /api/v1/overview", adminAuth(http.HandlerFunc(mgmtHandler.GetOverview)))
	mux.Handle("GET /api/v1/proxies", adminAuth(http.HandlerFunc(mgmtHandler.GetProxies)))
}
```

- [ ] **Step 6: Add integration test for router auth**

```go
// internal/api/middleware_test.go — append
func TestManagementRoutesRequireAdminToken(t *testing.T) {
	cfg := &config.Config{Admin: config.AdminConfig{ManagementToken: "tok"}}
	h := health.NewHandler()
	router := NewRouterWithManagement(cfg, h, nil, &ManagementHandler{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, path := range []string{"/api/v1/system", "/api/v1/overview", "/api/v1/proxies"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", path, rec.Code)
		}
	}
}
```

- [ ] **Step 7: Run full test suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/api/adminauth.go internal/api/adminauth_test.go internal/api/router.go internal/api/middleware_test.go
git commit -m "feat(security): protect Management API with admin token auth

- AdminAuthMiddleware: constant-time Bearer verify, fail-closed on empty token
- Wrap /api/v1/system|overview|proxies in admin auth
- Fixes audit finding S1.1 (unauthenticated management API)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Real SOCKS5 Dialer for Proxy Profiles

**Files:**
- Modify: `internal/proxy/transport.go:42-69` (transport builder)
- Modify: `go.mod`, `go.sum` (add `golang.org/x/net`)
- Test: `internal/proxy/proxy_test.go` (append)

**Interfaces:**
- Consumes: `proxy.Profile{Type, Host, Port, Username, Password}` (`internal/proxy/profile.go:21-32`)
- Produces: unchanged `BuildTransport(profile) (*http.Transport, error)` — but SOCKS5 type now uses `golang.org/x/net/proxy` dialer instead of `http.ProxyURL`. Other types unchanged.

- [ ] **Step 1: Add dependency**

Run: `go get golang.org/x/net/proxy`
Expected: adds `golang.org/x/net` to go.mod

- [ ] **Step 2: Write the failing test**

```go
// internal/proxy/proxy_test.go — append
func TestBuildTransportSOCKS5UsesRealDialer(t *testing.T) {
	profile := &Profile{
		ID:   "socks-test",
		Name: "SOCKS5-01",
		Type: TypeSOCKS5,
		Host: "127.0.0.1",
		Port: 1080,
	}
	tr, err := BuildTransport(profile)
	if err != nil {
		t.Fatalf("BuildTransport: %v", err)
	}
	if tr.DialContext == nil {
		t.Fatal("SOCKS5 transport has no custom DialContext")
	}
	// stdlib http.ProxyURL leaves DialContext nil; a real SOCKS5 dialer sets it
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/proxy/ -run TestBuildTransportSOCKS5UsesRealDialer -v`
Expected: FAIL — `SOCKS5 transport has no custom DialContext` (current code uses `http.ProxyURL`, no DialContext)

- [ ] **Step 4: Implement SOCKS5 dialer**

```go
// internal/proxy/transport.go — SOCKS5 branch of BuildTransport
case TypeSOCKS5:
	auth := &proxy.Auth{User: profile.Username, Password: profile.Password}
	if profile.Username == "" {
		auth = nil
	}
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", profile.Host, profile.Port), auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("socks5 dialer: %w", err)
	}
	return &http.Transport{
		Dial:            dialer.Dial,
		DialContext:     dialer.(proxy.ContextDialer).DialContext,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}, nil
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/proxy/ -run TestBuildTransportSOCKS5UsesRealDialer -v`
Expected: PASS

- [ ] **Step 6: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/proxy/transport.go go.mod go.sum internal/proxy/proxy_test.go
git commit -m "fix(proxy): real SOCKS5 dialer via golang.org/x/net/proxy

- http.ProxyURL does not dial SOCKS5 (stdlib limitation)
- Use proxy.SOCKS5 dialer, set Dial + DialContext
- Fixes audit finding S1.2

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Wire Audit Trail into Request Lifecycle

**Files:**
- Modify: `internal/api/dataplane.go` (add audit store, emit events)
- Modify: `internal/api/management.go` (emit CONFIG_CHANGED on mutations)
- Modify: `internal/api/router.go` (thread audit store)
- Modify: `cmd/proxygateway-api/main.go` (instantiate audit store, pass in)
- Test: `internal/api/audit_integration_test.go` (create)

**Interfaces:**
- Consumes: `audit.Store` interface (`Log(ctx, Event)`, `List(ctx)`), `audit.NewMemoryStore()`
- Produces: `DataPlaneHandler` gains `auditStore audit.Store` field; `ManagementHandler` gains same. Event types as string constants: `AUTH_SUCCESS`, `AUTH_FAILURE`, `POLICY_DENY`, `RATE_LIMITED`, `ROUTE_RESOLVED`, `CONFIG_CHANGED` (spec FEAT-011 §2).

- [ ] **Step 1: Define event type constants**

```go
// internal/audit/audit.go — append
const (
	EventAuthSuccess   = "AUTH_SUCCESS"
	EventAuthFailure   = "AUTH_FAILURE"
	EventPolicyDeny    = "POLICY_DENY"
	EventRateLimited   = "RATE_LIMITED"
	EventRouteResolved = "ROUTE_RESOLVED"
	EventConfigChanged = "CONFIG_CHANGED"
)
```

- [ ] **Step 2: Write failing test — AUTH_FAILURE emitted on 401**

```go
// internal/api/audit_integration_test.go
package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

func TestAuditAuthFailureEmitted(t *testing.T) {
	auditStore := audit.NewMemoryStore()
	keyStore := auth.NewMemoryKeyStore()
	adapter := &ninerouter.MockAdapter{Models: []ninerouter.ModelInfo{}}

	dp := NewDataPlaneHandlerWithRouting(
		adapter, keyStore, policy.NewEngine(), limiter.NewMemoryLimiter(),
		routing.NewEngine(nil), slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	dp.auditStore = auditStore

	// No auth header -> 401 -> AUTH_FAILURE
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	dp.ListModels(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	events, _ := auditStore.List(context.Background())
	found := false
	for _, e := range events {
		if e.EventType == audit.EventAuthFailure {
			found = true
		}
	}
	if !found {
		t.Error("no AUTH_FAILURE audit event emitted")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestAuditAuthFailureEmitted -v`
Expected: FAIL — no audit event, no field `auditStore`

- [ ] **Step 4: Implement audit wiring in dataplane**

```go
// internal/api/dataplane.go
// Add field to struct:
auditStore audit.Store

// In constructors, accept audit store via a setter or new param:
func (h *DataPlaneHandler) SetAuditStore(s audit.Store) { h.auditStore = s }

func (h *DataPlaneHandler) logAudit(ctx context.Context, eventType, actor, target, status string, meta map[string]string) {
	if h.auditStore == nil {
		return
	}
	_ = h.auditStore.Log(ctx, audit.Event{
		ID:        api.GenerateAuditID(),
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		EventType: eventType,
		Target:    target,
		Status:    status,
		Metadata:  meta,
	})
}

// Call sites:
// ListModels 401 branch (dataplane.go:102):
h.logAudit(ctx, audit.EventAuthFailure, "unknown", "/v1/models", "unauthorized", nil)
// ChatCompletions policy deny (dataplane.go:189-196):
h.logAudit(ctx, audit.EventPolicyDeny, keyIDOrUnknown(r), "chat.completions", "forbidden",
	map[string]string{"model": targetModel})
// Rate limit 429 (dataplane.go:198-207):
h.logAudit(ctx, audit.EventRateLimited, keyIDOrUnknown(r), "chat.completions", "rate_limited",
	map[string]string{"limit": "rpm"})
// Route resolved (after routingEngine.Resolve):
h.logAudit(ctx, audit.EventRouteResolved, keyIDOrUnknown(r), targetModel, "resolved",
	map[string]string{"requested": requestedModel, "strategy": decision.Reason})
```

- [ ] **Step 5: Add audit ID generator**

```go
// internal/api/audit.go (new file in package api)
package api

import "crypto/rand"
import "encoding/hex"

func GenerateAuditID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestAuditAuthFailureEmitted -v`
Expected: PASS

- [ ] **Step 7: Wire audit store through main + router**

```go
// cmd/proxygateway-api/main.go
auditStore := audit.NewMemoryStore()
dpHandler.SetAuditStore(auditStore)
mgmtHandler.SetAuditStore(auditStore)
```

- [ ] **Step 8: Emit CONFIG_CHANGED on management mutations**

```go
// internal/api/management.go — in each mutation handler
h.logAudit(ctx, audit.EventConfigChanged, "admin", path, "changed",
	map[string]string{"resource": "proxies"})
```

- [ ] **Step 9: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/audit/audit.go internal/api/dataplane.go internal/api/management.go internal/api/router.go cmd/proxygateway-api/main.go internal/api/audit_integration_test.go internal/api/audit.go
git commit -m "feat(observability): wire audit trail into request lifecycle

- Event types AUTH_SUCCESS/AUTH_FAILURE/POLICY_DENY/RATE_LIMITED/ROUTE_RESOLVED/CONFIG_CHANGED
- Emit audit events at auth, policy, rate-limit, route-resolve, config-change points
- Fixes audit finding S1.3 (zero production audit callers)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Honor AllowedOrigins in CORS (bonus from S3.8)

**Files:**
- Modify: `internal/api/middleware.go:90-108` (CORS uses cfg)
- Test: `internal/api/middleware_test.go` (append)

**Interfaces:**
- Consumes: `cfg.Admin.AllowedOrigins []string` — config defaults to `[]string{"*"}` (`config.go:86`); the audit flagged runbook `PG_ADMIN_ALLOWED_ORIGINS` never wired.
- Produces: CORS reflects the matching origin, or `*` when the list contains `*`; rejects non-listed origins (no ACAO header).

- [ ] **Step 1: Write failing test**

```go
// internal/api/middleware_test.go — append
func TestCORSAllowedOrigins(t *testing.T) {
	allowed := []string{"https://admin.example.com"}
	h := CORSMiddleware(allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Allowed origin
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Errorf("allowed origin: got %q", got)
	}

	// Disallowed origin — no ACAO header
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://evil.example.com")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin leaked: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestCORSAllowedOrigins -v`
Expected: FAIL — current code always sets `*`

- [ ] **Step 3: Implement origin check**

```go
// internal/api/middleware.go — replace CORSMiddleware body
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowAll(allowedOrigins) {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if originAllowed(allowedOrigins, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func allowAll(allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
	}
	return false
}

func originAllowed(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api/ -run TestCORSAllowedOrigins -v`
Expected: PASS

- [ ] **Step 5: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "fix(security): honor AllowedOrigins in CORS instead of hardcoded *

- Reflect matching origin, add Vary: Origin; reject non-listed origins
- Enables PG_ADMIN_ALLOWED_ORIGINS to take effect
- Fixes audit finding S3.8 CORS drift

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- S1.1 (auth mgmt API) → Task 1 ✓
- S1.2 (SOCKS5) → Task 2 ✓
- S1.3 (audit trail) → Task 3 ✓
- S3.8 (CORS origin) → Task 4 ✓
- PROMT §17 (no secret logging) → audit redaction already exists; new events pass only model/key metadata, never secrets ✓
- FEAT-007 auth/403 → Task 1 uses same OpenAI error schema as dataplane ✓

**Placeholder scan:** No TBD/TODO. All code blocks concrete. `keyIDOrUnknown` referenced in Task 3 Step 4 must be implemented — add to dataplane.go:

```go
func keyIDOrUnknown(r *http.Request) string {
	key := auth.GetAPIKey(r.Context())
	if key == nil {
		return "unknown"
	}
	return key.ID
}
```

**Type consistency:** `audit.Store.Log(ctx, Event)`, `audit.Event` fields, `audit.EventAuthFailure` constants — consistent across Task 3. `AdminAuthMiddleware(token) func(http.Handler) http.Handler` matches middleware chain pattern in `router.go:99-106`. `BuildTransport(profile)` signature unchanged. `CORSMiddleware(allowedOrigins)` signature unchanged.
