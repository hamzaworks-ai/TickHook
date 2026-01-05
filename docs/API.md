# TickHook API Documentation

> **OpenAPI Spec**: The complete API specification is available in [openapi.yaml](openapi.yaml) for use with Swagger UI, Postman, or code generators.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Common Headers](#common-headers)
- [Error Responses](#error-responses)
- [Endpoints](#endpoints)
  - [Health Check](#health-check)
  - [Create One-Shot Job](#create-one-shot-job)
  - [Create Recurring Job](#create-recurring-job)
  - [Get Job](#get-job)
  - [Delete Job](#delete-job)
- [Data Types](#data-types)
- [Examples](#examples)
- [Rate Limiting](#rate-limiting)
- [Idempotency](#idempotency)

## Overview

TickHook provides a RESTful API for managing scheduled webhooks. All API requests and responses use JSON format.

**Base URL**: `http://localhost:8080` (configurable via `--bind` flag)

**API Version**: v1

## Authentication

All API endpoints (except `/health`) require authentication via Bearer token.

### Request

Include the token in the `Authorization` header:

```http
Authorization: Bearer your-secret-token
```

### Response for Invalid Auth

```http
HTTP/1.1 401 Unauthorized
Content-Type: application/json
Server: TickHook/1.0

{
    "error": "unauthorized",
    "message": "Invalid token"
}
```

## Common Headers

### Request Headers

| Header | Required | Description |
|--------|----------|-------------|
| `Authorization` | Yes* | Bearer token for authentication (*except `/health`) |
| `Content-Type` | Yes** | Must be `application/json` for POST requests |

### Response Headers

All responses include:

| Header | Value | Description |
|--------|-------|-------------|
| `Server` | `TickHook/1.0` | Server identification |
| `Content-Type` | `application/json` | Response format |

## Error Responses

All errors follow a consistent format:

```json
{
    "error": "error_code",
    "message": "Human readable description"
}
```

### Common Error Codes

| Status | Code | Description |
|--------|------|-------------|
| 400 | `invalid_request` | Malformed request |
| 400 | `validation_error` | Request validation failed |
| 401 | `unauthorized` | Missing or invalid auth token |
| 404 | `not_found` | Resource not found |
| 500 | `internal_error` | Server error |
| 503 | `unhealthy` | Service unavailable (Redis down) |

## Endpoints

### Health Check

Check service health and Redis connectivity.

```http
GET /health
```

**Authentication**: Not required

**Response 200 OK**:
```json
{
    "status": "ok"
}
```

**Response 503 Service Unavailable**:
```json
{
    "error": "unhealthy",
    "message": "Redis connection failed"
}
```

---

### Create One-Shot Job

Schedule a webhook to execute once at a specific time.

```http
POST /v1/jobs/one-shot
```

**Request Body**:
```json
{
    "execute_at": "2026-01-15T10:00:00Z",
    "webhook": {
        "url": "https://example.com/webhook",
        "method": "POST",
        "headers": {
            "X-Custom-Header": "value",
            "Content-Type": "application/json"
        },
        "body": {
            "event": "scheduled",
            "data": "custom payload"
        },
        "timeout_ms": 5000
    },
    "retry": {
        "max_attempts": 3,
        "backoff_base_ms": 1000
    }
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `execute_at` | ISO 8601 | Yes | When to execute (UTC) |
| `webhook` | Object | Yes | Webhook configuration |
| `webhook.url` | String | Yes | Target URL (http/https) |
| `webhook.method` | String | No | HTTP method (default: POST) |
| `webhook.headers` | Object | No | Custom headers |
| `webhook.body` | Any | No | Request body (will be JSON encoded) |
| `webhook.timeout_ms` | Integer | No | Request timeout (default: 5000) |
| `retry` | Object | No | Retry configuration |
| `retry.max_attempts` | Integer | No | Max attempts including first (default: 3) |
| `retry.backoff_base_ms` | Integer | No | Base backoff delay (default: 1000) |

**Response 201 Created**:
```json
{
    "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response 400 Bad Request**:
```json
{
    "error": "validation_error",
    "message": "webhook.url is required"
}
```

---

### Create Recurring Job

Schedule a webhook to execute repeatedly at fixed intervals.

```http
POST /v1/jobs/recurring
```

**Request Body**:
```json
{
    "start_at": "2026-01-15T10:00:00Z",
    "interval_ms": 3600000,
    "webhook": {
        "url": "https://example.com/webhook",
        "method": "POST",
        "headers": {
            "X-Recurring": "true"
        },
        "body": {
            "type": "heartbeat"
        },
        "timeout_ms": 5000
    },
    "retry": {
        "max_attempts": 3,
        "backoff_base_ms": 1000
    }
}
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `start_at` | ISO 8601 | Yes | First execution time (UTC) |
| `interval_ms` | Integer | Yes | Interval between executions in milliseconds |
| `webhook` | Object | Yes | Same as one-shot job |
| `retry` | Object | No | Same as one-shot job |

**Response 201 Created**:
```json
{
    "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Notes**:
- Only one future execution is scheduled at a time
- Next execution is scheduled from the previous `due_at`, not completion time
- Minimum interval is 1ms (but practically should be >= 1000ms)

---

### Get Job

Retrieve details about a specific job.

```http
GET /v1/jobs/{job_id}
```

**Path Parameters**:
- `job_id`: UUID of the job

**Response 200 OK**:
```json
{
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "one_shot",
    "due_at_ms": 1736935200000,
    "interval_ms": 0,
    "attempt": 1,
    "max_attempts": 3,
    "backoff_base_ms": 1000,
    "url": "https://example.com/webhook",
    "method": "POST",
    "headers": {
        "X-Custom": "value"
    },
    "body": {
        "data": "payload"
    },
    "timeout_ms": 5000,
    "created_at_ms": 1736848800000,
    "updated_at_ms": 1736848900000,
    "last_error": "connection timeout",
    "last_http_code": 0,
    "status": "pending"
}
```

**Field Descriptions**:

| Field | Type | Description |
|-------|------|-------------|
| `job_id` | String | Unique job identifier |
| `type` | String | `one_shot` or `recurring` |
| `due_at_ms` | Integer | Next execution time (Unix ms) |
| `interval_ms` | Integer | Interval for recurring jobs (0 for one-shot) |
| `attempt` | Integer | Current attempt number (0-indexed) |
| `max_attempts` | Integer | Maximum retry attempts |
| `backoff_base_ms` | Integer | Base backoff for retries |
| `url` | String | Webhook URL |
| `method` | String | HTTP method |
| `headers` | Object | Custom headers |
| `body` | Any | Request body |
| `timeout_ms` | Integer | Request timeout |
| `created_at_ms` | Integer | Creation timestamp (Unix ms) |
| `updated_at_ms` | Integer | Last update timestamp (Unix ms) |
| `last_error` | String | Last error message (if any) |
| `last_http_code` | Integer | Last HTTP response code (if any) |
| `status` | String | `pending` or `failed` |

**Response 404 Not Found**:
```json
{
    "error": "not_found",
    "message": "Job not found"
}
```

---

### Delete Job

Cancel and remove a job.

```http
DELETE /v1/jobs/{job_id}
```

**Path Parameters**:
- `job_id`: UUID of the job

**Response 200 OK**:
```json
{
    "deleted": true
}
```

**Response 404 Not Found**:
```json
{
    "error": "not_found",
    "message": "Job not found"
}
```

**Notes**:
- Deletes both one-shot and recurring jobs
- Removes from schedule if pending
- Cannot be undone

## Data Types

### Webhook Methods

Allowed HTTP methods:
- `GET`
- `POST`
- `PUT`
- `PATCH`
- `DELETE`

### Job Types

- `one_shot`: Executes once and is deleted on success
- `recurring`: Executes repeatedly at fixed intervals

### Job Status

- `pending`: Job is scheduled or retrying
- `failed`: Job reached max attempts without success

### Timestamps

All timestamps use Unix milliseconds (UTC):
- Dates in requests: ISO 8601 format (e.g., `2026-01-15T10:00:00Z`)
- Timestamps in responses: Unix milliseconds (e.g., `1736935200000`)

## Examples

### Example: Schedule a Daily Report

```bash
curl -X POST http://localhost:8080/v1/jobs/recurring \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "start_at": "2026-01-15T09:00:00Z",
    "interval_ms": 86400000,
    "webhook": {
      "url": "https://api.example.com/reports/daily",
      "method": "POST",
      "headers": {
        "X-Report-Type": "daily",
        "X-Source": "tickhook"
      },
      "body": {
        "report_type": "sales",
        "format": "pdf"
      },
      "timeout_ms": 30000
    },
    "retry": {
      "max_attempts": 5,
      "backoff_base_ms": 5000
    }
  }'
```

### Example: Schedule a One-Time Notification

```bash
curl -X POST http://localhost:8080/v1/jobs/one-shot \
  -H "Authorization: Bearer your-secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "execute_at": "2026-02-14T12:00:00Z",
    "webhook": {
      "url": "https://hooks.slack.com/services/xxx",
      "method": "POST",
      "body": {
        "text": "Happy Valentine'\''s Day! 💝"
      }
    }
  }'
```

### Example: Check Job Status

```bash
curl http://localhost:8080/v1/jobs/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer your-secret-token"
```

### Example: Cancel a Job

```bash
curl -X DELETE http://localhost:8080/v1/jobs/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer your-secret-token"
```

## Rate Limiting

TickHook does not implement rate limiting at the API level. However:

1. **Global Concurrency**: Limited by `--max-inflight` flag
2. **Per-Domain Limits**: Limited by `--max-per-domain` flag
3. **Webhook Targets**: May implement their own rate limits (HTTP 429)

When a webhook returns HTTP 429, TickHook will retry with exponential backoff.

## Idempotency

### Automatic Headers

TickHook automatically adds these headers to every webhook request:

```http
X-Job-Id: 550e8400-e29b-41d4-a716-446655440000
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```

### Receiver Implementation

Webhook receivers should:

1. Track `Idempotency-Key` values
2. Return cached response for duplicate keys
3. Use reasonable TTL (e.g., 24 hours)

### Example Receiver (pseudo-code)

```python
def handle_webhook(request):
    key = request.headers['Idempotency-Key']

    # Check cache
    if cached_response := cache.get(key):
        return cached_response

    # Process webhook
    response = process_webhook(request)

    # Cache for 24 hours
    cache.set(key, response, ttl=86400)

    return response
```

## Best Practices

### Webhook URLs

- Use HTTPS for production
- Include authentication in headers or URL
- Implement request signature validation
- Set appropriate timeouts

### Scheduling

- Use UTC timestamps to avoid timezone issues
- For recurring jobs, consider interval vs specific times
- Account for execution time in intervals
- Set reasonable retry attempts

### Error Handling

- Make webhooks idempotent
- Return appropriate HTTP status codes
- Log failed attempts for debugging
- Monitor job failures

### Security

- Rotate auth tokens regularly
- Use TLS for API communication
- Validate webhook SSL certificates
- Limit webhook URLs to trusted domains

## Limitations

### V1 Limitations

- No exactly-once delivery guarantee
- Possible job loss during crash (small window)
- Single-consumer model (no HA)
- No cron expressions
- No timezone support

### Planned for V2

- High availability with multiple consumers
- Lease-based job claiming
- Job history and metrics
- Advanced scheduling (cron, timezones)