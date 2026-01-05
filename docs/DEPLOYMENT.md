# TickHook Deployment Guide

## Table of Contents

- [Requirements](#requirements)
- [Deployment Options](#deployment-options)
  - [Docker](#docker)
  - [Docker Compose](#docker-compose)
  - [Kubernetes](#kubernetes)
  - [Systemd](#systemd)
  - [Binary](#binary)
- [Configuration](#configuration)
- [Redis Setup](#redis-setup)
- [Production Considerations](#production-considerations)
- [Monitoring](#monitoring)
- [Backup and Recovery](#backup-and-recovery)
- [Troubleshooting](#troubleshooting)

## Requirements

### System Requirements

- **CPU**: 1 core minimum (2+ cores recommended)
- **Memory**: 256MB minimum (512MB+ recommended)
- **Disk**: 50MB for binary + logs
- **Network**: Outbound HTTPS for webhooks

### Software Requirements

- **Redis**: Version 6.0 or later
- **Go**: 1.22+ (only for building from source)

### Network Requirements

- Inbound: Port 8080 (configurable) for API
- Outbound: HTTPS (443) for webhook targets
- Internal: Redis port 6379 (or custom)

## Deployment Options

### Docker

#### Quick Start

```bash
docker run -d \
  --name tickhook \
  -p 8080:8080 \
  --restart unless-stopped \
  ghcr.io/cr0hn/tickhook:latest \
  --redis-url redis://redis:6379/0 \
  --auth-token your-secret-token
```

#### Production Docker Run

```bash
docker run -d \
  --name tickhook \
  -p 8080:8080 \
  --restart always \
  --memory 512m \
  --cpus 2 \
  -e TZ=UTC \
  --log-driver json-file \
  --log-opt max-size=10m \
  --log-opt max-file=3 \
  ghcr.io/cr0hn/tickhook:latest \
  --redis-url redis://redis:6379/0 \
  --auth-token "${TICKHOOK_TOKEN}" \
  --namespace prod \
  --max-inflight 500 \
  --max-per-domain 10 \
  --log-level info
```

### Docker Compose

#### Development Setup

```yaml
# docker-compose.yml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  tickhook:
    image: ghcr.io/cr0hn/tickhook:latest
    ports:
      - "8080:8080"
    environment:
      - AUTH_TOKEN=dev-token
    command:
      - --redis-url=redis://redis:6379
      - --auth-token=dev-token
      - --log-level=debug
    depends_on:
      - redis

volumes:
  redis_data:
```

#### Production Setup

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    restart: always
    command: redis-server --appendonly yes --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
      - ./redis.conf:/usr/local/etc/redis/redis.conf:ro
    networks:
      - tickhook_net

  tickhook:
    image: ghcr.io/cr0hn/tickhook:latest
    restart: always
    ports:
      - "8080:8080"
    environment:
      - AUTH_TOKEN=${TICKHOOK_TOKEN}
    command:
      - --redis-url=redis://:${REDIS_PASSWORD}@redis:6379
      - --auth-token=${TICKHOOK_TOKEN}
      - --namespace=prod
      - --max-inflight=500
      - --max-per-domain=10
      - --poll-ms=100
      - --log-level=info
    depends_on:
      - redis
    networks:
      - tickhook_net
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M

volumes:
  redis_data:
    driver: local

networks:
  tickhook_net:
    driver: bridge
```

### Kubernetes

#### Deployment

```yaml
# tickhook-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tickhook
  namespace: default
spec:
  replicas: 1  # V1: single instance only
  selector:
    matchLabels:
      app: tickhook
  template:
    metadata:
      labels:
        app: tickhook
    spec:
      containers:
      - name: tickhook
        image: ghcr.io/cr0hn/tickhook:latest
        ports:
        - containerPort: 8080
        args:
        - --redis-url=$(REDIS_URL)
        - --auth-token=$(AUTH_TOKEN)
        - --namespace=prod
        - --max-inflight=500
        - --max-per-domain=10
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: tickhook-secrets
              key: redis-url
        - name: AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: tickhook-secrets
              key: auth-token
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "2"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: tickhook
spec:
  selector:
    app: tickhook
  ports:
  - port: 8080
    targetPort: 8080
---
apiVersion: v1
kind: Secret
metadata:
  name: tickhook-secrets
type: Opaque
stringData:
  redis-url: "redis://:password@redis:6379"
  auth-token: "your-secret-token"
```

#### Helm Chart Structure (optional)

```
tickhook/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── secret.yaml
    └── ingress.yaml
```

### Systemd

#### Service File

```ini
# /etc/systemd/system/tickhook.service
[Unit]
Description=TickHook Webhook Scheduler
After=network.target redis.service
Wants=redis.service

[Service]
Type=simple
User=tickhook
Group=tickhook
WorkingDirectory=/opt/tickhook

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/tickhook/logs

# Resource limits
LimitNOFILE=65535
MemoryLimit=512M
CPUQuota=200%

# Environment
Environment="REDIS_URL=redis://localhost:6379"
EnvironmentFile=/etc/tickhook/tickhook.env

# Start command
ExecStart=/opt/tickhook/bin/tickhook \
  --redis-url ${REDIS_URL} \
  --auth-token ${AUTH_TOKEN} \
  --bind 0.0.0.0:8080 \
  --namespace prod \
  --max-inflight 500 \
  --max-per-domain 10 \
  --log-level info

# Restart policy
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

#### Installation

```bash
# Create user
sudo useradd -r -s /bin/false tickhook

# Create directories
sudo mkdir -p /opt/tickhook/{bin,logs}
sudo mkdir -p /etc/tickhook

# Copy binary
sudo cp tickhook /opt/tickhook/bin/
sudo chmod +x /opt/tickhook/bin/tickhook

# Create environment file
echo "AUTH_TOKEN=your-secret-token" | sudo tee /etc/tickhook/tickhook.env
sudo chmod 600 /etc/tickhook/tickhook.env

# Set permissions
sudo chown -R tickhook:tickhook /opt/tickhook

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable tickhook
sudo systemctl start tickhook

# Check status
sudo systemctl status tickhook
sudo journalctl -u tickhook -f
```

### Binary

#### Direct Execution

```bash
# Download binary
wget https://github.com/cr0hn/tickhook/releases/latest/download/tickhook-linux-amd64
chmod +x tickhook-linux-amd64

# Run
./tickhook-linux-amd64 \
  --redis-url redis://localhost:6379 \
  --auth-token your-secret-token \
  --bind 0.0.0.0:8080 \
  --namespace prod \
  --max-inflight 500 \
  --max-per-domain 10 \
  --log-level info
```

#### Using Supervisor

```ini
# /etc/supervisor/conf.d/tickhook.conf
[program:tickhook]
command=/opt/tickhook/bin/tickhook
  --redis-url redis://localhost:6379
  --auth-token %(ENV_AUTH_TOKEN)s
  --bind 0.0.0.0:8080
  --namespace prod
  --max-inflight 500
  --max-per-domain 10
  --log-level info
directory=/opt/tickhook
user=tickhook
autostart=true
autorestart=true
startretries=3
stderr_logfile=/var/log/tickhook/error.log
stdout_logfile=/var/log/tickhook/access.log
environment=AUTH_TOKEN="your-secret-token"
```

## Configuration

### Environment Variables vs Flags

TickHook uses CLI flags for configuration. For sensitive values, use environment variable substitution:

```bash
# Shell variable expansion
./tickhook --auth-token "${AUTH_TOKEN}"

# Docker
docker run -e AUTH_TOKEN=secret tickhook --auth-token "${AUTH_TOKEN}"
```

### Configuration Reference

| Flag | Environment | Default | Description |
|------|-------------|---------|-------------|
| `--redis-url` | `REDIS_URL` | - | Redis connection URL |
| `--auth-token` | `AUTH_TOKEN` | - | API authentication token |
| `--namespace` | `NAMESPACE` | tickhook | Redis key namespace |
| `--bind` | `BIND_ADDR` | 0.0.0.0:8080 | API bind address |
| `--poll-ms` | `POLL_MS` | 200 | Scheduler poll interval |
| `--batch` | `BATCH_SIZE` | 200 | Jobs per poll batch |
| `--max-inflight` | `MAX_INFLIGHT` | 200 | Global concurrent limit |
| `--max-per-domain` | `MAX_PER_DOMAIN` | 5 | Per-domain concurrent limit |
| `--default-timeout-ms` | `DEFAULT_TIMEOUT` | 5000 | Default webhook timeout |
| `--log-level` | `LOG_LEVEL` | info | Log level |

### Recommended Production Settings

```bash
--redis-url redis://localhost:6379/0  # Use database 0
--auth-token [32+ char random string]  # Strong token
--namespace prod                       # Environment namespace
--bind 127.0.0.1:8080                  # Bind to localhost only
--poll-ms 100                          # Lower latency
--batch 500                            # Higher throughput
--max-inflight 500                     # Based on capacity
--max-per-domain 10                    # Prevent overwhelming targets
--default-timeout-ms 10000             # Higher for slow endpoints
--log-level info                       # Production logging
```

## Redis Setup

### Local Redis

```bash
# Install Redis
sudo apt-get install redis-server  # Debian/Ubuntu
brew install redis                  # macOS

# Configure Redis (redis.conf)
bind 127.0.0.1
port 6379
requirepass your-redis-password
appendonly yes
save 900 1
save 300 10
save 60 10000
```

### Redis Cluster

For high availability Redis:

```bash
# Use Redis Sentinel or Redis Cluster
--redis-url redis-sentinel://localhost:26379/mymaster/0
```

### Managed Redis

- **AWS ElastiCache**: `redis://cache.xxx.amazonaws.com:6379`
- **Google Memorystore**: `redis://10.x.x.x:6379`
- **Azure Cache**: `rediss://cache.redis.cache.windows.net:6380`
- **Redis Cloud**: `redis://:password@redis-xxx.cloud.redislabs.com:16379`

## Production Considerations

### High Availability

**V1 Limitations**:
- Single instance only (no HA)
- Potential job loss on crash
- Plan for V2 if HA needed

**Mitigation**:
- Use process manager (systemd, supervisor)
- Monitor and auto-restart on failure
- Regular Redis backups
- Consider blue-green deployments

### Security

#### Network Security

```nginx
# Nginx reverse proxy with TLS
server {
    listen 443 ssl;
    server_name tickhook.example.com;

    ssl_certificate /etc/ssl/certs/tickhook.crt;
    ssl_certificate_key /etc/ssl/private/tickhook.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### Token Management

```bash
# Generate secure token
openssl rand -base64 32

# Rotate tokens
1. Deploy new instance with new token
2. Update clients
3. Deprecate old instance
```

### Performance Tuning

#### System Tuning

```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 30
fs.file-max = 65535
```

#### TickHook Tuning

Based on workload:

- **High volume, fast webhooks**: Increase `--max-inflight`
- **Slow webhooks**: Increase `--default-timeout-ms`
- **Many domains**: Tune `--max-per-domain`
- **Lower latency**: Decrease `--poll-ms`

### Logging

#### Structured Logging

```bash
# JSON logs for aggregation
./tickhook --log-level info 2>&1 | jq
```

#### Log Aggregation

```yaml
# Fluentd configuration
<source>
  @type tail
  path /var/log/tickhook/*.log
  tag tickhook
  format json
</source>

<match tickhook>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name tickhook
</match>
```

## Monitoring

### Health Checks

```bash
# Basic health check
curl http://localhost:8080/health

# Automated monitoring
while true; do
  if ! curl -f http://localhost:8080/health >/dev/null 2>&1; then
    echo "TickHook is unhealthy!"
    # Send alert
  fi
  sleep 10
done
```

### Metrics to Monitor

1. **Application Metrics**
   - API response time
   - Job execution rate
   - Retry rate
   - Failure rate

2. **System Metrics**
   - CPU usage
   - Memory usage
   - Network I/O
   - Open file descriptors

3. **Redis Metrics**
   - Connection count
   - Memory usage
   - Key count
   - Command rate

### Prometheus Metrics (Future)

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'tickhook'
    static_configs:
      - targets: ['localhost:9090']  # Future metrics endpoint
```

## Backup and Recovery

### Redis Backup

#### Manual Backup

```bash
# Save Redis snapshot
redis-cli BGSAVE

# Copy dump file
cp /var/lib/redis/dump.rdb /backup/redis-$(date +%Y%m%d).rdb
```

#### Automated Backup

```bash
#!/bin/bash
# /usr/local/bin/backup-redis.sh
BACKUP_DIR="/backup/redis"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

redis-cli BGSAVE
sleep 5
cp /var/lib/redis/dump.rdb ${BACKUP_DIR}/dump-${TIMESTAMP}.rdb

# Keep only last 7 days
find ${BACKUP_DIR} -name "dump-*.rdb" -mtime +7 -delete
```

### Recovery

```bash
# Stop TickHook
systemctl stop tickhook

# Restore Redis
systemctl stop redis
cp /backup/redis/dump-20260105.rdb /var/lib/redis/dump.rdb
chown redis:redis /var/lib/redis/dump.rdb
systemctl start redis

# Start TickHook
systemctl start tickhook
```

## Troubleshooting

### Common Issues

#### 1. Cannot Connect to Redis

```bash
# Check Redis is running
redis-cli ping

# Check connection URL
redis-cli -u redis://localhost:6379 ping

# Check logs
journalctl -u redis -n 50
```

#### 2. Jobs Not Executing

```bash
# Check scheduler is running
curl http://localhost:8080/health

# Check Redis for due jobs
redis-cli ZRANGE tickhook:schedules 0 -1 WITHSCORES

# Check logs for errors
journalctl -u tickhook -n 100 | grep ERROR
```

#### 3. High Memory Usage

```bash
# Check Redis memory
redis-cli INFO memory

# Check job count
redis-cli ZCARD tickhook:schedules

# Clean up old failed jobs
redis-cli --scan --pattern "tickhook:job:*" | while read key; do
  if redis-cli HGET $key status | grep -q failed; then
    redis-cli DEL $key
  fi
done
```

#### 4. Webhook Timeouts

```bash
# Increase timeout
--default-timeout-ms 30000

# Check target accessibility
curl -w "@curl-format.txt" -o /dev/null -s https://target.com/webhook
```

### Debug Mode

```bash
# Enable debug logging
./tickhook --log-level debug

# Trace specific job
tail -f /var/log/tickhook/tickhook.log | grep "job_id=xxx"
```

### Performance Profiling

```bash
# CPU profiling (requires debug build)
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Memory profiling
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

## Migration and Upgrades

### Version Upgrades

1. **Check changelog** for breaking changes
2. **Test in staging** environment
3. **Backup Redis** before upgrade
4. **Blue-green deployment**:
   ```bash
   # Start new version on different port
   ./tickhook-new --bind :8081 ...

   # Switch traffic
   # Update load balancer / proxy

   # Stop old version
   ```

### Data Migration

For schema changes (rare):

```bash
# Export job data
redis-cli --scan --pattern "tickhook:job:*" | while read key; do
  redis-cli HGETALL $key
done > jobs-backup.txt

# Run migration script
./migrate-v1-to-v2.sh

# Verify data
redis-cli ZCARD tickhook:schedules
```

## Support

- **Issues**: https://github.com/cr0hn/tickhook/issues
- **Documentation**: https://github.com/cr0hn/tickhook/docs
- **Docker Hub**: https://ghcr.io/cr0hn/tickhook