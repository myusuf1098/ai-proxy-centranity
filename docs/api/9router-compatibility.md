# 9Router API Compatibility Matrix

**Document ID:** `PG-COMPAT-001`  
**Target 9Router Image:** `decolua/9router:latest`  
**Compatibility Baseline:** Verified 2026-08-14  

---

## 1. Endpoint Compatibility Matrix

| Endpoint | Method | Role | Status | Notes |
|---|---|---|---|---|
| `/api/health` | GET | Health verification | Supported / Verified | Returns `{"ok":true}` |
| `/v1/models` | GET | Model & Combo discovery | Supported / Verified | Requires `Authorization: Bearer <token>` |
| `/v1/chat/completions` | POST | OpenAI-compatible chat & SSE streaming | Supported | Upstream LLM execution |
| `/api/v1/providers` | GET | Provider status (where enabled) | Planned | Optional management probe |

---

## 2. Error Translation Contract

| Upstream Status / Error | Normalization Code | User-Facing Action |
|---|---|---|
| 401 / "API key required" | `UPSTREAM_AUTH_ERROR` | Internal config error / check `PG_NINEROUTER_API_KEY` |
| 429 / Rate limit | `UPSTREAM_RATE_LIMIT` | Trigger circuit breaker / provider failover |
| 500 / 502 / 503 / 504 | `UPSTREAM_UNAVAILABLE` | Trigger fallback model or provider retry |
| Connection Refused / Timeout | `UPSTREAM_UNREACHABLE` | Mark 9Router health as unhealthy |
