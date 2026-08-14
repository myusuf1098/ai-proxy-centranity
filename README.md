# ProxyGateway Enterprise (v2.0)

[![Go Version](https://img.shields.io/badge/Go-1.26.6-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-Proprietary-red.svg)]()
[![Status](https://img.shields.io/badge/Status-Production%20Ready-green.svg)]()

**ProxyGateway Enterprise** is a high-performance, secure, OpenAI-compatible AI gateway and proxy orchestration platform engineered for enterprise multi-model governance, intelligent routing, and terminal administration.

---

## 🏛️ Architecture Overview

```
[ AI Client / SDK / App ] (e.g. Cursor, Claude Code, Python, LangChain)
          │
          │ Authorization: Bearer <sk-pg-xxx>
          ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        ProxyGateway Enterprise                         │
│                                                                        │
│   ┌───────────────────────────┐    ┌───────────────────────────────┐   │
│   │   Data Plane (:8088)      │    │    Control Plane (:8088)      │   │
│   │ • POST /v1/chat/completions│   │ • GET /api/v1/system          │   │
│   │ • GET  /v1/models         │    │ • GET /api/v1/overview        │   │
│   │ • GET  /metrics           │    │ • GET /api/v1/proxies         │   │
│   └─────────────┬─────────────┘    └───────────────┬───────────────┘   │
│                 │                                  │                   │
│   ┌─────────────▼──────────────────────────────────▼───────────────┐   │
│   │                      Core Engine Layer                         │   │
│   │ • Policy Engine (Deny > Allow Precedence)                      │   │
│   │ • Sliding-Window Rate Limiter & Token Quota Tracker            │   │
│   │ • Intelligent Routing Engine & Circuit Breaker (CLOSED/OPEN)   │   │
│   │ • Outbound Proxy Manager (DIRECT / HTTP / HTTPS / SOCKS5)      │   │
│   │ • Prometheus Telemetry & Structured Audit Logger               │   │
│   └─────────────────────────────┬──────────────────────────────────┘   │
│                                 │                                      │
└─────────────────────────────────┼──────────────────────────────────────┘
                                  │
                                  ├──────────────────────────────┐
                                  ▼                              ▼
                 ┌────────────────────────────────┐   ┌──────────────────────┐
                 │ 9Router Upstream (:20128)      │   │ PostgreSQL 16 & Redis│
                 │ (HaProxy / Model Aggregator)   │   │ (State & Limiter)    │
                 └────────────────────────────────┘   └──────────────────────┘
```

---

## ✨ Key Enterprise Features

- **OpenAI-Compatible Data Plane**: Full drop-in replacement for OpenAI SDKs (`/v1/chat/completions` with JSON payload and real-time SSE streaming, `/v1/models`).
- **9Router Integration**: Native HTTP adapter forwarding upstream requests to containerized 9Router with internal credential injection and header isolation.
- **Hierarchical Policy Plane**: Precedence enforcement (`Global Deny > Per-Key Deny > Per-Key Allow`) and sliding-window rate limiting (RPM/RPS) with HTTP 429 backoff.
- **Intelligent Routing & Aliases**: Symbolic model aliases (`coding`, `fast`, `reasoning`, `cheap`, `free`) with sub-microsecond resolution and automatic circuit breaker fallback.
- **Outbound Proxy Profiles**: Egress routing through `DIRECT`, `HTTP`, `HTTPS`, and `SOCKS5` proxies with strict secret masking (`json:"-"`).
- **Administrative Terminal UI**: 12-screen keyboard-first Bubble Tea & Lip Gloss dashboard communicating cleanly via Management API.
- **Full Observability**: Scrapable Prometheus metrics (`/metrics`), latency histograms, token counters, and structured audit logs.

---

## 🚀 Quick Start

### 1. Run with Docker Compose (Recommended)
```bash
# Clone and prepare configuration
cp .env.example .env

# Build and start all services
docker compose up -d

# Verify health probes
curl -s http://127.0.0.1:8088/health/live
curl -s http://127.0.0.1:8088/health/ready
```

### 2. Run Locally from Source
```bash
# Build binaries
make build

# Start Gateway API Server
./bin/proxygateway-api

# In a separate terminal, launch the Admin TUI
./bin/proxygateway-tui
```

---

## ⚙️ Environment Configuration

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PG_SERVER_PORT` | `8088` | HTTP listening port for Gateway API |
| `PG_NINEROUTER_BASE_URL` | `http://127.0.0.1:20128` | Upstream 9Router service endpoint |
| `PG_NINEROUTER_API_KEY` | - | 9Router upstream authorization key |
| `PG_DATABASE_HOST` | `127.0.0.1` | PostgreSQL database host |
| `PG_DATABASE_PORT` | `5432` | PostgreSQL database port |
| `PG_DATABASE_USER` | `proxygateway` | PostgreSQL username |
| `PG_DATABASE_PASSWORD` | `secret` | PostgreSQL password |
| `PG_REDIS_HOST` | `127.0.0.1` | Redis host for rate limiting |
| `PG_REDIS_PORT` | `6379` | Redis port |

---

## 📡 API Usage Examples

### Chat Completions with Alias Routing
```bash
curl -X POST http://127.0.0.1:8088/v1/chat/completions \
  -H "Authorization: Bearer sk-pg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "coding",
    "messages": [
      {"role": "user", "content": "Explain binary search in Go."}
    ],
    "stream": false
  }'
```

### Real-Time SSE Streaming
```bash
curl -N -X POST http://127.0.0.1:8088/v1/chat/completions \
  -H "Authorization: Bearer sk-pg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "fast",
    "messages": [
      {"role": "user", "content": "Write a short poem about clean code."}
    ],
    "stream": true
  }'
```

### Scrape Prometheus Metrics
```bash
curl -s http://127.0.0.1:8088/metrics
```

---

## 🖥️ Terminal User Interface (TUI)

Launch the interactive administrative dashboard:
```bash
./bin/proxygateway-tui
```

### Keyboard Shortcuts
- `Tab` / `Shift+Tab`: Navigate to next / previous screen
- `1` - `9`, `0`, `-`, `=`: Direct jump to screens 1–12
- `r`: Force telemetry and status refresh
- `q` / `Ctrl+C`: Exit dashboard

---

## 📚 Documentation Index

Detailed specifications, runbooks, and test reports are available in the `/docs` directory:
- [User Guide](docs/user-guide.md)
- [Operational Runbooks](docs/operations/deployment-runbook.md)
- [Disaster Recovery](docs/operations/disaster-recovery.md)
- [Troubleshooting Guide](docs/operations/troubleshooting.md)
- [Security Audit Report](docs/security/security-audit-report.md)
- [Benchmark Report](docs/benchmarks/benchmark-report.md)
- [Master Project Specifications](docs/specs/PROMT.md)
