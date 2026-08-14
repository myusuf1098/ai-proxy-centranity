# ProxyGateway Enterprise --- Refactoring & Modernization Plan

**Version:** 2.0\
**Purpose:** Convert the initial monitoring-oriented design into a
production-grade control and policy plane.

## 1. Refactoring Objective

The previous design treated 9Router primarily as a read/forwarding
upstream.

The target architecture changes this to:

``` text
ProxyGateway
=
Data Plane
+
Policy Plane
+
Control Plane
+
Observability
```

9Router remains an execution subsystem.

## 2. Target Module Boundaries

``` text
internal/
├── api
├── auth
├── policy
├── routing
├── limiter
├── quota
├── budget
├── model
├── provider
├── proxy
├── ninerouter
├── audit
├── usage
├── store
├── metrics
├── health
├── tui
└── config
```

Each module owns a clear interface.

## 3. Adapter Refactor

Bad:

``` text
business logic -> 9Router HTTP details
```

Good:

``` text
business logic
      |
      v
NineRouterPort
      |
      v
NineRouterHTTPAdapter
```

Example interface:

``` go
type NineRouterPort interface {
    ListModels(ctx context.Context) ([]Model, error)
    ListProviders(ctx context.Context) ([]Provider, error)
    GetUsage(ctx context.Context, q UsageQuery) (Usage, error)
    UpdateProvider(ctx context.Context, id string, req ProviderUpdate) error
    UpdateModel(ctx context.Context, id string, req ModelUpdate) error
}
```

Exact methods must be aligned to the installed 9Router API contract
during implementation.

## 4. API Refactor

Separate:

``` text
Data Plane
/v1/*
```

from:

``` text
Control Plane
/api/v1/*
```

Do not mix management authorization with client request authorization.

## 5. Policy Engine

Create one policy engine:

``` text
PolicyEngine.Evaluate(request, identity, context)
```

Returns:

``` text
Allow
Deny
RouteDecision
LimitDecision
BudgetDecision
```

All client paths must use this engine.

This prevents policy bypass.

## 6. Routing Engine

Create:

``` text
RouteEngine.Resolve(ctx, RequestContext)
```

It evaluates:

1.  explicit model
2.  alias
3.  client policy
4.  provider policy
5.  health
6.  quota
7.  routing strategy
8.  fallback

Routing must return an explainable decision.

Example:

``` text
alias=coding
preferred_model=deepseek-v4-flash-free
provider=OpenCode
reason=priority
fallback=mimo-v2.5-free
```

## 7. Limiter Refactor

Do not implement independent counters in every HTTP handler.

All limits go through:

``` text
Limiter
  |
  +-- RPS
  +-- RPM
  +-- TPM
  +-- concurrency
```

Use Redis atomic operations.

## 8. Quota Refactor

Separate rate limits from quotas.

Rate limit:

``` text
requests per time window
```

Quota:

``` text
tokens per day/month
```

Budget:

``` text
currency/value ceiling
```

These are different domains and must not share ambiguous fields.

## 9. API Key Refactor

Move from simple authentication to:

``` text
Identity
   |
   +-- API key
   +-- role
   +-- policy
   +-- quotas
   +-- allowed models
   +-- allowed providers
```

## 10. TUI Refactor

The TUI must not contain business logic.

Bad:

``` text
TUI -> PostgreSQL
TUI -> Redis
TUI -> 9Router
```

Good:

``` text
TUI
 |
 v
ProxyGateway Management API
 |
 +-> policy service
 +-> routing service
 +-> 9Router adapter
 +-> database
```

## 11. TUI State Model

Use separate state:

``` text
View State
Domain State
Async State
Error State
```

Avoid global mutable state.

## 12. Configuration Refactor

Introduce immutable configuration snapshots:

``` text
config v17
    |
    +-- validate
    +-- audit
    +-- apply
```

A failed apply must not partially mutate the system.

## 13. Audit Refactor

Every mutation command should pass through:

``` text
Command
  |
Authorization
  |
Validation
  |
Mutation
  |
Audit
```

Audit failure handling must be explicitly defined for security-sensitive
actions.

## 14. Database Refactor

Move production persistence from SQLite to PostgreSQL.

Migration principles:

-   additive changes first
-   backward-compatible schema
-   versioned migrations
-   transaction where possible
-   explicit rollback/forward-fix plan

## 15. Redis Refactor

Use Redis only for ephemeral/distributed state.

Never make Redis the authoritative source for:

-   provider definitions
-   model definitions
-   API key records
-   audit
-   long-term usage

## 16. Error Model

Create normalized errors:

``` text
AUTH_FAILED
FORBIDDEN
RATE_LIMITED
QUOTA_EXCEEDED
BUDGET_EXCEEDED
MODEL_NOT_ALLOWED
PROVIDER_NOT_ALLOWED
UPSTREAM_TIMEOUT
UPSTREAM_RATE_LIMIT
UPSTREAM_UNAVAILABLE
ROUTING_FAILED
CONFIG_INVALID
```

Map upstream-specific errors into these stable gateway errors.

## 17. Security Refactor

Mandatory:

-   secret redaction
-   API-key hashing
-   non-root containers
-   internal management endpoints
-   authorization on every mutation
-   audit
-   SSRF protection for user-controlled URLs
-   provider base URL allow/deny policy
-   outbound network restrictions where practical

## 18. Testing Refactor

Test pyramid:

``` text
              E2E
             /   \
        Integration
        /          \
     Contract      Component
        \          /
           Unit
```

Critical tests:

-   policy bypass
-   quota race
-   concurrent request limit
-   retry storm
-   circuit breaker
-   model alias resolution
-   provider failover
-   proxy credential redaction
-   9Router API compatibility
-   config rollback

## 19. Refactoring Sequence

### R1

Create interfaces and adapters.

### R2

Move 9Router logic behind adapter.

### R3

Introduce policy engine.

### R4

Introduce routing engine.

### R5

Introduce Redis limiter.

### R6

Move persistence to PostgreSQL.

### R7

Separate management API from data plane.

### R8

Convert TUI into API client.

### R9

Add audit and configuration versioning.

### R10

Add observability and security hardening.

## 20. Definition of Done

Refactoring is complete when:

-   TUI has no direct database access
-   TUI has no direct 9Router dependency
-   business logic has no HTTP-specific 9Router code
-   policy enforcement is centralized
-   rate limiting is centralized
-   routing is deterministic and testable
-   PostgreSQL is authoritative
-   Redis is ephemeral
-   all mutations are audited
-   critical flows have automated tests
-   Docker deployment is reproducible

## 21. Technical Debt Register

Track every intentional shortcut:

``` text
ID
Description
Risk
Owner
Mitigation
Target Release
Status
```

No critical technical debt may remain undocumented.

## 22. Refactoring Rule

Never refactor and add a major feature in the same uncontrolled change.

Use:

``` text
refactor
  |
tests
  |
feature
  |
tests
```

This preserves production safety while the project grows.
