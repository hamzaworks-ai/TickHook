# TickHook Internals

This document covers the internal implementation details of TickHook, including the Redis data model, execution model, and retry semantics.

## Table of Contents

- [Redis Data Model](#redis-data-model)
- [Execution Model](#execution-model)
- [Retry Semantics](#retry-semantics)
- [Concurrency Model](#concurrency-model)

## Redis Data Model

All keys are prefixed with the configured namespace (default: `tickhook`).

### Keys

| Key Pattern | Type | Description |
|-------------|------|-------------|
| `{ns}:schedules` | ZSET | Schedule sorted set (score = due_at_ms) |
| `{ns}:job:{job_id}` | HASH | Job metadata |

### Schedule ZSET

- **Member**: job_id (UUID string)
- **Score**: due_at_ms (Unix milliseconds UTC)

Jobs are fetched using `ZRANGEBYSCORE` to find all jobs where `due_at_ms <= now`.

### Job Hash Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Job ID (UUID) |
| `type` | string | `one_shot` or `recurring` |
| `due_at_ms` | int64 | Next execution time (Unix ms) |
| `interval_ms` | int64 | Interval for recurring jobs |
| `attempt` | int | Current attempt number (0-indexed) |
| `max_attempts` | int | Maximum retry attempts |
| `backoff_base_ms` | int64 | Base backoff delay |
| `url` | string | Webhook URL |
| `method` | string | HTTP method (GET, POST, PUT, PATCH, DELETE) |
| `headers_json` | string | JSON-encoded headers map |
| `body_json` | string | JSON-encoded body |
| `timeout_ms` | int | Request timeout in milliseconds |
| `created_at_ms` | int64 | Creation timestamp |
| `updated_at_ms` | int64 | Last update timestamp |
| `last_error` | string | Last error message (if failed) |
| `last_http_code` | int | Last HTTP status code |
| `status` | string | `pending` or `failed` |

## Execution Model

### Polling Loop

The scheduler polls Redis at a configurable interval (default: 200ms):

```
for {
    now_ms := current UTC epoch milliseconds
    job_ids := ZRANGEBYSCORE {ns}:schedules -inf now_ms LIMIT 0 {batch}

    for each job_id {
        ZREM {ns}:schedules job_id  // Remove from schedule
        dispatch to executor        // Send to worker pool
    }

    sleep(poll_interval)
}
```

### Worker Pool

The executor maintains a pool of workers that process jobs concurrently:

1. Worker receives job_id from dispatch queue
2. Loads job metadata from Redis (`HGETALL`)
3. Acquires global semaphore (blocks if at `max-inflight`)
4. Acquires per-domain semaphore (blocks if at `max-per-domain` for that domain)
5. Executes HTTP request with configured timeout
6. Handles result based on response

### Domain Extraction

Domains are extracted from webhook URLs and normalized:
- Lowercased
- Port stripped (if present)
- Used as key for per-domain semaphore map

Example: `https://API.Example.com:443/webhook` → `api.example.com`

## Retry Semantics

### Retry Flow

```
On execution:
    if success (HTTP 2xx):
        if one_shot:
            DELETE job hash
        else (recurring):
            next_due = previous_due + interval_ms
            reset attempt to 0
            ZADD to schedule with next_due

    else if retryable (HTTP 5xx, 429, network error):
        attempt++
        if attempt >= max_attempts:
            mark as failed (keep job hash)
        else:
            delay = min(base * 2^(attempt-1), 600000) + jitter
            next_due = now + delay
            ZADD to schedule with next_due

    else (HTTP 4xx except 429):
        mark as failed immediately (keep job hash)
```

### Backoff Calculation

```go
func CalculateBackoff(attempt, baseMs, jitterMaxMs int) int64 {
    // Exponential: base * 2^(attempt-1)
    delay := baseMs * (1 << (attempt - 1))

    // Cap at 10 minutes
    if delay > 600000 {
        delay = 600000
    }

    // Add random jitter
    jitter := rand.Intn(jitterMaxMs + 1)

    return int64(delay + jitter)
}
```

### Status Classification

| HTTP Status | Classification | Action |
|-------------|---------------|--------|
| 200-299 | Success | Complete job |
| 400-428, 430-499 | Client error | Fail immediately |
| 429 | Rate limited | Retry with backoff |
| 500-599 | Server error | Retry with backoff |
| Network error | Transient | Retry with backoff |

## Concurrency Model

### Two-Tier Semaphore System

```go
type Executor struct {
    globalSem   chan struct{}           // Size: max-inflight
    domainSems  map[string]chan struct{} // Each size: max-per-domain
    domainMu    sync.Mutex              // Protects domainSems map
}
```

### Acquisition Order

1. Acquire global semaphore (may block)
2. Acquire per-domain semaphore (may block)
3. Execute webhook
4. Release per-domain semaphore
5. Release global semaphore

This prevents:
- System overload (global limit)
- Overwhelming individual destinations (per-domain limit)
- Deadlocks (consistent acquisition order)

### Domain Semaphore Lifecycle

- Created on first request to a domain
- Never deleted (bounded by unique domains seen)
- Concurrent access protected by mutex

## V1 Crash Window

In V1, there is a crash window between:
1. `ZREM` removes job from schedule
2. Job execution completes

If the process crashes during this window, the job is lost. This is a documented limitation of V1's single-process design.

**Mitigation for V2**:
- Lease-based claiming with running set
- Atomic Lua scripts for claim operations
- Automatic job recovery on lease expiry
