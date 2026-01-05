# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-01-05

### Added

- Initial release of TickHook
- One-shot job scheduling (execute once at a specific time)
- Recurring job scheduling (execute repeatedly at fixed intervals)
- REST API with Bearer token authentication
- Redis backend for job persistence
- Exponential backoff with jitter for retries
- Global and per-domain concurrency limits
- Automatic `X-Job-Id` and `Idempotency-Key` headers
- Health check endpoint
- Configurable via CLI flags
- Structured logging with slog

### Notes

- V1 is designed for single-process operation
- No high availability (planned for V2)
- At-least-once delivery semantics
