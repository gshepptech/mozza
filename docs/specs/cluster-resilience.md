# Spec: Cluster Resilience — Informers, Cache, and Graceful Degradation

**Feature:** cluster-resilience
**Date:** 2026-03-17
**Status:** APPROVED

---

## Overview

Replace the current direct-K8s-API-per-request architecture with an informer-based cache layer that provides instant responses, graceful degradation when clusters are unreachable, and eliminates the 21+ console errors users see when no cluster is connected. Every page should load instantly with cached or "no cluster" state — never skeleton loaders that hang forever.

## Current State (Broken)

```
UI component mounts → fetch /api/v1/cluster/pods → Go handler → live K8s API call → timeout/crash → ERR_EMPTY_RESPONSE
```

- Every page load fires 3-5 independent cluster API calls
- No caching — same data re-fetched on every navigation
- Backend handlers panic when kubeconfig is invalid → TCP connection reset (not JSON error)
- Doctor page hangs forever with skeleton bars — no timeout, no error state
- Applications page shows "Failed to Load / Failed to fetch" — no context
- 21 console errors from navigating 8 pages

## Target State

```
UI component mounts → fetch /api/v1/cluster/pods → Go handler → read from cache → instant response (with freshness metadata)
                                                                    ↑
                                                     SharedInformer updates cache in background
                                                     SQLite stores last-known state as fallback
```

## Out of Scope

- Custom Resource Definitions (CRDs) for Mozza resources
- Multi-cluster informer federation
- Real-time WebSocket push to frontend (future enhancement)
- Prometheus metrics exporter

---

## User Stories

### US-1: Cluster Connection Status

**As a** user, **I want to** see the cluster connection status instantly on any page **so that** I know whether my cluster is reachable without waiting for API calls to fail.

**Acceptance Criteria:**
- AC-1.1: A background goroutine probes cluster health every 10 seconds via a lightweight API call (server version or namespace list).
- AC-1.2: Connection status is exposed at `GET /api/v1/cluster/status` returning `{"reachable": bool, "last_seen": timestamp, "error": string, "stale": bool}`.
- AC-1.3: All cluster API handlers check `clusterStatus.IsReachable()` before attempting K8s calls. If unreachable, return `503 {"error": "cluster unreachable", "last_seen": "...", "message": "..."}` immediately — no TCP timeout.
- AC-1.4: Frontend fetches cluster status once on app load and caches it in React context. All cluster-dependent pages read from this context.
- AC-1.5: Status badge in the dashboard header: green dot = connected, yellow dot = stale (>30s old), red dot = unreachable.
- AC-1.6: When unreachable, pages show a consistent "Cluster unreachable" banner with last-seen time and link to Clusters page — not generic "Failed to fetch".

### US-2: SharedInformer Cache Layer

**As the** backend, **I want to** maintain an in-memory cache of cluster resources updated by SharedInformers **so that** API handlers return instantly from cache instead of making live K8s calls.

**Acceptance Criteria:**
- AC-2.1: SharedInformers watch: Pods, Deployments, Nodes, Services, Events, Namespaces, Metrics (via metrics-server).
- AC-2.2: Informers start when a cluster is registered/connected and stop when disconnected.
- AC-2.3: Cache is a typed Go struct (`ClusterCache`) with thread-safe read access via `sync.RWMutex`.
- AC-2.4: All existing cluster handlers (`/api/v1/cluster/pods`, `/nodes`, `/deployments`, `/events`, `/services`, `/namespaces`, `/metrics`) read from `ClusterCache` instead of making live API calls.
- AC-2.5: Cache includes metadata: `lastUpdated` timestamp per resource type, `healthy` bool, `error` string.
- AC-2.6: Frontend receives a `_cache` metadata field with each response: `{"_cache": {"age_seconds": 5, "stale": false}}`.

### US-3: SQLite Fallback for Stale Data

**As the** backend, **I want to** persist the last-known cluster state in SQLite **so that** when the cluster disconnects, users see stale data with a "stale" badge instead of error screens.

**Acceptance Criteria:**
- AC-3.1: A background goroutine snapshots the ClusterCache to SQLite every 60 seconds (configurable).
- AC-3.2: Snapshot stores: pods (name, namespace, status, node, image, restarts), deployments (name, namespace, replicas, ready), nodes (name, status, cpu, memory), services, namespaces.
- AC-3.3: When informers disconnect, handlers fall back to SQLite snapshot data.
- AC-3.4: Stale responses include `"stale": true, "snapshot_age": "2m30s"` in the `_cache` metadata.
- AC-3.5: Frontend displays a yellow "Stale data" banner when responses have `stale: true`, showing how old the data is.
- AC-3.6: Stale data older than 10 minutes triggers a "Data may be outdated" warning instead of showing it as current.

### US-4: Backend Error Handling (No More Panics)

**As a** user, **I want** API calls to always return proper JSON errors **so that** the frontend can display meaningful messages instead of "Failed to fetch".

**Acceptance Criteria:**
- AC-4.1: All cluster handlers wrapped in a recovery middleware that catches panics and returns `500 {"error": "internal error", "details": "..."}`.
- AC-4.2: K8s client errors (unauthorized, forbidden, timeout, connection refused) mapped to specific HTTP status codes with human-readable messages.
- AC-4.3: No handler should ever return an empty response body or drop the TCP connection.
- AC-4.4: Error responses include a `"code"` field for frontend error matching: `CLUSTER_UNREACHABLE`, `CLUSTER_UNAUTHORIZED`, `CLUSTER_TIMEOUT`, `INTERNAL_ERROR`.
- AC-4.5: Handler timeout of 10 seconds — if K8s API doesn't respond in 10s, return timeout error from cache.

### US-5: Frontend Request Deduplication

**As the** frontend, **I want to** deduplicate cluster API calls **so that** navigating between pages doesn't fire redundant requests.

**Acceptance Criteria:**
- AC-5.1: A `useClusterData(resource)` hook that caches responses in React context with a TTL (default 15 seconds).
- AC-5.2: Multiple components requesting the same resource within the TTL get the cached response — no duplicate fetch.
- AC-5.3: Manual refresh (Retry/Refresh buttons) bypasses the cache and forces a fresh fetch.
- AC-5.4: Cache is invalidated when navigating to a different cluster or team.
- AC-5.5: Loading state shown only on first fetch — subsequent navigations show cached data immediately with a background refresh.

### US-6: Page-Specific Error States

**As a** user, **I want** each page to handle "no cluster" gracefully **so that** I always know what to do next.

**Acceptance Criteria:**
- AC-6.1: **Overview**: When unreachable, show "Connect a cluster to see your dashboard" with illustration and "Add Cluster" CTA button. No skeleton loaders.
- AC-6.2: **Applications**: When unreachable, show "No cluster connected" with link to Clusters page. Not "Failed to Load / Failed to fetch".
- AC-6.3: **Doctor**: Timeout after 15 seconds. Show "Doctor couldn't reach the cluster" with specific diagnostics (is kubeconfig valid? is cluster responding?). Never hang forever.
- AC-6.4: **Environments**: Show "Connect a cluster to manage environments" with CTA. Fill the empty space.
- AC-6.5: **Monitoring**: Show "Connect a cluster to see metrics" with CTA. Instant, not after 4s of skeletons.
- AC-6.6: All error states include: (1) what went wrong, (2) why, (3) what to do next. Never just "Failed to fetch".

### US-7: Backend Singleflight

**As the** backend, **I want to** deduplicate concurrent identical K8s API calls **so that** 5 frontend requests for pods result in 1 K8s API call.

**Acceptance Criteria:**
- AC-7.1: Use `golang.org/x/sync/singleflight` on all cluster API handlers.
- AC-7.2: Concurrent requests for the same resource type (same cluster, same namespace filter) are coalesced into a single K8s API call.
- AC-7.3: The first request triggers the call; subsequent concurrent requests wait and receive the same result.

---

## Functional Requirements

### FR-1: Cluster Health Monitor
- Background goroutine, 10-second probe interval
- Lightweight health check (list namespaces or server version)
- State machine: Connected → Stale → Unreachable → Reconnecting → Connected
- Exposed via `/api/v1/cluster/status` and checked by all handlers

### FR-2: SharedInformer Integration
- Use `client-go` informer factory for typed watches
- Resources: Pods, Deployments, ReplicaSets, StatefulSets, DaemonSets, CronJobs, Services, Nodes, Events, Namespaces
- Metrics via metrics-server API (separate client, not informer)
- Informer lifecycle tied to cluster connection status

### FR-3: ClusterCache
- Thread-safe in-memory store (`sync.RWMutex`)
- Per-resource-type timestamp tracking
- Typed accessors: `cache.Pods()`, `cache.Nodes()`, `cache.Deployments()`, etc.
- Automatic expiry marking when informer is unhealthy

### FR-4: SQLite Snapshot
- Periodic serialization of ClusterCache to SQLite
- Compact schema (not full K8s objects — just the fields the UI needs)
- Fallback read path when informer is disconnected
- Cleanup of snapshots older than 1 hour

### FR-5: Error Recovery Middleware
- Panic recovery on all cluster handlers
- Structured JSON error responses with error codes
- Request timeout enforcement (10s default)
- K8s error classification and mapping

### FR-6: Frontend Cache Layer
- React context for cluster data with TTL-based invalidation
- `useClusterData` hook with loading/error/stale states
- Cluster status context consumed by all pages
- Deduplication via shared promise (if fetch in flight, return same promise)

---

## Implementation Phases

### Phase 1: Error Handling Foundation
- Recovery middleware on all cluster handlers (no more panics/TCP drops)
- Structured JSON error responses with error codes
- Handler timeouts (10s)
- Singleflight on cluster API calls
- **Verify:** No more `ERR_EMPTY_RESPONSE`. All errors return proper JSON.

### Phase 2: Cluster Health Monitor + Status API
- Background health probe goroutine (10s interval)
- `/api/v1/cluster/status` endpoint
- All handlers check `IsReachable()` before K8s calls — return 503 instantly if unreachable
- Frontend status badge in header
- **Verify:** Pages return "cluster unreachable" instantly (no 4s wait). Header shows connection status.

### Phase 3: SharedInformer Cache Layer
- `ClusterCache` struct with typed accessors
- SharedInformer factory watching Pods, Deployments, Nodes, Services, Events, Namespaces
- All cluster handlers read from cache instead of live API
- Cache metadata (age, staleness) in responses
- **Verify:** Pages load instantly from cache. No per-request K8s API calls.

### Phase 4: SQLite Fallback + Stale Data
- Periodic snapshot of ClusterCache to SQLite (every 60s)
- Fallback read path when informers disconnect
- Stale data badge on frontend
- Snapshot cleanup
- **Verify:** Disconnect cluster → pages still show last-known data with "stale" badge.

### Phase 5: Frontend Cache + Deduplication
- `useClusterData` hook with TTL
- `ClusterStatusContext` consumed by all pages
- Request deduplication (shared promise)
- Background refresh with stale-while-revalidate pattern
- **Verify:** Navigate Overview → Apps → Overview = 1 fetch total, not 3.

### Phase 6: Page-Specific Error States
- Replace generic "Failed to Load" on every page
- Contextual empty states with CTAs
- Doctor page timeout + diagnostics
- Consistent "unreachable" banner component
- **Verify:** Every page handles no-cluster gracefully with actionable message.

---

## Technical Notes

### Backend Files to Create
- `internal/cluster/health.go` — Health monitor goroutine + status API
- `internal/cluster/cache.go` — ClusterCache struct with typed accessors
- `internal/cluster/informers.go` — SharedInformer factory + event handlers
- `internal/cluster/snapshot.go` — SQLite snapshot read/write
- `internal/server/middleware_recovery.go` — Panic recovery + timeout middleware

### Backend Files to Modify
- `internal/server/routes.go` — Add `/cluster/status`, wrap handlers with recovery middleware
- `internal/server/cluster_handlers.go` — Read from cache instead of live K8s API
- `internal/server/server.go` — Initialize ClusterCache, start informers + health monitor

### Frontend Files to Create
- `ui/src/context/ClusterContext.tsx` — Cluster status + data cache context
- `ui/src/hooks/useClusterData.ts` — Deduplicating data hook with TTL
- `ui/src/components/ClusterBanner.tsx` — Reusable unreachable/stale banner

### Frontend Files to Modify
- `ui/src/pages/OverviewPage.tsx` — Use ClusterContext, graceful empty state
- `ui/src/pages/ApplicationsPage.tsx` — Use ClusterContext, contextual error
- `ui/src/pages/MonitoringPage.tsx` — Use ClusterContext
- `ui/src/pages/EnvironmentsPage.tsx` — Use ClusterContext
- `ui/src/pages/DoctorPage.tsx` — Timeout + diagnostics
- `ui/src/App.tsx` or layout — Add ClusterStatusProvider + header badge

### Dependencies
- `k8s.io/client-go` (already in go.mod for K8s operations)
- `golang.org/x/sync/singleflight` (add to go.mod)
- No new frontend dependencies needed
