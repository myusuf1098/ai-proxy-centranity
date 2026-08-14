# ProxyGateway Enterprise --- Product Requirements Document (PRD)

**Version:** 2.0\
**Status:** Enterprise Baseline\
**Architecture:** Docker-native, 9Router-integrated Control & Policy
Plane\
**Primary Runtime:** Ubuntu Linux\
**Audience:** Product Owner, Architect, Backend/TUI Engineers,
DevOps/SRE, Security

## 1. Executive Summary

ProxyGateway Enterprise is a self-hosted AI API gateway and operational
control plane designed to sit in front of 9Router.

It provides:

-   OpenAI-compatible client access
-   model/provider selection and switching
-   model aliases
-   provider policies
-   API-key management
-   per-key model/provider allowlists
-   rate limits and token quotas
-   budget controls
-   retry/failover/circuit breaker
-   outbound proxy profile management
-   real-time request and usage telemetry
-   administrative TUI
-   audit logging
-   Prometheus/Grafana observability
-   Docker-native deployment

9Router remains the provider/model execution engine. ProxyGateway owns
policy enforcement, tenant/client controls, governance, and operational
management.

## 2. Product Vision

Create a secure, observable, deterministic AI gateway that allows one
Ubuntu Docker host to centrally control multiple AI providers, models,
9Router capabilities, client identities, quotas, routing policies, and
outbound network paths.

## 3. Goals

### P0

1.  Integrate with 9Router through supported HTTP APIs.
2.  Provide an OpenAI-compatible ProxyGateway API.
3.  Provide a full-featured terminal TUI.
4.  Manage model and provider selection.
5.  Support model aliases and routing policies.
6.  Enforce API-key authentication and policy.
7.  Enforce RPM/RPS/TPM/token-budget limits.
8.  Provide automatic retry and failover.
9.  Manage outbound proxy profiles without exposing proxy credentials.
10. Persist configuration, usage, and audit data.
11. Expose Prometheus metrics.
12. Run the complete stack through Docker Compose.
13. Protect 9Router management endpoints from direct external access.

### P1

-   PostgreSQL-backed policy/configuration
-   Redis-backed distributed rate limiting
-   configuration versioning
-   import/export
-   dry-run routing
-   health scoring
-   model/provider latency scoring
-   advanced usage analytics
-   role-based TUI access
-   backup/restore

### P2

-   multi-node ProxyGateway
-   HA PostgreSQL/Redis
-   GitOps configuration
-   OIDC/SSO
-   external secret manager
-   Kubernetes deployment

## 4. Non-Goals

ProxyGateway will not initially:

-   fork 9Router internals
-   replace provider OAuth implementations
-   directly implement every provider protocol
-   store provider secrets in source control
-   bypass provider terms, authentication, or quotas
-   expose 9Router administration directly to untrusted networks

## 5. Personas

### Administrator

Full control over providers, models, policies, keys, quotas, proxies,
routing and system configuration.

### Operator

Can inspect health, requests, usage, logs and execute safe operational
actions.

### Read-Only Auditor

Can view configuration, usage, audit logs and system state without
modifying configuration.

### API Client

Consumes the OpenAI-compatible API and is governed by an API-key policy.

## 6. Functional Requirements

### FR-001 --- API Gateway

ProxyGateway shall expose an OpenAI-compatible endpoint:

`POST /v1/chat/completions`

It shall support streaming where the upstream supports streaming.

### FR-002 --- 9Router Integration

ProxyGateway shall implement a versioned 9Router adapter.

The adapter shall support:

-   health/readiness
-   model discovery
-   provider discovery
-   provider/model management where supported by the installed 9Router
    API
-   combo/fallback management where supported
-   usage retrieval where supported
-   settings retrieval/update where supported

The adapter must isolate 9Router API changes from the core policy
engine.

### FR-003 --- Model Registry

Each model record shall contain:

-   canonical model ID
-   display name
-   provider
-   alias list
-   enabled state
-   priority
-   timeout
-   fallback policy
-   cost metadata
-   context metadata
-   capability tags

### FR-004 --- Model Input and Switching

The gateway shall accept:

-   canonical model IDs
-   aliases
-   policy-selected models
-   administrator-selected defaults

Examples:

`coding`, `fast`, `cheap`, `reasoning`, `free`.

### FR-005 --- Provider Registry

Provider records shall contain:

-   name
-   type
-   base URL where applicable
-   authentication reference
-   enabled state
-   priority
-   timeout
-   retry policy
-   proxy profile
-   capability metadata

### FR-006 --- Routing

Supported strategies:

-   explicit model
-   alias
-   provider priority
-   model priority
-   lowest latency
-   lowest observed error rate
-   weighted routing
-   round robin
-   failover
-   9Router combo/fallback

### FR-007 --- API Keys

Each API key shall have:

-   name
-   hashed secret
-   enabled state
-   expiration
-   allowed models
-   denied models
-   allowed providers
-   denied providers
-   RPM/RPS limits
-   TPM limits
-   daily/weekly/monthly token limits
-   budget limits
-   optional source-network restrictions

Raw API keys shall never be stored in plaintext.

### FR-008 --- Rate Limiting

Support:

-   requests/second
-   requests/minute
-   tokens/minute
-   concurrent requests
-   daily token quota
-   monthly token quota

Redis shall be the authoritative distributed limiter in multi-instance
mode.

### FR-009 --- Budget Controls

Support:

-   per-key budget
-   per-model budget
-   per-provider budget
-   global budget

Threshold actions:

-   warning
-   route to cheaper model
-   route to free model
-   block request

### FR-010 --- Failover

The gateway shall support:

-   retry with exponential backoff
-   retry-after interpretation
-   provider failover
-   model failover
-   circuit breaker
-   temporary quarantine of unhealthy targets

### FR-011 --- Proxy Profiles

Support outbound proxy profiles:

-   DIRECT
-   HTTP
-   HTTPS
-   SOCKS5

A proxy profile may be assigned to:

-   provider
-   model
-   policy
-   client/API key

Proxy credentials shall be stored as secrets/references.

### FR-012 --- TUI Administration

TUI screens:

1.  Overview
2.  Requests
3.  Models
4.  Providers
5.  API Keys
6.  Policies
7.  Routing
8.  Proxies
9.  Usage
10. Audit
11. System
12. Settings

Actions include:

-   add
-   edit
-   enable
-   disable
-   test
-   switch
-   reset
-   import
-   export
-   reload

Destructive actions require confirmation.

### FR-013 --- Audit

Record:

-   actor
-   action
-   target
-   timestamp
-   source
-   result
-   correlation ID
-   before/after metadata where safe

Secrets must never enter audit logs.

### FR-014 --- Observability

Expose:

-   request counters
-   active requests
-   latency
-   errors
-   retries
-   circuit states
-   token counts
-   quota usage
-   provider/model health
-   proxy health

### FR-015 --- Configuration Lifecycle

Configuration shall support:

-   validation
-   versioning
-   atomic update
-   rollback
-   export
-   import
-   dry-run

## 7. Non-Functional Requirements

### Security

-   least privilege containers
-   no privileged container unless explicitly required
-   internal-only 9Router management access
-   secrets via environment/secret files
-   hashed API keys
-   TLS-ready architecture
-   audit logging
-   secure headers
-   dependency scanning
-   container image scanning

### Reliability

Target:

-   API availability ≥ 99.9% for a single healthy Docker host
-   graceful restart
-   no configuration corruption during restart
-   retry/failover for transient upstream failures

### Performance

Initial target:

-   gateway overhead p95 \< 25 ms excluding upstream latency
-   TUI refresh 1--4 Hz
-   API should remain responsive during TUI operation

### Maintainability

-   modular Go codebase
-   contract tests for 9Router adapter
-   unit/integration/e2e tests
-   semantic versioning
-   ADRs for major decisions

## 8. Acceptance Criteria

Release is production-ready when:

-   Docker Compose starts all required services
-   health checks pass
-   9Router is reachable only through intended networks
-   API-key policy is enforced
-   model/provider routing works
-   quotas are enforced
-   retry/failover works
-   audit entries are generated
-   Prometheus metrics are available
-   TUI can view and modify permitted configuration
-   backup/restore has been tested
-   security scan has no release-blocking findings

## 9. Success Metrics

-   successful request rate
-   p95/p99 latency
-   upstream error rate
-   retry rate
-   quota rejection rate
-   provider health
-   model utilization
-   token utilization
-   configuration change failure rate
-   mean time to recovery

## 10. Enterprise Constraints

The system shall treat 9Router as an external subsystem with an adapter
boundary. Internal 9Router database schemas shall not become a hard
dependency.

All provider-specific secrets remain under secret-management controls.

All management operations require authorization and auditability.
