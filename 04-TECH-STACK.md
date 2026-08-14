# ProxyGateway Enterprise --- Technology Stack

**Version:** 2.0

## 1. Stack Summary

  Layer           Technology                 Purpose
  --------------- -------------------------- ---------------------------------
  Core            Go                         API, policy, routing
  TUI             Bubble Tea                 Terminal UI
  TUI styling     Lip Gloss                  Visual layout
  API             net/http                   HTTP server/client
  Streaming       SSE / HTTP streaming       AI response streaming
  Database        PostgreSQL                 Persistent state
  Cache/limits    Redis                      Rate limits and ephemeral state
  9Router         Docker                     AI provider gateway
  Metrics         Prometheus                 Metrics
  Dashboard       Grafana                    Observability
  Packaging       Docker                     Runtime isolation
  Orchestration   Docker Compose             Single-host deployment
  Logging         Go slog / JSON             Structured logs
  Config          YAML + environment         Deployment config
  Migrations      Versioned SQL              Schema management
  Testing         Go test                    Unit/integration
  API tests       Testcontainers / Compose   Integration
  Security        Trivy + govulncheck        Scanning
  SBOM            Syft/CycloneDX             Supply-chain metadata
  CI              GitHub Actions/GitLab CI   Automation

## 2. Go Version

Use a current supported Go release and pin it in:

-   `go.mod`
-   Docker builder image
-   CI

Do not use floating build images in production.

## 3. Repository Layout

``` text
proxygateway/
├── cmd/
│   ├── proxygateway-api/
│   └── proxygateway-tui/
├── internal/
│   ├── api/
│   ├── auth/
│   ├── policy/
│   ├── routing/
│   ├── limiter/
│   ├── quota/
│   ├── budget/
│   ├── model/
│   ├── provider/
│   ├── proxy/
│   ├── ninerouter/
│   ├── audit/
│   ├── usage/
│   ├── health/
│   ├── metrics/
│   ├── tui/
│   ├── store/
│   └── config/
├── migrations/
├── deployments/
│   └── docker/
├── docs/
├── tests/
└── Makefile
```

## 4. API Design

Public data-plane API:

``` text
/v1/models
/v1/chat/completions
```

Management API:

``` text
/api/v1/providers
/api/v1/models
/api/v1/keys
/api/v1/policies
/api/v1/routes
/api/v1/proxies
/api/v1/usage
/api/v1/audit
/api/v1/system
```

Health:

``` text
/health/live
/health/ready
```

Metrics:

``` text
/metrics
```

## 5. Database

PostgreSQL is the production source of truth.

Suggested domains:

``` text
providers
provider_nodes
models
model_aliases
routing_policies
api_keys
api_key_policies
rate_limit_policies
quota_policies
budget_policies
proxy_profiles
usage_events
usage_daily
audit_events
config_versions
```

## 6. API Key Storage

Store:

``` text
key_id
name
prefix
hash
status
created_at
expires_at
```

Never store recoverable plaintext API keys.

## 7. Token Accounting

Prefer provider-reported usage when available.

When unavailable:

-   mark token usage as estimated
-   do not treat estimates as billing-grade
-   record source=`provider|estimated`

## 8. Redis

Redis responsibilities:

-   rate limiting
-   concurrency counters
-   short TTL caches
-   health snapshots
-   distributed locks

PostgreSQL remains authoritative for configuration.

## 9. TUI

Libraries:

-   Bubble Tea
-   Lip Gloss
-   Bubbles

TUI must communicate through ProxyGateway management APIs.

It must not connect directly to PostgreSQL or Redis.

## 10. Docker Image Strategy

Use multi-stage builds:

``` text
golang builder
     |
     v
distroless/non-root runtime
```

For TUI, use a runtime image compatible with terminal/TTY execution.

Do not run the container as root unless an explicitly documented
requirement exists.

## 11. Compose Services

``` yaml
services:
  proxygateway-api:
  proxygateway-tui:
  9router:
  postgres:
  redis:
  prometheus:
  grafana:
```

Pin images to known versions for production.

## 12. Environment Configuration

Use:

``` text
.env.example
.env.production
Docker secrets
```

Never commit production secrets.

## 13. CI Pipeline

``` text
lint
  |
unit test
  |
integration test
  |
9Router contract test
  |
security scan
  |
SBOM
  |
Docker build
  |
image scan
  |
smoke deployment
  |
release
```

## 14. Quality Gates

Minimum:

-   gofmt
-   go vet
-   static analysis
-   unit coverage target ≥ 80% for core policy/routing
-   integration tests for critical flows
-   no critical/high container vulnerability without documented
    exception

## 15. Observability Standards

Metrics naming:

``` text
proxygateway_<domain>_<metric>_<unit>
```

Examples:

``` text
proxygateway_requests_total
proxygateway_request_duration_seconds
proxygateway_tokens_total
proxygateway_rate_limit_rejections_total
```

Log fields:

``` text
timestamp
level
service
request_id
trace_id
actor_id
model
provider
status
latency_ms
error_code
```

Never log:

-   API keys
-   OAuth tokens
-   proxy passwords
-   Authorization headers
-   prompt/response content by default

## 16. Security Stack

Use:

-   Trivy
-   govulncheck
-   Dependabot/Renovate
-   SBOM
-   signed release artifacts
-   non-root containers
-   read-only root filesystem where practical
-   dropped Linux capabilities
-   internal Docker networks

## 17. Dependency Policy

Dependencies must have:

-   active maintenance
-   compatible license
-   pinned version
-   vulnerability monitoring

Major upgrades require regression tests.

## 18. Versioning

Use Semantic Versioning:

``` text
MAJOR.MINOR.PATCH
```

Example:

``` text
1.0.0
1.1.0
1.1.1
```

9Router compatibility must be independently tracked:

``` text
ProxyGateway 1.0
  9Router 0.4.x supported
```

## 19. Configuration API Contract

Every mutation endpoint must support:

-   validation
-   authorization
-   audit
-   optimistic concurrency/version check
-   atomic commit
-   rollback where applicable

## 20. Enterprise Design Rule

Technology is selected to preserve these boundaries:

``` text
Client protocol
      ↓
ProxyGateway domain
      ↓
9Router adapter
      ↓
9Router
```

The domain layer must not import 9Router-specific implementation code.
