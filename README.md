# TickHook

[![CI](https://github.com/cr0hn/tickhook/workflows/CI/badge.svg)](https://github.com/cr0hn/tickhook/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/cr0hn/tickhook)](https://goreportcard.com/report/github.com/cr0hn/tickhook)
[![Go Reference](https://pkg.go.dev/badge/github.com/cr0hn/tickhook.svg)](https://pkg.go.dev/github.com/cr0hn/tickhook)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Fcr0hn%2Ftickhook-blue)](https://github.com/cr0hn/tickhook/pkgs/container/tickhook)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://go.dev/)
[![Redis](https://img.shields.io/badge/redis-6+-red.svg)](https://redis.io/)

**A lightweight, self-hosted webhook scheduler written in Go.**

Schedule HTTP webhooks to fire at specific times or repeat at fixed intervals. Single binary. Redis backend. That's it.

---

## Performance

> TickHook is designed for high performance with minimal resource usage.

<p align="center">
  <img src="docs/img/performance_overview.png" alt="Performance Overview" width="800"/>
</p>

| Metric | Value |
|--------|-------|
| **Binary Size** | 6.2 MB |
| **Memory (idle)** | ~8 MB |
| **Memory (load)** | ~17 MB |
| **API Throughput** | 46K req/s |
| **Job Creation** | 20K req/s |

---

## Quick Start

### 1. Start Redis

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 2. Run TickHook

```bash
# Using Docker
docker run -d -p 8080:8080 ghcr.io/cr0hn/tickhook:latest \
  --redis-url redis://host.docker.internal:6379 \
  --auth-token my-secret-token

# Or download binary
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-linux-amd64
chmod +x tickhook-linux-amd64
./tickhook-linux-amd64 --redis-url redis://localhost:6379 --auth-token my-secret-token
```

### 3. Schedule a Webhook

```bash
# One-shot: fire once at a specific time
curl -X POST http://localhost:8080/v1/jobs/one-shot \
  -H "Authorization: Bearer my-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "execute_at": "2026-01-15T10:00:00Z",
    "webhook": {
      "url": "https://httpbin.org/post",
      "method": "POST",
      "body": {"message": "Hello!"}
    }
  }'

# Recurring: fire every hour
curl -X POST http://localhost:8080/v1/jobs/recurring \
  -H "Authorization: Bearer my-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "start_at": "2026-01-15T10:00:00Z",
    "interval_ms": 3600000,
    "webhook": {
      "url": "https://httpbin.org/post",
      "method": "POST",
      "body": {"type": "hourly_check"}
    }
  }'
```

---

## Installation

### Docker (Recommended)

```bash
docker pull ghcr.io/cr0hn/tickhook:latest
```

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/cr0hn/tickhook/releases):

```bash
# Linux
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-linux-amd64

# macOS (Apple Silicon)
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-darwin-arm64

# Windows
Invoke-WebRequest -Uri https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-windows-amd64.exe -OutFile tickhook.exe
```

### From Source

```bash
git clone https://github.com/cr0hn/tickhook.git
cd tickhook
go build -o tickhook ./cmd/tickhook
```

### Go Install

```bash
go install github.com/cr0hn/tickhook/cmd/tickhook@latest
```

---

## Configuration

### CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--redis-url` | Yes | - | Redis URL (e.g., `redis://localhost:6379`) |
| `--auth-token` | Yes | - | Bearer token for API auth |
| `--bind` | No | `0.0.0.0:8080` | HTTP bind address |
| `--namespace` | No | `tickhook` | Redis key prefix |
| `--poll-ms` | No | `200` | Scheduler poll interval (ms) |
| `--batch` | No | `200` | Jobs per poll cycle |
| `--max-inflight` | No | `200` | Max concurrent webhooks |
| `--max-per-domain` | No | `5` | Max concurrent per domain |
| `--default-timeout-ms` | No | `5000` | Webhook timeout (ms) |
| `--log-level` | No | `info` | Log level |

### Example

```bash
./tickhook \
  --redis-url redis://localhost:6379 \
  --auth-token secret \
  --bind 0.0.0.0:9090 \
  --max-inflight 500 \
  --max-per-domain 10
```

### Docker Compose

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  tickhook:
    image: ghcr.io/cr0hn/tickhook:latest
    ports:
      - "8080:8080"
    command: >
      --redis-url redis://redis:6379
      --auth-token your-secret-token
    depends_on:
      - redis
```

---

## API Reference

All endpoints require `Authorization: Bearer <token>` header.

### Create One-Shot Job

```http
POST /v1/jobs/one-shot
```

```json
{
  "execute_at": "2026-01-15T10:00:00Z",
  "webhook": {
    "url": "https://example.com/hook",
    "method": "POST",
    "headers": {"X-Custom": "value"},
    "body": {"key": "value"},
    "timeout_ms": 5000
  },
  "retry": {
    "max_attempts": 3,
    "backoff_base_ms": 1000
  }
}
```

**Response:** `{"job_id": "uuid"}`

### Create Recurring Job

```http
POST /v1/jobs/recurring
```

```json
{
  "start_at": "2026-01-15T10:00:00Z",
  "interval_ms": 3600000,
  "webhook": { "url": "...", "method": "POST" },
  "retry": { "max_attempts": 3 }
}
```

**Response:** `{"job_id": "uuid"}`

### Get Job

```http
GET /v1/jobs/{job_id}
```

### Cancel Job

```http
DELETE /v1/jobs/{job_id}
```

**Response:** `{"deleted": true}`

### Health Check

```http
GET /health
```

No authentication required.

---

## How It Works

**One-shot jobs** execute once at the specified time and are deleted on success. On failure, retries occur with exponential backoff until max attempts is reached.

**Recurring jobs** execute at fixed intervals (in milliseconds). The next execution is scheduled from the previous due time, not completion time, ensuring consistent intervals.

### Retry Behavior

| Scenario | Behavior |
|----------|----------|
| HTTP 2xx | Success - job completes |
| HTTP 5xx, 429 | Retry with backoff |
| HTTP 4xx (except 429) | Fail immediately |
| Network error | Retry with backoff |

Backoff formula: `base_ms * 2^(attempt-1)` + jitter (0-250ms), capped at 10 minutes.

### Concurrency Control

TickHook limits concurrent webhook executions to prevent overwhelming your targets:

- **Global limit** (`--max-inflight`): Total concurrent webhooks
- **Per-domain limit** (`--max-per-domain`): Per destination domain

### Idempotency

Every webhook includes automatic headers for idempotent handling:

```
X-Job-Id: <job_id>
Idempotency-Key: <job_id>
```

---

## Requirements

- Redis 6+
- Go 1.22+ (if building from source)

---

## Limitations

- **Single process**: V1 runs as one instance. If it crashes mid-execution, the job may be lost.
- **At-least-once**: Webhooks may be delivered more than once. Make receivers idempotent.
- **No cron**: Uses fixed intervals only, no cron expressions.
- **UTC only**: All times are UTC. Handle timezone conversion client-side.

---

## Documentation

- [API Reference](docs/API.md) - Complete API documentation
- [Architecture](docs/ARCHITECTURE.md) - System design and internals
- [Deployment](docs/DEPLOYMENT.md) - Docker, Kubernetes, systemd guides

---

## Roadmap

**V2 (Planned)**
- High availability with lease-based job claiming
- Prometheus metrics
- Multiple consumer support

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests
4. Submit a pull request

---

## License

MIT License. See [LICENSE](LICENSE).

---

**Author:** Dani (cr0hn)
