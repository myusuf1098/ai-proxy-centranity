# ProxyGateway Enterprise — Implementation Gap Matrix

**Document ID:** `PG-GAP-000`  
**Date:** 2026-08-14  
**Status:** Baseline Established (Phase 0)  
**Applies To:** ProxyGateway Enterprise & 9Router Integration  

---

## 1. Domain Gap Analysis Matrix

| Domain | Specification (PRD / Architecture) | Existing State (VPS / Repo) | Gap | Required Action | Risk |
|---|---|---|---|---|---|
| **Product** | Full AI Proxy Gateway with Control Plane, Data Plane & TUI | PRD & Architecture docs created; no executable application yet | Full application implementation needed | Implement Go-based gateway and TUI | Low (Clear specifications) |
| **Architecture** | Control Plane + Data Plane + 9Router Adapter + Admin TUI | High-level architecture defined in `03-ARCHITECTURE.md` | Code architecture scaffolding missing | Create `cmd/` and `internal/` packages | Low |
| **Docker** | Isolated Docker Compose stack (API, TUI, Postgres, Redis) | Standalone 9Router container (`rys-ninerouter`) running on port 20128 | Compose file for ProxyGateway, Postgres & Redis missing | Build multi-stage Dockerfile & `docker-compose.yml` | Low |
| **Database** | PostgreSQL 16 as authoritative persistence store | PostgreSQL inactive on host OS | Database container & versioned schema migrations missing | Add Postgres container in Compose + migration engine | Medium (Schema design integrity) |
| **Cache / Rate Limit** | Redis 7 for distributed rate limiting & ephemeral locks | Redis inactive on host OS | Redis container & Redis client missing | Add Redis container in Compose + atomic limiter | Low |
| **API (Data Plane)** | OpenAI-compatible `POST /v1/chat/completions`, `GET /v1/models` with SSE streaming | None | Full Data Plane API required | Build HTTP handlers, streaming forwarder & policy gate | Medium (Streaming latency/cancellation) |
| **API (Control Plane)** | Management REST API `/api/v1/*` (providers, models, keys, policies, routes, proxies, audit) | None | Full Control Plane API required | Build REST endpoints with validation, auth, and audit | Low |
| **9Router** | Adapter interface (`NineRouterPort`) isolating upstream details | 9Router `decolua/9router:latest` active on `127.0.0.1:20128` | Go adapter client & contract tests missing | Implement `NineRouterHTTPAdapter` + contract test suite | Low (9Router verified running & healthy) |
| **Models** | Model registry, capability tags, context limits, alias mapping | 9Router exposes models (`cc-haiku`, `cc-sonnet`, `gemini...`) | Dynamic resolution & policy mapping missing | Implement Model Registry & Alias resolver in `internal/model` | Low |
| **Providers** | Provider registry, priority, timeout, proxy assignment | Upstream providers managed in 9Router | Provider metadata & failover logic missing | Implement Provider Registry in `internal/provider` | Low |
| **Routing** | Priority, weighted, lowest-latency, failover, 9Router combo | 9Router has internal combos | Policy-based route evaluator missing | Implement `RouteEngine` in `internal/routing` | Medium (Deterministic fallback) |
| **Policies** | Global, API Key, Model, Provider allow/deny precedence | None | Policy engine missing | Implement centralized `PolicyEngine` in `internal/policy` | Medium (Prevent bypass) |
| **API Keys** | Hashed API key storage, prefix matching, per-key allow/deny | None | Auth middleware & key store missing | Implement SHA-256 hashed API key management in `internal/auth` | High (Security-critical) |
| **Limits / Quotas** | RPS, RPM, TPM, concurrency, daily/monthly token quotas, budgets | None | Atomic limiters & quota tracking missing | Implement Redis token-bucket/sliding-window in `internal/limiter` & `internal/quota` | Medium (Race condition prevention) |
| **Proxy** | Outbound proxy profiles (DIRECT, HTTP, HTTPS, SOCKS5) with credential redaction | Basic env proxy on host | Proxy profile registry & secret protection missing | Implement Proxy profile store in `internal/proxy` with zero plaintext leakage | Medium |
| **Observability** | Prometheus `/metrics`, structured JSON logging (`slog`), health endpoints | Basic container logs | Metrics exporter & structured logger missing | Implement `internal/metrics`, `internal/health`, and `slog` wrapper | Low |
| **Security** | Non-root containers, hashed secrets, internal 9Router isolation, audit trail | Host hardened (UFW, key-only SSH) | Application-level audit logging & input validation missing | Implement `internal/audit` & container security parameters | High (Release gate) |
| **TUI** | Bubble Tea + Lip Gloss admin interface connecting to Management API | None | Full TUI client missing | Build 12 administrative TUI screens in `cmd/proxygateway-tui` | Medium (UX density & keyboard nav) |
| **Testing** | TDD Unit, Component, Contract, Integration, E2E, Security tests | None | Full test suite missing | Write unit tests, contract tests for 9Router, and integration tests | Low (TDD discipline) |
| **Documentation** | Traceable requirements (FR-XXX -> FEAT-XXX -> ADR-NNNN -> MIG-NNNN -> TEST-YYYYMMDD) | PRD, Roadmap, Architecture, Tech Stack, Refactory, TUI, Docs standards present | Implementation evidence & change logs pending | Maintain documentation synchronicity per phase | Low |

---

## 2. Baseline Conclusions & Action Plan

1. **9Router Runtime:** Verified running healthy at `http://127.0.0.1:20128`. API key authentication validated.
2. **Phase 1 Target:** Proceed with Go Module initialization (`go1.26`), directory layout creation, Docker Compose (PostgreSQL 16 + Redis 7), configuration loader, structured logging, health checks, and database migration scaffolding.
