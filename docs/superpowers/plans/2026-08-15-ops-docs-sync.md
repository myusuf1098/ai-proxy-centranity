# Operations & Docs Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close operations/doc-drift findings from the 2026-08-15 audit: complete proxy CRUD, add the missing TUI service to docker-compose, align runbook env var names with code, and sync the doc set so it reflects reality.

**Architecture:** Add DELETE + single-GET + create/update handlers to the proxy management API (backed by the existing `proxy.Store`); add a `proxygateway-tui` compose service; correct runbook/config var names; document the Redis persistence decision; update CHANGELOG.

**Tech Stack:** Go 1.26, `net/http`, Docker Compose v3.8, Markdown docs.

**Spec:** [FEAT-009 proxy CRUD](docs/features/FEAT-009-proxy-management.md), [FEAT-012 deployment](docs/features/FEAT-012-deployment-operations.md), [FEAT-014 handover](docs/features/FEAT-014-final-polish-handover.md), [deployment-runbook](docs/operations/deployment-runbook.md), [disaster-recovery](docs/operations/disaster-recovery.md), audit findings S2.6/S3.2/S3.3/S3.4/S3.8

## Global Constraints

- Proxy credentials (`Username`/`Password`) stay `json:"-"` — never in responses, logs, or audit (FEAT-009, PROMT §17)
- Compose changes must preserve the existing shared network with `rys-centranity_default` (Phase 12 contract)
- TUI service must not expose a public port (03-ARCHITECTURE §4: "No public listening port required")
- Doc changes must match code reality — no aspirational docs (PROMT §34: cross-reference before documenting)
- Every change updates `docs/CHANGELOG.md`

---

### Task 1: Complete Proxy Profile CRUD API

**Files:**
- Modify: `internal/api/management.go` (add Create/Update/Delete/GetById handlers)
- Modify: `internal/api/router.go` (register new routes)
- Modify: `internal/proxy/profile.go` (add Delete to Store interface + impl)
- Test: `internal/api/management_test.go` (append)

**Interfaces:**
- Consumes: `proxy.Store` (`Get`, `Save`, `List` — `profile.go:35-39`); `proxy.Profile` struct
- Produces:
  - `Store` gains `Delete(ctx, id) error`
  - Management routes: `POST /api/v1/proxies` (create), `PUT /api/v1/proxies/{id}` (update), `DELETE /api/v1/proxies/{id}` (delete), `GET /api/v1/proxies/{id}` (single)
  - Credential fields accepted in JSON **request** body but never echoed in **response**

- [ ] **Step 1: Add Delete to Store + failing test**

```go
// internal/proxy/profile.go — extend interface
type Store interface {
	Get(ctx context.Context, id string) (*Profile, error)
	Save(ctx context.Context, profile *Profile) error
	List(ctx context.Context) ([]*Profile, error)
	Delete(ctx context.Context, id string) error
}

// MemoryStore impl
func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[id]; !exists {
		return errors.New("proxy profile not found")
	}
	delete(s.profiles, id)
	return nil
}
```

```go
// internal/proxy/proxy_test.go — append
func TestMemoryStoreDelete(t *testing.T) {
	s := NewMemoryStore()
	p := &Profile{ID: "p1", Name: "HTTP-01", Type: TypeHTTP, Host: "h", Port: 8080}
	if err := s.Save(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(context.Background(), "p1"); err == nil {
		t.Fatal("expected not-found after delete")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy/ -run TestMemoryStoreDelete -v`
Expected: FAIL — `Store does not implement Delete`

- [ ] **Step 3: Write management handler tests**

```go
// internal/api/management_test.go — append
func TestProxyCRUD(t *testing.T) {
	store := proxy.NewMemoryStore()
	h := NewManagementHandler(nil, auth.NewMemoryKeyStore(), routing.NewEngine(nil), store, testLogger())
	router := http.NewServeMux()
	router.Handle("POST /api/v1/proxies", http.HandlerFunc(h.CreateProxy))
	router.Handle("GET /api/v1/proxies/{id}", http.HandlerFunc(h.GetProxy))
	router.Handle("DELETE /api/v1/proxies/{id}", http.HandlerFunc(h.DeleteProxy))

	// Create
	createBody := `{"name":"HTTP-01","type":"HTTP","host":"proxy.example","port":8080,"username":"u","password":"p"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxies", strings.NewReader(createBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", rec.Code)
	}
	// response must NOT contain password
	if strings.Contains(rec.Body.String(), "p") && strings.Contains(rec.Body.String(), "password") {
		t.Fatal("password leaked in create response")
	}

	// Single GET
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/proxies/p1", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", rec2.Code)
	}
	if strings.Contains(rec2.Body.String(), `"username"`) {
		t.Fatal("username/secret leaked in get response")
	}

	// Delete
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/proxies/p1", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", rec3.Code)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestProxyCRUD -v`
Expected: FAIL — `h.CreateProxy undefined`

- [ ] **Step 5: Implement management handlers**

```go
// internal/api/management.go — append
// ProxyRequest is the write model; credentials allowed on input only
type ProxyRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

func (h *ManagementHandler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Type == "" || req.Host == "" || req.Port <= 0 {
		h.writeJSONError(w, http.StatusBadRequest, "name, type, host, port required")
		return
	}
	profile := &proxy.Profile{
		ID:       "proxy_" + generateID(),
		Name:     req.Name,
		Type:     proxy.Type(req.Type),
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Enabled:  req.Enabled,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.proxyStore.Save(r.Context(), profile); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(profile) // json:"-" redacts creds
}

func (h *ManagementHandler) GetProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.proxyStore == nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy store unavailable")
		return
	}
	p, err := h.proxyStore.Get(r.Context(), id)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(p)
}

func (h *ManagementHandler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.proxyStore.Delete(r.Context(), id); err != nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ManagementHandler) writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

- [ ] **Step 6: Register routes**

```go
// internal/api/router.go — mgmt block, inside adminAuth wrapper
adminAuth := AdminAuthMiddleware(cfg.Admin.ManagementToken)
mux.Handle("GET /api/v1/system", adminAuth(http.HandlerFunc(mgmtHandler.GetSystem)))
mux.Handle("GET /api/v1/overview", adminAuth(http.HandlerFunc(mgmtHandler.GetOverview)))
mux.Handle("GET /api/v1/proxies", adminAuth(http.HandlerFunc(mgmtHandler.GetProxies)))
mux.Handle("POST /api/v1/proxies", adminAuth(http.HandlerFunc(mgmtHandler.CreateProxy)))
mux.Handle("GET /api/v1/proxies/{id}", adminAuth(http.HandlerFunc(mgmtHandler.GetProxy)))
mux.Handle("DELETE /api/v1/proxies/{id}", adminAuth(http.HandlerFunc(mgmtHandler.DeleteProxy)))
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test ./internal/proxy/ ./internal/api/ -run 'TestMemoryStoreDelete|TestProxyCRUD' -v`
Expected: PASS

- [ ] **Step 8: Run full suite**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/proxy/profile.go internal/api/management.go internal/api/router.go internal/proxy/proxy_test.go internal/api/management_test.go
git commit -m "feat(proxy): complete profile CRUD API

- Store.Delete + management handlers Create/GetById/Delete
- Credentials accepted on input, redacted on output (json:\"-\")
- Fixes audit finding S2.6 (proxy CRUD incomplete)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 2: Add TUI Service to Docker Compose

**Files:**
- Modify: `docker-compose.yml` (add proxygateway-tui service)
- Test: `tests/deployment/deployment_test.go` (append service check)

**Interfaces:**
- Consumes: `deployments/docker/Dockerfile.tui` (exists, `:1-27`)
- Produces: `proxygateway-tui` service on `frontend` network, `stdin_open: true` + `tty: true` (interactive TUI), no published port, connects to `http://proxygateway-api:8088`

- [ ] **Step 1: Add failing test for TUI service presence**

```go
// tests/deployment/deployment_test.go — append
func TestDeployment_HasTUIComposeService(t *testing.T) {
	data, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "proxygateway-tui:") {
		t.Fatal("docker-compose.yml missing proxygateway-tui service")
	}
	if !strings.Contains(s, "Dockerfile.tui") {
		t.Fatal("proxygateway-tui service must use Dockerfile.tui")
	}
	if strings.Contains(s, `"8089:8089"`) {
		t.Fatal("TUI must not publish a port")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./tests/deployment/ -run TestDeployment_HasTUIComposeService -v`
Expected: FAIL — `missing proxygateway-tui service`

- [ ] **Step 3: Add service to compose**

```yaml
  proxygateway-tui:
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile.tui
    container_name: proxygateway-tui
    restart: unless-stopped
    depends_on:
      proxygateway-api:
        condition: service_healthy
    environment:
      - PG_API_BASE_URL=http://proxygateway-api:8088
    networks:
      - frontend
    stdin_open: true
    tty: true
```

- [ ] **Step 4: Add healthcheck to API service (needed for depends_on condition)**

```yaml
# under proxygateway-api:
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1:8088/health/live || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 10s
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./tests/deployment/ -run TestDeployment_HasTUIComposeService -v`
Expected: PASS

- [ ] **Step 6: Validate compose syntax**

Run: `docker compose config`
Expected: PASS (no YAML/service errors)

- [ ] **Step 7: Commit**

```bash
git add docker-compose.yml tests/deployment/deployment_test.go
git commit -m "feat(deploy): add proxygateway-tui service to compose

- Interactive TUI container, no published port, on frontend network
- API healthcheck enables depends_on condition
- Fixes audit finding S3.2 (runbook references missing TUI service)

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Align Runbook Env Vars + Config Surface

**Files:**
- Modify: `docs/operations/deployment-runbook.md` (fix var names)
- Modify: `docs/operations/disaster-recovery.md` (Redis persistence reality)
- Modify: `.env.example` (add missing keys)
- Modify: `docs/operations/troubleshooting.md` (note admin token requirement)
- Test: `tests/deployment/deployment_test.go` (append env fallback check)

**Interfaces:**
- Consumes: actual env keys from `internal/config/config.go` (verified: `PG_DB_PASS` used in compose, `PG_NINEROUTER_URL`, `PG_DB_CONN_MAX_LIFETIME`, `PG_REDIS_DB`)
- Produces: docs reference real keys; `.env.example` covers full config surface

- [ ] **Step 1: Fix runbook var names**

In `docs/operations/deployment-runbook.md` §2, replace:
- `PG_DATABASE_PASSWORD=<strong-db-password>` → `PG_DB_PASS=<strong-db-password>`
- `PG_ADMIN_ALLOWED_ORIGINS=https://admin.yourdomain.com` → remove (CORS now honors `AllowedOrigins` from config; document that origins are compile-config or via `PG_ALLOWED_ORIGINS` if added later — for now note default `*`)
- `PG_NINEROUTER_BASE_URL` → `PG_NINEROUTER_URL`

- [ ] **Step 2: Fix disaster-recovery Redis reality**

In `docs/operations/disaster-recovery.md` §1.2, replace the AOF/RDB claim with:
```markdown
> **Note (2026-08-15):** Redis persistence is currently DISABLED in
> `docker-compose.yml` (`redis-server --save "" --appendonly no`). Rate-limit
> state is ephemeral by design (PROMT §17: do not back up transient Redis
> state as primary recovery). Re-enable AOF only if non-transient data moves
> to Redis.
```

- [ ] **Step 3: Add missing env keys to `.env.example`**

Append:
```bash
# Database pool tuning
PG_DB_CONN_MAX_LIFETIME=5m
# Redis logical DB
PG_REDIS_DB=0
```

- [ ] **Step 4: Update troubleshooting.md**

Add row to diagnostic matrix:
```
| `/api/v1/*` returns 401 | `PG_ADMIN_TOKEN` not set or mismatch | Set `PG_ADMIN_TOKEN` in `.env`; Management API fails closed |
```

- [ ] **Step 5: Add deployment test for env fallback parity**

```go
// tests/deployment/deployment_test.go — append
func TestDeployment_EnvExampleCoversConfigSurface(t *testing.T) {
	data, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, key := range []string{"PG_DB_CONN_MAX_LIFETIME", "PG_REDIS_DB", "PG_ADMIN_TOKEN", "PG_NINEROUTER_URL"} {
		if !strings.Contains(s, key) {
			t.Errorf(".env.example missing %s", key)
		}
	}
}
```

- [ ] **Step 6: Run deployment tests**

Run: `go test ./tests/deployment/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add docs/operations/deployment-runbook.md docs/operations/disaster-recovery.md docs/operations/troubleshooting.md .env.example tests/deployment/deployment_test.go
git commit -m "docs(ops): align runbook env vars + env surface with config

- Fix PG_DB_PASS / PG_NINEROUTER_URL names
- Document Redis persistence reality (disabled, ephemeral by design)
- Add missing .env.example keys, troubleshooting row
- Fixes audit findings S3.3/S3.8

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Sync Architecture Doc + CHANGELOG

**Files:**
- Modify: `docs/specs/03-ARCHITECTURE.md` (topology reality)
- Modify: `docs/CHANGELOG.md` (working-tree notes)

**Interfaces:**
- Consumes: actual compose services (postgres, redis, proxygateway-api, proxygateway-tui, networks: frontend/backend/centranity_shared)
- Produces: architecture doc reflects deployed reality; Prometheus/Grafana marked as future work

- [ ] **Step 1: Update architecture topology**

In `docs/specs/03-ARCHITECTURE.md` §3, annotate the topology diagram:
```markdown
> **Deployed reality (2026-08-15):** Compose ships `postgres`, `redis`,
> `proxygateway-api`, `proxygateway-tui`. `prometheus` + `grafana` are
> **planned, not yet deployed** — observability network deferred. Metrics
> endpoint `/metrics` is available on the API service.
```

- [ ] **Step 2: Update CHANGELOG**

Append under `## Unreleased / Working Tree`:
```markdown
- `feat(security)`: Admin token auth on `/api/v1/*` (fail-closed)
- `fix(proxy)`: real SOCKS5 dialer via `golang.org/x/net/proxy`
- `feat(observability)`: audit trail wired into request lifecycle; token/upstream-error counters increment
- `feat(routing)`: fallback chain executes on upstream 5xx
- `feat(quota)`: daily/monthly token quota enforcement
- `feat(limiter)`: RPS sliding-window enforcement
- `feat(proxy)`: complete profile CRUD API
- `feat(deploy)`: `proxygateway-tui` service in compose
- `fix(security)`: CORS honors `AllowedOrigins`
- `docs(ops)`: runbook env vars aligned, Redis persistence reality documented
```

- [ ] **Step 3: Commit**

```bash
git add docs/specs/03-ARCHITECTURE.md docs/CHANGELOG.md
git commit -m "docs: sync architecture reality + changelog

- Mark Prometheus/Grafana as planned, not deployed
- Record all working-tree changes in CHANGELOG
- Fixes audit findings S3.4

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- S2.6 (proxy CRUD) → Task 1 ✓
- S3.2 (TUI service) → Task 2 ✓
- S3.3 (runbook env names) + S3.8 (.env.example) → Task 3 ✓
- S3.4 (architecture topology) → Task 4 ✓
- FEAT-009 credential redaction → Task 1 uses `json:"-"` on Profile; request accepts creds, response strips ✓
- Phase 12 shared network preserved → Task 2 keeps `centranity_shared` on API service ✓

**Placeholder scan:** No TBD. `testLogger()` reused from other plans (must exist in package test helpers). `writeJSONError` defined in Task 1 Step 5. All code concrete.

**Type consistency:** `Store.Delete(ctx, id) error` matches existing `Get`/`Save`/`List` patterns. `ProxyRequest` used only in management.go. Routes use Go 1.22 `{id}` pattern matching existing `r.PathValue` usage. `generateID()` already exists in `middleware.go:120` (package api). Admin token auth from Plan 1 is a prerequisite — Task 1 Step 6 references `AdminAuthMiddleware`; **dependency: execute Plan 1 Task 1 first.**
