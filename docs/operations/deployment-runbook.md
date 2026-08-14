# Operational Runbook — Production Deployment

## Purpose
This runbook describes the end-to-end procedure for deploying, upgrading, and monitoring **ProxyGateway Enterprise** in containerized staging and production environments.

---

## 1. Prerequisites
- Docker Engine 24.0+ and Docker Compose v2.20+
- Host listening port `8088` (or custom `PG_SERVER_PORT`) available
- Upstream 9Router accessible at `http://127.0.0.1:20128` (or configured `PG_NINEROUTER_BASE_URL`)
- PostgreSQL 16 & Redis 7 containers configured with isolated named volumes

---

## 2. Environment Configuration
Copy and configure production secrets from `.env.example`:
```bash
cp .env.example .env
chmod 600 .env
# Edit .env and supply secure secrets:
# PG_DATABASE_PASSWORD=<strong-db-password>
# PG_NINEROUTER_API_KEY=<valid-9router-bearer-key>
# PG_ADMIN_ALLOWED_ORIGINS=https://admin.yourdomain.com
```

---

## 3. Deployment Steps

### Step 3.1: Build & Launch Containers
```bash
docker compose build --pull
docker compose up -d
```

### Step 3.2: Verify Service Health
Check health probes:
```bash
# Liveness Probe (Should return HTTP 200 {"status":"ok"})
curl -s -i http://127.0.0.1:8088/health/live

# Readiness Probe (Should return HTTP 200 with checkers status)
curl -s -i http://127.0.0.1:8088/health/ready
```

### Step 3.3: Verify Prometheus Metrics Exporter
```bash
curl -s http://127.0.0.1:8088/metrics | grep "pg_"
```

---

## 4. Zero-Downtime Rolling Update Procedure
1. Build updated Docker images:
   ```bash
   docker compose build proxygateway-api
   ```
2. Re-create API container gracefully:
   ```bash
   docker compose up -d --no-deps --build proxygateway-api
   ```
3. Verify that active in-flight requests complete within the 10-second graceful shutdown window.

---

## 5. TUI Operations
Launch the administrative terminal UI:
```bash
docker compose run --rm proxygateway-tui
# Or locally:
./bin/proxygateway-tui
```
