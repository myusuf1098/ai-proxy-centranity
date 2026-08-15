# Docker Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden `docker-compose.yml` + `.env.example` + deployment tests + ops docs for ProxyGateway Enterprise, matching the sibling rys-centranity stack pattern, without touching the locked Orange Pi 5 Max kernel.

**Architecture:** Single-host Docker Compose (v5.4.0), cgroup v2. Apply `mem_limit` (compose non-swarm — `deploy.resources` is ignored), `backend` network `internal: true`, required-secret env expansion `${VAR:?}` (no hardcoded secret defaults), remove deprecated `version:`, add compose-level API healthcheck. Mirror rys-centranity conventions. Docker daemon log rotation + weekly prune already server-wide.

**Tech Stack:** Docker Compose, Go 1.26 (tests), YAML.

**Spec:** `docs/superpowers/specs/2026-08-15-docker-hardening-design.md`

## Global Constraints

- **NEVER touch kernel/boot/initramfs/bootloader/DTB/apt kernel ops** — kernel `6.1.0-1025-rockchip` is apt-mark-held + unattended-upgrades-blacklisted (boot hang root cause). Docker/userland only.
- Host: Orange Pi 5 Max, Ubuntu 24.04 arm64, RK3588, 16GB RAM / 512GB disk. Compose v5.4.0, cgroup v2, systemd cgroup driver.
- Use `mem_limit` NOT `deploy.resources` — compose non-swarm ignores the latter.
- Secrets never hardcoded in compose: required vars use `${VAR:?}`. Non-secrets keep `:-default`.
- Keep 3-network design (frontend / backend / centranity_shared external → `rys-centranity_default`). Do not add/remove networks.
- Shared network `centranity_shared` must stay — it is the only link to 9Router upstream (`ninerouter:20128`).
- Every merge: `make test` (`go test -v -race ./...`) + `go vet ./...` GREEN. Docs travel with code.

---

### Task 1: Harden docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

**Interfaces:**
- Consumes: nothing (standalone config file).
- Produces: `docker-compose.yml` with `mem_limit` on all 3 services, `backend` `internal: true`, `${PG_DB_PASS:?...}` / `${PG_ADMIN_TOKEN:?...}`, no `version:` line, compose-level API healthcheck. Later tasks read/assert this file.

- [ ] **Step 1: Read current compose to confirm baseline**

Run: `cat docker-compose.yml`
Confirm: contains `version: '3.8'`, `backend: internal: false`, `PG_DB_PASS:-pg_centranity_secure_2026`, `PG_ADMIN_TOKEN:-pg_admin_centranity_token_2026`, no `mem_limit`.

- [ ] **Step 2: Remove `version: '3.8'` line (first line)**

Delete line 1 (`version: '3.8'`).

- [ ] **Step 3: Add required-var expansion for secrets**

In `postgres` `environment`, change:
```yaml
POSTGRES_PASSWORD: ${PG_DB_PASS:-pg_centranity_secure_2026}
```
to:
```yaml
POSTGRES_PASSWORD: ${PG_DB_PASS:?set PG_DB_PASS in .env}
```

In `proxygateway-api` `environment`, change:
```yaml
- PG_ADMIN_TOKEN=${PG_ADMIN_TOKEN:-pg_admin_centranity_token_2026}
```
to:
```yaml
- PG_ADMIN_TOKEN=${PG_ADMIN_TOKEN:?set PG_ADMIN_TOKEN in .env}
```

Also update the two `PG_DATABASE_URL` values that embed the default password:
```yaml
PG_DATABASE_URL=postgres://${PG_DB_USER:-proxygateway}:${PG_DB_PASS:-pg_centranity_secure_2026}@postgres:5432/${PG_DB_NAME:-proxygateway}?sslmode=disable
```
→
```yaml
PG_DATABASE_URL=postgres://${PG_DB_USER:-proxygateway}:${PG_DB_PASS:?set PG_DB_PASS in .env}@postgres:5432/${PG_DB_NAME:-proxygateway}?sslmode=disable
```

Leave `PG_DB_USER`/`PG_DB_NAME` with `:-` defaults (non-secret).

- [ ] **Step 4: Set `backend` network internal**

Change:
```yaml
  backend:
    internal: false
    driver: bridge
```
to:
```yaml
  backend:
    internal: true
    driver: bridge
```

- [ ] **Step 5: Add `mem_limit` to all services** (top-level service key, sibling of `restart:`)

- `postgres`: add `mem_limit: 1g`
- `redis`: add `mem_limit: 256m`
- `proxygateway-api`: add `mem_limit: 512m`

- [ ] **Step 6: Add compose-level healthcheck on `proxygateway-api`**

Add under `proxygateway-api` service (alongside `restart:`), matching Dockerfile probe:
```yaml
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8088/health/live"]
      interval: 15s
      timeout: 5s
      retries: 3
      start_period: 10s
```

- [ ] **Step 7: Validate compose parses**

Run: `docker compose config --quiet`
Expected: exit 0, no output, no `version` warning. If it errors on `${...:?}` missing vars, that is EXPECTED when `.env` lacks them — re-run with vars:
Run: `PG_DB_PASS=x PG_ADMIN_TOKEN=y docker compose config --quiet`
Expected: exit 0.

- [ ] **Step 8: Confirm no secret default remains**

Run: `grep -n "pg_centranity_secure_2026\|pg_admin_centranity_token_2026" docker-compose.yml`
Expected: no matches.

- [ ] **Step 9: Commit**

```bash
git add docker-compose.yml
git commit -m "feat(deploy): harden compose — mem_limit, internal backend, required secrets"
```

---

### Task 2: Update .env.example contract

**Files:**
- Modify: `.env.example`

**Interfaces:**
- Consumes: nothing.
- Produces: `.env.example` documenting required vars. Task 1's `${VAR:?}` now fail fast; `.env.example` is the source of truth for what must be set.

- [ ] **Step 1: Mark required secrets**

In `.env.example`, update `PG_DB_PASS` line:
```bash
PG_DB_PASS=pg_centranity_secure_2026
```
→
```bash
# REQUIRED — compose fails if unset. Replace with a strong unique value.
PG_DB_PASS=
```

Update `PG_ADMIN_TOKEN` line:
```bash
PG_ADMIN_TOKEN=pg_admin_centranity_token_2026
```
→
```bash
# REQUIRED — compose fails if unset. Replace with a strong unique value.
PG_ADMIN_TOKEN=
```

Also update the `PG_DATABASE_URL` example to keep it consistent (placeholder only):
```bash
PG_DATABASE_URL=postgres://proxygateway:CHANGE_ME@localhost:5432/proxygateway?sslmode=disable
```

- [ ] **Step 2: Ensure `.env` is gitignored (do not commit it)**

Run: `grep -n "^\.env$" .gitignore`
Expected: match. If absent, add `.env` line to `.gitignore`.

- [ ] **Step 3: Commit**

```bash
git add .env.example .gitignore
git commit -m "chore(env): document required secrets, keep .env ignored"
```

---

### Task 3: Extend deployment tests (RED → GREEN)

**Files:**
- Modify: `tests/deployment/deployment_test.go`
- Test: `tests/deployment/deployment_test.go` (same file)

**Interfaces:**
- Consumes: `docker-compose.yml` shape produced by Task 1.
- Produces: new assertions in `TestDeployment_DockerComposeArtifacts` guarding hardening contract. `make test` must pass.

- [ ] **Step 1: Write the failing test additions**

In `tests/deployment/deployment_test.go`, replace `TestDeployment_DockerComposeArtifacts` body with hardening asserts:

```go
func TestDeployment_DockerComposeArtifacts(t *testing.T) {
	composeContent, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatalf("failed to read docker-compose.yml: %v", err)
	}

	content := string(composeContent)
	requiredServices := []string{"proxygateway-api", "postgres", "redis"}
	for _, svc := range requiredServices {
		if !strings.Contains(content, svc) {
			t.Errorf("docker-compose.yml missing expected service: %s", svc)
		}
	}

	if strings.HasPrefix(content, "version:") {
		t.Errorf("docker-compose.yml must not declare a deprecated top-level version")
	}

	// backend network must be internal (postgres/redis have no host ingress)
	if !strings.Contains(content, "backend:") || !strings.Contains(content, "internal: true") {
		t.Errorf("docker-compose.yml must set backend network internal: true")
	}

	// every service must carry a mem_limit (compose non-swarm ignores deploy.resources)
	for _, svc := range []string{"postgres:", "redis:", "proxygateway-api:"} {
		idx := strings.Index(content, svc)
		if idx < 0 {
			t.Errorf("docker-compose.yml missing service block: %s", svc)
			continue
		}
		block := content[idx : idx+400]
		if !strings.Contains(block, "mem_limit:") {
			t.Errorf("service %s missing mem_limit", svc)
		}
	}

	// no hardcoded secret defaults may remain
	for _, secret := range []string{"pg_centranity_secure_2026", "pg_admin_centranity_token_2026"} {
		if strings.Contains(content, secret) {
			t.Errorf("docker-compose.yml contains hardcoded secret default: %s", secret)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `cd /opt/ai-proxy-centranity && rtk go test ./tests/deployment/ -run TestDeployment_DockerComposeArtifacts -v`
Expected: FAIL — asserts on `version:`, `internal: true`, `mem_limit`, secret defaults all fail against current compose (Task 1 not yet applied). Failure message must be the intended `t.Errorf` strings, NOT a compile/panic error.

- [ ] **Step 3: Apply Task 1 changes if not yet applied**

If Task 1 was already merged into this branch, skip — test should now pass. Otherwise apply Task 1 now (compose edits), then continue.

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `cd /opt/ai-proxy-centranity && rtk go test ./tests/deployment/ -run TestDeployment_DockerComposeArtifacts -v`
Expected: PASS.

- [ ] **Step 5: Run full suite**

Run: `cd /opt/ai-proxy-centranity && rtk go test -v -race ./...`
Expected: all PASS. If any unrelated test fails, investigate before committing (Strict Red-Repair Protocol).

- [ ] **Step 6: Run vet**

Run: `cd /opt/ai-proxy-centranity && go vet ./...`
Expected: no output (clean).

- [ ] **Step 7: Commit**

```bash
git add tests/deployment/deployment_test.go
git commit -m "test(deploy): assert compose hardening contract (mem_limit, internal backend, no secret defaults)"
```

---

### Task 4: Docs sync

**Files:**
- Modify: `docs/CHANGELOG.md`
- Modify: `docs/operations/deployment-runbook.md`
- Modify: `docs/operations/troubleshooting.md`

**Interfaces:**
- Consumes: Task 1/2 changes (compose + env contract).
- Produces: docs reflecting new behavior; troubleshooting row for new failure mode.

- [ ] **Step 1: Add CHANGELOG entry**

Append to `docs/CHANGELOG.md` (Unreleased section):
```markdown
## Unreleased
### Docker Hardening
- `docker-compose.yml`: remove deprecated `version:`; `backend` network now `internal: true`; `mem_limit` on postgres (1g), redis (256m), api (512m); secrets `PG_DB_PASS`/`PG_ADMIN_TOKEN` now required (`${VAR:?}` fails fast); compose-level healthcheck on `proxygateway-api`.
- `.env.example`: mark `PG_DB_PASS`/`PG_ADMIN_TOKEN` REQUIRED.
- `tests/deployment/deployment_test.go`: assert hardening contract (no `version:` line, `internal: true`, per-service `mem_limit`, no hardcoded secret defaults).
```

- [ ] **Step 2: Update deployment-runbook.md**

In `docs/operations/deployment-runbook.md`, under Section 2 (Environment Configuration), add:
```markdown
> `PG_DB_PASS` and `PG_ADMIN_TOKEN` are **required** — `docker compose up` fails fast if either is unset in `.env`.
```
Under Section 3.1, add note:
```markdown
> All services carry `mem_limit`; `postgres`/`redis` live on the `backend` network (`internal: true`), reachable only by compose peers, never from the host.
```

- [ ] **Step 3: Add troubleshooting row**

In `docs/operations/troubleshooting.md`, append to the diagnostic matrix:
```markdown
| **Compose fails with `PG_DB_PASS` / `PG_ADMIN_TOKEN` variable not set** | Missing required secret in `.env` | `grep -E 'PG_DB_PASS|PG_ADMIN_TOKEN' .env` | Set both in `.env` (`chmod 600 .env`) and re-run `docker compose up -d`. |
```

- [ ] **Step 4: Commit**

```bash
git add docs/CHANGELOG.md docs/operations/deployment-runbook.md docs/operations/troubleshooting.md
git commit -m "docs: sync docker hardening (changelog, runbook, troubleshooting)"
```

---

### Task 5: Full verification + branch merge prep

**Files:**
- None new. Verifies branch state.

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces: proof of GREEN before merge.

- [ ] **Step 1: Full test + vet**

Run: `cd /opt/ai-proxy-centranity && make test`
Expected: 100% GREEN (all packages pass, `-race` enabled).

Run: `cd /opt/ai-proxy-centranity && go vet ./...`
Expected: clean.

- [ ] **Step 2: Compose parse + secret sweep**

Run: `PG_DB_PASS=x PG_ADMIN_TOKEN=y docker compose config --quiet`
Expected: exit 0.

Run: `grep -rn "pg_centranity_secure_2026\|pg_admin_centranity_token_2026" docker-compose.yml .env.example`
Expected: no matches in either file (`.env.example` no longer carries the real defaults).

- [ ] **Step 3: Live deploy smoke (optional, only with user consent)**

If user approves: `docker compose up -d`, then `docker compose ps` (all healthy), `curl -s http://127.0.0.1:8088/health/ready` (HTTP 200). Do NOT restart containers without explicit user approval — this touches the production data plane.

- [ ] **Step 4: Report + handoff**

Summarize: files changed, test/vet GREEN output, compose validated, secret sweep clean. Note live-deploy smoke as optional/pending user approval.

---

## Self-Review (completed at write time)

1. **Spec coverage:** Every §3.1–3.4 item maps to a task: compose edits (T1), env contract (T2), test asserts (T3), docs (T4), verification (T5). §4 out-of-scope items (kernel, rys stack, daemon.json, swarm) deliberately have NO task. ✓
2. **Placeholder scan:** All steps carry concrete YAML/Go/docs content; no TBD/TODO. Test code is full and compiles. ✓
3. **Type consistency:** Service names `proxygateway-api`/`postgres`/`redis`, mem_limit values, and secret strings match across compose (T1), test asserts (T3), and docs (T4). `mem_limit` used everywhere — never `deploy.resources`. ✓
