# TUI Management Write — Gap Closure Design

> **Date**: 2026-08-16
> **Status**: Draft — awaiting review
> **Related**: `06-TUI-UX-SPEC.md` (interaction model), `01-PRD.md` FR-012 (TUI admin actions), `02-ROADMAP.md` Phase 7, `FEAT-010-admin-tui.md`

## 1. Context & Gap

### Documentation requirement
- **PRD FR-012** (`01-PRD.md:283`): TUI screens + **Actions include: add, edit, enable, disable, test, switch, reset, import, export, reload. Destructive actions require confirmation.**
- **PRD `:410`**: "TUI can **view and modify** permitted configuration."
- **ROADMAP Phase 7 Exit Gate**: "All safe P0 management operations are executable from TUI and API."
- **UX-SPEC §1**: operator needs to "manage API keys and policies; configure proxy profiles; switch routing; execute safe operational actions."

### Current implementation (verified)
- **TUI** (`internal/tui/app.go`): **read-only**. Only `get()` (HTTP GET), `fetchData()`, `render*` screens. No write, no form/input. `Update()` = keyboard nav + refresh only.
- **Backend management API** (`internal/api/management.go`): **only proxies CRUD** (POST/PUT/DELETE `/api/v1/proxies`, from PR #13). No keys/policies/routing management routes.
- **Stores**: `KeyStore` (Create/GetByHash only), `policy.Engine.SetGlobalDeny`, `routing.Engine.SetAlias` — mutation methods exist but **no management routes** expose them.

### Gap
TUI is a **monitoring viewer**, not a **management TUI**. FR-012 actions (add/edit/enable/disable/switch/reset) are **not executable from TUI**. Backend lacks management routes for keys/policies/routing.

## 2. Goal
Make the TUI a real **management TUI**: operators can add/edit/enable/disable/delete API keys, configure proxy profiles, set global-deny policies, and switch routing aliases — through keyboard-first modal forms, with confirmation on destructive actions. All writes go through the Management API (never direct DB access, per UX-SPEC §1).

## 3. Design

### 3.1 Backend — new Management API routes (under existing `AdminAuthMiddleware`)

| Route | Method | Handler | Store call |
|:--|:--|:--|:--|
| `/api/v1/keys` | GET | `ListKeys` | `KeyStore.List` (new) |
| `/api/v1/keys` | POST | `CreateKey` | `KeyStore.Create` |
| `/api/v1/keys/{id}` | PUT | `UpdateKey` | `KeyStore.Update` (new) |
| `/api/v1/keys/{id}` | DELETE | `DeleteKey` | `KeyStore.Delete` (new) |
| `/api/v1/policies` | GET | `GetPolicies` | `policy.Engine.GetGlobalDeny` (new) |
| `/api/v1/policies` | PUT | `UpdatePolicies` | `policy.Engine.SetGlobalDeny` |
| `/api/v1/routes` | GET | `ListRoutes` | `routing.Engine.GetAliases` (new) |
| `/api/v1/routes/{alias}` | PUT | `UpdateRoute` | `routing.Engine.SetAlias` |
| `/api/v1/routes/{alias}` | DELETE | `DeleteRoute` | `routing.Engine.DeleteAlias` (new) |

**Store additions:**
- `internal/auth/auth.go`: `KeyStore` interface + `MemoryKeyStore` gains `List(ctx)`, `Update(ctx, key)`, `Delete(ctx, id)`. `Delete` sets `Enabled=false` (soft-disable) OR removes — **decide: soft-disable** (safe, reversible, matches "disable" action).
- `internal/policy/engine.go`: `GetGlobalDeny()` returns copy (matches existing defensive-copy in `SetGlobalDeny`).
- `internal/routing/engine.go`: `GetAliases()` returns copy; `DeleteAlias(name)`.

**Auth/security:** all routes under `AdminAuthMiddleware` (fail-closed, already in `router.go:122-128` pattern). Secrets: key hashes never serialized (`json:"-"` on `Hash`). Audit-log all mutations (existing `h.logAudit`).

**Key generation:** `POST /api/v1/keys` returns the raw key once (`KeyStore.Create` generates; store hash, return plaintext in response `Key` field `json:"key"` only on create).

### 3.2 TUI — management write + modal forms

**HTTP client** (`internal/tui/app.go`):
- Add `post(path, body)`, `put(path, body)`, `del(path)` — mirror existing `get()`, set `Authorization: Bearer m.adminToken` (already wired).

**Form modal** (new file `internal/tui/form.go`):
- `type formState struct { title string; fields []formField; focused int; values []string; onSubmit func(map[string]string) }`.
- `formField{ label string; secret bool }`.
- Renders overlay above active screen (Lip Gloss, per UX-SPEC palette).
- Keys: Tab/Esc navigate, Enter submit, Esc cancel.
- Validation: required fields non-empty (mirrors `management.go` CreateProxy validation).

**Actions per screen** (`internal/tui/app.go` `Update()` + renderers):
- **PROXIES** (`renderProxies`): `a` add, `e` edit selected, `d` delete (confirm), `x` toggle enable/disable.
- **API KEYS** (`renderKeys`): `a` add, `e` edit, `d` delete (confirm), `x` disable/enable.
- **POLICIES** (`renderPolicies`): `a`/`e` set global deny models/providers.
- **ROUTING** (`renderRouting`): `a`/`e` set alias targets, `d` delete alias (confirm).
- Selection state: track selected row per list screen (`selectedIdx`).
- **Confirmation** (FR-012): destructive actions (delete, disable) show confirm prompt `y/N`.

**Fetch:** `fetchData()` extended to also fetch keys, policies, routes (`GET /api/v1/keys`, `/policies`, `/routes`); store in Model; renderers read real data (replace hardcoded `CREATE_KEY` dummy rows in `renderAudit`/`renderRequests`).

### 3.3 Testing (TDD)
- **Backend**: `internal/api/management_test.go` — List/Create/Update/Delete keys, Get/Update policies, List/Update/Delete routes; admin-auth enforced (401 without token); audit-log fired on mutation; key hash never in response.
- **Store**: `internal/auth/auth_test.go`, `internal/routing/engine_test.go` — new methods.
- **TUI**: `internal/tui/tui_test.go` — form render, submit calls POST/PUT/DELETE (httptest server asserts method+path+auth header), confirmation blocks delete without `y`, Esc cancels.

## 4. Out of Scope (YAGNI)
- Full FR-012 actions `test/switch/reset/import/export/reload` — only where backend supports (no model-test endpoint yet). Add when backend exists.
- Persistence (Postgres/Redis) for keys/policies/routing — stores are in-memory (existing). Not changed.
- Read-only screens (Overview/Requests/Models/Providers/Usage/Audit/System/Settings) — remain read-only; only the 4 management screens get write.
- UX-SPEC visual polish beyond modal + palette consistency.

## 5. Rollback
Per-resource revert (git revert per commit). Backend routes added behind AdminAuthMiddleware — no data migration (in-memory stores). TUI modal is additive; screens unchanged when no modal open.

## 6. Verification
1. `make test` GREEN (new + existing).
2. `go vet ./...` clean.
3. `docker compose config` OK (no compose change expected).
4. Manual TUI smoke (user): PROXIES add/edit/delete proxy; KEYS add key (get raw key once); POLICIES set deny; ROUTING set alias. Confirm destructive prompts appear.
5. API contract: `curl` POST/PUT/DELETE `/api/v1/keys` with `Authorization: Bearer $PG_ADMIN_TOKEN` → 200/201/204; without → 401.
