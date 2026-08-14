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
Redis persistence uses Append-Only File (AOF) + RDB snapshots saved to `pg_redis_data` volume:
```bash
docker exec pg_redis redis-cli BGSAVE
```

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
