# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please report security vulnerabilities by emailing:

**cr0hn@cr0hn.com**

Include the following information:

1. **Description**: A clear description of the vulnerability
2. **Impact**: What an attacker could achieve
3. **Steps to Reproduce**: Detailed steps to reproduce the issue
4. **Affected Versions**: Which versions are affected
5. **Suggested Fix**: If you have one (optional)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Initial Assessment**: We will provide an initial assessment within 7 days
- **Resolution Timeline**: We aim to resolve critical issues within 30 days
- **Credit**: We will credit you in the security advisory (unless you prefer anonymity)

### Disclosure Policy

- We follow coordinated disclosure practices
- We will work with you to understand and resolve the issue
- We will publicly disclose the vulnerability after a fix is available
- We will credit reporters in security advisories

## Security Best Practices

### Deployment

1. **Use HTTPS**: Always deploy behind a TLS-terminating proxy
2. **Strong Auth Token**: Use a cryptographically random token (32+ characters)
3. **Network Isolation**: Run TickHook in a private network, not exposed to the internet
4. **Redis Security**: Use Redis authentication and TLS (`rediss://`)
5. **Minimal Permissions**: Run with least-privilege principles

### Configuration

```bash
# Generate a secure token
openssl rand -base64 32

# Use TLS for Redis
./tickhook --redis-url rediss://user:password@redis:6379 --auth-token $SECURE_TOKEN
```

### Docker Security

```yaml
# docker-compose.yml security recommendations
services:
  tickhook:
    image: ghcr.io/cr0hn/tickhook:latest
    read_only: true
    security_opt:
      - no-new-privileges:true
    environment:
      # Use secrets management, not plain text
      - AUTH_TOKEN_FILE=/run/secrets/auth_token
```

### Webhook Endpoints

1. **Validate the Job ID**: Use the `X-Job-Id` header to verify requests
2. **Implement Idempotency**: Use the `Idempotency-Key` header
3. **Use HTTPS**: Ensure your webhook endpoints use TLS
4. **Authenticate Webhooks**: Consider adding shared secrets to webhook headers

## Known Limitations

1. **At-Least-Once Delivery**: Webhooks may be delivered multiple times
2. **Single Process**: No HA support in current version
3. **No Encryption at Rest**: Job data in Redis is not encrypted
4. **Token in Memory**: Auth token is stored in process memory

## Security Features

- Bearer token authentication on all API endpoints (except `/health`)
- Request timeout enforcement (prevents slow-loris attacks)
- Input validation on all endpoints
- No SQL/NoSQL injection vectors
- Structured error responses (no internal details leaked)
- Minimal container image (scratch-based, no shell)

## Security Updates

Security updates will be released as patch versions. Subscribe to:

- [GitHub Releases](https://github.com/cr0hn/TickHook/releases) for update notifications
- [Security Advisories](https://github.com/cr0hn/TickHook/security/advisories) for vulnerability disclosures
