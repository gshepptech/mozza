---
domain: infra
file: reverse-proxy-ssl
depends_on:
  - foundation/data-model
estimated_complexity: high
---

## Purpose

Implement a built-in reverse proxy with automatic TLS via certmagic. Routes traffic by domain to deployed apps on ports 80/443. Zero-config HTTPS with Let's Encrypt. Integrated into `mozza serve` as a single process.

## Scope

**Included:**
- `internal/proxy/proxy.go` — HTTP/HTTPS reverse proxy using certmagic for TLS
- `internal/proxy/router.go` — domain-based routing table, updated on deploy/undeploy
- `internal/proxy/health.go` — backend health checking, remove unhealthy targets
- Certmagic integration for automatic Let's Encrypt certificates
- Integration with `mozza serve` — single process handles dashboard + proxy + TLS
- CLI: `mozza proxy status` shows routing table
- Self-signed cert mode for local/development use

**Excluded:**
- DNS management (user must configure DNS)
- Rate limiting per domain (defer to v1.1)
- Load balancing across multiple instances (single backend per domain)

## Requirements

- REQ-1: Listen on ports 80 and 443 for incoming traffic
- REQ-2: Route requests to correct app based on `domain` field in recipe
- REQ-3: Auto-provision Let's Encrypt TLS certificate when recipe contains `domain "app.example.com"`
- REQ-4: Use certmagic (Caddy's Go library) embedded in binary
- REQ-5: ACME HTTP-01 challenge handled automatically (requires port 80 accessible)
- REQ-6: Certificate auto-renewed before expiry
- REQ-7: Certificate storage in Mozza's data directory
- REQ-8: HTTP automatically redirects to HTTPS
- REQ-9: Multiple domains supported (one per app/slice)
- REQ-10: `mozza serve --domain admin.example.com` provisions TLS for dashboard
- REQ-11: Graceful fallback: if cert provisioning fails, serve HTTP with clear warning
- REQ-12: Self-signed cert mode for local/development use
- REQ-13: Health-check-aware routing: unhealthy backends removed from rotation
- REQ-14: WebSocket support (passthrough)
- REQ-15: Custom proxy headers (X-Forwarded-For, X-Real-IP, X-Forwarded-Proto)
- REQ-16: Dashboard accessible at configured admin domain
- REQ-17: `mozza proxy status` shows routing table
- REQ-18: Proxy adds < 5ms latency (NFR-5)
- REQ-19: TLS provisioning in < 30 seconds (NFR-8)

## Explicit Behaviors

- Use `github.com/caddyserver/certmagic` for TLS management
- Certmagic configured with: ACME directory (Let's Encrypt production), email from config, HTTP-01 challenge solver
- Certificate storage: `{data_dir}/certificates/` (survives restarts)
- Routing table: in-memory map[string]*Backend, persisted to config for restart recovery
- On deploy with `domain` field: add route to table → provision cert → start proxying
- On undeploy: remove route → keep cert (may be reused)
- Health checks: HTTP GET to backend health endpoint every 30s; 3 failures → mark unhealthy, route to 503 page
- When backend recovers (3 consecutive successes) → restore to rotation
- WebSocket: detect `Upgrade: websocket` header → transparent passthrough
- Proxy headers set on every request before forwarding:
  - `X-Forwarded-For`: client IP
  - `X-Real-IP`: client IP
  - `X-Forwarded-Proto`: http or https
  - `X-Request-ID`: generated UUID
- Self-signed mode: `mozza serve --self-signed` uses `crypto/tls` to generate self-signed cert
- Admin domain: `mozza serve --domain admin.mozza.local` routes the dashboard itself through the proxy

## Dependencies

- foundation/data-model — certificates table for cert metadata

## Interfaces

**API Endpoints:**
```
GET /api/v1/proxy/routes       — current routing table
GET /api/v1/proxy/certificates — TLS certificate status per domain
```

**Internal:**
- `proxy.Server` with `Start`, `Stop`, `AddRoute(domain, backend)`, `RemoveRoute(domain)` methods
- `proxy.Router` with `Route(host string) *Backend` method
- `proxy.Backend{URL, HealthEndpoint, Healthy bool, LastCheck time.Time}`
- Integrates with deploy flow: `deployer.OnDeploy` → `proxy.AddRoute`

**CLI:**
```
mozza proxy status  — show routing table with health status and cert expiry
mozza serve --domain <domain> --self-signed  — serve modes
```

## Constraints

- Port 80 must be accessible for ACME HTTP-01 challenge
- < 5ms latency added by proxy (NFR-5)
- TLS provisioning < 30 seconds (NFR-8)
- Single process: proxy runs in same `mozza serve` process as API server
- No dependency on external reverse proxy (nginx, caddy, traefik)

## Edge Cases

- Port 80 already in use → clear error with instructions to free port
- Let's Encrypt rate limit hit → serve HTTP with warning, queue retry
- Certificate renewal fails → log error, keep using existing cert until expiry
- Domain points to wrong IP → ACME challenge fails, clear error
- Multiple apps with same domain → error at deploy time
- Proxy target crashes mid-request → 502 with retry-after header
- Very long WebSocket connections → no timeout (passthrough)
- Dashboard and app on same port (80/443) → route by Host header

## Acceptance Criteria

- [ ] Proxy listens on ports 80 and 443
- [ ] Domain-based routing works for deployed apps
- [ ] Auto-SSL provisions Let's Encrypt cert for configured domain
- [ ] HTTP → HTTPS redirect works
- [ ] Certificate auto-renewal works
- [ ] Health-check-aware routing removes unhealthy backends
- [ ] WebSocket passthrough works
- [ ] Custom proxy headers set correctly
- [ ] Self-signed cert mode works for local development
- [ ] `mozza proxy status` shows routing table
- [ ] `mozza serve --domain` provisions cert for dashboard
- [ ] Proxy latency < 5ms
- [ ] `make test` passes

## Definition of Done

Full flow: deploy app with domain → cert provisioned → HTTPS works → health checks active → unhealthy backends get 503. Self-signed mode works for local dev. Dashboard accessible at admin domain.

## Related Files

- backend/preview-deploys.md (wildcard routing for preview domains)
- backend/monitoring.md (request metrics from proxy middleware)

## Testing Strategy

- Test routing table add/remove/lookup
- Test health check state machine
- Test WebSocket detection and passthrough
- Test proxy header injection
- Test self-signed cert generation
- Mock ACME for cert provisioning tests (don't hit Let's Encrypt in tests)
- Benchmark proxy latency
- Run: `go test ./internal/proxy/...`
