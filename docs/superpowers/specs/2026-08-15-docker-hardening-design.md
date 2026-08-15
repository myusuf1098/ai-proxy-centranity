# Docker Hardening — ProxyGateway Enterprise (ai-proxy-centranity)

> **Date**: 2026-08-15
> **Status**: Draft — awaiting review
> **Related**: `/opt/rys-centranity` (sibling stack, hardening reference), `docs/KERNEL-LOCK.md`, `docs/ORANGE-PI5-SYSTEM-HEALTH-SECURITY-GUIDE.md`

## 1. Context & Constraints

### Host
Orange Pi 5 Max — RK3588, 16GB RAM / 512GB NVMe, Ubuntu 24.04 (Noble, arm64), kernel `6.1.0-1025-rockchip` (locked).

### Boot Incident — Root Cause (do NOT touch kernel)
Boot hang on OP5 was traced to **rockchip kernel package auto-upgrade / initramfs mismatch** (memory record `mem_msry32th`, `docs/KERNEL-LOCK.md`). Mitigation already live:
- `apt-mark hold` on 6 rockchip kernel packages.
- `linux-*` blacklist in `/etc/apt/apt.conf.d/50unattended-upgrades`.

**Hard constraint:** this plan makes NO kernel, initramfs, bootloader, or boot config changes. No `apt` kernel ops, no recompile, no DTB edits. Docker/userland only.

### Reference stack (rys-centranity) — already hardened, use as pattern
- `mem_limit` on every service (compose non-swarm — `deploy.resources` is ignored by `docker compose up`).
- `env_file: .env` — no secrets hardcoded in compose files.
- Healthcheck on every service; `depends_on` with `condition: service_healthy`.
- Image tags major/digest-pinned.
- Single default network (no explicit networks) for rys stack.
- Docker daemon global log rotation (`/etc/docker/daemon.json`, max-size 10m / max-file 3) + weekly auto-prune (`/etc/cron.weekly/docker-prune`) — server-wide, already live.

## 2. Current State (ai-proxy-centranity)

`docker-compose.yml`:
- `version: '3.8'` — deprecated, remove.
- 3 services: `postgres` (16-alpine), `redis` (7-alpine), `proxygateway-api` (build).
- Networks: `frontend`, `backend` (`internal: false`), `centranity_shared` (`external: true` → `rys-centranity_default`).
- Secrets hardcoded as defaults: `PG_DB_PASS:-pg_centranity_secure_2026`, `PG_ADMIN_TOKEN:-pg_admin_centranity_token_2026`.
- No `mem_limit` anywhere. No per-service healthcheck on API (Dockerfile has one; OK).
- Named volumes: postgres + redis data.

`deployments/docker/Dockerfile.api`:
- Multi-stage golang:alpine → alpine:3.20. Non-root user `appuser:10001`. `CGO_ENABLED=0`. HEALTHCHECK present. Already good.

## 3. Changes

### 3.1 `docker-compose.yml`
1. **Remove `version: '3.8'`** (top-level) — deprecated in Compose v2.
2. **`backend` → `internal: true`** — postgres/redis need no host ingress (no published ports). Matches least-privilege.
3. **Secrets via `${VAR:?...}` required** — replace `:-default` on `PG_DB_PASS` and `PG_ADMIN_TOKEN` so compose fails fast when `.env` missing them. Non-secret defaults (`PG_DB_USER`, `PG_DB_NAME`) stay as `:-`.
4. **`mem_limit`** on all 3 services (match rys pattern): postgres 1g, redis 256m, api 512m. Use `mem_limit` not `deploy.resources` — compose non-swarm.
5. **Healthcheck on `proxygateway-api`** at compose level (wget `/health/live`, mirroring Dockerfile) + `depends_on` conditions already correct.
6. **Network hygiene**: keep 3-network design (frontend/backend/centranity_shared) — this is correct and already the multi-network + shared-network pattern. No change.

### 3.2 `.env.example` / `.env`
- Note required vars (`PG_DB_PASS`, `PG_ADMIN_TOKEN`) in `.env.example` comments. Do NOT commit `.env`.

### 3.3 Tests
`tests/deployment/deployment_test.go`:
- Extend `TestDeployment_DockerComposeArtifacts` to assert: no `version:` line, `backend` has `internal: true`, `mem_limit` present on each service, no `:-<secret>` default on `PG_DB_PASS`/`PG_ADMIN_TOKEN`.
- Keep existing asserts (services present, Dockerfile multi-stage).

### 3.4 Docs
- `docs/CHANGELOG.md` — add hardening entry.
- `docs/operations/deployment-runbook.md` — note `mem_limit`, required env vars, internal backend network.
- `docs/operations/troubleshooting.md` — add row: "Compose fails with `variable not set`" → set `PG_DB_PASS`/`PG_ADMIN_TOKEN` in `.env`.
- `docs/features/FEAT-012-deployment-operations.md` — update if behavior contract changes.

## 4. Out of Scope
- **Kernel / boot / initramfs / bootloader / DTB** — locked, not touched (boot incident).
- rys-centranity stack changes — already hardened; ai-proxy only.
- Docker daemon.json / cron prune — already server-wide live.
- Swarm/K8s migration — YAGNI, single host.
- Port layout, image base changes, network topology redesign.

## 5. Rollback
- Revert `docker-compose.yml` via git (`git revert` of the commit). Secrets: restore prior `.env` (gitignored, keep backup before edit). `internal: true` on backend is cosmetic — no data impact; postgres/redis stay on the same bridge.

## 6. Verification
1. `docker compose config` — validate compose parses, no `version:` warning, required-var expansion.
2. `make test` — `tests/deployment` assertions pass (RED → GREEN).
3. `make build` — binary builds.
4. `docker compose up -d` — all 3 containers healthy (`docker compose ps`).
5. `curl http://127.0.0.1:8088/health/ready` → 200.
6. Confirm no secret string `pg_centranity_secure_2026` / `pg_admin_centranity_token_2026` remains in compose.
