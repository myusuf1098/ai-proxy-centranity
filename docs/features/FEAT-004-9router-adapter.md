# FEAT-004 — 9Router Adapter & Port Boundary

## Requirement Mapping
- **PRD:** FR-002 (9Router Integration)
- **Roadmap Phase:** Phase 2
- **Architecture:** Section 8 (9Router Integration Contract) & Section 20
- **Refactory Plan:** Section 3 (`NineRouterPort` Interface)

## Objective
Provide an isolated, decoupled adapter for communicating with the 9Router execution subsystem. The core ProxyGateway domain and TUI must interact only with `NineRouterPort` interface methods and never depend on 9Router internal implementations.

## Scope
1. Define `NineRouterPort` Go interface:
   - `ListModels(ctx context.Context) ([]ModelInfo, error)`
   - `CheckHealth(ctx context.Context) error`
   - `ForwardChatCompletion(ctx context.Context, body io.Reader, headers http.Header) (*http.Response, error)`
2. Implement `NineRouterHTTPAdapter`:
   - Configurable Base URL & API Key authentication injection (`Authorization: Bearer <token>`)
   - HTTP connection pooling and configurable timeout
   - Error normalization (translating upstream network/auth errors to standard gateway errors)
3. Automated Contract Tests verifying request/response compliance.

## Non-Scope
- Direct modification of 9Router internal databases or source code.
- Storing 9Router credentials in source control.

## Security & Boundary Controls
- 9Router upstream credentials (`PG_NINEROUTER_API_KEY`) are managed strictly in memory/env and never logged.
- The adapter strips client authentication headers and injects internal upstream credentials securely.
