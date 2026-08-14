# ProxyGateway Enterprise Documentation v2

**Documentation Baseline:** 2.0  
**System:** Docker-native ProxyGateway + 9Router integration

## Document Map

| ID | File | Authority |
|---|---|---|
| 01 | `01-PRD.md` | Product requirements and acceptance |
| 02 | `02-ROADMAP.md` | Delivery phases and release gates |
| 03 | `03-ARCHITECTURE.md` | System boundaries and runtime architecture |
| 04 | `04-TECH-STACK.md` | Technology and engineering standards |
| 05 | `05-REFACTORY-PLAN.md` | Code/module refactoring strategy |
| 06 | `06-TUI-UX-SPEC.md` | TUI visual and interaction design |
| 07 | `07-IMPLEMENTATION-DOCUMENTATION.md` | Traceability, ADRs, tests, migrations, release/incident records |
| 00 | `PROMT.md` | Mandatory implementation contract and agent execution rules |

## Authority Model

```text
PRD
 ├── Roadmap
 ├── Architecture
 │    ├── Tech Stack
 │    └── Refactory Plan
 ├── TUI UX
 └── Implementation Documentation
```

Each document owns a distinct concern. A lower-level document must not silently redefine a higher-level decision.

## Required Traceability

Production features should follow:

```text
FR-XXX
  ↓
FEAT-XXX
  ↓
Code / API / DB / TUI
  ↓
TEST-YYYYMMDD
  ↓
Release
```

Architecture changes require an ADR. Database changes require a MIG record. Incidents require an INC record.

## TUI

The TUI is an API client of ProxyGateway. It must not access PostgreSQL, Redis, or 9Router data stores directly.

See `06-TUI-UX-SPEC.md`.

## Implementation Records

See `07-IMPLEMENTATION-DOCUMENTATION.md` for mandatory documentation templates and repository structure.
