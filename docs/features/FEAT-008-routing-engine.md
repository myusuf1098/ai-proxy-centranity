# FEAT-008 — Intelligent Routing Engine, Model Aliases & Circuit Breaker

## Requirement Mapping
- **PRD:** FR-003 (Model Registry), FR-004 (Model Input & Switching), FR-006 (Routing Strategies), FR-010 (Failover & Circuit Breaker)
- **Roadmap Phase:** Phase 5 (Routing)
- **Architecture:** Section 6 (Request Flow), Section 9 (Model Resolution), Section 12 (Circuit Breaker)
- **Refactory Plan:** Section 6 (Routing Engine)

## Objective
Provide intelligent model alias resolution, priority-based failover, and circuit breaker protection. Clients can request symbolic aliases (e.g. `coding`, `fast`, `reasoning`, `cheap`), which the `RouteEngine` dynamically resolves to target models and upstream providers with automatic fallback if targets are degraded or circuit-broken.

## Scope
1. **Model Aliases (`internal/routing`)**:
   - Dynamic alias mapping registry: `alias -> []RouteTarget` (ordered by priority/weight).
   - Built-in aliases:
     - `coding` -> `["cc-sonnet", "cc-haiku"]`
     - `fast` -> `["cc-haiku", "gemini-flash"]`
     - `reasoning` -> `["cc-opus", "cc-sonnet"]`
     - `free` / `cheap` -> `["cc-haiku"]`
2. **Circuit Breaker (`internal/routing`)**:
   - States: `CLOSED` (normal), `OPEN` (quarantined after failure threshold), `HALF_OPEN` (testing recovery).
   - Configurable consecutive failure threshold (default: 5) and cooldown duration (default: 30s).
   - Automatic bypass of OPEN targets in routing chain.
3. **Routing Resolution**:
   - `Resolve(ctx, requestedModel)` returns deterministic `RouteDecision` containing:
     - Selected Target Model ID
     - Fallback chain
     - Reason / Strategy applied
4. **Data Plane Integration**:
   - Dynamic rewrite of `model` field in chat payload to resolved target model ID before forwarding to 9Router.
   - Automatic fallback execution if primary upstream target returns `5xx` / timeout.
