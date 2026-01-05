# TickHook

[![CI](https://github.com/cr0hn/tickhook/workflows/CI/badge.svg)](https://github.com/cr0hn/tickhook/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/cr0hn/tickhook)](https://goreportcard.com/report/github.com/cr0hn/tickhook)
[![Go Reference](https://pkg.go.dev/badge/github.com/cr0hn/tickhook.svg)](https://pkg.go.dev/github.com/cr0hn/tickhook)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Image](https://img.shields.io/badge/docker-ghcr.io%2Fcr0hn%2Ftickhook-blue)](https://github.com/cr0hn/tickhook/pkgs/container/tickhook)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://go.dev/)
[![Redis](https://img.shields.io/badge/redis-6+-red.svg)](https://redis.io/)

A lightweight, self-hosted webhook scheduler written in Go.

TickHook allows you to schedule HTTP webhooks to be executed at a specific time (one-shot) or repeatedly at fixed intervals (recurring). It uses Redis as its only backend for persistence and scheduling.

## Table of Contents

- [What is TickHook](#what-is-tickhook)
- [Non-Goals](#non-goals)
- [Architecture Overview](#architecture-overview)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Reference](#api-reference)
- [Execution Model](#execution-model)
- [Retry Semantics](#retry-semantics)
- [Idempotency](#idempotency)
- [Redis Data Model](#redis-data-model)
- [Limitations](#limitations)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

## What is TickHook

TickHook is a minimal webhook scheduler that:

- Executes HTTP webhooks at scheduled times
- Supports one-shot (execute once) and recurring (execute repeatedly) jobs
- Provides retry logic with exponential backoff
- Controls concurrency globally and per-domain
- Runs as a single binary with Redis as the only dependency

TickHook is intentionally minimal. It focuses on doing one thing well: scheduling webhooks based on time.

## Non-Goals

TickHook V1 does **not** aim to provide:

- High availability (multiple active consumers)
- Exactly-once execution guarantees
- Cron expressions
- Timezone-aware scheduling (DST, calendars)
- Conditional workflows or branching logic
- UI or dashboard
- Cloud-managed services

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        TickHook Process                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │  HTTP API    │    │  Scheduler   │    │   Executor   │      │
│  │   Server     │    │    Loop      │    │  Worker Pool │      │
│  │              │    │              │    │              │      │
│  │ • Create job │    │ • Poll every │    │ • Concurrent │      │
│  │ • Get job    │    │   200ms      │    │   workers    │      │
│  │ • Delete job │    │ • Fetch due  │    │ • Per-domain │      │
│  │              │    │   jobs       │    │   limits     │      │
│  │              │    │ • Dispatch   │    │ • HTTP exec  │      │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘      │
│         │                   │                   │               │
│         └───────────────────┼───────────────────┘               │
│                             │                                   │
│                    ┌────────┴────────┐                         │
│                    │   Redis Store   │                         │
│                    │                 │                         │
│                    │ • ZSET schedules│                         │
│                    │ • HASH job:*    │                         │
│                    └─────────────────┘                         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │      Redis      │
                    └─────────────────┘
```

## Requirements

- Go 1.22 or later (for building from source)
- Redis 6 or later

## Installation

### Pre-built Binaries

Download the latest release from the [GitHub Releases](https://github.com/cr0hn/tickhook/releases) page:

```bash
# Linux (amd64)
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-linux-amd64
chmod +x tickhook-linux-amd64
./tickhook-linux-amd64 --help

# macOS (Apple Silicon)
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-darwin-arm64
chmod +x tickhook-darwin-arm64
./tickhook-darwin-arm64 --help

# Windows
Invoke-WebRequest -Uri https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-windows-amd64.exe -OutFile tickhook.exe
.\tickhook.exe --help
```

### Docker

```bash
docker pull ghcr.io/cr0hn/tickhook:latest

docker run -d \
  --name tickhook \
  -p 8080:8080 \
  ghcr.io/cr0hn/tickhook:latest \
  --redis-url redis://host.docker.internal:6379 \
  --auth-token your-secret-token
```

### From Source

```bash
git clone https://github.com/cr0hn/tickhook.git
cd tickhook
go build -o tickhook ./cmd/tickhook
```

### Using Go Install

```bash
go install github.com/cr0hn/tickhook/cmd/tickhook@latest
```

## Quick Start

### 1. Start Redis

Using Docker:

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

Or with docker-compose:

```yaml
# docker-compose.yml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  tickhook:
    build: .
    ports:
      - "8080:8080"
    environment:
      - REDIS_URL=redis://redis:6379
      - AUTH_TOKEN=your-secret-token
    command: >
      tickhook
      --redis-url redis://redis:6379
      --auth-token your-secret-token
    depends_on:
      - redis
```

### 2. Run TickHook

```bash
./tickhook --redis-url redis://localhost:6379 --auth-token your-secret-token
```

### 3. Create a One-Shot Job

```bash
curl -X POST http://localhost:8080/v1/jobs/one-shot \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "execute_at": "2026-01-15T10:00:00Z",
    "webhook": {
      "url": "https://httpbin.org/post",
      "method": "POST",
      "headers": {"X-Custom-Header": "value"},
      "body": {"message": "Hello from TickHook!"},
      "timeout_ms": 5000
    },
    "retry": {
      "max_attempts": 3,
      "backoff_base_ms": 1000
    }
  }'
```

Response:

```json
{"job_id": "550e8400-e29b-41d4-a716-446655440000"}
```

### 4. Create a Recurring Job

```bash
curl -X POST http://localhost:8080/v1/jobs/recurring \
  -H "Authorization: Bearer your-secret-token" \
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

### 5. Get Job Status

```bash
curl http://localhost:8080/v1/jobs/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer your-secret-token"
```

### 6. Cancel a Job

```bash
curl -X DELETE http://localhost:8080/v1/jobs/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer your-secret-token"
```

## Configuration

### CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--redis-url` | Yes | - | Redis connection URL (e.g., `redis://localhost:6379`) |
| `--auth-token` | Yes | - | Bearer token for API authentication |
| `--namespace` | No | `tickhook` | Redis key namespace |
| `--bind` | No | `0.0.0.0:8080` | HTTP server bind address |
| `--poll-ms` | No | `200` | Scheduler poll interval in milliseconds |
| `--batch` | No | `200` | Maximum jobs to fetch per poll cycle |
| `--max-inflight` | No | `200` | Maximum concurrent webhook executions (global) |
| `--max-per-domain` | No | `5` | Maximum concurrent executions per domain |
| `--default-timeout-ms` | No | `5000` | Default webhook timeout in milliseconds |
| `--log-level` | No | `info` | Log level (debug, info, warn, error) |

### Example

```bash
./tickhook \
  --redis-url redis://localhost:6379 \
  --auth-token my-secret-token \
  --namespace myapp \
  --bind 0.0.0.0:9090 \
  --poll-ms 100 \
  --max-inflight 500 \
  --max-per-domain 10 \
  --log-level debug
```

## API Reference

All endpoints require authentication via Bearer token in the `Authorization` header.

All responses include the header: `Server: TickHook/1.0`

### Create One-Shot Job

**POST** `/v1/jobs/one-shot`

Creates a job that executes once at the specified time.

**Request Body:**

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

**Response (201 Created):**

```json
{"job_id": "uuid"}
```

### Create Recurring Job

**POST** `/v1/jobs/recurring`

Creates a job that executes repeatedly at fixed intervals.

**Request Body:**

```json
{
  "start_at": "2026-01-15T10:00:00Z",
  "interval_ms": 3600000,
  "webhook": {
    "url": "https://example.com/hook",
    "method": "POST"
  },
  "retry": {
    "max_attempts": 3,
    "backoff_base_ms": 1000
  }
}
```

**Response (201 Created):**

```json
{"job_id": "uuid"}
```

### Get Job

**GET** `/v1/jobs/{job_id}`

Retrieves job details.

**Response (200 OK):**

```json
{
  "job_id": "uuid",
  "type": "one_shot",
  "due_at_ms": 1736935200000,
  "attempt": 0,
  "max_attempts": 3,
  "backoff_base_ms": 1000,
  "url": "https://example.com/hook",
  "method": "POST",
  "headers": {"X-Custom": "value"},
  "body": {"key": "value"},
  "timeout_ms": 5000,
  "created_at_ms": 1736848800000,
  "updated_at_ms": 1736848800000,
  "status": "pending"
}
```

### Delete Job

**DELETE** `/v1/jobs/{job_id}`

Cancels and removes a job.

**Response (200 OK):**

```json
{"deleted": true}
```

**Response (404 Not Found):**

```json
{"error": "not_found", "message": "Job not found"}
```

### Health Check

**GET** `/health`

Health check endpoint (no authentication required).

**Response (200 OK):**

```json
{"status": "ok"}
```

## Execution Model

### Polling Loop

The scheduler polls Redis every 200ms (configurable) using `ZRANGEBYSCORE` to find jobs where `due_at_ms <= now`. Due jobs are removed from the schedule and dispatched to the worker pool.

### Worker Pool

The executor maintains a pool of workers that execute webhooks concurrently. Two levels of concurrency control are applied:

1. **Global limit** (`--max-inflight`): Maximum total concurrent executions
2. **Per-domain limit** (`--max-per-domain`): Maximum concurrent executions to the same domain

This prevents overwhelming any single destination while maximizing throughput.

### Webhook Execution

For each job:

1. Load job metadata from Redis
2. Acquire global semaphore
3. Acquire per-domain semaphore
4. Execute HTTP request with configured timeout
5. Handle result (success, retry, or failure)

## Retry Semantics

### Retryable Errors

The following are considered retryable:

- Network errors (connection refused, timeout, etc.)
- HTTP 5xx responses (server errors)
- HTTP 429 (Too Many Requests)

### Non-Retryable Errors

The following cause immediate failure (no retry):

- HTTP 4xx responses (except 429)

### Backoff Calculation

Retries use exponential backoff with jitter:

```
delay = min(base_ms * 2^(attempt-1), 600000) + random(0, 250)
```

- Base delay: configurable via `backoff_base_ms` (default: 1000ms)
- Maximum delay: 10 minutes (600000ms)
- Jitter: 0-250ms random

### Failure Handling

- **One-shot jobs**: Deleted on success, retained on final failure for inspection
- **Recurring jobs**: On success, schedule next execution from previous `due_at` + `interval_ms`; on failure, apply retry logic for the current occurrence

## Idempotency

TickHook automatically adds idempotency headers to every webhook request:

```
X-Job-Id: <job_id>
Idempotency-Key: <job_id>
```

These headers help webhook receivers implement idempotent handling. Recommended practices for receivers:

1. Track received `Idempotency-Key` values
2. On duplicate key, return the same response without re-processing
3. Use a reasonable TTL for tracking (e.g., 24 hours)

## Redis Data Model

### Keys

All keys are prefixed with the configured namespace (default: `tickhook`).

| Key Pattern | Type | Description |
|-------------|------|-------------|
| `{ns}:schedules` | ZSET | Schedule sorted set (score = due_at_ms) |
| `{ns}:job:{job_id}` | HASH | Job metadata |

### Schedule ZSET

- **Member**: job_id (UUID string)
- **Score**: due_at_ms (Unix milliseconds)

### Job Hash Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Job ID |
| `type` | string | `one_shot` or `recurring` |
| `due_at_ms` | int64 | Next execution time (Unix ms) |
| `interval_ms` | int64 | Interval for recurring jobs |
| `attempt` | int | Current attempt number |
| `max_attempts` | int | Maximum retry attempts |
| `backoff_base_ms` | int64 | Base backoff delay |
| `url` | string | Webhook URL |
| `method` | string | HTTP method |
| `headers_json` | string | JSON-encoded headers |
| `body_json` | string | JSON-encoded body |
| `timeout_ms` | int | Request timeout |
| `created_at_ms` | int64 | Creation timestamp |
| `updated_at_ms` | int64 | Last update timestamp |
| `last_error` | string | Last error message |
| `last_http_code` | int | Last HTTP status code |
| `status` | string | `pending` or `failed` |

## Limitations

### V1 Single-Consumer Model

TickHook V1 is designed for single-process operation. There is a crash window between when a job is removed from the schedule (ZREM) and when it completes execution. If the process crashes during this window, the job may be lost.

For most use cases, this is acceptable:

- Jobs are idempotent (receivers handle duplicates)
- The crash window is very small (milliseconds to seconds)
- Critical jobs can be recreated by the caller if not acknowledged

### No Exactly-Once Guarantee

TickHook provides at-least-once semantics. In rare cases (network issues, crashes), a webhook might be delivered multiple times. Receivers should be idempotent.

### No Cron Expressions

Recurring jobs use fixed intervals in milliseconds. Cron expressions are not supported.

### No Timezone Awareness

All times are in UTC. Timezone conversions and DST handling must be done by the caller.

## Roadmap

### V2 (Planned)

- **High Availability**: Multiple consumers with lease-based job claiming
- **Lua Scripts**: Atomic claim operations to prevent duplicate execution
- **Running Set**: Track in-flight jobs for crash recovery
- **Metrics**: Prometheus endpoint for monitoring

## Contributing

Contributions are welcome. Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

MIT License. See [LICENSE](LICENSE) for details.

---

**Author:** Dani (cr0hn)
