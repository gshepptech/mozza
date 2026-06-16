# Specification: Get Mozza Ready for Production

**Feature:** Production-ready Mozza platform
**Created:** 2026-03-10
**Status:** APPROVED

## Overview

Transform Mozza from a working prototype into a production-grade container orchestration platform. This involves replacing SQLite with Postgres, adding an operator process for deployment execution, hardening API security, integrating Kubernetes via client-go, and building the UI pages needed for the full user journey.

## Problem Statement

Mozza has a functional CLI, web dashboard, auth system, and recipe compiler — but the deployment pipeline is stubbed out (`handleTriggerDeploy` marks everything as "succeeded" immediately), the database is SQLite (no NOTIFY, no multi-instance), API endpoints expose infrastructure data without auth, and there's no mechanism to actually execute deployments against real clusters.

## User Journey

1. Admin deploys `mozza serve` + `mozza operator` + Postgres (via docker-compose or K8s)
2. First user registers via the web UI → becomes admin
3. Admin creates a team, invites members
4. Admin adds a K8s cluster (kubeconfig) via the UI clusters page
5. A developer writes a `.mozza` recipe describing their app
6. Developer uploads the recipe via UI or API
7. Developer clicks 'Deploy' in UI → selects cluster + environment
8. API creates deployment record, sends PG NOTIFY
9. Operator picks up, executes via client-go or docker compose, streams logs
10. UI shows deployment progress + logs in real-time
11. Team can see deployment history, rollback, promote between environments

## Scope

### In Scope

1. Replace SQLite with Postgres (goose migrations)
2. Lock all API endpoints behind auth (except `/health`, `/version`)
3. Secure cookies (Secure flag) + rate limiting on auth endpoints
4. Cluster management: DB table, CRUD API, AES-GCM encryption for credentials, UI page
5. Operator process: `mozza operator` command with PG LISTEN/NOTIFY
6. Docker Compose deployment execution in operator
7. Kubernetes deployment execution via client-go in operator
8. Deployment log streaming (operator → DB → UI via SSE)
9. UI pages: cluster management, deploy trigger, deployment log viewer
10. Comprehensive test suite with testcontainers-go for Postgres

### Out of Scope

- SSO / OIDC integration (future milestone)
- API tokens for CI/CD (future milestone)
- Multi-region / HA Mozza deployment
- GitOps / webhook-triggered deployments
- Cost tracking / resource usage dashboards
- Custom domain management for deployed apps
- Backup/restore for Mozza's own Postgres database

## Architecture

### Components

| Component | Binary | Purpose |
|-----------|--------|---------|
| API Server | `mozza serve` | HTTP API + embedded React UI |
| Operator | `mozza operator` | Watches for new deployments, executes them |
| Postgres | External | Shared database for both components |

### Communication Flow

```
User → UI → API Server → Postgres (INSERT deployment + NOTIFY)
                                    ↓
                              PG LISTEN/NOTIFY
                                    ↓
                              Operator (picks up deployment)
                                    ↓
                         Execute (client-go / docker compose)
                                    ↓
                         Update deployment status + logs in Postgres
                                    ↓
                              UI polls/streams logs via SSE
```

### Configuration

Both `mozza serve` and `mozza operator` share:
- `MOZZA_DATABASE_URL` — Postgres connection string
- `MOZZA_ENCRYPTION_KEY` — AES-GCM key for encrypting cluster credentials

Operator-specific:
- `MOZZA_KUBECONFIG` — default kubeconfig path (fallback if cluster has no stored credentials)

## User Stories

### US-1: Postgres Store Migration

**Description:** As a platform admin, I want Mozza to use Postgres so that it supports PG NOTIFY, concurrent access, and production-grade persistence.

**Acceptance Criteria:**
- [ ] `mozza serve --db-url postgres://...` connects to Postgres
- [ ] `mozza operator --db-url postgres://...` connects to the same Postgres
- [ ] `MOZZA_DATABASE_URL` env var is supported
- [ ] Goose migrations create all tables (users, sessions, teams, team_members, recipes, deployments, clusters)
- [ ] All existing store operations (user CRUD, team CRUD, recipe CRUD, deployment CRUD, session management) work against Postgres
- [ ] SQLite code is removed
- [ ] `go test ./internal/store/...` passes with testcontainers-go Postgres

### US-2: API Security Hardening

**Description:** As a platform admin, I want all API endpoints to require authentication so that infrastructure details are not exposed publicly.

**Acceptance Criteria:**
- [ ] Only `/api/v1/health`, `/api/v1/version`, and auth endpoints (register, login, logout) are public
- [ ] `/api/v1/plan`, `/plan/slices`, `/plan/slices/{name}`, `/plan/ingredients`, `/doctor`, `/status` require session cookie
- [ ] Session cookie sets `Secure: true` when served over HTTPS (detect via X-Forwarded-Proto or config flag)
- [ ] Auth endpoints (register, login) are rate-limited (e.g., 10 requests/minute per IP)
- [ ] Unauthenticated requests to protected endpoints return 401 with `{"error": "authentication required"}`
- [ ] `go test ./internal/server/...` covers auth middleware on previously-public endpoints

### US-3: Cluster Management API

**Description:** As a platform admin, I want to register K8s clusters so that teams can deploy to them.

**Acceptance Criteria:**
- [ ] `POST /api/v1/clusters` creates a cluster record with name, kubeconfig (encrypted), and metadata
- [ ] `GET /api/v1/clusters` lists clusters (kubeconfig NOT included in list response)
- [ ] `GET /api/v1/clusters/{id}` returns cluster details (kubeconfig NOT included)
- [ ] `DELETE /api/v1/clusters/{id}` removes a cluster
- [ ] Kubeconfig is encrypted at rest using AES-GCM with `MOZZA_ENCRYPTION_KEY`
- [ ] Missing or invalid encryption key returns a clear error on startup
- [ ] `go test ./internal/store/...` covers cluster CRUD with encryption round-trip
- [ ] `go test ./internal/crypto/...` covers AES-GCM encrypt/decrypt

### US-4: Operator Process (mozza operator)

**Description:** As a platform admin, I want a separate operator process that watches for new deployments and executes them.

**Acceptance Criteria:**
- [ ] `mozza operator` is a new Cobra subcommand
- [ ] Operator connects to Postgres using `MOZZA_DATABASE_URL`
- [ ] Operator uses PG LISTEN on a `deployments` channel
- [ ] When a NOTIFY is received, operator queries for `status='pending'` deployments
- [ ] Operator updates deployment status to `running` before execution
- [ ] On success: status → `succeeded`, log updated with execution output
- [ ] On failure: status → `failed`, log updated with error details
- [ ] Operator handles graceful shutdown (context cancellation)
- [ ] Operator logs all activity via slog
- [ ] Only one operator instance processes a given deployment (use `SELECT ... FOR UPDATE SKIP LOCKED`)

### US-5: Docker Compose Execution

**Description:** As a developer, I want the operator to execute Docker Compose deployments so that I can deploy locally from the UI.

**Acceptance Criteria:**
- [ ] When deployment target is `local` or `docker-compose`, operator runs `docker compose up -d`
- [ ] Operator writes docker-compose.yml from the recipe before executing
- [ ] Deployment log captures stdout/stderr from docker compose
- [ ] Failed docker compose execution sets deployment status to `failed`
- [ ] ProjectDir for compose is derived from deployment metadata

### US-6: Kubernetes Execution via client-go

**Description:** As a developer, I want the operator to apply K8s manifests to a registered cluster so that I can deploy to Kubernetes from the UI.

**Acceptance Criteria:**
- [ ] When deployment target is `kubernetes`, operator uses client-go to apply manifests
- [ ] Operator decrypts the cluster's kubeconfig from the database
- [ ] Operator creates a K8s client from the decrypted kubeconfig
- [ ] Manifests are generated from the recipe using the existing K8s compiler
- [ ] Each manifest is applied via `client.Create` or `client.Apply` (server-side apply)
- [ ] Namespace is created if it doesn't exist
- [ ] Deployment log captures apply results per resource
- [ ] Failed applies set deployment status to `failed` with details
- [ ] `go test ./internal/operator/...` covers execution flow (mocked client)

### US-7: Deployment Trigger + PG NOTIFY

**Description:** As a developer, I want to trigger deployments from the API that are picked up by the operator in real-time.

**Acceptance Criteria:**
- [ ] `POST /api/v1/deployments` creates a deployment with `status='pending'`
- [ ] After INSERT, the API sends `NOTIFY deployments, '{deployment_id}'`
- [ ] Request body includes `recipe_id`, `team_id`, `target` (kubernetes/docker-compose), `cluster_id` (for K8s), `environment`
- [ ] API validates team membership, recipe existence, and cluster existence before creating
- [ ] Response returns 202 Accepted with the deployment record
- [ ] `GET /api/v1/deployments/{id}/logs` streams logs via SSE as the operator updates them

### US-8: Cluster Management UI Page

**Description:** As a platform admin, I want a UI page to manage K8s clusters.

**Acceptance Criteria:**
- [ ] New route `/clusters` in the React app
- [ ] Page lists all registered clusters with name and status
- [ ] "Add Cluster" form accepts name and kubeconfig (textarea or file upload)
- [ ] "Delete" button removes a cluster with confirmation dialog
- [ ] API errors are displayed inline
- [ ] Page is only accessible when authenticated

### US-9: Deploy Trigger UI

**Description:** As a developer, I want to trigger deployments from the UI.

**Acceptance Criteria:**
- [ ] Deploy button on the recipe detail page or a dedicated deploy page
- [ ] Deploy form allows selecting: target (K8s/Docker Compose), cluster (for K8s), environment
- [ ] On submit, calls `POST /api/v1/deployments` and navigates to deployment detail
- [ ] Deployment detail page shows status, metadata, and log output
- [ ] Log output updates in real-time via SSE connection to `/deployments/{id}/logs`

### US-10: Comprehensive Test Suite

**Description:** As a developer, I want comprehensive tests so that the production codebase is reliable.

**Acceptance Criteria:**
- [ ] Store tests use testcontainers-go to run real Postgres
- [ ] Crypto tests verify AES-GCM encrypt/decrypt round-trip, key validation, and error cases
- [ ] Operator tests verify PG LISTEN/NOTIFY flow with real Postgres
- [ ] API tests cover auth middleware on all protected endpoints
- [ ] Rate limiting tests verify request throttling
- [ ] `go test -race ./...` passes
- [ ] Test coverage > 70% on new packages

## Technical Design

### New Packages

| Package | Purpose |
|---------|---------|
| `internal/store` | Refactored for Postgres (pgx driver) |
| `internal/crypto` | AES-GCM encryption/decryption for cluster credentials |
| `internal/operator` | Deployment executor with PG LISTEN/NOTIFY |
| `internal/operator/docker` | Docker Compose execution strategy |
| `internal/operator/k8s` | Kubernetes execution strategy via client-go |

### Database Schema (new/modified tables)

```sql
-- Clusters table (new)
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    kubeconfig_encrypted BYTEA NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Deployments table (modified - add cluster_id)
ALTER TABLE deployments ADD COLUMN cluster_id UUID REFERENCES clusters(id);
```

### Dependencies to Add

| Dependency | Purpose |
|------------|---------|
| `github.com/jackc/pgx/v5` | Postgres driver |
| `github.com/pressly/goose/v3` | Migration management |
| `k8s.io/client-go` | Kubernetes API client |
| `k8s.io/apimachinery` | K8s resource types |
| `github.com/testcontainers/testcontainers-go` | Test-time Postgres containers |

### Rate Limiting

Use a simple in-memory token bucket per IP on auth endpoints. No external dependency needed — implement with `sync.Map` + `time.Ticker`. Configurable via `MOZZA_RATE_LIMIT_RPM` (default: 10).

### Encryption

```go
// internal/crypto/aes.go
func Encrypt(key, plaintext []byte) ([]byte, error) // AES-256-GCM
func Decrypt(key, ciphertext []byte) ([]byte, error)
func ValidateKey(key []byte) error // must be 32 bytes
```

Key derived from `MOZZA_ENCRYPTION_KEY` env var (base64-encoded 32-byte key).

## Implementation Phases

### Phase 1: Foundation (Postgres + Store + Crypto)
- [ ] Add pgx and goose dependencies
- [ ] Write goose migrations for all tables
- [ ] Refactor `internal/store` to use pgx instead of modernc/sqlite
- [ ] Implement `internal/crypto` (AES-GCM)
- [ ] Add clusters table and store methods
- [ ] Update `mozza serve` to accept `--db-url` / `MOZZA_DATABASE_URL`
- [ ] Remove SQLite dependency
- **Verification:** `go test ./internal/store/... ./internal/crypto/...` with testcontainers

### Phase 2: API Hardening + Cluster CRUD
- [ ] Move plan/doctor/status endpoints behind auth middleware
- [ ] Add Secure flag to session cookies (configurable)
- [ ] Implement rate limiting middleware on auth endpoints
- [ ] Add cluster CRUD API endpoints
- [ ] Update deployment trigger to validate cluster_id
- [ ] Send PG NOTIFY after deployment INSERT
- **Verification:** `go test ./internal/server/...` — all endpoints tested for auth

### Phase 3: Operator
- [ ] Create `mozza operator` Cobra command
- [ ] Implement PG LISTEN loop with graceful shutdown
- [ ] Implement deployment pickup with `SELECT ... FOR UPDATE SKIP LOCKED`
- [ ] Implement Docker Compose execution strategy
- [ ] Implement Kubernetes execution strategy via client-go
- [ ] Stream execution logs back to deployment record
- **Verification:** `go test ./internal/operator/...` — PG NOTIFY flow tested with testcontainers

### Phase 4: UI + Integration
- [ ] Clusters management page
- [ ] Deploy trigger form (recipe → cluster → environment)
- [ ] Deployment detail page with SSE log streaming
- [ ] Build embedded UI (`npm run build` in `ui/`)
- [ ] End-to-end manual test: register → create team → add cluster → upload recipe → deploy → view logs
- **Verification:** UI builds successfully, manual E2E walkthrough passes

## Definition of Done

This feature is complete when:
- [ ] All acceptance criteria in user stories US-1 through US-10 pass
- [ ] All implementation phases verified
- [ ] Tests pass: `go test -race ./...`
- [ ] Lint passes: `golangci-lint run`
- [ ] Build succeeds: `go build ./cmd/mozza`
- [ ] UI builds: `cd ui && npm run build`
- [ ] Full user journey works end-to-end (register → deploy → view logs)

## Open Questions

None — all architectural decisions resolved during interview.

## Risk Matrix

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| client-go dependency bloat | High | Medium | Careful import pruning, only import needed packages |
| PG NOTIFY message loss | Low | Medium | SELECT FOR UPDATE SKIP LOCKED ensures pickup even if NOTIFY missed |
| Encryption key rotation | Medium | High | Defer to future milestone, document limitation |
| Operator crash during deployment | Medium | High | Deployment stays in `running` state, operator picks it up on restart with timeout check |
