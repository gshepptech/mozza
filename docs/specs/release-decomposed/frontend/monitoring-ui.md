---
domain: frontend
file: monitoring-ui
depends_on:
  - backend/monitoring
estimated_complexity: medium
---

## Purpose

Implement the built-in monitoring dashboard UI: per-app CPU/memory/network graphs, health status indicators, alert badges. Simple, no-dependency monitoring without Prometheus/Grafana.

## Scope

**Included:**
- `ui/src/components/monitoring/MetricsChart.tsx` — time-series graph component
- `ui/src/components/monitoring/AppHealth.tsx` — health status indicator
- `ui/src/components/monitoring/MonitoringDashboard.tsx` — per-app metrics overview
- Time period selector (1h, 6h, 24h, 7d)
- Alert badges for resource warnings

**Excluded:**
- Metrics collection logic (see backend/monitoring.md)
- Alerting/notifications (post-launch)

## Requirements

- REQ-1: Dashboard shows per-app: CPU usage, memory usage, network I/O, request count
- REQ-2: Simple time-series graphs (last 1h, 6h, 24h, 7d)
- REQ-3: Health status indicator per app (healthy/degraded/down)
- REQ-4: Alert indicators: "Memory usage above 80%" as warning badge
- REQ-5: Responsive design (desktop + mobile)

## Explicit Behaviors

- Charts rendered with a lightweight charting library (recharts or custom SVG — no heavy dependencies)
- Time period selector: tabs or button group (1h | 6h | 24h | 7d)
- Auto-refresh: every 30 seconds when page is visible
- Health indicator: green circle (healthy), yellow (degraded), red (down) with tooltip showing last check
- Alert badges: orange/red badges on app card when thresholds exceeded
- Monitoring page: grid of app cards, each with mini sparkline charts
- Click app card → expanded view with full-size charts
- CPU chart: line graph, 0-100% scale
- Memory chart: area graph, shows used/total with color bands for warning/critical
- Network chart: dual-line (rx/tx) with auto-scaling units (B/KB/MB)
- Empty state: "No monitoring data yet. Deploy an app to start seeing metrics."
- Error state: "Unable to fetch metrics. Is the app running?"

## Dependencies

- backend/monitoring — API endpoints for metrics and health data

## Interfaces

**API (consumed):**
```
GET /api/v1/apps/{id}/metrics  — time-series data
GET /api/v1/apps/{id}/health   — health status
GET /api/v1/monitoring/summary — all apps overview
```

**React Components:**
- `MonitoringDashboard` — page with app grid and overview
- `MetricsChart` — configurable chart (type, period, data)
- `AppHealth` — health status indicator with tooltip
- `AlertBadge` — warning/critical badge component

## Constraints

- No heavy charting dependencies (Recharts is OK, D3 is too heavy)
- Charts must render smoothly with 7 days of data points
- Auto-refresh must not cause visible flicker

## Edge Cases

- No metrics data for app (just deployed) → show "Collecting data..." placeholder
- App stopped → show last known metrics with "App stopped" indicator
- Very high metric values → auto-scale axis labels
- Multiple apps with varying metric ranges → independent scales per app

## Acceptance Criteria

- [ ] Per-app metrics charts for CPU, memory, network
- [ ] Time period selector works (1h, 6h, 24h, 7d)
- [ ] Health status indicators displayed
- [ ] Alert badges shown for threshold breaches
- [ ] Auto-refresh every 30 seconds
- [ ] Responsive layout
- [ ] Smooth chart rendering with large datasets

## Definition of Done

Monitoring page shows real metrics for running apps with charts, health indicators, and alert badges. Time period selection works. Responsive on all devices.

## Related Files

- backend/monitoring.md (metrics collection and API)

## Testing Strategy

- Component tests with mock metrics data
- Test time period switching
- Test health indicator states
- Test chart rendering with edge case data (zeros, very large, gaps)
