# CLAUDE.md — TickHook

You are Claude Code. Your job is to generate a complete, buildable Go project named **tickhook** from scratch, implementing the PRD in `docs/PRD.md`.

This repository is **open source** and must be **well documented**.

Author: Dani (cr0hn)

---

## 0) Hard requirements (do not skip)

- Language: **Go >= 1.21**
- Single binary that runs:
  - HTTP API server
  - scheduler loop (polling Redis)
  - webhook worker pool
- Backend: **Redis** (only persistence)
- REST API + token auth (static token configured via CLI flag)
- Poll Redis every **200ms** by default (configurable)
- Concurrency limits:
  - global max inflight
  - per-domain max inflight
- Retries with exponential backoff + jitter
- One-shot jobs:
  - deleted on success
  - retained on final failure for inspection
- Recurring jobs:
  - fixed interval in ms
  - schedule is computed from the previously scheduled due_at (not from completion time)
  - only one future execution exists at any time
- All HTTP responses must include:
  - `Server: TickHook/1.0`
- Open source documentation:
  - `README.md` with a real table of contents and all required sections
  - `LICENSE` (MIT or Apache-2.0) — choose MIT by default unless otherwise required
- Repository must build with `go build ./...` and tests pass with `go test ./...`

---

## 1) Project scope: V1 only (no HA)

This is **V1** and must NOT implement high-availability consumer logic (no leases, no running-set, no Lua claiming).
Assume a single process instance consumes and executes jobs.

However, the internal code structure should not block a future V2 that adds HA.
Keep claim logic isolated behind a small interface.

---

## 2) Repository layout (create these folders)

Use a standard, maintainable Go structure:

- `cmd/tickhook/`  
  - `main.go` (CLI entrypoint)
- `internal/config/`  
  - CLI flags parsing + validation
- `internal/httpapi/`  
  - router, middleware, handlers, DTOs
- `internal/store/`  
  - Redis store (keys, serialization, atomic updates)
- `internal/scheduler/`  
  - polling loop, batch fetch, dispatch
- `internal/executor/`  
  - worker pool, concurrency limits, domain semaphore, HTTP client
- `internal/model/`  
  - Job structs, enums, validation
- `internal/util/`  
  - time helpers, jitter, domain parsing, errors
- `docs/` (optional but nice)  
  - extra docs if needed
- root files:
  - `README.md`
  - `PRD_TickHook.md` (already exists)
  - `LICENSE`
  - `CHANGELOG.md` (optional; create minimal)
  - `.gitignore`

---

## 3) CLI specification

Binary: `tickhook`

Required flags:
- `--redis-url` string (example: `redis://localhost:6379`)
- `--auth-token` string (non-empty)

Optional flags (provide defaults):
- `--namespace` string (default: `tickhook`)
- `--bind` string (default: `0.0.0.0:8080`)
- `--poll-ms` int (default: 200)
- `--batch` int (default: 200)
- `--max-inflight` int (default: 200)
- `--max-per-domain` int (default: 5)
- `--default-timeout-ms` int (default: 5000)
- `--log-level` string (default: `info`)

On startup, print a short startup log including bind address, namespace, poll interval, limits.

---

## 4) API specification (implement exactly)

### Auth
All API endpoints require:
- `Authorization: Bearer <token>`

Return 401 if missing or wrong.

### Endpoints

1) Create one-shot
- `POST /v1/jobs/one-shot`
- Request JSON:
```json
{
  "execute_at": "2026-01-10T10:00:00Z",
  "webhook": {
    "url": "https://example.com/hook",
    "method": "POST",
    "headers": { "X-Test": "1" },
    "body": { "foo": "bar" },
    "timeout_ms": 5000
  },
  "retry": {
    "max_attempts": 5,
    "backoff_base_ms": 1000
  }
}
```
- Response:
```json
{ "job_id": "uuid" }
```

2) Create recurring
- `POST /v1/jobs/recurring`
- Request JSON:
```json
{
  "start_at": "2026-01-10T10:00:00Z",
  "interval_ms": 86400000,
  "webhook": { ... },
  "retry": { ... }
}
```
- Response: `{ "job_id": "uuid" }`

3) Get job
- `GET /v1/jobs/{job_id}`
- Response must include the stored fields, including current `due_at_ms`, attempt counters, and webhook config.

4) Cancel job
- `DELETE /v1/jobs/{job_id}`
- Response:
```json
{ "deleted": true }
```
If job does not exist, return 404.

### HTTP response banner
Every response must include:
- `Server: TickHook/1.0`

Implement this as middleware.

---

## 5) Redis keys and storage rules

All keys must be namespaced:
- schedules ZSET: `{ns}:schedules`
- job hash: `{ns}:job:{job_id}`

### ZSET schedules
- member: `job_id`
- score: `due_at_ms` (UTC epoch millis)

### Job hash fields (minimum)
- `type` = `one_shot|recurring`
- `due_at_ms`
- `interval_ms` (recurring only)
- `attempt`
- `max_attempts`
- `backoff_base_ms`
- `url`
- `method`
- `headers_json`
- `body_json`
- `timeout_ms`
- `created_at_ms`
- `updated_at_ms`
- `last_error` (optional)
- `last_http_code` (optional)

Store JSON strings for headers/body (do not use RedisJSON; plain Redis only).

Validation:
- URL must be absolute http/https
- method allowed: GET, POST, PUT, PATCH, DELETE
- interval_ms must be > 0
- max_attempts must be >= 1

---

## 6) Scheduler behavior (polling)

Default poll interval: 200ms.

Each tick:
1) now_ms := current UTC epoch ms
2) Fetch due jobs:
   - `ZRANGEBYSCORE {ns}:schedules -inf now LIMIT 0 {batch}`
3) For each job_id:
   - `ZREM` it from schedules (to avoid reprocessing in this single-consumer V1)
   - Dispatch `job_id` to executor queue

Important: In V1 there is no HA. If the process crashes after ZREM but before completion, the job can be lost. This is acceptable for V1 but must be documented in README limitations.

---

## 7) Execution behavior (worker pool)

Implement a worker pool that processes job_ids:
- load job metadata from Redis
- execute HTTP webhook
- update Redis and reschedule / delete accordingly

### Concurrency limits
- global semaphore of size `max-inflight`
- per-domain semaphore map, each with size `max-per-domain`
  - domain is derived from the webhook URL host (normalize to lowercase, strip port if present)

### HTTP client
- Use a single `http.Client` with sane defaults
- Each request uses per-job timeout via context deadline

### Automatic request headers
Always add:
- `X-Job-Id: <job_id>`
- `Idempotency-Key: <job_id>`

Merge with user headers (user headers may override? Prefer: user headers cannot remove these; if same key exists, keep TickHook value).

---

## 8) Retry logic

On execution failure (network error or HTTP status >= 500; treat 429 as retryable too):
- attempt := attempt + 1
- if attempt >= max_attempts:
  - mark job as failed:
    - set `last_error`, `last_http_code`, `updated_at_ms`
    - do NOT re-add to schedules
    - keep job hash for inspection
- else:
  - compute next_due_ms = now + backoff(attempt) + jitter
  - update job hash `attempt`, `due_at_ms`, `last_error`, `last_http_code`, `updated_at_ms`
  - `ZADD {ns}:schedules next_due_ms job_id`

Backoff:
- exponential: `backoff_base_ms * 2^(attempt-1)`
- cap to a reasonable max (e.g. 10 minutes) to avoid runaway delays
- jitter: random 0..250ms (configurable constant)

Success criteria:
- HTTP status 200-299 is success
- HTTP status 400-499 (except 429) is non-retryable failure:
  - treat as final failure immediately (attempt set to max, store last_http_code/error)
  - keep job hash

---

## 9) One-shot vs recurring behavior

### One-shot
On success:
- `DEL {ns}:job:{job_id}`
On final failure:
- keep job hash
- do not reschedule

### Recurring
On success:
- next_due_ms := previous_due_at_ms + interval_ms
- update job hash `due_at_ms`, reset attempt to 0, clear last_error/last_http_code, update updated_at_ms
- `ZADD schedules next_due_ms job_id`

On failure:
- apply retry logic (retries are for the current occurrence)

---

## 10) Documentation requirements (README.md)

Generate a comprehensive `README.md` in English with:

- Title + short description
- Table of Contents (real links)
- What is TickHook
- Non-goals
- Architecture overview (diagram in ASCII or Mermaid is OK)
- Requirements
- Installation (from source)
- QuickStart (docker compose for Redis + run TickHook + curl examples)
- Configuration (CLI flags table)
- API reference (endpoints + examples)
- Execution model (polling, worker pool, concurrency)
- Retry semantics (what retries, what doesn’t)
- Idempotency (headers and recommended receiver behavior)
- Redis data model (keys)
- Limitations (explicitly mention V1 crash window after ZREM)
- Roadmap (V2: HA with leases + Lua)
- Contributing (basic)
- License (MIT)

Also add:
- `LICENSE` file (MIT)
- `.gitignore` for Go
- Optional: `CONTRIBUTING.md` minimal

---

## 11) Testing requirements

Create a small but real test suite:

- Unit tests:
  - domain extraction logic
  - backoff + jitter range
  - request header merge behavior
- Integration tests (if feasible without external deps):
  - use a local Redis if available via env; otherwise skip
  - alternatively, mock store interface for scheduler/executor logic

At minimum: `go test ./...` must pass.

---

## 12) Deliverables checklist

You must produce:

- Working Go module (`go.mod`, `go.sum`)
- `cmd/tickhook/main.go` running the service
- Internal packages as described
- Redis store implementation
- API handlers and middleware
- Scheduler loop
- Worker pool with concurrency limits
- Retry logic
- `README.md` complete
- `LICENSE` (MIT)
- `PRD_TickHook.md` unchanged
- Basic tests

---

## 13) Implementation notes (keep it minimal)

- Prefer stdlib over large frameworks.
- Keep dependency count small.
- Keep code readable; document key decisions in README (not in huge comments).
- Return consistent JSON errors from the API (include `error` and `message` fields).
- Validate input thoroughly (bad schedules must return 400 with a clear message).
- Use structured logging (slog) with `job_id` field on execution logs.

---

## 14) Build/run commands (must work)

- Build:
  - `go build ./...`
- Run (example):
  - `tickhook --redis-url redis://localhost:6379 --auth-token secret`
- Tests:
  - `go test ./...`
