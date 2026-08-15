# Changelog — ProxyGateway Enterprise

All phases delivered 2026-08-14 via isolated `feat/phase-N-*` branches → PR → `main`. Full detail per phase: `docs/PHASE-*-COMPLETION-REPORT.md`; feature specs: `docs/features/FEAT-*.md`.

## [2.0] — 2026-08-14

### Phase 0 — Discovery & Baseline (PR #1)
- Inventory installed 9Router (`decolua/9router:latest`, container `rys-ninerouter`)
- Verified 9Router API connectivity & auth (`:20128/api/health`, `/v1/models` with Bearer)
- Established implementation gap matrix (`docs/gap-matrix.md`)

### Phase 1 — Platform Foundation & Data Model (PR #1)
- Go module, structured logging, PostgreSQL 16 & Redis 7 pools
- SQL migrations, health probes (`/health/live`, `/health/ready`)

### Phase 2 — 9Router Adapter & Integration (PR #2)
- `NineRouterPort` interface, live contract tests, header isolation
- Compatibility matrix (`docs/api/9router-compatibility.md`)

### Phase 3 — Data Plane Core, OpenAI-Compatible (PR #3)
- `POST /v1/chat/completions` with JSON payload + real-time SSE streaming
- `GET /v1/models`

### Phase 4 — Policy Plane, Auth & Rate Limits (PR #4)
- SHA-256 API key hashing
- `PolicyEngine`, precedence `Global Deny > Per-Key Deny > Per-Key Allow`
- Sliding-window RPM/RPS limiter + token quota, HTTP 429 backoff

### Phase 5 — Routing Engine & Circuit Breaker (PR #5)
- Model aliases (`coding`, `fast`, `reasoning`, `cheap`, `free`)
- `CircuitBreaker` (CLOSED / OPEN / HALF_OPEN) + fallback routing

### Phase 6 — Outbound Proxy Management (PR #6)
- Egress profiles `DIRECT` / `HTTP` / `HTTPS` / `SOCKS5`
- Strict credential redaction (`json:"-"`)

### Phase 7 — Administrative Terminal UI (PR #7)
- Bubble Tea & Lip Gloss 12-screen keyboard-first dashboard
- Management API (`/api/v1/*`)

### Phase 8 — Observability & Audit Trail (PR #8)
- Prometheus exporter (`/metrics`), latency histograms, token counters
- Structured audit logger

### Phase 9 — Deployment & Operations (PR #9)
- Docker Compose, multi-stage builds
- Operational runbooks (`docs/operations/`)

### Phase 10 — Hardening & Benchmarks (PR #10)
- Security hardening test suite (`tests/security/`)
- Microbenchmarks: Policy ~46ns, Route ~1µs (`docs/benchmarks/benchmark-report.md`)

### Phase 11 — Final Polish & Clean Handover (PR #11)
- Planning docs relocated to `docs/specs/`
- Enterprise `README.md`, `docs/user-guide.md`

### Phase 12 — Shared Network Integration (PR #12)
- Seamless Docker DNS resolution to `rys-centranity_default` (9Router upstream)

---

## Unreleased / Working Tree

### Docker Hardening
- `docker-compose.yml`: remove deprecated `version:`; `backend` network now `internal: true`; `mem_limit` on postgres (1g), redis (256m), api (512m); secrets `PG_DB_PASS`/`PG_ADMIN_TOKEN` now required (`${VAR:?}` fails fast); compose-level healthcheck on `proxygateway-api`.
- `.env.example`: mark `PG_DB_PASS`/`PG_ADMIN_TOKEN` REQUIRED.
- `tests/deployment/deployment_test.go`: assert hardening contract (no `version:` line, `internal: true`, per-service `mem_limit`, no hardcoded secret defaults).

- `CLAUDE.md` + `AGENTS.md` repository guides added at repo root (adapted from global `/home/infinity/.claude/CLAUDE.md` + `/opt/rys-centranity/AGENTS.md`)

## Note
The 12-phase roadmap contract lives in `docs/specs/PROMT.md` (PG-CONTRACT-000 v2.1); master delivery scorecard in `docs/PROJECT-COMPLETION-REPORT.md`.
