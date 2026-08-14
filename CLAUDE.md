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

---

## MCP Tools & Skills (harness-wide)

> Same tooling contract as `/opt/rys-centranity`. All tools mounted in this harness; use them before default Read/Grep/Shell when they fit.

### Superpowers (process skills — use FIRST)
Invoke before any non-trivial work; if unavailable, run the equivalent local workflow.
- **brainstorming / spec** — new feature, building, changing behavior → frame requirements before implementation.
- **systematic-debugging** — any bug, test failure, or unexpected behavior → before proposing fixes.
- **test-driven-development** — any feature/bugfix → RED → GREEN → REFACTOR before implementation code.
- **verification-before-completion** — before claiming done/committing → run the verification and confirm output; evidence before assertions.

### Graphify (MACRO — blast radius)
Knowledge graph over the codebase (`graphify-out/graph.json`, indexed). Before reading any file in full, map which files a diff/change impacts.
- MCP: `graphify_query_graph` / `graphify_get_node` / `graphify_god_nodes` / `graphify_shortest_path` — pass `project_path: /opt/ai-proxy-centranity`.
- Shell: `rtk graphify query|path|explain|update <path>`.
- `update <path>` re-extracts on code changes; `extract <path> --code-only` headless AST index (no API key).

### CodeGraph (MICRO — surgical symbol)
SQLite symbol graph (`.codegraph/`, indexed). Pull ONLY the affected symbol + call path — do not read a whole file when a symbol suffices.
- MCP: `codegraph_explore` (verbatim source + call paths) — pass `projectPath`.
- Shell: `codegraph explore "<symbols>"` / `codegraph node <name>` / `codegraph sync`.

### AgentMemory (durable project memory)
- `memory_save` / `remember` — record insights, decisions, architecture, PR summaries after every merge.
- `memory_recall` / `recall` — before decisions, check what past sessions learned.
- `handoff` — resume the most recent session for this cwd, leading with any unanswered question.
- Post-merge retention is mandatory (AGENTS.md §7 / U2).

### Ponytail (implementation style — default full)
Lazy senior dev: YAGNI, reuse existing code, stdlib/native platform first, no speculative dependency, smallest safe diff, one runnable check for non-trivial logic. Never skip validation, security, error handling, accessibility, or user-requested scope in the name of minimalism. Mark deliberate shortcuts with `ponytail:` comments.
- `/ponytail lite|full|ultra` — `/ponytail-review`, `/ponytail-audit` for over-engineering sweeps.

### Caveman (communication — terse)
Drop articles/filler/pleasantries/hedging; fragments OK; technical terms exact; code/commits/security stay normal.
- Switch: `/caveman lite|full|ultra|wenyan`. Stop: `stop caveman` / `normal mode`.
- Compress context: `/caveman-compress`. Review diffs: `/caveman-review`. Commits: `/caveman-commit`.
- Auto-clarity: drop caveman for security warnings, irreversible actions, user confused. Resume after.

### RTK (Rust Token Killer — terminal filter)
Route every shell / git / build / test through `rtk` (60–90% fewer tokens). Never run a raw chatty command when an `rtk` filter exists. Full reference below. Global copy: `~/.claude/RTK.md`.

---

## RTK Command Reference

**Golden rule**: always prefix with `rtk` — even in `&&` chains. If no filter exists it passes through unchanged (safe always).

| Category | Commands | Typical savings |
|----------|----------|-----------------|
| Tests | `rtk go test` · `rtk vitest` · `rtk jest` · `rtk pytest` · `rtk playwright test` · `rtk test <cmd>` | 90–99% |
| Build | `rtk cargo build` · `rtk cargo check` · `rtk tsc` · `rtk lint` · `rtk next build` | 80–87% |
| Git | `rtk git status|log|diff|add|commit|push|pull|branch|fetch|stash|worktree|show` | 59–80% |
| GitHub | `rtk gh pr view|checks` · `rtk gh run list` · `rtk gh issue list` · `rtk gh api` | 26–87% |
| Package | `rtk pnpm list|outdated|install` · `rtk npm run <s>` · `rtk npx <cmd>` · `rtk prisma` | 70–90% |
| Files | `rtk ls` · `rtk read` · `rtk grep` · `rtk find` | 60–75% |
| Debug | `rtk err <cmd>` · `rtk log <f>` · `rtk json <f>` · `rtk deps` · `rtk env` · `rtk summary <cmd>` · `rtk diff` | 70–90% |
| Infra | `rtk docker ps|images|logs` · `rtk kubectl get|logs` | 85% |
| Network | `rtk curl` · `rtk wget` | 65–70% |
| Meta | `rtk gain` · `rtk gain --history` · `rtk discover` · `rtk proxy <cmd>` · `rtk init` | — |

---

## Development Methodology Defaults

For development and maintenance tasks, use available Superpowers/Ponytail capabilities automatically.
- If Superpowers skills are available, invoke the relevant skill before action: brainstorming/spec for new features, systematic-debugging for bugs, test-driven-development for non-trivial changes, verification-before-completion before final status.
- If Superpowers is unavailable, follow the equivalent local workflow instead of failing.
- Ponytail full is the default implementation style.
- Never skip required validation, security, error handling, accessibility, docs, or user-requested scope in the name of minimalism.

---

## Documentation Workflow (mandatory after every code-changing session)

```
1. Finish code changes
2. Update docs/CHANGELOG.md (every merged change)
3. Update matching docs/features/FEAT-*.md spec + phase completion report when behavior changes
4. New failure modes → add row to docs/operations/troubleshooting.md
5. Update this CLAUDE.md only when structure/commands/architecture change
6. git commit + push (docs always travel with code)
```

`docs/` is the canonical history of the codebase — see the index below. No Ruler here: CLAUDE.md/AGENTS.md are hand-maintained (unlike rys-centranity).

---

## Documentation Index (modular history)

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

Both indexes live: `.codegraph/` (codegraph) + `graphify-out/` (graphify). Query with explicit `projectPath`/`project_path: /opt/ai-proxy-centranity` — server default is `rys-centranity`. Indexes are gitignored; re-sync with `codegraph sync` / `graphify update .`.
