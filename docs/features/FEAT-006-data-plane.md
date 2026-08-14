# FEAT-006 — OpenAI-Compatible Data Plane Core

## Requirement Mapping
- **PRD:** FR-001 (API Gateway), FR-004 (Model Input), FR-010 (Failover Scaffolding)
- **Roadmap Phase:** Phase 3 (Data Plane)
- **Architecture:** Section 6 (Request Flow), Section 9 (Model Resolution)
- **Tech Stack:** Section 4 (API Design)

## Objective
Provide the public, drop-in OpenAI-compatible Data Plane API on ProxyGateway. External AI clients can consume `POST /v1/chat/completions` and `GET /v1/models` transparently with support for standard JSON responses as well as real-time Server-Sent Events (SSE) streaming.

## Scope
1. Endpoints:
   - `GET /v1/models`: Returns OpenAI-format JSON list of available models resolved from NineRouterPort.
   - `POST /v1/chat/completions`: Receives OpenAI chat payload (`messages`, `model`, `stream`, etc.), validates structure, forwards to 9Router adapter, and streams/returns normalized response.
2. Streaming & Cancellation:
   - Preserves SSE stream (`text/event-stream`) without buffering or truncation.
   - Handles client context cancellation (closing connection gracefully if client disconnects).
3. Error Normalization:
   - Formats all gateway and upstream errors into standard OpenAI error schemas (`{"error":{"message":"...","type":"...","code":"..."}}`).

## Non-Scope
- Advanced API-key authentication/policy enforcement (Phase 4).
- Dynamic model aliases & multi-provider fallback routing (Phase 5).
