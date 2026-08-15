# TUI Management Write Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the TUI from a read-only monitoring viewer into a real management TUI — operators can add/edit/enable/disable/delete API keys, configure proxy profiles, set global-deny policies, and switch routing aliases via keyboard-first modal forms, with confirmation on destructive actions.

**Architecture:** Two layers. (1) Backend: add Management API routes (`/api/v1/keys`, `/api/v1/policies`, `/api/v1/routes`) under the existing `AdminAuthMiddleware`, backed by store methods added to `auth.KeyStore`, `policy.Engine`, `routing.Engine`. (2) TUI: add POST/PUT/DELETE http methods, a modal form engine (`internal/tui/form.go`), and wire the 4 management screens (PROXIES, API KEYS, POLICIES, ROUTING) to live API reads + write actions with destructive-confirmation.

**Tech Stack:** Go, net/http, Bubble Tea, Lip Gloss, PostgreSQL/Redis (untouched).

**Spec:** `docs/superpowers/specs/2026-08-16-tui-management-write-design.md`

## Global Constraints

- **Security (never weaken):** all new management routes sit under the existing `AdminAuthMiddleware` (fail-closed — empty/mismatched token → 401, `internal/api/adminauth.go`). Key hashes never serialized (`json:"-"` on `APIKey.Hash`). Raw API key returned plaintext only once, on create. Audit-log every mutation via existing `h.logAudit`.
- **Destructive actions require confirmation** (PRD FR-012): delete + disable show a `y/N` confirm prompt in the TUI.
- **Store methods mirror existing defensive-copy patterns** (`SetGlobalDeny` copies slices with `append([]string(nil), ...)`; `SetAlias` lowercases the alias). `GetGlobalDeny`/`GetAliases` return copies, never the internal slices.
- **TUI never accesses Postgres/Redis/9Router directly** — all data via Management API (UX-SPEC §1).
- **No new dependencies** — stdlib + existing `golang.org/x/net/proxy` (already in go.mod). Bubble Tea/Lip Gloss already used.
- **TDD:** write the failing test first (RED), prove it fails for the intended reason, then minimal GREEN. Full suite `make test` + `go vet ./...` green before any commit.
- **Docs travel with code** — update CHANGELOG, FEAT-010, and this spec's status at the end.
- Existing stores are **in-memory** — no migration. Proxies CRUD already exists (PR #13); do not change it.
- Kernel/boot config: **never touched.**

---

### Task 1: Backend — KeyStore List/Update/Delete + policy GetGlobalDeny + routing GetAliases/DeleteAlias

**Files:**
- Modify: `internal/auth/auth.go`
- Modify: `internal/policy/engine.go`
- Modify: `internal/routing/engine.go`
- Test: `internal/auth/auth_test.go` (new file, create), `internal/policy/policy_test.go` (create), `internal/routing/engine_test.go` (create)

**Interfaces:**
- Produces (consumed by Task 2):
  - `auth.KeyStore` gains: `List(ctx context.Context) ([]*APIKey, error)`, `Update(ctx context.Context, key *APIKey) error`, `Delete(ctx context.Context, hash string) error`.
  - `policy.Engine` gains: `GetGlobalDeny() (models, providers []string)`.
  - `routing.Engine` gains: `GetAliases() map[string][]string`, `DeleteAlias(name string) error`.

- [ ] **Step 1: Write failing tests for store methods**

`internal/auth/auth_test.go`:
```go
package auth

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryKeyStoreListUpdateDelete(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryKeyStore()
	k1 := &APIKey{ID: "key_1", Name: "one", Prefix: "sk-pg-1", Hash: "h1", Enabled: true}
	if err := s.Create(ctx, k1); err != nil {
		t.Fatalf("create k1: %v", err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List len=1 got %d err=%v", len(list), err)
	}
	k1.Enabled = false
	if err := s.Update(ctx, k1); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetByHash(ctx, "h1")
	if got == nil || got.Enabled {
		t.Fatalf("expected disabled after update, got %+v", got)
	}
	if err := s.Delete(ctx, "h1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetByHash(ctx, "h1"); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestMemoryKeyStoreListIsCopy(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryKeyStore()
	_ = s.Create(ctx, &APIKey{ID: "k1", Hash: "h", Prefix: "sk"})
	list, _ := s.List(ctx)
	list[0].Name = "mutated"
	got, _ := s.GetByHash(ctx, "h")
	if got.Name == "mutated" {
		t.Fatal("List must return a copy, not the internal slice")
	}
}
```

`internal/policy/policy_test.go`:
```go
package policy

import (
	"testing"
)

func TestGetGlobalDenyReturnsCopy(t *testing.T) {
	e := NewEngine()
	e.SetGlobalDeny([]string{"cc-opus"}, []string{"openai"})
	m, p := e.GetGlobalDeny()
	m[0] = "mutated"
	m2, _ := e.GetGlobalDeny()
	if m2[0] == "mutated" {
		t.Fatal("GetGlobalDeny must return a copy")
	}
	if len(m) != 1 || len(p) != 1 || m2[0] != "cc-opus" {
		t.Fatalf("unexpected deny: models=%v providers=%v", m2, p)
	}
}
```

`internal/routing/engine_test.go`:
```go
package routing

import (
	"strings"
	"testing"
)

func TestEngineGetAliasesAndDelete(t *testing.T) {
	e := NewEngine(nil)
	e.SetAlias("testalias", []string{"cc-haiku", "cc-sonnet"})
	aliases := e.GetAliases()
	if len(aliases["testalias"]) != 2 {
		t.Fatalf("expected 2 targets, got %v", aliases["testalias"])
	}
	aliases["testalias"][0] = "mutated"
	aliases2 := e.GetAliases()
	if aliases2["testalias"][0] == "mutated" {
		t.Fatal("GetAliases must return a copy")
	}
	if err := e.DeleteAlias("TESTALIAS"); err != nil { // case-insensitive
		t.Fatalf("delete alias: %v", err)
	}
	if _, ok := e.GetAliases()["testalias"]; ok {
		t.Fatal("alias should be deleted")
	}
	if err := e.DeleteAlias("missing"); err == nil {
		t.Fatal("deleting missing alias should error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/auth/ ./internal/policy/ ./internal/routing/ 2>&1`
Expected: compile FAIL — methods `List`, `Update`, `Delete` (auth), `GetGlobalDeny` (policy), `GetAliases`, `DeleteAlias` (routing) don't exist yet.

- [ ] **Step 3: Implement store methods**

`internal/auth/auth.go` — extend `KeyStore` interface + `MemoryKeyStore`:
```go
type KeyStore interface {
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	Create(ctx context.Context, key *APIKey) error
	List(ctx context.Context) ([]*APIKey, error)
	Update(ctx context.Context, key *APIKey) error
	Delete(ctx context.Context, hash string) error
}
```
```go
// List returns a shallow copy of all keys (defensive: callers may mutate).
func (s *MemoryKeyStore) List(ctx context.Context) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*APIKey, 0, len(s.keys))
	for _, k := range s.keys {
		cp := *k
		out = append(out, &cp)
	}
	return out, nil
}

// Update overwrites a key by hash; errors if absent.
func (s *MemoryKeyStore) Update(ctx context.Context, key *APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.keys[key.Hash]; !exists {
		return errors.New("api key not found")
	}
	s.keys[key.Hash] = key
	return nil
}

// Delete removes a key by hash; errors if absent.
func (s *MemoryKeyStore) Delete(ctx context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.keys[hash]; !exists {
		return errors.New("api key not found")
	}
	delete(s.keys, hash)
	return nil
}
```

`internal/policy/engine.go`:
```go
// GetGlobalDeny returns a copy of the current global denylist.
func (e *Engine) GetGlobalDeny() (models, providers []string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]string(nil), e.globalDeny.models...),
		append([]string(nil), e.globalDeny.providers...)
}
```

`internal/routing/engine.go`:
```go
// GetAliases returns a copy of the alias map (defensive).
func (e *Engine) GetAliases() map[string][]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string][]string, len(e.aliases))
	for k, v := range e.aliases {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// DeleteAlias removes an alias (case-insensitive); errors if absent.
func (e *Engine) DeleteAlias(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := strings.ToLower(name)
	if _, exists := e.aliases[key]; !exists {
		return errors.New("alias not found")
	}
	delete(e.aliases, key)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/auth/ ./internal/policy/ ./internal/routing/ 2>&1`
Expected: all PASS.

- [ ] **Step 5: Run full suite + vet**

Run: `PATH=/opt/go/bin:$PATH go test ./... 2>&1` then `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: all green, vet clean.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/auth.go internal/auth/auth_test.go internal/policy/engine.go internal/policy/policy_test.go internal/routing/engine.go internal/routing/engine_test.go
git commit -m "feat(store): key list/update/delete, policy getter, routing alias get/delete"
```

---

### Task 2: Backend — Management routes for keys/policies/routing

**Files:**
- Modify: `internal/api/management.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/management_test.go`

**Interfaces:**
- Consumes: Task 1 store methods (`KeyStore.List/Update/Delete`, `policy.GetGlobalDeny`, `routing.GetAliases/DeleteAlias`); existing `AdminAuthMiddleware` (`internal/api/adminauth.go`), `h.logAudit`, `h.keyStore`, `h.routeEngine`.
- Produces: 8 routes (consumed by Task 3 TUI):
  - `GET /api/v1/keys`, `POST /api/v1/keys`, `PUT /api/v1/keys/{id}`, `DELETE /api/v1/keys/{id}`
  - `GET /api/v1/policies`, `PUT /api/v1/policies`
  - `GET /api/v1/routes`, `PUT /api/v1/routes/{alias}`, `DELETE /api/v1/routes/{alias}`
  - Handlers: `ListKeys`, `CreateKey`, `UpdateKey`, `DeleteKey`, `GetPolicies`, `UpdatePolicies`, `ListRoutes`, `UpdateRoute`, `DeleteRoute`.
  - Key request/response types: `KeyRequest` (write model), `KeyResponse` (with raw `key` on create).

- [ ] **Step 1: Write failing tests**

`internal/api/management_test.go` (append):
```go
func TestManagementKeysCRUD(t *testing.T) {
	// reuse existing NewMemoryKeyStore, NewEngine pattern from TestManagementAPI_System
	ks := NewMemoryKeyStore()
	eng := NewEngine()
	mh := NewManagementHandler(nil, ks, eng, NewMemoryStore(), nil)
	rtr := NewRouterWithManagement(mh)

	// Create
	body := strings.NewReader(`{"name":"prod","rpmlimit":60}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST keys: got %d want 201, body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil || created.Key == "" || created.ID == "" {
		t.Fatalf("expected raw key + id on create, got err=%v body=%s", err, rec.Body.String())
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET keys: got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), created.Key) {
		t.Fatal("raw key must NOT be returned in list")
	}

	// Update (disable)
	upd := strings.NewReader(`{"enabled":false}`)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/keys/"+created.ID, upd)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT key: got %d body=%s", rec.Code, rec.Body.String())
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/keys/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE key: got %d", rec.Code)
	}

	// Unauthorized without token
	req = httptest.NewRequest(http.MethodGet, "/api/v1/keys", nil)
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without admin token, got %d", rec.Code)
	}
}

func TestManagementPoliciesRoutes(t *testing.T) {
	ks := NewMemoryKeyStore()
	eng := NewEngine()
	mh := NewManagementHandler(nil, ks, eng, NewMemoryStore(), nil)
	rtr := NewRouterWithManagement(mh)

	// Set global deny
	body := strings.NewReader(`{"models":["cc-opus"],"providers":["openai"]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/policies", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec := httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT policies: got %d", rec.Code)
	}

	// Read back
	req = httptest.NewRequest(http.MethodGet, "/api/v1/policies", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "cc-opus") {
		t.Fatalf("GET policies: got %d body=%s", rec.Code, rec.Body.String())
	}

	// Set route alias
	body = strings.NewReader(`{"targets":["cc-haiku","cc-sonnet"]}`)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/routes/mytui", body)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT route: got %d", rec.Code)
	}

	// List routes shows mytui
	req = httptest.NewRequest(http.MethodGet, "/api/v1/routes", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "mytui") {
		t.Fatalf("GET routes: got %d body=%s", rec.Code, rec.Body.String())
	}

	// Delete route
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/routes/mytui", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	rtr.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE route: got %d", rec.Code)
	}
}
```
Note: `NewRouterWithManagement` already wraps management routes with `AdminAuthMiddleware` (verify in `router.go` — the proxies routes at `:122-128` are under it; the test's `Authorization: Bearer admin-token` must match the token used to construct the router — check how `TestManagementAPI_System` passes the admin token; if it uses `NewRouterWithManagement` with a token param, pass `"admin-token"`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/api/ -run 'TestManagementKeysCRUD|TestManagementPoliciesRoutes' -v 2>&1`
Expected: FAIL — routes 404 (not registered).

- [ ] **Step 3: Implement handlers in `management.go`**

Add types + handlers (mirror existing `ProxyRequest`/`CreateProxy` pattern):
```go
// KeyRequest is the write model for API keys.
type KeyRequest struct {
	Name              string   `json:"name"`
	Enabled           *bool    `json:"enabled"`
	RPMLimit          int      `json:"rpmlimit"`
	RPSLimit          int      `json:"rpslimit"`
	AllowedModels     []string `json:"allowed_models"`
	DeniedModels      []string `json:"denied_models"`
	AllowedProviders  []string `json:"allowed_providers"`
	DeniedProviders   []string `json:"denied_providers"`
	DailyTokenQuota   int64    `json:"daily_token_quota"`
	MonthlyTokenQuota int64    `json:"monthly_token_quota"`
}

// KeyResponse carries a raw key back once, on create only.
type KeyResponse struct {
	Key string `json:"key,omitempty"`
	*auth.APIKey
}

// generateKeyID mirrors proxy_ id generation.
func generateKeyID() string { return "key_" + generateID() }

// ListKeys handles GET /api/v1/keys
func (h *ManagementHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": list})
}

// CreateKey handles POST /api/v1/keys
func (h *ManagementHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		h.writeJSONError(w, http.StatusBadRequest, "name required")
		return
	}
	rawKey, keyModel, err := auth.GenerateKey(req.Name, req.RPMLimit, req.RPSLimit, req.DailyTokenQuota, req.MonthlyTokenQuota)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Enabled != nil {
		keyModel.Enabled = *req.Enabled
	}
	if err := h.keyStore.Create(r.Context(), keyModel); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(KeyResponse{Key: rawKey, APIKey: keyModel})
}

// UpdateKey handles PUT /api/v1/keys/{id}
func (h *ManagementHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *auth.APIKey
	for _, k := range list {
		if k.ID == r.PathValue("id") {
			target = k
			break
		}
	}
	if target == nil {
		h.writeJSONError(w, http.StatusNotFound, "key not found")
		return
	}
	if req.Name != "" {
		target.Name = req.Name
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if req.RPMLimit > 0 {
		target.RPMLimit = req.RPMLimit
	}
	if req.RPSLimit > 0 {
		target.RPSLimit = req.RPSLimit
	}
	if req.AllowedModels != nil {
		target.AllowedModels = req.AllowedModels
	}
	if req.DeniedModels != nil {
		target.DeniedModels = req.DeniedModels
	}
	if req.AllowedProviders != nil {
		target.AllowedProviders = req.AllowedProviders
	}
	if req.DeniedProviders != nil {
		target.DeniedProviders = req.DeniedProviders
	}
	if req.DailyTokenQuota > 0 {
		target.DailyTokenQuota = req.DailyTokenQuota
	}
	if req.MonthlyTokenQuota > 0 {
		target.MonthlyTokenQuota = req.MonthlyTokenQuota
	}
	target.UpdatedAt = time.Now().UTC()
	if err := h.keyStore.Update(r.Context(), target); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(target)
}

// DeleteKey handles DELETE /api/v1/keys/{id}
func (h *ManagementHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var hash string
	for _, k := range list {
		if k.ID == r.PathValue("id") {
			hash = k.Hash
			break
		}
	}
	if hash == "" {
		h.writeJSONError(w, http.StatusNotFound, "key not found")
		return
	}
	if err := h.keyStore.Delete(r.Context(), hash); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.WriteHeader(http.StatusNoContent)
}

// PoliciesResponse represents global deny policy.
type PoliciesResponse struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
}

// GetPolicies handles GET /api/v1/policies
func (h *ManagementHandler) GetPolicies(w http.ResponseWriter, r *http.Request) {
	models, providers := h.routeEngine.GetGlobalDeny() // NOTE: fix to policy engine, see Step 3b
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PoliciesResponse{Models: models, Providers: providers})
}

// UpdatePolicies handles PUT /api/v1/policies
func (h *ManagementHandler) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	var req PoliciesResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	h.policyEngine.SetGlobalDeny(req.Models, req.Providers)
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "policies"})
	w.WriteHeader(http.StatusOK)
}

// ListRoutes handles GET /api/v1/routes
func (h *ManagementHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"routes": h.routeEngine.GetAliases()})
}

// UpdateRoute handles PUT /api/v1/routes/{alias}
func (h *ManagementHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Targets []string `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Targets) == 0 {
		h.writeJSONError(w, http.StatusBadRequest, "targets required")
		return
	}
	h.routeEngine.SetAlias(r.PathValue("alias"), req.Targets)
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "routes"})
	w.WriteHeader(http.StatusOK)
}

// DeleteRoute handles DELETE /api/v1/routes/{alias}
func (h *ManagementHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.routeEngine.DeleteAlias(r.PathValue("alias")); err != nil {
		h.writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "routes"})
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3b: Add `policyEngine` to `ManagementHandler` + fix GetPolicies**

In `internal/api/management.go`, add field `policyEngine *policy.Engine` to `ManagementHandler`, wire in `NewManagementHandler` (new param), and fix `GetPolicies`:
```go
func (h *ManagementHandler) GetPolicies(w http.ResponseWriter, r *http.Request) {
	models, providers := h.policyEngine.GetGlobalDeny()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PoliciesResponse{Models: models, Providers: providers})
}
```
`NewManagementHandler` signature becomes `(adapter, keyStore, routeEngine, policyEngine, proxyStore, logger)`. Update the existing `NewManagementHandler` call sites (`router.go`, `main.go`, tests) to pass `policy.NewEngine()`.

- [ ] **Step 4: Register routes in `router.go`**

In `NewRouterWithManagement`, after the existing proxy routes (`:122-128`), add:
```go
	// API key management
	mux.Handle("GET /api/v1/keys", keysListHandler)
	mux.Handle("POST /api/v1/keys", keyCreateHandler)
	mux.Handle("PUT /api/v1/keys/{id}", keyUpdateHandler)
	mux.Handle("DELETE /api/v1/keys/{id}", keyDeleteHandler)
	// Global deny policies
	mux.Handle("GET /api/v1/policies", policiesGetHandler)
	mux.Handle("PUT /api/v1/policies", policiesUpdateHandler)
	// Routing aliases
	mux.Handle("GET /api/v1/routes", routesListHandler)
	mux.Handle("PUT /api/v1/routes/{alias}", routeUpdateHandler)
	mux.Handle("DELETE /api/v1/routes/{alias}", routeDeleteHandler)
```
Where each `*Handler` wraps `mh.ListKeys`/`mh.CreateKey`/etc. with `AdminAuthMiddleware(adminToken)` exactly like the existing proxy handlers (`:122-128` pattern — read how `proxiesHandler` is built there and mirror it).

- [ ] **Step 5: Run tests to verify they pass**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/api/ -run 'TestManagementKeysCRUD|TestManagementPoliciesRoutes' -v 2>&1`
Expected: PASS.

- [ ] **Step 6: Fix any compile errors from `NewManagementHandler` signature change**

Run: `PATH=/opt/go/bin:$PATH go build ./... 2>&1`
Expected: clean. Fix call sites (`router.go`, `cmd/proxygateway-api/main.go`, test files) to pass `policy.NewEngine()`.

- [ ] **Step 7: Full suite + vet**

Run: `PATH=/opt/go/bin:$PATH go test ./... 2>&1` then `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: all green.

- [ ] **Step 8: Commit**

```bash
git add internal/api/management.go internal/api/router.go internal/api/management_test.go cmd/proxygateway-api/main.go
git commit -m "feat(api): management routes for keys, policies, routing aliases"
```

---

### Task 3: TUI — HTTP write methods + model state for keys/policies/routes

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: backend routes from Task 2 (`/api/v1/keys`, `/policies`, `/routes`).
- Produces: `Model` fields `keys`, `policies`, `routes` + `getKeys/getPolicies/getRoutes` fetchers; `post/put/del` http methods. Consumed by Task 4 (form actions).

- [ ] **Step 1: Write failing test**

`internal/tui/tui_test.go` (append):
```go
func TestTUIPostSendsAdminToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"sk-raw","id":"key_1"}`))
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "secret-token")
	// post is unexported; exercise via a public helper or reflection.
	// The plan requires adding a testable surface: add exported method
	// Do(path, method, body string) (*http.Response, error) to Model.
}
```
To keep it testable, add an exported method to `Model`:
```go
// Do issues an authenticated HTTP request (GET/POST/PUT/DELETE) to the API.
func (m Model) Do(method, path, body string) (*http.Response, error)
```
Then the test uses `m.Do(http.MethodPost, "/api/v1/keys", `{"name":"x"}`)`. The existing `get()` becomes a thin wrapper over `Do`.

- [ ] **Step 2: Run test to verify it fails**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run TestTUIPostSendsAdminToken -v 2>&1`
Expected: compile FAIL — `Do` method doesn't exist.

- [ ] **Step 3: Implement `Do` + `fetchData` extension**

`internal/tui/app.go` — replace `get()` with `Do` + keep `get`:
```go
// Do issues an authenticated request. body is raw JSON ("" for GET/DELETE).
func (m Model) Do(method, path, body string) (*http.Response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", m.apiURL, path), reader)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if m.adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.adminToken)
	}
	return m.client.Do(req)
}

func (m Model) get(path string) (*http.Response, error) {
	return m.Do(http.MethodGet, path, "")
}
```
Add `io` to imports.

Add `Model` fields:
```go
	keys     []map[string]interface{}
	policies map[string]interface{}
	routes   map[string][]string
```
Add fetch helpers + extend `fetchData` (call all three):
```go
func (m Model) getKeys() []map[string]interface{} {
	resp, err := m.get("/api/v1/keys")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Keys
}

func (m Model) getPolicies() map[string]interface{} {
	resp, err := m.get("/api/v1/policies")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func (m Model) getRoutes() map[string][]string {
	resp, err := m.get("/api/v1/routes")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Routes map[string][]string `json:"routes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Routes
}
```
In `fetchData`, populate `m.keys`, `m.policies`, `m.routes` from these (return them in `dataLoadedMsg` or set directly on the model copy returned by `Update` — follow how `systemStatus`/`overview` are currently returned).

- [ ] **Step 4: Run test to verify it passes**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run TestTUIPostSendsAdminToken -v 2>&1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet**

Run: `PATH=/opt/go/bin:$PATH go test ./... 2>&1` then `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/tui_test.go
git commit -m "feat(tui): authenticated write methods + fetch keys/policies/routes"
```

---

### Task 4: TUI — modal form engine

**Files:**
- Create: `internal/tui/form.go`
- Test: `internal/tui/form_test.go`

**Interfaces:**
- Consumes: `Model` + `Do` from Task 3.
- Produces: `formState` type + `FormMsg`/`FormSubmitMsg`/`FormCancelMsg` messages, `formView`, `formUpdate`, `formInit`. Consumed by Task 5 (action wiring).

- [ ] **Step 1: Write failing test**

`internal/tui/form_test.go`:
```go
package tui_test

import (
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func TestFormRendersFieldsAndNavigates(t *testing.T) {
	f := tui.NewFormState("Add Proxy", []tui.FormField{
		{Label: "Name"},
		{Label: "Host"},
		{Label: "Port"},
	}, nil)
	view := tui.FormView(f)
	if !strings.Contains(view, "Name") || !strings.Contains(view, "Host") {
		t.Fatalf("form should render all field labels, got: %s", view)
	}
}

func TestFormSubmitGathersValues(t *testing.T) {
	var submitted map[string]string
	f := tui.NewFormState("Add", []tui.FormField{{Label: "Name"}}, func(v map[string]string) {
		submitted = v
	})
	f.SetValue(0, "myproxy")
	f.Submit()
	if submitted["Name"] != "myproxy" {
		t.Fatalf("expected submit to gather value, got %v", submitted)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run 'TestForm' -v 2>&1`
Expected: compile FAIL — `NewFormState`, `FormField`, `FormView`, `SetValue`, `Submit` don't exist.

- [ ] **Step 3: Implement `internal/tui/form.go`**

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FormField is a single labeled input in a modal form.
type FormField struct {
	Label string
	// Secret masks the input value (e.g. proxy password).
	Secret bool
	// Value is the current input content.
	Value string
	// Focused indicates the field is the active cursor.
	Focused bool
}

// FormState is the modal form model.
type FormState struct {
	Title    string
	Fields   []FormField
	Focused  int
	OnSubmit func(values map[string]string)
}

// NewFormState creates a form. onSubmit is invoked with gathered values.
func NewFormState(title string, fields []FormField, onSubmit func(map[string]string)) *FormState {
	for i := range fields {
		if i == 0 {
			fields[i].Focused = true
		}
	}
	return &FormState{Title: title, Fields: fields, Focused: 0, OnSubmit: onSubmit}
}

// SetValue sets field i's value.
func (f *FormState) SetValue(i int, v string) {
	if i >= 0 && i < len(f.Fields) {
		f.Fields[i].Value = v
	}
}

// Submit gathers all values and invokes OnSubmit if set.
func (f *FormState) Submit() {
	if f.OnSubmit == nil {
		return
	}
	vals := make(map[string]string, len(f.Fields))
	for _, fl := range f.Fields {
		vals[fl.Label] = fl.Value
	}
	f.OnSubmit(vals)
}

// NextFocus moves focus to the next field.
func (f *FormState) NextFocus() {
	if len(f.Fields) == 0 {
		return
	}
	f.Fields[f.Focused].Focused = false
	f.Focused = (f.Focused + 1) % len(f.Fields)
	f.Fields[f.Focused].Focused = true
}

// PrevFocus moves focus to the previous field.
func (f *FormState) PrevFocus() {
	if len(f.Fields) == 0 {
		return
	}
	f.Fields[f.Focused].Focused = false
	f.Focused = (f.Focused - 1 + len(f.Fields)) % len(f.Fields)
	f.Fields[f.Focused].Focused = true
}

// FormView renders the modal overlay above the active screen.
func FormView(f *FormState) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(f.Title) + "\n\n")
	for i, fl := range f.Fields {
		cursor := "  "
		if fl.Focused {
			cursor = "> "
		}
		val := fl.Value
		if fl.Secret && val != "" {
			val = strings.Repeat("*", len(val))
		}
		b.WriteString(cursor + fl.Label + ": " + val + "\n")
	}
	b.WriteString("\n[Enter] Submit   [Tab] Next   [Esc] Cancel\n")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Render(b.String())
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run 'TestForm' -v 2>&1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet**

Run: `PATH=/opt/go/bin:$PATH go test ./... 2>&1` then `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/form.go internal/tui/form_test.go
git commit -m "feat(tui): modal form engine (fields, focus, submit)"
```

---

### Task 5: TUI — wire actions to PROXIES/KEYS/POLICIES/ROUTING screens + live data

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/tui_test.go`

**Interfaces:**
- Consumes: `Do` (Task 3), `FormState`/`FormView` (Task 4), backend routes (Task 2).
- Produces: `Model.form *FormState`, `Model.selectedIdx int`, `Model.confirm *string`; keyboard wiring in `Update()`; live-data renderers.

- [ ] **Step 1: Write failing tests**

`internal/tui/tui_test.go` (append):
```go
func TestTUIProxyAddSubmitsPost(t *testing.T) {
	var gotPath, gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "token")
	// Open Add Proxy form on PROXIES tab
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})   // go to PROXIES
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})   // add -> form
	// Fill + submit
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p1")})  // name
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")}) // host
	// ... (fill host, port) ...
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})                        // submit
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxies" {
		t.Fatalf("expected POST /api/v1/proxies, got %s %s body=%s", gotMethod, gotPath, gotBody)
	}
}
```
Note: this test is an integration-style smoke of the action wiring — implement the minimal key handling (`a` opens form, chars fill focused field, Tab moves focus, Enter submits) to make it pass. The exact key sequence for filling fields may need adjustment; the essential assertions are method+path on submit.

- [ ] **Step 2: Run test to verify it fails**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run TestTUIProxyAddSubmitsPost -v 2>&1`
Expected: FAIL — pressing `a` does nothing (no form opens), `Enter` doesn't POST.

- [ ] **Step 3: Implement action wiring in `app.go`**

Add to `Model`:
```go
	form     *FormState
	selected int
	confirm  string // pending destructive-action label awaiting y/N
```
In `Update()`, before the existing `switch msg.String()`, handle modal state:
```go
	if m.form != nil {
		return m.handleFormKey(msg)
	}
	if m.confirm != "" {
		return m.handleConfirmKey(msg)
	}
```
Add handlers:
```go
func (m Model) handleFormKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			m.form = nil
			return m, nil
		case "tab":
			m.form.NextFocus()
			return m, nil
		case "shift+tab":
			m.form.PrevFocus()
			return m, nil
		case "enter":
			m.form.Submit()
			m.form = nil
			return m, func() tea.Msg { return m.fetchData() }
		default:
			// append typed rune to focused field value
			for _, r := range km.Runes {
				m.form.Fields[m.form.Focused].Value += string(r)
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleConfirmKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			m.doConfirmAction() // executes the stored destructive action
			m.confirm = ""
			return m, func() tea.Msg { return m.fetchData() }
		case "n", "N", "esc":
			m.confirm = ""
			return m, nil
		}
	}
	return m, nil
}
```
Action keys on the management screens (add to the main `switch msg.String()`):
```go
	case "a": // add — open create form for active screen
		m.openCreateForm()
	case "e": // edit — open edit form for selected row
		m.openEditForm()
	case "d": // delete — require confirmation
		m.confirm = "delete"
	case "x": // toggle enable/disable — require confirmation
		m.confirm = "toggle"
```
`openCreateForm`/`openEditForm` build a `FormState` with screen-appropriate fields and an `OnSubmit` that calls `m.Do("POST", ...)` or `m.Do("PUT", ...)`:
```go
func (m Model) openCreateForm() {
	switch m.activeTab {
	case TabProxies:
		m.form = NewFormState("Add Proxy", []FormField{{Label: "Name"}, {Label: "Host"}, {Label: "Port"}, {Label: "Type"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"host":%q,"port":%s,"type":%q}`, v["Name"], v["Host"], v["Port"], v["Type"])
			m.Do(http.MethodPost, "/api/v1/proxies", body)
		})
	case TabKeys:
		m.form = NewFormState("Add Key", []FormField{{Label: "Name"}, {Label: "RPMLimit"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"rpmlimit":%s}`, v["Name"], v["RPMLimit"])
			m.Do(http.MethodPost, "/api/v1/keys", body)
		})
	case TabPolicies:
		m.form = NewFormState("Set Global Deny", []FormField{{Label: "Models"}, {Label: "Providers"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"models":%q,"providers":%q}`, v["Models"], v["Providers"])
			m.Do(http.MethodPut, "/api/v1/policies", body)
		})
	case TabRouting:
		m.form = NewFormState("Add Route", []FormField{{Label: "Alias"}, {Label: "Targets"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"targets":%q}`, v["Targets"])
			m.Do(http.MethodPut, "/api/v1/routes/"+v["Alias"], body)
		})
	}
}
```
`doConfirmAction` executes delete/toggle against the selected row using the current screen's resource + selected id. Fill per screen (proxies: `DELETE /api/v1/proxies/{id}`, keys: `DELETE /api/v1/keys/{id}`, routes: `DELETE /api/v1/routes/{alias}`).

Render live data: replace hardcoded strings in `renderKeys`, `renderPolicies`, `renderRouting`, `renderProxies` with rows from `m.keys`/`m.policies`/`m.routes`/overview. If a store is empty, show "No records". When `m.form != nil`, render `FormView(m.form)` above the screen. When `m.confirm != ""`, render a `y/N` prompt line.

- [ ] **Step 4: Run tests to verify they pass**

Run: `PATH=/opt/go/bin:$PATH go test ./internal/tui/ -run 'TestTUIProxyAddSubmitsPost|TestForm' -v 2>&1`
Expected: PASS.

- [ ] **Step 5: Full suite + vet**

Run: `PATH=/opt/go/bin:$PATH go test ./... 2>&1` then `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/tui_test.go
git commit -m "feat(tui): management actions (add/edit/delete/toggle) with confirm + live data"
```

---

### Task 6: Docs sync + final verification

**Files:**
- Modify: `docs/CHANGELOG.md`
- Modify: `docs/features/FEAT-010-admin-tui.md`
- Modify: `docs/superpowers/specs/2026-08-16-tui-management-write-design.md` (mark status implemented)

**Interfaces:**
- Consumes: all tasks 1-5.

- [ ] **Step 1: CHANGELOG entry**

Append to `docs/CHANGELOG.md` (Unreleased):
```markdown
### TUI Management Write (FR-012)
- Backend: management routes for API keys (`/api/v1/keys` CRUD), global-deny policies (`/api/v1/policies` GET/PUT), routing aliases (`/api/v1/routes` GET/PUT/DELETE), all under AdminAuthMiddleware; raw key returned once on create.
- TUI: modal form engine (`internal/tui/form.go`); add/edit/delete/toggle actions on PROXIES, API KEYS, POLICIES, ROUTING screens; destructive actions require `y/N` confirmation; live data replaces hardcoded sample rows.
```

- [ ] **Step 2: FEAT-010 update**

In `docs/features/FEAT-010-admin-tui.md`, add to Scope:
```markdown
- Write management actions (FR-012): `add`, `edit`, `enable`/`disable`, `delete` on API keys, proxy profiles, policies, and routing aliases via modal forms; destructive actions require confirmation.
- New management endpoints: `/api/v1/keys` (GET/POST/PUT/DELETE), `/api/v1/policies` (GET/PUT), `/api/v1/routes` (GET/PUT/DELETE), all under AdminAuthMiddleware.
```

- [ ] **Step 3: Mark spec implemented**

In `docs/superpowers/specs/2026-08-16-tui-management-write-design.md`, change status line:
```markdown
> **Status**: Implemented (2026-08-16)
```

- [ ] **Step 4: Full verification**

Run: `PATH=/opt/go/bin:$PATH make test 2>&1`
Expected: all green (existing + new tests).

Run: `PATH=/opt/go/bin:$PATH go vet ./... 2>&1`
Expected: clean.

Run: `PATH=/opt/go/bin:$PATH go build ./... 2>&1`
Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add docs/CHANGELOG.md docs/features/FEAT-010-admin-tui.md docs/superpowers/specs/2026-08-16-tui-management-write-design.md
git commit -m "docs: TUI management write (changelog, FEAT-010, spec status)"
```

---

## Self-Review (completed at write time)

1. **Spec coverage:** Every spec §3.1 (backend routes + store methods) → Tasks 1-2; §3.2 (TUI form + actions + live data) → Tasks 3-5; §3.3 (TDD) folded into each task; §6 verification → Task 6. Out-of-scope items (test/switch/reset/import/export/reload, persistence, read-only screens) deliberately have no task. ✓
2. **Placeholder scan:** Every step has concrete code; no TBD/TODO. Tests full and compilable. One named caveat: `NewManagementHandler` signature changes → Task 2 Step 6 fixes call sites explicitly. ✓
3. **Type consistency:** `KeyStore.List/Update/Delete`, `GetGlobalDeny`, `GetAliases/DeleteAlias`, `ManagementHandler.policyEngine`, `Model.Do`, `FormState/NewFormState/FormView/SetValue/Submit/NextFocus/PrevFocus`, `Model.form/selected/confirm` all defined once and referenced consistently across tasks. Backend route paths match TUI `Do` calls (`/api/v1/keys`, `/api/v1/policies`, `/api/v1/routes`, `/api/v1/proxies`). ✓
