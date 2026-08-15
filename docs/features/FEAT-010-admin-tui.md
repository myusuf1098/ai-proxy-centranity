# FEAT-010 — Administrative Terminal UI (TUI) & Management API

## Requirement Mapping
- **PRD:** FR-012 (TUI Administration)
- **Roadmap Phase:** Phase 7 (Administrative TUI)
- **Architecture:** Section 2 (TUI Role), Section 14 (Security Boundaries)
- **UX Spec:** `06-TUI-UX-SPEC.md`

## Objective
Provide an enterprise-grade, keyboard-first, terminal user interface (TUI) for administration and operations. The TUI communicates exclusively through the ProxyGateway Management API (`/api/v1/*`) and never accesses PostgreSQL, Redis, or 9Router databases directly.

## Scope
1. **Management API Endpoints (`internal/api`)**:
   - `GET /api/v1/system`: System health, version, uptime, and component status.
   - `GET /api/v1/overview`: Real-time operational summary metrics.
   - `GET /api/v1/models`: List of registered and discovered models.
   - `GET /api/v1/keys`: List of active API keys (hashes/prefixes only).
   - `GET /api/v1/routes`: Current alias routes and circuit states.
   - `GET /api/v1/proxies`: Outbound proxy profiles.
2. **Bubble Tea TUI Framework (`internal/tui`)**:
   - 12 Screen views mapped to `06-TUI-UX-SPEC.md`:
     1. `OVERVIEW`
     2. `REQUESTS`
     3. `MODELS`
     4. `PROVIDERS`
     5. `API KEYS`
     6. `POLICIES`
     7. `ROUTING`
     8. `PROXIES`
     9. `USAGE`
     10. `AUDIT`
     11. `SYSTEM`
     12. `SETTINGS`
   - Navigation: Top tab bar wrapped 6 tabs per row (rows of 1-6 and 7-12, never spilling past the right edge), keyboard shortcuts (`Tab`, `1`-`9`, `0`, `-`, `=`, `q`, `r`, `Enter`, `Esc`), status footer.
   - Lip Gloss design tokens: High readability, professional monochromatic/subdued color palette, responsive terminal width/height adaptivity.
3. **Execution**:
   - `cmd/proxygateway-tui`: Interactive TUI application connecting to Gateway API.
