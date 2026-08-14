# FEAT-009 — Outbound Proxy Profiles & Credential Protection

## Requirement Mapping
- **PRD:** FR-011 (Proxy Profiles)
- **Roadmap Phase:** Phase 6 (Proxy Management)
- **Architecture:** Section 13 (Proxy Architecture)
- **Tech Stack:** Section 16 (Security Stack)

## Objective
Manage and health-check outbound proxy profiles (`DIRECT`, `HTTP`, `HTTPS`, `SOCKS5`) for routing upstream traffic while ensuring that proxy credentials (usernames/passwords) are strictly redacted and never exposed in logs, API responses, TUI displays, or audit records.

## Scope
1. **Proxy Profiles Registry (`internal/proxy`)**:
   - Supported types: `DIRECT`, `HTTP`, `HTTPS`, `SOCKS5`.
   - Attributes: `ID`, `Name`, `Type`, `Host`, `Port`, `SecretRef` (or credential reference), `Enabled`, `CreatedAt`, `UpdatedAt`.
   - Zero credential leakage: JSON serialization masks or strips secrets (`json:"-"`).
2. **Proxy Health Probing**:
   - `CheckHealth(ctx, profile)` validates outbound connectivity and latency.
3. **Transport Builder**:
   - `BuildTransport(profile)` configures `*http.Transport` with standard dialer or SOCKS5 dialer.
