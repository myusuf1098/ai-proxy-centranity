# FEAT-007 — Policy Plane, API Keys, Rate Limiting & Quotas

## Requirement Mapping
- **PRD:** FR-007 (API Keys), FR-008 (Rate Limiting), FR-009 (Budget Controls)
- **Roadmap Phase:** Phase 4 (Policy Plane)
- **Architecture:** Section 7 (Policy Precedence), Section 11 (Rate-Limit Architecture)
- **Refactory Plan:** Section 5 (Policy Engine), Section 7 (Limiter Refactor), Section 8 (Quota Refactor)

## Objective
Enforce strict security, authentication, and governance across all incoming client requests. Every request to the Data Plane must be authenticated by a valid SHA-256 hashed API key and evaluated through a centralized `PolicyEngine` to enforce model/provider allowlists, denylists, rate limits (RPM/RPS/TPM), and token quotas.

## Scope
1. **API Key Authentication (`internal/auth`)**:
   - Extraction of Bearer token from `Authorization: Bearer <key>`.
   - SHA-256 hashing of incoming raw key and matching against stored key metadata.
   - Per-key attributes: `AllowedModels`, `DeniedModels`, `AllowedProviders`, `DeniedProviders`, `RPMLimit`, `RPSLimit`, `TPMLimit`, `DailyTokenQuota`, `MonthlyTokenQuota`.
   - Rejection with 401 Unauthorized for invalid, disabled, or expired keys.
2. **Policy Engine (`internal/policy`)**:
   - Centralized `Evaluate(ctx, key, requestContext)` method.
   - Precedence: Global Deny > Per-Key Deny > Per-Key Allow.
   - Model access policy enforcement (blocking unauthorized models with 403 Forbidden).
3. **Rate Limiting & Token Quotas (`internal/limiter`, `internal/quota`)**:
   - Atomic rate limiter supporting requests per second (RPS) and requests per minute (RPM).
   - Rate limit rejection with HTTP 429 Too Many Requests (`RATE_LIMITED`).
   - Daily and monthly token quota tracking interface.
4. **Integration with Data Plane**:
   - Auth & policy evaluation middleware protecting `/v1/chat/completions` and `/v1/models`.
