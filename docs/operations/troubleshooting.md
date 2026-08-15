# Operational Runbook — Troubleshooting Guide

## Diagnostic Matrix

| Symptom | Probable Cause | Diagnostic Command | Remediation |
| :--- | :--- | :--- | :--- |
| **HTTP 502 Bad Gateway** | Upstream 9Router unreachable | `curl -i http://127.0.0.1:20128/api/health` | Verify `rys-ninerouter` container is running and healthy. Check `PG_NINEROUTER_API_KEY`. |
| **HTTP 429 Too Many Requests** | Client exceeded per-key RPM limit | Check `X-RateLimit-*` and `Retry-After` headers | Inspect key RPM limit or wait for sliding window cooldown. |
| **HTTP 403 Forbidden (`model_not_allowed`)** | Requested model blocked by Deny rule | Review API key allowed/denied models in TUI Screen 5/6 | Adjust model permission list for client API key. |
| **Circuit Breaker OPEN (`routing_failed`)** | Upstream model exceeded 5 consecutive 5xx errors | Check TUI Screen 7 (Routing) or `/metrics` | Wait 30s for automatic HALF_OPEN canary probe or resolve upstream failure. |
| **PostgreSQL Connection Refused** | PostgreSQL container starting / down | `docker logs pg_postgres` | Check volume permissions and verify database port 5432 is accessible. |
| **Redis Rate Limiter Errors** | Redis down or out of memory | `docker exec pg_redis redis-cli PING` | Check Redis AOF log and memory consumption. |
| **Compose fails with `PG_DB_PASS` / `PG_ADMIN_TOKEN` variable not set** | Missing required secret in `.env` | `grep -E 'PG_DB_PASS|PG_ADMIN_TOKEN' .env` | Set both in `.env` (`chmod 600 .env`) and re-run `docker compose up -d`. |

---

## Log Analysis
To view structured JSON error logs:
```bash
docker compose logs -f --tail=100 proxygateway-api | grep '"level":"ERROR"'
```
