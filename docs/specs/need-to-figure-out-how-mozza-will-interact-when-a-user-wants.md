# Mozza Deployment Pipeline Specification

*Finalized: 2026-03-10*

## Overview

This specification defines the complete deployment pipeline for Mozza — from a `.mozza` recipe file on disk to running containers in a Kubernetes cluster. It covers the CLI flow, the API flow, the internal architecture, and the operational commands (status, logs, rollback, teardown).

Mozza's value proposition is making production Kubernetes deployments accessible to non-engineers. Every decision in this spec reflects that: human-friendly error messages, automatic resource ordering, pre-validation, and sensible defaults.

## Scope

### In Scope

- Refactoring the K8s compiler to emit typed Go structs (`runtime.Object`) instead of YAML strings
- Implementing a K8s deployer using `client-go` with server-side apply
- Atomic rollback on deploy failure (in-memory tracking)
- Recipe-based intentional rollback (store raw .mozza in SQLite)
- New recipe directives: `secret` and `pull secret`
- Pre-validation of images (registry API) and secrets (K8s API)
- Readiness waiting with progress bar UX
- Namespace auto-creation with user confirmation
- Deploy API endpoint (async with polling)
- Store redesign with goose migrations
- Full deploy audit trail in SQLite
- `mozza status` with summary + drill-down
- `mozza logs` with multiplexed pod streaming
- `mozza down` for K8s with PVC confirmation
- `mozza doctor` RBAC permission checks
- Resource cleanup detection on redeploy (removed slices)

### Out of Scope

- Multi-cluster federation (deploying across clusters simultaneously)
- Custom K8s resource support (CRDs, StatefulSets, DaemonSets, Jobs, CronJobs)
- CI/CD integration (GitHub Actions, GitLab CI auto-deploy)
- Staging/production environment concept (users manage via kubeconfig context)
- `mozza promote` command (left as stub)
- Dry-run mode (users can inspect via `mozza compile --export`)
- API authentication (server assumed to run on trusted network for MVP)

## Architecture

### Pipeline Flow

```
.mozza file
  → Lexer (tokenize)                          [internal/recipe]
  → Parser (build AST → Recipe)               [internal/recipe]
  → Build (Recipe → AppPlan)                   [internal/plan]
  → Validate (DNS, ports, deps, resources)     [internal/plan]
  → Compile (AppPlan → []runtime.Object)       [internal/k8s]
  → Pre-validate (images + secrets)            [internal/deploy/k8s]
  → Apply (server-side apply via client-go)    [internal/deploy/k8s]
  → Wait for readiness                         [internal/deploy/k8s]
  → Record deploy to state store               [internal/store]
```

### Package Layout

| Package | Responsibility |
|---------|---------------|
| `internal/recipe` | Lexer, parser, AST (unchanged) |
| `internal/plan` | Build, validate (unchanged) |
| `internal/k8s` | Compile AppPlan → typed K8s objects. Resource builder pattern: `BuildDeployment()`, `BuildService()`, etc. |
| `internal/deploy` | Deployer interface definition |
| `internal/deploy/k8s` | K8s deployer implementation (client-go, server-side apply, readiness wait, rollback) |
| `internal/store` | Redesigned SQLite store with goose migrations (replaces `internal/state`) |
| `internal/cli` | CLI commands (deploy, status, logs, down, rollback) |
| `internal/server` | API handlers (deploy endpoint, status polling) |

### Deployer Interface

```go
// internal/deploy/deployer.go
type Deployer interface {
    Deploy(ctx context.Context, plan *plan.AppPlan, opts DeployOptions) (*DeployResult, error)
    Rollback(ctx context.Context, deployID string) error
    Status(ctx context.Context, appName string) (*AppStatus, error)
    Logs(ctx context.Context, appName string, opts LogOptions) (io.ReadCloser, error)
    Down(ctx context.Context, appName string, opts DownOptions) error
}

type DeployOptions struct {
    Timeout    time.Duration // default 5m, configurable via --timeout
    Context    string        // kubeconfig context override
    Export     bool          // write manifests to disk without applying
}

type DeployResult struct {
    DeployID   string
    Resources  []DeployedResource
    Duration   time.Duration
    Status     DeployStatus // success, failed, rolled-back
}

type DeployedResource struct {
    Kind      string // Deployment, Service, Ingress, PVC, Namespace
    Name      string
    Namespace string
    Status    string // applied, failed, rolled-back
}
```

## Key Decisions

### 1. Deployment Mechanism: client-go API

Mozza uses the official Kubernetes Go client (`k8s.io/client-go`) to apply resources directly to the cluster. This provides:
- Type-safe resource creation
- Server-side apply with field manager "mozza"
- Programmatic readiness checking
- No dependency on kubectl binary

### 2. Cluster Credentials: Both with Fallback

Credential resolution order:
1. Check `.mozza.yaml` for explicit cluster config (`kubernetes.host`, `kubernetes.token`, etc.)
2. Fall back to `~/.kube/config` (respects `KUBECONFIG` env var)
3. CLI `--context` flag overrides the kubeconfig context

### 3. Server-Side Apply

All resource mutations use K8s server-side apply with field manager name `"mozza"`. This provides:
- Conflict detection (if another tool modified the same fields)
- Partial updates (only fields mozza manages are touched)
- No need for read-modify-write loops

### 4. Typed Structs from Compiler

The K8s compiler is refactored to return `[]runtime.Object` (typed Go structs) instead of YAML strings. Each resource type has a builder function:

```go
// internal/k8s/deployment.go
func BuildDeployment(slice plan.Slice, namespace string) *appsv1.Deployment

// internal/k8s/service.go
func BuildService(slice plan.Slice, namespace string) *corev1.Service

// internal/k8s/ingress.go
func BuildIngress(slice plan.Slice, namespace string) *networkingv1.Ingress

// internal/k8s/pvc.go
func BuildPVC(slice plan.Slice, namespace string) *corev1.PersistentVolumeClaim
```

YAML export is available via `mozza deploy --export` or `mozza compile`, which serializes the typed objects to YAML files.

### 5. Explicit Resource Ordering

Resources are applied in a deterministic order to avoid transient failures:

```
1. Namespace (if auto-creating)
2. PersistentVolumeClaims (storage must exist before pods mount it)
3. Deployments (pods that use storage and secrets)
4. Services (networking for pods)
5. Ingresses (external routing to services)
```

Within each step, slices are ordered by the dependency graph (Ingredients). A slice's resources are fully applied before its dependents begin.

### 6. Readiness Wait with Progress Bar

After applying all resources, mozza blocks until all Deployments report the `Available` condition:

```
[====>   ] 4/7 resources applied
[========] Waiting for readiness...
  api: 1/2 pods ready
  api: 2/2 pods ready ✓
  db:  1/1 pods ready ✓
Deploy complete in 34s
```

- Default timeout: 5 minutes total
- Configurable via `--timeout` flag (e.g., `--timeout 10m`)
- On timeout: trigger atomic rollback

### 7. Atomic Rollback on Failure

During a deploy, mozza tracks all created/updated resources in an in-memory slice. If any step fails or readiness times out:

1. Iterate the tracked resources in reverse order
2. Delete each resource that was created in this deploy
3. Report what was rolled back
4. Record the deploy as `rolled-back` in the state store

This is in-memory only — if mozza crashes mid-deploy, the user must clean up manually. This is acceptable for MVP.

### 8. Namespace Auto-Creation with Confirmation

If the target namespace doesn't exist:

```
Namespace "myapp" does not exist. Create it? [y/N]
```

If the user confirms, mozza creates the namespace as the first step of the deploy. If denied, the deploy aborts with a clear message.

### 9. Human-Friendly Error Messages

All errors surfaced to the user avoid Kubernetes jargon:

| K8s Error | Mozza Message |
|-----------|--------------|
| `ImagePullBackOff` | "Your app 'api' failed to start: the container image 'myorg/api:v2' could not be found. Check that the image exists in your registry." |
| `CrashLoopBackOff` | "Your app 'api' keeps crashing on startup. Check your app's logs with: mozza logs api" |
| `Forbidden` | "Mozza doesn't have permission to create Deployments in namespace 'myapp'. Run: mozza doctor" |
| `PVC resize failed` | "Could not resize storage for 'db' from 10Gi to 20Gi. Your cluster may not support storage expansion." |

### 10. Environment Model: Target Only

There is no staging/production concept in mozza. "Environment" means the deployment target:
- `mozza up` → local Docker (via Docker Compose)
- `mozza deploy` → Kubernetes (via client-go)

Users who need staging vs production manage that through kubeconfig contexts (`--context staging-cluster` vs `--context prod-cluster`).

## Recipe Extensions

### Secret Directive

New syntax for referencing K8s Secrets:

```
secret DB_PASS from db-creds
secret DB_PASS from db-creds key password
```

**Behavior:**
- `secret DB_PASS from db-creds` → env var `DB_PASS` sourced from Secret `db-creds`, key `DB_PASS`
- `secret DB_PASS from db-creds key password` → env var `DB_PASS` sourced from Secret `db-creds`, key `password`
- Pre-validated: mozza checks that the Secret and key exist in the target namespace before deploying
- Compiles to `corev1.EnvVarSource{SecretKeyRef: ...}` in the container spec

### Pull Secret Directive

New syntax for image pull credentials:

```
pull secret registry-creds
```

**Behavior:**
- Adds `imagePullSecrets` to the pod spec for that slice
- Pre-validated: mozza checks that the Secret exists in the target namespace
- The Secret must be of type `kubernetes.io/dockerconfigjson`

### Lexer/Parser Changes

New tokens: `TokenSecret`, `TokenPull`, `TokenKey`, `TokenFrom`

New AST fields on `Slice`:
```go
Secrets     []SecretRef  // from "secret KEY from NAME [key KEYNAME]"
PullSecret  string       // from "pull secret NAME"
```

```go
type SecretRef struct {
    EnvVar     string // environment variable name
    SecretName string // K8s Secret name
    Key        string // key within the Secret (defaults to EnvVar if not specified)
}
```

## Image Pre-Validation

Before deploying, mozza validates that every image referenced in the recipe exists in its registry:

1. Parse image references from all slices
2. For each image, query the container registry API (Docker Registry HTTP API V2)
3. Use credentials from `~/.docker/config.json` for authentication
4. If any image is not found, abort with a human-friendly error before any K8s resources are created

This catches typos and missing images before they cause cryptic `ImagePullBackOff` errors.

## Rollback (Intentional)

`mozza rollback` reverts to a previous deploy by reapplying its recipe:

1. Query the state store for the previous successful deploy of the app
2. Retrieve the stored raw `.mozza` recipe content
3. Run the full pipeline: parse → build → validate → compile → deploy
4. Record the new deploy as a rollback in the audit trail

```
$ mozza rollback
Rolling back "myapp" to deploy #3 (2026-03-09 14:30:00)...
[========] 7/7 resources applied
Deploy complete in 28s (rollback from #4 to #3)
```

## Status Command

### Default View (compact)

```
$ mozza status
myapp (namespace: myapp, context: prod-cluster)
  api    ✓ running  2/2 pods  age: 3d  restarts: 0
  db     ✓ running  1/1 pods  age: 3d  restarts: 0
  cache  ✓ running  1/1 pods  age: 3d  restarts: 0
```

### Detail View

```
$ mozza status --detail
$ mozza status api
api (Deployment)
  Replicas: 2/2 ready
  Image: myorg/api:v1.2.3
  Port: 8080 (public via Ingress)
  Health: /healthz
  CPU: 45m/500m  Memory: 128Mi/256Mi
  Last deploy: 2026-03-09 14:30:00 (#4)
  Events:
    Normal  Scheduled  3d  ...
```

## Logs Command

`mozza logs` streams pod logs from all slices, multiplexed with slice name prefix:

```
$ mozza logs
[api-7d4b9c-x2k4f] 2026-03-10 12:00:01 GET /healthz 200
[api-7d4b9c-m8j2p] 2026-03-10 12:00:01 GET /healthz 200
[db-5f8a2b-r9k3n]  2026-03-10 12:00:05 LOG: checkpoint complete

$ mozza logs api
[api-7d4b9c-x2k4f] 2026-03-10 12:00:01 GET /healthz 200
[api-7d4b9c-m8j2p] 2026-03-10 12:00:01 GET /healthz 200
```

Uses `client-go` pod log streaming. Follows all pods matching the app's label selector. Supports `--follow` (default on) and `--since` flags.

## Teardown (mozza down)

`mozza down` removes all K8s resources for the app:

1. Find all resources with label `app.kubernetes.io/managed-by: mozza` and `app.kubernetes.io/name: {app}`
2. Delete Ingresses, Services, Deployments (non-storage resources)
3. Detect PVCs and prompt:
   ```
   Found persistent storage for "db" (10Gi). Delete it? This is irreversible. [y/N]
   ```
4. If confirmed, delete PVCs
5. Record teardown in audit trail

## Resource Cleanup on Redeploy

When a user removes a slice from their `.mozza` recipe and redeploys:

1. Compare current recipe slices against the previous deploy's slices (from state store)
2. Identify removed slices
3. Show what would be deleted and ask for confirmation:
   ```
   Slice "cache" was removed from the recipe. Delete its K8s resources? [y/N]
   ```
4. If confirmed, delete the removed slice's resources
5. Continue with the normal deploy for remaining slices

## PVC Resize Behavior

When a recipe changes a storage size (e.g., `10Gi` → `20Gi`):

1. Detect the size change by comparing against the existing PVC
2. Attempt to patch the PVC's `spec.resources.requests.storage`
3. If the storage class supports expansion (`allowVolumeExpansion: true`), it succeeds
4. If not, warn the user: "Could not resize storage for 'db'. Your cluster may not support storage expansion." and continue the deploy with the existing size

## Deploy API

### POST /api/deploy

```
POST /api/deploy
Content-Type: text/plain

App: simple-api
Api:
  from image myorg/api:latest
  open to the public on port 8080
  ...
```

Response:
```json
{
  "deploy_id": "d-20260310-143000-abc123",
  "status": "in_progress",
  "app_name": "simple-api"
}
```

HTTP 202 Accepted.

### GET /api/deploys/{deploy_id}

Poll for status:

```json
{
  "deploy_id": "d-20260310-143000-abc123",
  "status": "in_progress",
  "app_name": "simple-api",
  "progress": {
    "resources_total": 7,
    "resources_applied": 4,
    "current_phase": "applying",
    "slices": [
      {"name": "db", "status": "ready"},
      {"name": "api", "status": "waiting", "pods_ready": 1, "pods_desired": 2}
    ]
  }
}
```

Status values: `in_progress`, `success`, `failed`, `rolled_back`

### No Authentication (MVP)

The API server is assumed to run on a trusted network. Authentication will be added in a future iteration.

## State Store Redesign

### Technology

- SQLite with `goose` migrations (embedded in binary via `go:embed`)
- Migrations auto-run on startup (transparent to users)
- Database location: `mozza.db` (configurable in `.mozza.yaml`)

### Schema

#### deploys table

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PK | Deploy ID (e.g., `d-20260310-143000-abc123`) |
| app_name | TEXT | Application name from recipe |
| target | TEXT | `kubernetes` or `local` |
| k8s_context | TEXT | Kubeconfig context used (nullable for local) |
| namespace | TEXT | K8s namespace (nullable for local) |
| recipe_content | TEXT | Raw .mozza file content at deploy time |
| status | TEXT | `success`, `failed`, `rolled_back`, `in_progress` |
| triggered_by | TEXT | `cli` or `ui` |
| error_message | TEXT | Error details if failed (nullable) |
| duration_ms | INTEGER | Deploy duration in milliseconds |
| created_at | DATETIME | Deploy start timestamp |
| completed_at | DATETIME | Deploy completion timestamp (nullable) |
| rollback_of | TEXT FK | If this is a rollback, references the deploy being rolled back (nullable) |

#### deploy_resources table

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER PK | Auto-increment |
| deploy_id | TEXT FK | References deploys.id |
| kind | TEXT | Resource kind (Deployment, Service, etc.) |
| name | TEXT | Resource name |
| namespace | TEXT | Resource namespace |
| status | TEXT | `applied`, `failed`, `rolled_back` |
| applied_at | DATETIME | When the resource was applied |

#### deploy_images table

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER PK | Auto-increment |
| deploy_id | TEXT FK | References deploys.id |
| slice_name | TEXT | Slice that uses this image |
| image | TEXT | Full image reference |

### Queries

- **Get previous deploy for rollback**: `SELECT * FROM deploys WHERE app_name = ? AND status = 'success' ORDER BY created_at DESC LIMIT 1 OFFSET 1`
- **Get deploy history**: `SELECT * FROM deploys WHERE app_name = ? ORDER BY created_at DESC`
- **Get resources for rollback tracking**: `SELECT * FROM deploy_resources WHERE deploy_id = ? ORDER BY applied_at DESC`

## Doctor RBAC Checks

`mozza doctor` adds K8s permission checks:

```
$ mozza doctor
Checking Kubernetes permissions...
  ✓ Can create Deployments in namespace "myapp"
  ✓ Can create Services in namespace "myapp"
  ✓ Can create Ingresses in namespace "myapp"
  ✓ Can create PersistentVolumeClaims in namespace "myapp"
  ✗ Cannot create Namespaces (cluster-scoped)
    → Namespace auto-creation will not work. Pre-create namespaces manually.
```

Uses the K8s `SelfSubjectAccessReview` API to check permissions without attempting actual operations.

## User Stories

### US-1: Deploy a Simple App to Kubernetes

**Description:** As a developer, I want to run `mozza deploy` and have my app running in Kubernetes.

**Acceptance Criteria:**
- [ ] `mozza deploy` with a valid .mozza file and kubeconfig creates all K8s resources
- [ ] Resources are created in the correct order (Namespace → PVC → Deployment → Service → Ingress)
- [ ] Command blocks until all pods are ready
- [ ] Progress bar shows resource apply and readiness status
- [ ] Deploy recorded in SQLite with full audit trail
- [ ] `go test ./internal/deploy/k8s/...` passes

### US-2: Rollback a Failed Deploy

**Description:** As a developer, I want my app rolled back automatically if a deploy fails.

**Acceptance Criteria:**
- [ ] If any resource fails to apply, all previously applied resources are deleted
- [ ] If readiness times out, all resources are rolled back
- [ ] Rollback status reported to user with human-friendly message
- [ ] Deploy recorded as `rolled_back` in state store
- [ ] `go test ./internal/deploy/k8s/... -run TestRollback` passes

### US-3: Intentional Rollback to Previous Version

**Description:** As a developer, I want to run `mozza rollback` to go back to the previous working version.

**Acceptance Criteria:**
- [ ] `mozza rollback` retrieves previous successful deploy's recipe from SQLite
- [ ] Re-runs the full pipeline (parse → build → validate → compile → deploy)
- [ ] New deploy recorded with `rollback_of` reference
- [ ] `go test ./internal/cli/... -run TestRollback` passes

### US-4: Secret References in Recipes

**Description:** As a developer, I want to reference K8s Secrets in my recipe so containers get credentials.

**Acceptance Criteria:**
- [ ] `secret DB_PASS from db-creds` compiles to `SecretKeyRef` in container env
- [ ] `secret DB_PASS from db-creds key password` uses custom key
- [ ] Deploy fails with clear error if referenced Secret doesn't exist
- [ ] `go test ./internal/recipe/... -run TestSecret` passes
- [ ] `go test ./internal/k8s/... -run TestSecret` passes

### US-5: Image Pre-Validation

**Description:** As a developer, I want mozza to check my images exist before deploying.

**Acceptance Criteria:**
- [ ] Before applying, mozza queries the container registry for each image
- [ ] Uses `~/.docker/config.json` for registry authentication
- [ ] Deploy aborts with human-friendly message if image not found
- [ ] `go test ./internal/deploy/k8s/... -run TestImageValidation` passes

### US-6: App Status and Logs

**Description:** As a developer, I want to check my app's health and stream logs.

**Acceptance Criteria:**
- [ ] `mozza status` shows compact slice health table
- [ ] `mozza status --detail` or `mozza status api` shows deep dive
- [ ] `mozza logs` streams all pods, prefixed with slice name
- [ ] `mozza logs api` filters to one slice
- [ ] `go test ./internal/deploy/k8s/... -run TestStatus` passes

### US-7: Teardown with PVC Protection

**Description:** As a developer, I want to tear down my K8s app with `mozza down`.

**Acceptance Criteria:**
- [ ] `mozza down` deletes Deployments, Services, Ingresses
- [ ] Prompts before deleting PVCs: "Delete persistent storage? This is irreversible."
- [ ] PVCs only deleted if user confirms
- [ ] `go test ./internal/deploy/k8s/... -run TestDown` passes

### US-8: Deploy via API

**Description:** As a UI user, I want to trigger deploys from the web dashboard.

**Acceptance Criteria:**
- [ ] `POST /api/deploy` accepts raw recipe text, returns 202 + deploy_id
- [ ] `GET /api/deploys/{id}` returns current progress (resources applied, pod readiness)
- [ ] Status transitions: `in_progress` → `success` | `failed` | `rolled_back`
- [ ] `go test ./internal/server/... -run TestDeployAPI` passes

### US-9: Store Redesign with Migrations

**Description:** As a maintainer, I want a proper store with versioned schema migrations.

**Acceptance Criteria:**
- [ ] goose migrations embedded via `go:embed`, auto-run on startup
- [ ] `deploys`, `deploy_resources`, `deploy_images` tables created
- [ ] Existing state data is not lost (migration handles existing tables)
- [ ] `go test ./internal/store/... ` passes

### US-10: K8s Compiler Refactor to Typed Objects

**Description:** As a maintainer, I want the K8s compiler to emit typed Go structs.

**Acceptance Criteria:**
- [ ] `BuildDeployment()`, `BuildService()`, `BuildIngress()`, `BuildPVC()` return typed objects
- [ ] Compiler returns `[]runtime.Object` (or a custom typed slice)
- [ ] Existing compile tests pass with updated assertions
- [ ] `go test ./internal/k8s/...` passes

## Implementation Phases

### Phase 1: Foundation

- [ ] Redesign `internal/store` with goose migrations and deploy schema
- [ ] Refactor K8s compiler to resource builder pattern returning typed objects
- [ ] Add `secret` and `pull secret` directives to lexer/parser/AST
- [ ] Add `SecretRef` and `PullSecret` fields to plan builder
- **Verification:** `go test ./internal/store/... ./internal/k8s/... ./internal/recipe/... ./internal/plan/...`

### Phase 2: Core Deploy Pipeline

- [ ] Create `internal/deploy` interface and `internal/deploy/k8s` implementation
- [ ] Implement kubeconfig resolution (mozza.yaml → kubeconfig fallback)
- [ ] Implement server-side apply with explicit resource ordering
- [ ] Implement readiness wait with timeout
- [ ] Implement atomic rollback (in-memory tracking)
- [ ] Implement image pre-validation (Docker registry API)
- [ ] Implement secret pre-validation
- [ ] Implement namespace auto-creation with confirmation
- [ ] Wire into `mozza deploy` CLI command
- **Verification:** `go test ./internal/deploy/...` + manual test against a cluster

### Phase 3: Operational Commands

- [ ] Implement `mozza status` (compact + detail views) via client-go
- [ ] Implement `mozza logs` (multiplexed pod streaming) via client-go
- [ ] Implement `mozza rollback` (recipe-based, from state store)
- [ ] Implement `mozza down` (with PVC confirmation)
- [ ] Implement resource cleanup on redeploy (detect removed slices)
- [ ] Add RBAC checks to `mozza doctor`
- **Verification:** `go test ./internal/deploy/... ./internal/cli/...`

### Phase 4: API + Integration

- [ ] Implement `POST /api/deploy` (async, returns deploy-id)
- [ ] Implement `GET /api/deploys/{id}` (status polling)
- [ ] Implement progress bar CLI output
- [ ] Human-friendly error message mapping
- [ ] Wire recipe upload/editor/history in UI (separate UI spec)
- **Verification:** `go test ./internal/server/...` + `go build ./...`

## Non-Functional Requirements

- **NFR-1:** Deploy of a 3-slice app completes server-side apply in under 10 seconds (excluding readiness wait)
- **NFR-2:** Readiness wait polls every 2 seconds, not more frequently
- **NFR-3:** Rollback completes within 30 seconds for apps with up to 10 resources
- **NFR-4:** All K8s API calls use `context.Context` with timeouts
- **NFR-5:** No K8s client-go dependency leaks outside `internal/deploy/k8s` and `internal/k8s`
- **NFR-6:** State store migrations are idempotent (safe to re-run)
- **NFR-7:** Human-friendly error messages for all common K8s failure modes

## Definition of Done

This feature is complete when:
- [ ] All acceptance criteria in user stories US-1 through US-10 pass
- [ ] All implementation phases verified
- [ ] Tests pass: `go test ./...`
- [ ] Lint check: `make lint`
- [ ] Build succeeds: `make build`
- [ ] Manual verification: deploy a multi-slice app to a real K8s cluster

## Open Questions (for future iterations)

- How should mozza handle Horizontal Pod Autoscalers?
- Should mozza support canary/blue-green deployment strategies?
- How to handle config maps (non-secret configuration)?
- Should `mozza deploy` support deploying from a git repo URL?
