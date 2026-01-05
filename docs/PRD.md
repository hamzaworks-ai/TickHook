# Product Requirements Document (PRD)

## Project: TickHook  
**Author:** Dani (cr0hn)  
**Status:** Version 1.0  
**License:** Open Source (MIT or Apache-2.0)  
**Target Language:** Go  
**Repository Type:** Public (GitHub)

---

## 1. Overview

TickHook is a lightweight, self-hosted webhook scheduler.

It runs as a single Go binary and allows users to schedule HTTP webhooks:
- at a specific date and time (one-shot)
- repeatedly at a fixed interval (recurring)

TickHook uses Redis as its only backend for persistence and scheduling.  
It exposes a REST API for job management and executes webhooks asynchronously with controlled concurrency.

TickHook is intentionally minimal. It does not aim to be a workflow engine, a cron replacement, or a general automation platform.

---

## 2. Goals

- Provide a simple and predictable way to execute webhooks based on time.
- Be easy to deploy, operate, and reason about.
- Require minimal infrastructure (single binary + Redis).
- Be fully open source and well documented.
- Allow safe evolution toward HA in future versions without redesign.

---

## 3. Non-Goals (V1)

- High availability (multiple active consumers)
- Exactly-once execution guarantees
- Cron expressions
- Timezone-aware scheduling (DST, calendars)
- Conditional workflows or branching logic
- UI or dashboard
- Cloud-managed services

---

## 4. Design Principles

1. Simplicity over features  
2. Open source first  
3. Timestamp-based scheduling  
4. At-least-once semantics  
5. Single-process execution (V1)

---

## 5. Architecture Overview

TickHook runs as a single process composed of four internal components:

1. HTTP API Server  
2. Scheduler Loop  
3. Worker Pool  
4. Redis Store  

---

## 6. Technology Stack

- Language: Go (>= 1.21)
- Backend: Redis (>= 6)
- Redis client: go-redis

---

## 7. HTTP Server Identity

All HTTP responses MUST include:

Server: TickHook/1.0

---

## 8. Redis Data Model

### Scheduling Set

ZSET schedules:
- member: job_id
- score: due_at_ms

### Job Metadata

HASH job:{job_id} with job configuration.

---

## 9. REST API

Endpoints:
- POST /v1/jobs/one-shot
- POST /v1/jobs/recurring
- GET /v1/jobs/{job_id}
- DELETE /v1/jobs/{job_id}

Authentication via Bearer token.

---

## 10. Scheduler Loop

- Poll every 200 ms
- Fetch due jobs via ZRANGEBYSCORE
- Dispatch to workers

---

## 11. Webhook Execution

- Global and per-domain concurrency limits
- Automatic Idempotency headers
- Configurable timeouts

---

## 12. Retry Policy

- Exponential backoff with jitter
- Retry until max_attempts

---

## 13. Recurring Jobs

- Fixed interval scheduling
- One future execution at a time

---

## 14. One-Shot Jobs

- Deleted on success
- Retained on failure

---

## 15. CLI Interface

Required flags:
- --redis-url
- --auth-token

---

## 16. Documentation Requirements

Project MUST include a full README.md with:
- QuickStart
- Configuration
- API reference
- Architecture
- License

---

## 17. License

MIT or Apache-2.0

---

## 18. Acceptance Criteria

- Jobs execute correctly
- Retries work
- Server header present
- Documentation sufficient

---

End of document.
