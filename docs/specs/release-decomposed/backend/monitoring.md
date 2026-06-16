---
domain: backend
file: monitoring
depends_on:
  - foundation/data-model
estimated_complexity: medium
---

## Purpose

Implement built-in monitoring: collect CPU, memory, network metrics from Docker stats API, store in SQLite with 7-day retention, expose via REST API for dashboard graphs.

## Scope

**Included:**
- `internal/monitor/collector.go` — periodic Docker stats / K8s metrics collection
- `internal/monitor/store.go` — time-series storage in SQLite (7-day retention, auto-pruning)
- `internal/monitor/api.go` — REST endpoints for dashboard graphs
- Health status per app based on health check results

**Excluded:**
- Dashboard UI (see frontend/monitoring-ui.md)
- Alerting/notifications (post-launch)
- Prometheus/Grafana integration (explicitly out of scope)

## Requirements

- REQ-1: Per-app metrics: CPU usage, memory usage, network I/O, request count
- REQ-2: Simple time-series storage (last 1h, 6h, 24h, 7d)
- REQ-3: Metrics collected from Docker stats API (local)
- REQ-4: No external dependencies (no Prometheus, no Grafana)
- REQ-5: Health status indicator per app (healthy/degraded/down) based on health check results
- REQ-6: Alert indicators for resource warnings (e.g., "Memory above 80%")
- REQ-7: Response time metrics if reverse proxy is handling traffic
- REQ-8: Lightweight: < 1% CPU overhead (NFR-6)
- REQ-9: Data retention: 7 days in SQLite, automatic pruning
- REQ-10: REST API for querying metrics with time range and resolution

## Explicit Behaviors

- Collector runs as background goroutine, polls Docker stats every 15 seconds
- Docker stats via Docker API client (not exec into container)
- CPU calculation: `(delta_cpu / delta_system) * num_cores * 100`
- Memory: `usage_bytes - cache_bytes` (actual RSS)
- Network: `rx_bytes`, `tx_bytes` deltas between polls
- Request count: tracked by reverse proxy middleware (if active), otherwise N/A
- Health status: poll health check endpoints every 30 seconds
  - healthy: 3 consecutive 2xx responses
  - degraded: 1-2 failures in last 5 checks
  - down: 3+ consecutive failures
- Pruning: run every hour, `DELETE FROM metrics WHERE timestamp < now - 7 days`
- Data resolution: raw (15s) for last 1h, 1-minute aggregates for 1h-24h, 5-minute aggregates for 1d-7d
- Alert thresholds: CPU > 80%, Memory > 80%, Disk > 90% (configurable)

## Dependencies

- foundation/data-model — metrics table

## Interfaces

**API Endpoints:**
```
GET /api/v1/apps/{id}/metrics  — query params: period (1h|6h|24h|7d), metric (cpu|memory|network|requests)
GET /api/v1/apps/{id}/health   — current health status with last check time
GET /api/v1/monitoring/summary — all apps health overview
```

**Response format (metrics):**
```json
{
  "app_id": "...",
  "period": "1h",
  "resolution": "15s",
  "data": [
    {"timestamp": 1234567890, "cpu_percent": 23.5, "memory_bytes": 104857600, "network_rx": 1024, "network_tx": 2048}
  ],
  "alerts": [{"type": "memory_high", "message": "Memory usage above 80%", "value": 85.2}]
}
```

**Internal:**
- `monitor.Collector` with `Start`, `Stop` methods
- `monitor.HealthChecker` with `Check(appID)`, `Status(appID)` methods
- `monitor.QueryParams{AppID, Period, Metric, Resolution}`

## Constraints

- < 1% CPU overhead for collection (NFR-6)
- 7-day retention with automatic pruning
- No external service dependencies
- Must work with both local Docker and Kubernetes targets

## Edge Cases

- Container not running → skip collection, don't error
- Docker API unavailable → log warning, retry next cycle
- Very high collection rate (many apps) → batch inserts
- Clock skew between host and container → use host timestamps
- App restarted → health status resets to checking state
- Pruning during active query → read committed isolation

## Acceptance Criteria

- [ ] Metrics collected every 15 seconds from Docker stats API
- [ ] CPU, memory, network metrics recorded per app
- [ ] Health status tracked (healthy/degraded/down)
- [ ] API returns time-series data for specified period
- [ ] Data resolution adjusts based on time range
- [ ] 7-day automatic pruning works
- [ ] Alert indicators generated for threshold breaches
- [ ] Collection overhead < 1% CPU
- [ ] `make test` passes

## Definition of Done

Collector running in background, metrics stored in SQLite, API returns graph-ready time-series data, health checks running, pruning keeping database size manageable.

## Related Files

- frontend/monitoring-ui.md (dashboard for viewing metrics)
- infra/reverse-proxy-ssl.md (request count metrics from proxy)

## Testing Strategy

- Mock Docker API responses for stats
- Test metric aggregation at different resolutions
- Test health status state machine transitions
- Test pruning logic with time-based data
- Benchmark collection overhead
- Run: `go test ./internal/monitor/...`
