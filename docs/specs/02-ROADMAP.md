# ProxyGateway Enterprise --- Roadmap

**Version:** 2.0\
**Delivery Model:** Incremental, gated releases\
**Runtime:** Docker Compose on Ubuntu Linux

## 1. Delivery Principles

-   Security before feature breadth.
-   Contract-first integration with 9Router.
-   Every phase must be deployable and rollbackable.
-   No direct dependency on 9Router internal database schema.
-   Production changes require migration and backup strategy.
-   Every new control surface requires audit logging.

## 2. Phase 0 --- Discovery and Baseline

### Deliverables

-   inventory installed 9Router version
-   capture supported management/API endpoints
-   identify current Docker volumes
-   identify current proxy configuration
-   establish baseline performance
-   establish backup/restore procedure

### Exit Gate

-   9Router version documented
-   management API authentication verified
-   current deployment backed up
-   integration contract frozen for MVP

## 3. Phase 1 --- Platform Foundation

### Deliverables

-   Go repository
-   Docker multi-stage build
-   Docker Compose
-   PostgreSQL
-   Redis
-   health endpoints
-   structured logging
-   configuration loader
-   migrations
-   CI pipeline

### Exit Gate

A clean Ubuntu host can deploy the platform from a versioned Compose
bundle.

## 4. Phase 2 --- 9Router Adapter

### Deliverables

-   9Router client
-   authentication
-   model discovery
-   provider discovery
-   management adapters
-   health checks
-   contract tests
-   compatibility matrix

### Exit Gate

All supported 9Router operations are covered by automated contract
tests.

## 5. Phase 3 --- Data Plane

### Deliverables

-   OpenAI-compatible API
-   request correlation
-   streaming
-   model resolution
-   provider resolution
-   timeout
-   retry
-   circuit breaker
-   error normalization

### Exit Gate

Client can successfully use ProxyGateway as a drop-in endpoint.

## 6. Phase 4 --- Policy Plane

### Deliverables

-   API keys
-   model allow/deny
-   provider allow/deny
-   RPM/RPS
-   TPM
-   concurrency
-   token quotas
-   budgets
-   policy precedence

### Exit Gate

No request can bypass policy through alternate model/provider fields.

## 7. Phase 5 --- Routing

### Deliverables

-   aliases
-   manual model switch
-   provider priority
-   weighted routing
-   lowest-latency routing
-   failover
-   9Router combo integration
-   dry-run routing

### Exit Gate

Routing decisions are deterministic, observable and auditable.

## 8. Phase 6 --- Proxy Management

### Deliverables

-   proxy profiles
-   HTTP/HTTPS/SOCKS5 support
-   proxy assignment policies
-   health checks
-   latency scoring
-   quarantine
-   credential references

### Exit Gate

Proxy credentials never appear in logs, TUI output or audit records.

## 9. Phase 7 --- Administrative TUI

### Deliverables

-   Overview
-   Requests
-   Models
-   Providers
-   API Keys
-   Policies
-   Routing
-   Proxies
-   Usage
-   Audit
-   Settings

### Exit Gate

All safe P0 management operations are executable from TUI and API.

## 10. Phase 8 --- Observability

### Deliverables

-   Prometheus metrics
-   Grafana dashboards
-   usage analytics
-   SLO dashboards
-   alert rules
-   audit search

### Exit Gate

Operator can identify request, provider, model, policy and proxy
failures without SSHing into containers.

## 11. Phase 9 --- Hardening

### Deliverables

-   container hardening
-   image scanning
-   dependency scanning
-   SBOM
-   secret checks
-   rate-limit abuse tests
-   SSRF tests
-   authorization tests
-   backup/restore drill

### Exit Gate

No release-blocking security finding remains.

## 12. Phase 10 --- Production Release

### Deliverables

-   v1.0.0 release
-   signed images
-   pinned dependencies
-   documented upgrade
-   documented rollback
-   disaster recovery runbook
-   operations manual

## 13. Phase 11 --- Post-v1

Optional:

-   multi-node gateway
-   OIDC
-   external secrets
-   Kubernetes
-   GitOps
-   HA data stores
-   policy-as-code
-   multi-tenant organization model

## 14. Release Gates

Every production release must pass:

1.  unit tests
2.  integration tests
3.  9Router contract tests
4.  API compatibility tests
5.  policy bypass tests
6.  migration tests
7.  backup/restore tests
8.  container security scan
9.  smoke deployment
10. rollback rehearsal

## 15. Rollback

Application rollback:

-   revert image tag
-   keep compatible database schema
-   restore previous configuration if necessary

Database rollback:

-   prefer forward-compatible migrations
-   use backup restore only for destructive migration failures

9Router rollback:

-   pin known-good image/version
-   keep adapter compatibility matrix
-   never silently upgrade production 9Router
