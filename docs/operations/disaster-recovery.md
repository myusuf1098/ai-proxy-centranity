# Operational Runbook — Disaster Recovery & Backup

## Purpose
This document provides recovery strategies for data loss, corrupted schemas, hardware failures, or datacenter outages affecting **ProxyGateway Enterprise**.

---

## 1. Backup Strategy

### 1.1 PostgreSQL State Backup
Automated daily logical backup via `pg_dump`:
```bash
docker exec -t pg_postgres pg_dump -U proxygateway proxygateway | gzip > /backups/proxygateway_$(date +%Y%m%d_%H%M%S).sql.gz
```

### 1.2 Redis Snapshotting

> **Note (2026-08-15):** Redis persistence is currently DISABLED in
> `docker-compose.yml` (`redis-server --save "" --appendonly no`). Rate-limit
> state is ephemeral by design (PROMT §17: do not back up transient Redis
> state as primary recovery). Re-enable AOF only if non-transient data moves
> to Redis.

---

## 2. Recovery Procedures

### 2.1 Restoring PostgreSQL from Backup
1. Stop API container:
   ```bash
   docker compose stop proxygateway-api
   ```
2. Drop and recreate database:
   ```bash
   docker exec -i pg_postgres psql -U proxygateway -c "DROP DATABASE proxygateway;"
   docker exec -i pg_postgres psql -U proxygateway -c "CREATE DATABASE proxygateway;"
   ```
3. Restore SQL dump:
   ```bash
   gunzip -c /backups/proxygateway_YYYYMMDD_HHMMSS.sql.gz | docker exec -i pg_postgres psql -U proxygateway -d proxygateway
   ```
4. Restart API container (auto-migration verifies schema):
   ```bash
   docker compose start proxygateway-api
   ```

### 2.2 Cold Start Recovery on Fresh Host
1. Clone repository to new host.
2. Restore `.env` configuration.
3. Start stack with `docker compose up -d`.
4. Restore latest PostgreSQL backup dump.
5. Verify `/health/ready` probe returns HTTP 200.
