# FEAT-011 — Enterprise Observability, Prometheus Metrics & Audit Trail

## Requirement Mapping
- **PRD:** FR-013 (Observability & Prometheus), FR-014 (Audit Trail)
- **Roadmap Phase:** Phase 8 (Observability)
- **Architecture:** Section 8 (Telemetry & Metrics), Section 14 (Audit Logging)

## Objective
Provide comprehensive observability with Prometheus metrics on `/metrics`, request latency histograms, token counters, error tracking, and secure structured audit event logging with zero secret/PII leakage.

## Scope
1. **Prometheus Metrics (`internal/telemetry`)**:
   - `pg_http_requests_total`: Counter (labels: `method`, `path`, `status`).
   - `pg_request_duration_seconds`: Histogram (labels: `path`, `model`).
   - `pg_tokens_total`: Counter (labels: `key_id`, `model`, `type`).
   - `pg_upstream_errors_total`: Counter (labels: `provider`, `code`).
   - `GET /metrics`: Standard Prometheus scrapable text format endpoint.
2. **Audit Event Logging (`internal/audit`)**:
   - Event types: `AUTH_SUCCESS`, `AUTH_FAILURE`, `POLICY_DENY`, `RATE_LIMITED`, `ROUTE_RESOLVED`, `CONFIG_CHANGED`.
   - Structural record: `ID`, `Timestamp`, `Actor`, `EventType`, `Target`, `Status`, `Metadata`.
   - Credential redaction: PII, raw tokens, and passwords strictly redacted.
3. **HTTP Middleware Integration**:
   - Telemetry middleware automatically tracking latency and status codes.
