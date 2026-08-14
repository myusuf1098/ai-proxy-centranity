# ProxyGateway Enterprise — Repository Guide

OpenAI-compatible AI gateway & proxy orchestration platform built in Go.
Control Plane + Data Plane around 9Router (a policy/control layer, **does not fork 9Router**).

Tech: Go · net/http · Bubble Tea / Lip Gloss (TUI) · PostgreSQL 16 · Redis 7 · Prometheus · Docker Compose.
Shared Docker network with `rys-centranity` for the 9Router upstream.

## Commands

```bash
make build          # build bin/proxygateway-api + bin/proxygateway-tui
make test           # go test -v -race ./...
make run-api        # go run ./cmd/proxygateway-api
make run-tui        # go run ./cmd/proxygateway-tui
make docker-up      # docker compose up -d
cp .env.example .env
```

## Architecture

- `cmd/proxygateway-api` — HTTP server: Data Plane (`/v1/chat/completions`, `/v1/models`, `/metrics`) + Management API.
- `cmd/proxygateway-tui` — 12-screen keyboard-first admin dashboard (Bubble Tea).
- `internal/proxy` — request lifecycle, streaming (JSON + SSE).
- `internal/auth` — SHA-256 API key hashing.
- `internal/policy` — PolicyEngine, precedence `Global Deny > Per-Key Deny > Per-Key Allow`.
- `internal/limiter` — sliding-window RPM/RPS + token quota.
- `internal/routing` — model aliases (`coding`, `fast`, `reasoning`, `cheap`, `free`) + CircuitBreaker (CLOSED/OPEN/HALF_OPEN).
- `internal/ninerouter` — 9Router adapter, internal credential injection, header isolation.
- `internal/proxy` — outbound profiles DIRECT / HTTP / HTTPS / SOCKS5, credential redaction (`json:"-"`).
- `internal/store`, `internal/health`, `internal/telemetry`, `internal/audit`, `internal/config`, `internal/api`, `internal/tui`.
- `migrations/` — SQL schema. `deployments/docker/` — Docker assets.
- `tests/` — contract, security, deployment, benchmark suites.

## Environment

All config via env (`PG_*` prefix): server/metrics ports, timeouts, DB/Redis URLs, `PG_NINEROUTER_URL` + `PG_NINEROUTER_API_KEY`, `PG_ADMIN_TOKEN`. See `.env.example`.

## Norms

- **TDD**: Red-Green-Refactor. Feature work lands via isolated `feat/phase-N-*` branches and PRs (see git history). 23+ test files across `internal/` and `tests/`.
- **Ponytail**: smallest safe diff, YAGNI, stdlib first. Mark deliberate shortcuts with `ponytail:` comments.
- **Docs-first**: each phase ships a spec (`docs/features/FEAT-NNN-*.md`) before implementation, then a completion report. Keep that rhythm.
- **Security**: secrets masked in JSON, header isolation on upstream calls, audit logging on sensitive ops. Security review before merging auth/policy changes.

## Documentation Index (modular history)

`/docs/` is the canonical record of the whole codebase lifecycle. Read the relevant slice before touching a subsystem.

### Master contract & specs
| File | Purpose |
| :--- | :--- |
| `docs/specs/PROMT.md` | Master Project Contract (PG-CONTRACT-000 v2.1) — requirements, contract, acceptance |
| `docs/specs/01-PRD.md` | Product requirements |
| `docs/specs/02-ROADMAP.md` | 12-phase roadmap |
| `docs/specs/03-ARCHITECTURE.md` | Architecture: control/data plane, logical components |
| `docs/specs/04-TECH-STACK.md` | Technology stack + decisions |
| `docs/specs/05-REFACTORY-PLAN.md` | Refactor plan |
| `docs/specs/06-TUI-UX-SPEC.md` | TUI UX spec |
| `docs/specs/07-IMPLEMENTATION-DOCUMENTATION.md` | Implementation documentation |
| `docs/specs/templates/` | ADR / FEATURE / MIGRATION / TEST-RESULT / INCIDENT templates |

### Feature specs (phase-by-phase)
`docs/features/FEAT-004-9router-adapter.md` · `FEAT-006-data-plane.md` · `FEAT-007-policy-plane.md` · `FEAT-008-routing-engine.md` · `FEAT-009-proxy-management.md` · `FEAT-010-admin-tui.md` · `FEAT-011-observability.md` · `FEAT-012-deployment-operations.md` · `FEAT-013-hardening-benchmarking.md` · `FEAT-014-final-polish-handover.md`

### Changelog & history
`docs/CHANGELOG.md` — full version history (v2.0, Phase 0–12) + unreleased working-tree notes. **Update this file with every merged change.**

### Phase completion reports
`docs/PHASE-0-COMPLETION-REPORT.md` … `docs/PHASE-11-COMPLETION-REPORT.md`, plus `docs/PROJECT-COMPLETION-REPORT.md` (master scorecard), `docs/gap-matrix.md`, `docs/user-guide.md`.

### Security, benchmarks, operations, API
- `docs/security/security-audit-report.md`
- `docs/benchmarks/benchmark-report.md`
- `docs/operations/deployment-runbook.md` · `troubleshooting.md` (diagnostic matrix) · `disaster-recovery.md`
- `docs/api/9router-compatibility.md`

## Codegraph

If `.codegraph/` exists at repo root, prefer `codegraph_explore` before broad grep/read. (Not yet indexed — indexing is your call.)

## Shared tooling (harness-wide)

Superpowers (brainstorming, systematic-debugging, TDD, verification-before-completion), graphify/codegraph, agentmemory (recall/remember), ponytail. Global config in `/home/infinity/.claude/CLAUDE.md` + `RTK.md` (`rtk` token-saving prefix for git/test/docker).
