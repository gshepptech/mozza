# Mozza Production Deployment Guide

## System Requirements

- **OS**: Linux (amd64/arm64), macOS, Windows
- **Runtime**: Single static binary — no dependencies
- **Database**: SQLite (dev/small) or PostgreSQL 14+ (production)
- **Memory**: 128MB minimum, 512MB recommended
- **Disk**: 100MB for binary + database storage
- **Network**: Port 8080 (default) for HTTP/HTTPS

## TLS/HTTPS

### Native TLS

Mozza can terminate TLS directly:

```bash
mozza serve --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
```

Or via environment variables:

```bash
export MOZZA_TLS_CERT=/path/to/cert.pem
export MOZZA_TLS_KEY=/path/to/key.pem
mozza serve
```

Both `--tls-cert` and `--tls-key` must be provided together. Providing only one
returns an error at startup.

### Behind a Reverse Proxy

Mozza works behind nginx, Caddy, or a cloud load balancer. Run Mozza on HTTP
and let the proxy handle TLS:

**nginx example:**
```nginx
server {
    listen 443 ssl;
    server_name mozza.example.com;

    ssl_certificate     /etc/ssl/certs/mozza.pem;
    ssl_certificate_key /etc/ssl/private/mozza.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Caddy example:**
```
mozza.example.com {
    reverse_proxy localhost:8080
}
```

The `X-Forwarded-Proto: https` header ensures session cookies are set with the
`Secure` flag even when Mozza runs on HTTP behind a TLS proxy.

## Database

### SQLite (Development / Small Deployments)

Default — no configuration needed:

```bash
mozza serve  # Uses ./mozza.db
mozza serve --db /data/mozza.db  # Custom path
```

SQLite uses WAL mode and foreign keys automatically.

### PostgreSQL (Production)

```bash
export MOZZA_DATABASE_URL="postgres://user:password@host:5432/mozza?sslmode=require"
mozza serve
```

Or via flag:

```bash
mozza serve --db-url "postgres://user:password@host:5432/mozza?sslmode=require"
```

**Setup:**
```sql
CREATE DATABASE mozza;
CREATE USER mozza WITH ENCRYPTED PASSWORD 'your-password';
GRANT ALL PRIVILEGES ON DATABASE mozza TO mozza;
```

Migrations run automatically on startup for both SQLite and PostgreSQL.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MOZZA_DATABASE_URL` | Database connection string (postgres:// or file path) | `mozza.db` |
| `MOZZA_ENCRYPTION_KEY` | Base64-encoded AES-256 key for encrypting cluster kubeconfigs at rest | (none — kubeconfigs stored in plaintext) |
| `MOZZA_TLS_CERT` | Path to TLS certificate PEM file | (none — HTTP mode) |
| `MOZZA_TLS_KEY` | Path to TLS private key PEM file | (none — HTTP mode) |
| `MOZZA_METRICS` | Set to `true` to enable Prometheus /metrics endpoint | `false` |

## CLI Flags

```
mozza serve [flags]

Flags:
  --host string      HTTP server host (default "localhost")
  --port int         HTTP server port (default 8080)
  --db string        SQLite database path (default "mozza.db")
  --db-url string    Database URL (postgres://... or file path)
  --no-auth          Allow non-localhost binding without auth
  --tls-cert string  Path to TLS certificate file
  --tls-key string   Path to TLS private key file
  --metrics          Enable Prometheus /metrics endpoint
  --recipe string    Path to recipe file (default "app.mozza")
```

## Docker Deployment

### Build

```bash
docker build -t mozza:latest .
```

### Run

```bash
# Basic (SQLite, localhost only)
docker run -p 8080:8080 mozza:latest

# Production (PostgreSQL, TLS, metrics)
docker run -p 443:8080 \
  -e MOZZA_DATABASE_URL="postgres://user:pass@db:5432/mozza" \
  -e MOZZA_ENCRYPTION_KEY="your-base64-key" \
  -e MOZZA_TLS_CERT=/certs/cert.pem \
  -e MOZZA_TLS_KEY=/certs/key.pem \
  -e MOZZA_METRICS=true \
  -v /path/to/certs:/certs:ro \
  mozza:latest serve --host 0.0.0.0 --no-auth
```

The `--host 0.0.0.0 --no-auth` flags are required when binding to all interfaces
inside a container (since the container network is isolated).

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mozza
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mozza
  template:
    metadata:
      labels:
        app: mozza
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: mozza
        image: mozza:latest
        args: ["serve", "--host", "0.0.0.0", "--no-auth", "--metrics"]
        ports:
        - containerPort: 8080
        env:
        - name: MOZZA_DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: mozza-secrets
              key: database-url
        - name: MOZZA_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: mozza-secrets
              key: encryption-key
        - name: MOZZA_METRICS
          value: "true"
        readinessProbe:
          httpGet:
            path: /api/v1/readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: mozza
spec:
  selector:
    app: mozza
  ports:
  - port: 80
    targetPort: 8080
---
apiVersion: v1
kind: Secret
metadata:
  name: mozza-secrets
type: Opaque
stringData:
  database-url: "postgres://mozza:password@postgres:5432/mozza?sslmode=require"
  encryption-key: "your-base64-aes256-key"
```

## RBAC Setup

Mozza uses three roles: `viewer`, `deployer`, `admin`.

- **First registered user** automatically becomes `admin`
- Subsequent users register as `viewer` by default
- Admins promote users via API:

```bash
# Promote a user to deployer
curl -X PUT http://localhost:8080/api/v1/users/{id}/role \
  -H "Content-Type: application/json" \
  -b "mozza_session=..." \
  -d '{"role": "deployer"}'
```

| Role | Read | Deploy/Modify | Manage Users |
|------|------|---------------|--------------|
| viewer | Yes | No | No |
| deployer | Yes | Yes | No |
| admin | Yes | Yes | Yes |

## Monitoring

### Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'mozza'
    scrape_interval: 15s
    static_configs:
      - targets: ['mozza:8080']
```

### Key Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `mozza_http_requests_total` | Counter | Total HTTP requests by method, path, status |
| `mozza_http_request_duration_seconds` | Histogram | Request latency distribution |
| `mozza_deployments_total` | Counter | Deployments by target and status |
| `mozza_rate_limit_rejections_total` | Counter | Rate limit 429 responses |

### Health Endpoints

| Endpoint | Purpose | Auth | Response |
|----------|---------|------|----------|
| `GET /api/v1/health` | Liveness probe | No | Always 200 |
| `GET /api/v1/readyz` | Readiness probe | No | 200 if DB reachable, 503 if not |

## Backup & Restore

### SQLite

```bash
# Backup (safe with WAL mode)
sqlite3 mozza.db ".backup /backups/mozza-$(date +%Y%m%d).db"

# Restore
cp /backups/mozza-20260317.db mozza.db
```

### PostgreSQL

```bash
# Backup
pg_dump -Fc mozza > /backups/mozza-$(date +%Y%m%d).dump

# Restore
pg_restore -d mozza /backups/mozza-20260317.dump
```

## Upgrade Procedures

1. Back up the database
2. Replace the binary (or update the container image)
3. Restart — migrations run automatically on startup
4. Verify with `GET /api/v1/readyz` and `GET /api/v1/version`

Migrations are idempotent — restarting with the same version is safe.

## Troubleshooting

### "binding to X exposes the API without authentication"

Add `--no-auth` flag when binding to non-localhost addresses. This is a safety
guard — make sure TLS and/or network isolation is in place.

### Database connection failures

Check `GET /api/v1/readyz` for database status. Verify the connection string
format: `postgres://user:pass@host:5432/dbname?sslmode=require`

### Encryption key errors

`MOZZA_ENCRYPTION_KEY` must be a valid base64-encoded 32-byte (AES-256) key.
Generate one:

```bash
openssl rand -base64 32
```

### Rate limiting

Auth endpoints: 10 requests/minute per IP. Mutation endpoints: 30 requests/minute
per IP. Check `X-RateLimit-Remaining` and `Retry-After` headers.
