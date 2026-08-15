# ProxyGateway Enterprise --- Architecture

**Version:** 2.0\
**Architecture Style:** Control Plane + Data Plane\
**Deployment:** Docker Compose\
**Host:** Ubuntu Linux

## 1. Architectural Principle

ProxyGateway is a policy/control layer around 9Router.

It does not fork 9Router.

``` text
Client
  |
  v
ProxyGateway Data Plane
  |
  +-- Authentication
  +-- Policy
  +-- Rate Limit
  +-- Quota
  +-- Routing
  +-- Retry
  +-- Circuit Breaker
  |
  v
9Router
  |
  +-- Provider
  +-- Model
  +-- OAuth
  +-- Translation
  +-- Upstream proxy
  |
  v
AI Provider
```

## 2. Logical Components

### ProxyGateway API

Responsibilities:

-   public client API
-   request lifecycle
-   authorization
-   policy enforcement
-   routing
-   upstream adapter
-   streaming

### Control Plane

Responsibilities:

-   configuration
-   provider/model registry
-   API keys
-   policies
-   routing rules
-   proxy profiles
-   audit

### TUI

The TUI is an administrative client of the ProxyGateway management API.

It does not write PostgreSQL directly.

This guarantees one authorization and audit path.

### PostgreSQL

Authoritative store for:

-   configuration
-   policies
-   API key metadata
-   audit
-   usage aggregates
-   configuration versions

### Redis

Used for:

-   distributed rate limiting
-   request counters
-   short-lived locks
-   cache
-   health state

### 9Router

External subsystem adapter target.

Its internal database is not accessed directly.

## 3. Docker Topology

``` text
proxygateway-network
|
+-- proxygateway-api
|
+-- proxygateway-tui
|
+-- postgres
|
+-- redis
|
+-- 9router
|
+-- prometheus
|
+-- grafana
```

> **Deployed reality (2026-08-15):** Compose ships `postgres`, `redis`,
> `proxygateway-api`, `proxygateway-tui`. `prometheus` + `grafana` are
> **planned, not yet deployed** — observability network deferred. Metrics
> endpoint `/metrics` is available on the API service.

Recommended external exposure:

``` text
LAN / reverse proxy
        |
        v
proxygateway-api
```

9Router management/API ports should remain on an internal Docker network
unless there is a documented reason to expose them.

## 4. Service Roles

### proxygateway-api

Ports:

-   `8088` API
-   `9099` metrics

### proxygateway-tui

No public listening port required.

Connects to:

`http://proxygateway-api:8088`

### 9router

Default upstream:

`http://9router:20128/v1`

Management endpoints use the internal 9Router address.

### PostgreSQL

Internal only.

### Redis

Internal only.

### Prometheus

Internal or LAN-admin only.

### Grafana

LAN-admin only or protected by reverse proxy/authentication.

## 5. Networks

Use separate logical networks:

``` text
frontend
backend
observability
```

Data-plane API can attach to frontend/backend.

PostgreSQL and Redis attach only to backend.

Prometheus attaches to backend/observability.

Grafana attaches to observability.

## 6. Request Flow

``` text
1. Client sends API request
2. Gateway authenticates API key
3. Gateway loads policy
4. Gateway checks quota/rate/concurrency
5. Gateway resolves model alias
6. Gateway selects provider/model
7. Gateway checks health/circuit
8. Gateway sends request to 9Router
9. 9Router selects provider/upstream
10. Response streams through gateway
11. Usage and latency are recorded
12. Audit/metrics are updated
13. Client receives normalized response
```

## 7. Policy Precedence

Recommended order:

``` text
Global safety policy
    >
API key policy
    >
Client/application policy
    >
Model policy
    >
Provider policy
    >
Routing policy
    >
9Router defaults
```

A deny rule always overrides an allow rule at the same policy scope.

## 8. Configuration State Machine

``` text
DRAFT
  |
VALIDATE
  |
DRY-RUN
  |
APPROVED
  |
APPLY
  |
ACTIVE
  |
ROLLBACK
```

Configuration changes should be atomic.

## 9. Model Resolution

Example:

``` text
client model = "coding"

Alias registry
       |
       v
coding
  |
  +-- preferred: deepseek-v4-flash-free
  +-- fallback: mimo-v2.5-free
  +-- fallback: gemini-flash
```

## 10. Provider Resolution

Example:

``` text
preferred provider:
  1. OpenCode
  2. DeepSeek
  3. Google

health + policy
       |
       v
selected provider
```

## 11. Rate-Limit Architecture

Redis key examples:

``` text
rl:{api_key}:rps
rl:{api_key}:rpm
rl:{api_key}:tpm
rl:{api_key}:concurrency
quota:{api_key}:daily
quota:{api_key}:monthly
```

Use atomic Redis operations/scripts to avoid race conditions.

## 12. Circuit Breaker

States:

``` text
CLOSED
  |
failure threshold
  v
OPEN
  |
cooldown
  v
HALF_OPEN
  |
success -> CLOSED
failure -> OPEN
```

Circuit state should be visible in TUI and metrics.

## 13. Proxy Architecture

ProxyGateway stores proxy profile metadata and credentials references.

Example:

``` text
PROXY-SOCKS5-01
type: socks5
host: proxy.example
port: 1080
secret_ref: proxy/socks5/01
```

The actual outbound proxy may be applied through 9Router configuration
where supported.

ProxyGateway should not silently mutate 9Router internals.

## 14. Security Boundaries

``` text
UNTRUSTED CLIENT
      |
      v
[API AUTH + POLICY]
      |
      v
[PROXYGATEWAY]
      |
      v
[9ROUTER INTERNAL]
      |
      v
[INTERNET]
```

Management interfaces are privileged.

TUI must call management APIs rather than direct database writes.

## 15. Secrets

Never store:

-   provider API keys
-   OAuth refresh tokens
-   outbound proxy passwords
-   gateway secret material

in Git.

Preferred order:

1.  Docker secrets
2.  external secret manager
3.  protected environment file for single-host deployment

## 16. Persistence

Volumes:

``` text
postgres-data
redis-data
9router-data
prometheus-data
grafana-data
```

ProxyGateway application data resides in PostgreSQL.

## 17. Backup

Back up:

-   PostgreSQL
-   9Router persistent data
-   Grafana provisioning/config
-   Compose files
-   secrets references/configuration metadata

Do not back up transient Redis rate-limit state as a primary recovery
dependency.

## 18. Failure Domains

If TUI fails:

-   API continues.

If Grafana fails:

-   API continues.

If Prometheus fails:

-   API continues.

If Redis fails:

-   fail closed for strict quota policies or enter documented degraded
    mode.

If 9Router fails:

-   API returns normalized upstream unavailable errors and can invoke
    configured alternate routing only where policy allows.

If PostgreSQL fails:

-   control-plane changes stop; data-plane behavior must follow an
    explicitly defined degraded-mode policy.

## 19. Scalability

Single-host MVP:

``` text
1 API
1 TUI
1 9Router
1 PostgreSQL
1 Redis
```

Future:

``` text
N ProxyGateway API
N TUI clients
HA PostgreSQL
HA Redis
9Router cluster or multiple 9Router instances
```

## 20. Architectural Rule

No component other than the 9Router adapter should depend on
9Router-specific implementation details.

This is the main anti-lock-in boundary.
