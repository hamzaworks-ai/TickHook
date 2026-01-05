# Contributing to TickHook

Thank you for your interest in contributing to TickHook! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

Before submitting a bug report:

1. Check the [existing issues](https://github.com/cr0hn/TickHook/issues) to avoid duplicates
2. Use the bug report template when creating a new issue
3. Include as much detail as possible:
   - TickHook version
   - Go version
   - Redis version
   - Operating system
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs

### Suggesting Features

1. Check [existing issues](https://github.com/cr0hn/TickHook/issues) for similar suggestions
2. Use the feature request template
3. Explain the use case and why it would benefit users

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. Make your changes following our coding standards
4. Write or update tests as needed
5. Ensure all tests pass:
   ```bash
   go test -race ./...
   ```
6. Run the linter:
   ```bash
   golangci-lint run
   ```
7. Commit your changes with a clear message
8. Push to your fork and submit a pull request

## Development Setup

### Prerequisites

- Go 1.22 or later
- Redis 6 or later
- Docker (optional, for testing)

### Getting Started

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/TickHook.git
cd TickHook

# Install dependencies
go mod download

# Run tests
go test -race ./...

# Run linter
golangci-lint run

# Build
go build ./cmd/tickhook

# Run locally (requires Redis)
./tickhook --redis-url redis://localhost:6379 --auth-token dev-token
```

### Running Tests with Redis

```bash
# Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# Run tests
REDIS_URL=redis://localhost:6379 go test -race ./...
```

## Coding Standards

### Go Style

- Follow standard Go conventions and [Effective Go](https://go.dev/doc/effective_go)
- Use `gofmt` and `goimports` for formatting
- Keep functions focused and small
- Write descriptive variable and function names
- Add comments for exported functions and complex logic

### Error Handling

- Always handle errors explicitly
- Wrap errors with context: `fmt.Errorf("failed to do X: %w", err)`
- Use structured error types when appropriate
- Return appropriate HTTP status codes

### Logging

- Use `slog` for structured logging
- Include relevant context (job_id, status, duration)
- Use appropriate log levels:
  - `Debug`: Detailed debugging information
  - `Info`: General operational information
  - `Warn`: Warning conditions
  - `Error`: Error conditions

### Testing

- Write unit tests for new functionality
- Place tests in `*_test.go` files alongside source
- Use table-driven tests where appropriate
- Mock external dependencies (Redis, HTTP)
- Aim for meaningful coverage, not just high numbers

### Commit Messages

Use clear, descriptive commit messages:

```
type: short description

Longer explanation if needed. Explain the why, not just the what.

Fixes #123
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `ci`, `chore`

Examples:
- `feat: add webhook retry configuration`
- `fix: handle Redis connection timeout`
- `docs: update API documentation`

## Project Structure

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
docs/                  # Documentation
```

## Review Process

1. All PRs require at least one approval
2. CI must pass (tests, linting)
3. Changes should include tests when applicable
4. Documentation should be updated for user-facing changes

## Questions?

- Open a [discussion](https://github.com/cr0hn/TickHook/discussions) for general questions
- Check [existing issues](https://github.com/cr0hn/TickHook/issues) for known problems
- Review the [documentation](docs/) for usage guidance

## License

By contributing to TickHook, you agree that your contributions will be licensed under the MIT License.
