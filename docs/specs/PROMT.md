# ProxyGateway Enterprise — Master Implementation Contract

**Document:** `PROMT.md`  
**Document ID:** `PG-CONTRACT-000`  
**Version:** 2.1  
**Status:** **MANDATORY / BINDING IMPLEMENTATION CONTRACT**  
**Applies To:** All AI coding agents, autonomous agents, human developers, reviewers, CI/CD automation, and implementation tooling.

---

# 0. CONTRACT DECLARATION

This document is the master execution instruction for implementing ProxyGateway Enterprise.

The project Markdown documentation is the engineering specification of the system.

The agent MUST NOT treat the documentation as optional reference material.

The agent MUST:

1. read the complete documentation set;
2. understand the relationships between documents;
3. implement according to the approved specifications;
4. preserve architectural boundaries;
5. maintain traceability;
6. test every implementation unit;
7. document every material change;
8. stop when an unresolved specification conflict materially affects implementation.

**Working code is not sufficient for completion.**

The implementation is considered correct only when:

```text
Requirement
    ↓
Architecture
    ↓
Implementation
    ↓
Test
    ↓
Documentation
    ↓
Release
```

is traceable.

---

# 1. PROJECT DOCUMENT SET

The authoritative documents are:

```text
01-PRD.md
02-ROADMAP.md
03-ARCHITECTURE.md
04-TECH-STACK.md
05-REFACTORY-PLAN.md
06-TUI-UX-SPEC.md
07-IMPLEMENTATION-DOCUMENTATION.md
PROMT.md
README.md
```

Additional documentation may exist under:

```text
docs/
docs/adr/
docs/features/
docs/api/
docs/testing/
docs/operations/
docs/security/
docs/releases/
docs/incidents/
docs/templates/
```

The agent MUST inspect these directories when they exist.

---

# 2. DOCUMENT AUTHORITY MODEL

Each document owns a specific concern.

| Document | Authority |
|---|---|
| `PROMT.md` | Implementation contract and execution rules |
| `01-PRD.md` | WHAT the product must do |
| `02-ROADMAP.md` | WHEN/how delivery is phased |
| `03-ARCHITECTURE.md` | HOW the system is structured |
| `04-TECH-STACK.md` | WHICH technologies and engineering standards are used |
| `05-REFACTORY-PLAN.md` | HOW source modules are structured/refactored |
| `06-TUI-UX-SPEC.md` | HOW the TUI looks and behaves |
| `07-IMPLEMENTATION-DOCUMENTATION.md` | HOW implementation evidence and history are recorded |
| `README.md` | Documentation index/navigation |

## Conflict rule

If two documents conflict:

1. identify the conflict;
2. determine whether it is an apparent scope/interpretation difference;
3. inspect related documents and existing implementation;
4. do NOT silently choose a destructive interpretation;
5. if materially unresolved, STOP and report the conflict;
6. for architectural changes, create an ADR proposal before implementation.

`PROMT.md` controls **how the agent works**. It does not silently redefine product requirements.

---

# 3. MANDATORY INITIALIZATION

Before modifying application code, the agent MUST perform a repository and documentation audit.

Required sequence:

```text
1. Inspect repository structure
2. Locate all Markdown documentation
3. Read all authoritative documents
4. Inspect source code
5. Inspect Docker files
6. Inspect environment/configuration
7. Inspect database/migrations
8. Inspect API definitions
9. Inspect tests
10. Inspect CI/CD
11. Inspect observability
12. Inspect existing 9Router integration
13. Build implementation gap matrix
14. Validate roadmap starting point
15. Only then begin implementation
```

The agent MUST NOT immediately start coding after seeing the task.

---

# 4. IMPLEMENTATION GAP MATRIX

Before Phase 1 implementation, create an internal or repository-tracked matrix containing:

| Domain | Specification | Existing State | Gap | Required Action | Risk |
|---|---|---|---|---|---|
| Product | | | | | |
| Architecture | | | | | |
| Docker | | | | | |
| Database | | | | | |
| API | | | | | |
| 9Router | | | | | |
| Models | | | | | |
| Providers | | | | | |
| Routing | | | | | |
| Policies | | | | | |
| API Keys | | | | | |
| Limits | | | | | |
| Quotas | | | | | |
| Proxy | | | | | |
| Observability | | | | | |
| Security | | | | | |
| TUI | | | | | |
| Testing | | | | | |
| Documentation | | | | | |

Do not implement blindly when the existing repository already contains relevant functionality.

---

# 5. IMPLEMENTATION PHILOSOPHY

The agent must optimize for:

```text
Correctness
Security
Maintainability
Testability
Observability
Traceability
Operational Safety
Simplicity
```

Do not optimize primarily for:

```text
speed of coding
number of files changed
number of features superficially completed
```

Enterprise quality does not mean unnecessary complexity.

Prefer the simplest architecture that satisfies the approved requirements.

---

# 6. NON-NEGOTIABLE RULES

The agent MUST NOT:

- invent undocumented APIs;
- invent undocumented 9Router internals;
- invent database structures without architectural justification;
- bypass documented security controls;
- bypass authentication/authorization;
- hardcode secrets;
- expose credentials in logs;
- directly access database from TUI;
- directly couple TUI to 9Router internals;
- silently change architecture;
- silently change API contracts;
- silently perform destructive migrations;
- skip tests to declare a feature complete;
- skip documentation;
- ignore existing working functionality without justification;
- replace a subsystem merely because the agent prefers another technology;
- introduce dependencies without justification;
- create duplicate implementations of an existing capability.

---

# 7. UNKNOWN BEHAVIOR RULE

When required behavior is unknown:

```text
Inspect repository
    ↓
Inspect documentation
    ↓
Inspect API contracts
    ↓
Inspect tests
    ↓
Inspect local/official technical documentation
    ↓
Safe runtime verification where appropriate
    ↓
Decision
```

The agent MUST NOT guess when uncertainty affects:

- security;
- data integrity;
- routing;
- provider behavior;
- 9Router compatibility;
- API contracts;
- database migrations;
- authentication;
- authorization;
- production traffic.

If uncertainty remains, stop the affected implementation path and report it.

---

# 8. 9ROUTER INTEGRATION CONTRACT

9Router is an integrated external subsystem.

The architecture MUST preserve an explicit boundary:

```text
                  ProxyGateway
                       |
                       v
             +--------------------+
             | 9Router Adapter     |
             | / Integration      |
             | Boundary            |
             +--------------------+
                       |
                       v
                    9Router
                       |
                       v
              Provider / Upstream
```

The ProxyGateway domain layer MUST NOT depend on undocumented 9Router internals.

The adapter is responsible for:

- connection;
- authentication;
- API compatibility;
- model discovery;
- provider discovery;
- routing operations;
- health operations;
- error translation;
- version compatibility.

Every supported 9Router capability MUST be documented.

Every production-critical 9Router capability MUST have compatibility tests.

---

# 9. TUI CONTRACT

The TUI is an administrative client.

The TUI MUST communicate through the ProxyGateway Management API.

The TUI MUST NOT directly access:

```text
PostgreSQL
Redis
9Router database
internal service storage
```

Required boundary:

```text
TUI
 |
 v
Management API
 |
 +--> Domain Services
 +--> PostgreSQL
 +--> Redis
 +--> 9Router Adapter
 +--> Data Plane
```

All TUI implementation MUST comply with:

`06-TUI-UX-SPEC.md`

---

# 10. TUI VISUAL CONTRACT

The TUI must be:

- professional;
- restrained;
- readable;
- keyboard-first;
- information-dense without being cluttered;
- visually consistent.

Required:

- minimal icons;
- simple symbols;
- subdued semantic colors;
- centralized theme tokens;
- consistent spacing;
- consistent borders;
- clear focus state.

Forbidden:

- 3D icons;
- emoji-heavy controls;
- neon/cyberpunk appearance;
- excessive gradients;
- flashing UI;
- excessive animation;
- arbitrary per-screen colors;
- decorative elements that reduce information density.

The UI must remain usable without color.

---

# 11. CORE PRODUCT SCOPE

The implementation must support the approved requirements for:

```text
ProxyGateway Control Plane
ProxyGateway Data Plane
9Router integration
Provider management
Model management
Model aliases
Provider switching
Model switching
Routing policies
Priority routing
Weighted routing
Fallback
Retry
Circuit breaker
Health checks
API key management
Authentication
Authorization
RPS/RPM limits
TPM limits
Concurrency limits
Daily/monthly quotas
Budget controls
Proxy profiles
Outbound proxy configuration
Usage tracking
Audit logging
Metrics
Structured logs
Docker deployment
Backup/restore
Security hardening
Professional TUI
```

Only implement capabilities approved by the PRD/roadmap.

---

# 12. IMPLEMENTATION PHASE CONTRACT

Follow the approved roadmap.

Do not skip phases merely because a later feature appears easy.

Preferred dependency sequence:

```text
Phase 0  Documentation / Repository Audit
Phase 1  Project Foundation
Phase 2  Docker / Runtime Infrastructure
Phase 3  Configuration / Secrets
Phase 4  Database / Migrations
Phase 5  Core Domain
Phase 6  Management API
Phase 7  9Router Integration
Phase 8  Provider Management
Phase 9  Model Management
Phase 10 Routing Engine
Phase 11 Policies
Phase 12 API Keys
Phase 13 Limits / Quotas / Budgets
Phase 14 Proxy Profiles
Phase 15 Retry / Fallback / Circuit Breaker
Phase 16 Observability
Phase 17 Audit
Phase 18 TUI Foundation
Phase 19 TUI Screens
Phase 20 Security Hardening
Phase 21 Integration Testing
Phase 22 E2E Testing
Phase 23 Performance Testing
Phase 24 Documentation Verification
Phase 25 Release Preparation
```

The exact phase mapping MUST be reconciled with `02-ROADMAP.md`.

If the roadmap differs, follow the approved roadmap and update this execution plan only through documented change control.

---

# 13. FEATURE IMPLEMENTATION CONTRACT

Every feature must follow:

```text
FR-XXX
   ↓
Design
   ↓
FEAT-XXX
   ↓
Implementation
   ↓
Unit Tests
   ↓
Integration / Contract Tests
   ↓
Security Review
   ↓
Documentation
   ↓
Release
```

Every material feature must have traceability.

---

# 14. PRE-IMPLEMENTATION CHECKLIST

Before coding a feature, identify:

```text
[ ] PRD requirement
[ ] Roadmap phase
[ ] Architecture components
[ ] Tech stack requirements
[ ] Refactory/module boundaries
[ ] TUI impact
[ ] API impact
[ ] Database impact
[ ] 9Router impact
[ ] Security impact
[ ] Observability impact
[ ] Test requirements
[ ] Rollback strategy
[ ] Documentation changes
```

If a required item cannot be determined, investigate before implementation.

---

# 15. DATABASE CONTRACT

Persistent state must have an explicit source of truth.

Database schema changes MUST use migrations.

Migration format:

```text
MIG-NNNN
```

Every migration must document:

- purpose;
- forward migration;
- compatibility;
- rollback;
- data risk;
- backup requirement;
- verification;
- deployment order.

Destructive migration without verified rollback/backup is prohibited.

---

# 16. API CONTRACT

Every production API endpoint must define:

```text
authentication
authorization
request validation
response schema
error schema
audit behavior
metrics/logging where appropriate
OpenAPI documentation
```

Do not expose secrets or unnecessary internal details.

API changes require documentation updates.

Breaking changes require explicit approval/change documentation.

---

# 17. SECURITY CONTRACT

Security is a release-blocking concern.

Review:

```text
Authentication
Authorization
API keys
Provider credentials
Proxy credentials
Secret storage
Secret exposure
SSRF
Arbitrary URL access
Command injection
Path traversal
Privilege escalation
Rate-limit bypass
Quota bypass
Audit integrity
Sensitive logging
Container isolation
Network boundaries
```

Never log:

- complete API keys;
- provider secrets;
- proxy passwords;
- authorization headers;
- bearer tokens;
- sensitive request content.

---

# 18. OBSERVABILITY CONTRACT

Production-critical behavior must be observable.

Where applicable document and implement:

```text
request count
success rate
error rate
latency
input tokens
output tokens
total tokens
provider health
model health
retry count
fallback count
circuit state
rate-limit events
quota events
budget events
proxy health
9Router health
```

Every important operational mutation should have an audit event.

---

# 19. TESTING CONTRACT

A feature is not complete because it compiles.

Required test layers as applicable:

```text
Unit
Component
Contract
Integration
E2E
Security
Performance
Regression
Docker Smoke
```

Critical path:

```text
Client
  ↓
ProxyGateway
  ↓
9Router Adapter
  ↓
9Router
  ↓
Provider
  ↓
Upstream
```

must be covered by integration/E2E testing.

---

# 20. FAILURE MODE CONTRACT

External dependency failures must have defined behavior.

At minimum consider:

```text
9Router unavailable
Provider unavailable
Model unavailable
Proxy unavailable
Database unavailable
Redis unavailable
Timeout
429
5xx
Invalid API key
Unauthorized
Rate limit exceeded
Quota exceeded
Budget exceeded
Circuit open
```

Each failure should have:

```text
classification
user-visible behavior
retry behavior
fallback behavior
logging
metrics
audit where appropriate
```

---

# 21. DOCKER CONTRACT

The complete stack must be runnable using Docker.

The implementation must address:

```text
reproducible builds
health checks
service dependencies
networking
persistent volumes
configuration
secrets
restart behavior
graceful shutdown
backup
restore
upgrade
rollback
```

A clean environment must be able to start the documented stack using the documented procedure.

---

# 22. CONFIGURATION CONTRACT

Never hardcode secrets.

Separate:

```text
application configuration
runtime configuration
development configuration
production configuration
secret configuration
```

Every new configuration item must document:

```text
name
type
default
required/optional
secret/non-secret
validation
reload behavior
restart requirement
```

---

# 23. CHANGE CONTROL

### Normal code change

Implement → test → document.

### Architecture change

Create ADR:

```text
docs/adr/ADR-NNNN-short-title.md
```

### Requirement change

Update:

```text
01-PRD.md
```

### Roadmap change

Update:

```text
02-ROADMAP.md
```

### Architecture change

Update:

```text
03-ARCHITECTURE.md
```

and create ADR where appropriate.

### Technology change

Update:

```text
04-TECH-STACK.md
```

### Refactoring boundary change

Update:

```text
05-REFACTORY-PLAN.md
```

### TUI behavior/design change

Update:

```text
06-TUI-UX-SPEC.md
```

### Implementation/test/release record

Update:

```text
07-IMPLEMENTATION-DOCUMENTATION.md
```

---

# 24. NO SILENT ARCHITECTURAL DRIFT

The agent MUST NOT:

- introduce a new service without documenting it;
- merge services that are architecturally separated without justification;
- bypass the 9Router adapter;
- bypass Management API from TUI;
- move persistence responsibility without documenting it;
- change the routing model silently;
- change authentication behavior silently.

---

# 25. REFACTORING CONTRACT

Before refactoring:

```text
Understand
 ↓
Map dependencies
 ↓
Protect existing behavior with tests
 ↓
Refactor incrementally
 ↓
Run regression tests
 ↓
Document
```

Do not perform destructive rewrites unless required and justified.

Avoid changing behavior during a pure refactor.

---

# 26. DEPENDENCY MANAGEMENT

Before adding a dependency:

1. verify the requirement;
2. check whether the current stack already solves it;
3. evaluate security;
4. evaluate maintenance;
5. evaluate licensing;
6. evaluate container impact;
7. evaluate operational complexity;
8. document major additions.

Do not add libraries solely for convenience.

---

# 27. DOCUMENTATION CONTRACT

Documentation is part of implementation.

Every material change must update the appropriate documentation.

Required documentation categories:

```text
Architecture
Feature
API
Database
Testing
Security
Operations
Release
Incident
ADR
```

Documentation must use stable identifiers:

```text
FR-XXX
FEAT-XXX
ADR-NNNN
MIG-NNNN
TEST-YYYYMMDD
INC-YYYYMMDD-NNN
```

---

# 28. TUI SCREEN DOCUMENTATION CONTRACT

Every implemented screen must document:

```text
Screen ID
Purpose
Route
API dependency
Roles
Keyboard shortcuts
Mutation actions
Validation
Confirmation
Error state
Empty state
Loading state
Responsive behavior
Audit behavior
```

The implementation must match `06-TUI-UX-SPEC.md`.

---

# 29. RELEASE CONTRACT

Before release:

```text
[ ] PRD requirements verified
[ ] Roadmap phase complete
[ ] Architecture synchronized
[ ] Tech stack synchronized
[ ] Refactory synchronized
[ ] TUI synchronized
[ ] API documented
[ ] DB migrations documented
[ ] 9Router compatibility verified
[ ] Security review completed
[ ] Tests pass
[ ] Docker smoke test passes
[ ] Backup verified
[ ] Restore verified
[ ] Rollback documented
[ ] Release notes prepared
[ ] Known issues documented
```

---

# 30. DEFINITION OF DONE

A feature is DONE only when:

```text
Implementation
+
Tests
+
Documentation
+
Security review
+
Observability
+
Audit
+
Rollback knowledge
+
Traceability
```

are complete.

"Code exists" is NOT the Definition of Done.

---

# 31. PHASE COMPLETION REPORT

At the end of every roadmap phase, produce:

```markdown
# Phase Completion Report

## Phase
<phase>

## Status
Complete / Partial / Blocked

## Requirements Implemented
- FR-XXX
- FR-XXX

## Features
- FEAT-XXX

## Implementation
- ...

## API
- ...

## Database
- ...

## 9Router
- ...

## TUI
- ...

## Security
- ...

## Tests
- ...

## Documentation
- ...

## Known Issues
- ...

## Deviations
- ...

## ADRs
- ...

## Migrations
- ...

## Next Phase
- ...
```

---

# 32. STOP CONDITIONS

The agent MUST STOP the affected implementation path when:

1. authoritative documents materially conflict;
2. security boundary is unclear;
3. required behavior is unknown;
4. 9Router behavior materially differs from assumptions;
5. destructive database migration has no safe rollback;
6. implementation requires bypassing an architectural boundary;
7. API breaking change is required without approval;
8. data integrity may be compromised;
9. implementation would expose secrets;
10. required infrastructure is unavailable and cannot be safely mocked.

Do not silently make high-impact assumptions.

---

# 33. SAFE ASSUMPTION RULE

Minor implementation details may be chosen by engineering judgment when:

- they do not conflict with documentation;
- they do not affect public behavior;
- they do not affect security;
- they do not affect persistence;
- they do not affect architecture;
- they are covered by tests.

Material decisions must be documented.

---

# 34. AGENT EXECUTION MODE

The agent must operate in this cycle:

```text
READ
 ↓
UNDERSTAND
 ↓
CROSS-REFERENCE
 ↓
PLAN
 ↓
IMPLEMENT
 ↓
TEST
 ↓
REVIEW
 ↓
DOCUMENT
 ↓
VERIFY
 ↓
REPORT
 ↓
NEXT PHASE
```

Never:

```text
Read one file
 ↓
Guess architecture
 ↓
Generate everything
```

---

# 35. FINAL PROJECT QUALITY GATE

Before declaring ProxyGateway Enterprise production-ready:

```text
[ ] Complete documentation read
[ ] Documentation/code consistency verified
[ ] All approved PRD requirements implemented
[ ] Roadmap completed
[ ] Architecture implemented as documented
[ ] Technology stack compliant
[ ] Refactory boundaries respected
[ ] 9Router adapter implemented
[ ] 9Router compatibility tested
[ ] Provider management operational
[ ] Model management operational
[ ] Routing operational
[ ] Policies operational
[ ] API keys operational
[ ] Limits operational
[ ] Quotas operational
[ ] Budgets operational
[ ] Proxy management operational
[ ] Retry/fallback operational
[ ] Circuit breaker operational
[ ] Observability operational
[ ] Audit operational
[ ] TUI compliant with UX specification
[ ] Security review completed
[ ] Unit tests passing
[ ] Integration tests passing
[ ] E2E tests passing
[ ] Security tests passing
[ ] Performance tests passing where required
[ ] Docker deployment verified
[ ] Backup verified
[ ] Restore verified
[ ] Rollback verified/documented
[ ] Documentation synchronized
[ ] Release notes complete
```

Only after all applicable gates pass may the project be declared production-ready.

---

# 36. FINAL INSTRUCTION TO THE AGENT

You are not being asked merely to write code.

You are responsible for implementing a coherent enterprise system according to an approved engineering contract.

Treat the documentation as the project's specification.

Preserve the architecture.

Respect the 9Router integration boundary.

Respect the TUI UX specification.

Protect data and secrets.

Test behavior.

Document decisions.

Maintain traceability.

Do not silently guess.

Do not silently change architecture.

Do not skip phases.

Do not declare completion based only on compilation.

When uncertain about a material decision, stop and surface the issue.

When implementation reveals that the documentation is incomplete or incorrect, document the discrepancy and use formal change control.

The final result must be:

```text
Correct
Secure
Tested
Observable
Maintainable
Docker-native
Documented
Traceable
Operationally safe
```

This contract remains active for the entire lifecycle of the project.
