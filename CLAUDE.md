# CLAUDE.md — TickHook

Project guidelines for Claude Code working on the TickHook codebase.

**Author:** Dani (cr0hn)

---

## Project Status: V1 Complete

TickHook V1 is fully implemented and production-ready. The project includes:

- Complete webhook scheduler with one-shot and recurring jobs
- REST API with Bearer token authentication
- Redis backend for persistence and scheduling
- Worker pool with global and per-domain concurrency limits
- Exponential backoff retry logic with jitter
- Comprehensive test suite
- Docker support with multi-stage builds
- GitHub Actions CI/CD pipeline
- Performance benchmarks and documentation

---

## Quick Reference

### Build & Test

```bash
# Build
go build ./cmd/tickhook

# Test
go test ./...

# Run
./tickhook --redis-url redis://localhost:6379 --auth-token secret

# Docker
docker build -t tickhook .
docker-compose up
```

### Project Structure

```
cmd/tickhook/          # CLI entrypoint
internal/
  config/              # CLI flags parsing
  httpapi/             # REST API (handlers, middleware)
  store/               # Redis store implementation
  scheduler/           # Polling loop
  executor/            # Worker pool, concurrency control
  model/               # Job structs, DTOs, validation
  util/                # Helpers (backoff, domain extraction)
docs/
  API.md               # API documentation
  ARCHITECTURE.md      # System design
  DEPLOYMENT.md        # Deployment guides
  INTERNALS.md         # Redis data model, execution details
  PRD.md               # Original requirements
  img/                 # Performance graphs
```

---

## Technical Specifications

### Requirements

- **Go 1.22+** (uses new ServeMux route patterns)
- **Redis 6+**

### CLI Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--redis-url` | Yes | - | Redis connection URL |
| `--auth-token` | Yes | - | Bearer token for API auth |
| `--bind` | No | `0.0.0.0:8080` | HTTP bind address |
| `--namespace` | No | `tickhook` | Redis key prefix |
| `--poll-ms` | No | `200` | Scheduler poll interval |
| `--batch` | No | `200` | Jobs fetched per poll |
| `--max-inflight` | No | `200` | Max concurrent webhooks |
| `--max-per-domain` | No | `5` | Max concurrent per domain |
| `--default-timeout-ms` | No | `5000` | Default webhook timeout |
| `--log-level` | No | `info` | Log level |

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/jobs/one-shot` | Create one-shot job |
| POST | `/v1/jobs/recurring` | Create recurring job |
| GET | `/v1/jobs/{job_id}` | Get job details |
| DELETE | `/v1/jobs/{job_id}` | Cancel/delete job |
| GET | `/health` | Health check (no auth) |

All responses include `Server: TickHook/1.0` header.

### Redis Keys

- `{namespace}:schedules` — ZSET (score = due_at_ms)
- `{namespace}:job:{job_id}` — HASH (job metadata)

---

## Development Guidelines

### Code Style

- Use stdlib over external frameworks when possible
- Structured logging with `slog` (include `job_id` in execution logs)
- Consistent JSON error responses: `{"error": "code", "message": "description"}`
- Validate all inputs at API boundary

### Testing

- Unit tests in `*_test.go` files alongside source
- Benchmark tests in `*_bench_test.go`
- Integration tests skip if Redis unavailable

### Documentation

- **README.md** — User-focused (QuickStart, Installation, Configuration, API)
- **docs/** — Technical details (Architecture, Internals, Deployment)
- Keep README concise; link to docs for deep dives

---

## V1 Limitations (Documented)

1. **Single process** — No HA, crash between ZREM and completion loses job
2. **At-least-once** — Webhooks may be delivered multiple times
3. **No cron** — Fixed intervals only
4. **UTC only** — No timezone handling

---

## V2 Roadmap (Not Implemented)

Future enhancements planned:
- High availability with lease-based job claiming
- Lua scripts for atomic claim operations
- Running set for crash recovery
- Prometheus metrics endpoint

---

## Performance Benchmarks

| Metric | Value |
|--------|-------|
| Binary size | 6.2 MB |
| Memory (idle) | ~8 MB |
| Memory (load) | ~17 MB |
| API throughput | 46K req/s |
| Job creation | 20K req/s |

Run benchmarks:
```bash
go test -bench=. -benchmem ./...
```

---

## File Locations

| Purpose | Location |
|---------|----------|
| Main entry | `cmd/tickhook/main.go` |
| Config parsing | `internal/config/config.go` |
| API handlers | `internal/httpapi/handlers.go` |
| Middleware | `internal/httpapi/middleware.go` |
| Redis store | `internal/store/redis.go` |
| Scheduler | `internal/scheduler/scheduler.go` |
| Executor | `internal/executor/executor.go` |
| Job model | `internal/model/job.go` |
| Utilities | `internal/util/util.go` |
| Dockerfile | `Dockerfile` |
| CI/CD | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
