# ProxyGateway Enterprise — Implementation Documentation Standard

**Document ID:** PG-DOC-007  
**Version:** 2.0  
**Status:** Mandatory Engineering Standard  
**Related Documents:** `01-PRD.md`, `02-ROADMAP.md`, `03-ARCHITECTURE.md`, `04-TECH-STACK.md`, `05-REFACTORY-PLAN.md`, `06-TUI-UX-SPEC.md`

---

## 1. Purpose

This document defines how every implementation decision, feature, change, deployment, migration, test, incident, and deviation must be documented.

Its purpose is to prevent:

- requirements becoming disconnected from code;
- architecture drift;
- undocumented behavior;
- TUI/backend mismatch;
- 9Router compatibility regressions;
- undocumented security exceptions;
- accidental feature duplication;
- incomplete release notes.

Documentation is part of the Definition of Done.

---

## 2. Documentation Hierarchy

The project follows this hierarchy:

```text
PRD
 |
 +--> Roadmap
 |
 +--> Architecture
 |      |
 |      +--> Tech Stack
 |      |
 |      +--> Refactory Plan
 |
 +--> TUI UX Specification
 |
 +--> Implementation Documentation
          |
          +--> ADRs
          +--> Feature Specs
          +--> API Contracts
          +--> Test Records
          +--> Release Notes
          +--> Change Log
          +--> Incident Records
```

A lower-level document MUST NOT contradict a higher-level approved requirement without an explicit change record.

---

## 3. Source-of-Truth Rules

### Product Requirement

Source of truth:

`01-PRD.md`

### Schedule / Release

Source of truth:

`02-ROADMAP.md`

### Architecture

Source of truth:

`03-ARCHITECTURE.md`

### Technology

Source of truth:

`04-TECH-STACK.md`

### Refactoring/module boundaries

Source of truth:

`05-REFACTORY-PLAN.md`

### UI/UX

Source of truth:

`06-TUI-UX-SPEC.md`

### Implementation evidence and traceability

Source of truth:

`07-IMPLEMENTATION-DOCUMENTATION.md`

---

## 4. Documentation Directory

Recommended repository structure:

```text
docs/
├── 01-PRD.md
├── 02-ROADMAP.md
├── 03-ARCHITECTURE.md
├── 04-TECH-STACK.md
├── 05-REFACTORY-PLAN.md
├── 06-TUI-UX-SPEC.md
├── 07-IMPLEMENTATION-DOCUMENTATION.md
│
├── adr/
│   ├── ADR-0001-control-data-plane.md
│   ├── ADR-0002-postgresql-redis.md
│   └── ...
│
├── features/
│   ├── FEAT-001-9router-integration.md
│   ├── FEAT-002-api-key-policy.md
│   └── ...
│
├── api/
│   ├── openapi.yaml
│   └── 9router-compatibility.md
│
├── testing/
│   ├── TEST-PLAN.md
│   ├── TEST-RESULTS.md
│   └── compatibility/
│
├── operations/
│   ├── RUNBOOK.md
│   ├── BACKUP-RESTORE.md
│   └── DISASTER-RECOVERY.md
│
├── security/
│   ├── THREAT-MODEL.md
│   └── SECURITY-TESTS.md
│
├── releases/
│   ├── CHANGELOG.md
│   └── RELEASE-NOTES.md
│
└── incidents/
```

---

## 5. Requirement Traceability

Every implemented P0/P1 feature receives an ID.

Format:

```text
FR-XXX
```

Example:

```text
FR-007 API key management
```

Implementation ticket:

```text
FEAT-012 API key policy
```

Traceability:

```text
FR-007
  |
  +--> FEAT-012
          |
          +--> implementation
          +--> tests
          +--> audit behavior
          +--> release
```

No production feature should exist without a traceability path.

---

## 6. Feature Documentation Template

Each feature file must contain:

```markdown
# FEAT-XXX — Feature Name

## Requirement Mapping
- PRD: FR-XXX
- Roadmap Phase: X
- Architecture: section X
- UX: section X

## Objective

## Scope

## Non-Scope

## User Flow

## API Contract

## Data Model

## Authorization

## Audit

## Failure Modes

## Metrics

## Tests

## Rollout

## Rollback

## Implementation Status

## Known Limitations
```

---

## 7. Architecture Decision Records

ADR filename:

```text
ADR-NNNN-short-title.md
```

Template:

```markdown
# ADR-NNNN — Title

## Status

Proposed | Accepted | Superseded | Rejected

## Context

## Decision

## Alternatives Considered

## Consequences

## Security Impact

## Operational Impact

## Migration / Rollback

## Related Documents
```

Architecture changes MUST have an ADR.

---

## 8. Change Classification

Every implementation change must be classified:

```text
FEATURE
BUGFIX
REFACTOR
SECURITY
PERFORMANCE
DOCUMENTATION
OPERATIONS
BREAKING
```

Breaking changes require:

- migration notes;
- compatibility impact;
- upgrade procedure;
- rollback plan.

---

## 9. Implementation Record

Every material implementation should record:

```text
Implementation ID
Date
Author
Change class
Requirement IDs
Files/modules affected
Database migration
API changes
9Router compatibility impact
TUI impact
Security impact
Tests
Deployment impact
Rollback
Status
```

---

## 10. 9Router Compatibility Record

Because 9Router is an integrated subsystem, every release must record:

```text
ProxyGateway version
9Router version
supported endpoints
management endpoint compatibility
authentication behavior
model/provider behavior
known incompatibilities
test result
```

Example:

```text
ProxyGateway: 1.0.0
9Router: 0.4.x

Contract:
- /v1/chat/completions       PASS
- /v1/models                 PASS
- provider management        PASS
- model management           PASS
- combo management           PASS
```

Exact endpoint support must be confirmed against the installed 9Router version before release.

---

## 11. API Change Documentation

Every API change must update:

- OpenAPI contract;
- request/response examples;
- authorization behavior;
- error codes;
- metrics;
- audit behavior;
- compatibility notes.

No undocumented endpoint may be considered production-ready.

---

## 12. Database Migration Documentation

Every migration receives:

```text
MIG-NNNN
```

Template:

```markdown
# MIG-NNNN — Description

## Purpose

## Forward Migration

## Backward Compatibility

## Rollback

## Data Risk

## Backup Requirement

## Verification Query

## Deployment Order
```

Destructive migrations require a backup/restore rehearsal.

---

## 13. Configuration Change Documentation

For new configuration fields:

```text
Name
Type
Default
Required
Secret
Validation
Reload behavior
Restart requirement
Compatibility
```

Example:

```text
rate_limit.rpm
type: integer
default: 60
required: false
secret: no
reload: yes
restart: no
```

---

## 14. TUI Documentation

Every TUI screen must have:

```text
Screen ID
Route
Purpose
Data source
Allowed roles
Keyboard shortcuts
Mutation actions
Validation
Confirmation requirements
Error states
Empty states
Responsive behavior
Audit behavior
```

Example:

```text
Screen: MODELS
API: /api/v1/models
Roles: admin/operator/read-only
Mutation: admin only
```

UI changes must reference `06-TUI-UX-SPEC.md`.

---

## 15. Test Documentation

Every completed feature must provide:

### Unit

Business-rule coverage.

### Component

Module interaction.

### Contract

9Router API compatibility.

### Integration

Docker service interaction.

### E2E

Client → ProxyGateway → 9Router → upstream.

### Security

Authorization, secret handling, SSRF, rate-limit bypass.

---

## 16. Test Result Template

```markdown
# TEST-YYYYMMDD — Feature/Release

## Environment

## Version Matrix

## Test Cases

| ID | Scenario | Expected | Actual | Result |
|----|----------|----------|--------|--------|

## Regression

## Security

## Performance

## Open Issues

## Sign-off
```

---

## 17. Observability Documentation

Every major feature must document:

```text
metrics
logs
audit events
health checks
alerts
dashboards
```

A feature without observability is incomplete when the feature can affect production traffic.

---

## 18. Security Documentation

Any security-sensitive feature must document:

- trust boundary;
- threat model impact;
- secrets;
- authorization;
- audit;
- abuse cases;
- rate-limit behavior;
- SSRF implications;
- data retention.

---

## 19. Release Documentation

Every release must have:

```text
Release version
Date
Highlights
Features
Bug fixes
Security changes
Breaking changes
Database migrations
9Router compatibility
Docker image changes
Known issues
Upgrade steps
Rollback steps
```

---

## 20. Incident Documentation

Incident filename:

```text
INC-YYYYMMDD-NNN.md
```

Required:

```text
Summary
Impact
Timeline
Detection
Root Cause
Contributing Factors
Mitigation
Recovery
Data Integrity
Security Impact
Corrective Actions
Preventive Actions
Lessons Learned
```

---

## 21. Change Log

Use:

```text
Added
Changed
Deprecated
Removed
Fixed
Security
```

Entries must reference feature/bug/ADR IDs where possible.

---

## 22. Definition of Done

A feature is NOT complete until:

- implementation exists;
- automated tests exist;
- docs are updated;
- traceability is recorded;
- metrics are documented;
- audit behavior is documented;
- security impact is reviewed;
- rollback is known;
- release note entry is prepared.

For UI features, `06-TUI-UX-SPEC.md` MUST also be updated.

---

## 23. Documentation Review Gates

### Design Review

Before coding:

- PRD mapping complete;
- architecture impact reviewed;
- UX impact reviewed;
- security impact reviewed.

### Implementation Review

Before merge:

- tests pass;
- docs updated;
- API/DB changes documented;
- ADR created if required.

### Release Review

Before production:

- compatibility checked;
- backups verified;
- rollback documented;
- release notes complete.

---

## 24. Drift Control

A quarterly or release-based documentation review should compare:

```text
Requirements
Architecture
Code
API
TUI
Deployment
Tests
Operations
```

Any mismatch becomes a tracked remediation item.

---

## 25. Cross-Document Linking Standard

Use relative Markdown links.

Examples:

```markdown
[PRD](../01-PRD.md)
[Architecture](../03-ARCHITECTURE.md)
[TUI UX](../06-TUI-UX-SPEC.md)
```

Avoid hard-coded local filesystem paths.

Use stable document IDs in prose:

```text
FR-007
ADR-0002
FEAT-012
MIG-0003
```

---

## 26. Master Documentation Rule

No document is allowed to redefine another document's source-of-truth domain.

```text
PRD        = what
Roadmap    = when
Architecture = how at system level
Tech Stack = what technology
Refactory  = how code is structured/refactored
TUI UX     = how it looks/behaves
Implementation Docs = evidence, traceability, and history
```

This rule is mandatory to prevent mismatch.
