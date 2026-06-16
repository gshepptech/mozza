# Deploy Polish — Production-Ready UX

> **Status:** Draft
> **Date:** 2026-03-16
> **Scope:** Post-deploy URL, live progress, health waiting, rollback, private repos, build-from-source, landing page update, guided wizard

---

## 1. Post-Deploy URL

After a successful deploy, show the user where their app is accessible.

### Backend

Update `runDeployWith` in `internal/server/deploy_handlers.go`:
1. After successful K8s apply, query for Ingress resources in the namespace
2. Extract the host/URL from the Ingress spec (or LoadBalancer external IP from Service)
3. Store the URL in the deployment record: add `AccessURL string` field to `store.Deployment`
4. Add `access_url` column to deployments table (migration)
5. Return `access_url` in deployment API responses

```go
// After deploy success, detect access URL
func detectAccessURL(namespace string) string {
    // 1. Check Ingress resources for .spec.rules[].host
    // 2. Check Services of type LoadBalancer for .status.loadBalancer.ingress[].ip
    // 3. Check Services of type NodePort for nodeIP:nodePort
    // 4. Fall back to ClusterIP:port (internal only)
}
```

### Frontend

In `DeployWizard.tsx` deploy success state and `DeploymentsPage.tsx`:
- Show a prominent "Your app is live at" card with the URL as a clickable link
- "Open App" button that opens the URL in a new tab
- If no URL detected (ClusterIP only), show "Internal service — accessible within the cluster"

---

## 2. Deploy Progress (Live Log Streaming)

The SSE endpoint `/api/v1/deployments/{id}/logs` already exists. Wire it to the UI.

### Backend

Update `runDeployWith` to emit more granular log steps:
```
Step 1/5: Parsing recipe...
Step 2/5: Building plan (9 services)...
Step 3/5: Validating plan...
Step 4/5: Applying to kubernetes (hobbyfarm-system)...
Step 5/5: Waiting for pods to be healthy...
✓ All 9 services healthy. Deploy complete in 45s.
```

Each step logged via `AppendDeploymentLog` so SSE picks it up.

### Frontend

Create `ui/src/components/deploy/DeployProgress.tsx`:
- Connect to SSE endpoint on mount
- Parse log lines to show step-by-step progress
- Show a stepper/progress bar: Parse → Plan → Validate → Apply → Healthy
- Each step turns green when complete
- Show elapsed time
- On error, show the error message with retry option
- On success, show the access URL + confetti animation

Replace the current "Preparing your order..." static view with this live progress component.

---

## 3. Health Waiting

After applying K8s manifests, wait for all pods to reach Running/Ready before marking deploy as "succeeded."

### Backend

Add to the K8s deployer (`internal/deploy/k8s/deployer.go`):
```go
func (d *Deployer) WaitForHealthy(ctx context.Context, namespace string, timeout time.Duration) error {
    // Poll every 5s:
    // 1. List all Deployments in namespace — check .status.readyReplicas == .spec.replicas
    // 2. List all StatefulSets — same check
    // 3. List all Jobs — check .status.succeeded >= 1
    // 4. List all DaemonSets — check .status.numberReady == .status.desiredNumberScheduled
    // Timeout after 5 minutes with partial status report
}
```

Log progress during wait:
```
Waiting for pods... (3/9 services ready)
Waiting for pods... (7/9 services ready)
✓ All 9 services healthy.
```

If timeout: mark deploy as "partial" with details on which services aren't ready.

---

## 4. Rollback

If a deploy fails, offer one-click rollback to the previous version.

### Backend

1. Track deploy versions: add `version int` field to deployments (auto-increment per app/namespace)
2. Before deploying, snapshot the current state: `kubectl get -o yaml` all resources in the namespace → store as `previous_state` JSON blob in the deployment record
3. Add `RollbackDeploy(deployID string)` method that re-applies the previous state
4. API endpoint: `POST /api/v1/deployments/{id}/rollback`

### Frontend

In `DeploymentsPage.tsx`:
- Wire the existing "Rollback" button (currently disabled/`cursor-not-allowed`) to call the rollback API
- Show confirmation dialog: "Roll back to previous version?"
- After rollback, show "Rolled back successfully" with the previous version number
- Update deployment status to "rolled_back"

---

## 5. Private Repo Auth

Support GitHub personal access tokens for scanning private repositories.

### Backend

Update `internal/importer/github.go`:
- Accept optional `token string` parameter in all API calls
- Set `Authorization: Bearer {token}` header when token is provided
- Handle 401/403 with helpful error: "Authentication failed — check your token has 'repo' scope"

Update `POST /api/v1/import/scan` to accept optional `token` field in request body.

### Frontend

In `GitHubImport.tsx`:
- Add a collapsible "Private repository?" section below the URL input
- Show a token input field (password type) with label "GitHub Personal Access Token"
- Help text: "Create a token at github.com/settings/tokens with 'repo' scope"
- Token is sent with the scan request, never stored server-side

---

## 6. Build from Source

For repos with a Dockerfile but no pre-built images, help users build and push.

### Backend

This is a guidance feature, not an actual build pipeline (that requires CI infrastructure).

Update the scan result when only a Dockerfile is found:
```json
{
    "generated": {
        "source": "...",
        "method": "from-dockerfile",
        "needs_build": true,
        "build_instructions": "This repo needs a container image built from its Dockerfile.\n\n1. Clone the repo: git clone {url}\n2. Build: docker build -t ghcr.io/{owner}/{repo}:latest .\n3. Push: docker push ghcr.io/{owner}/{repo}:latest\n4. Then deploy with Mozza using the image above."
    }
}
```

If GitHub Container Registry (ghcr.io) already has packages for the repo, detect that and use the image directly (check `https://ghcr.io/v2/{owner}/{repo}/tags/list`).

### Frontend

In `GitHubImport.tsx`, when `needs_build` is true:
- Show an info card: "This repo needs to be built first"
- Show the build instructions in a copyable code block
- Show a "Check for Pre-built Image" button that checks ghcr.io
- If pre-built image found, auto-update the recipe and enable deploy

---

## 7. Landing Page Update

Update the demo recipe to showcase the full workload taxonomy.

### Changes to LandingPage.tsx

Replace the current `acme-shop.mozza` demo (4 slices: web, api, postgres, cache) with a richer example that shows off the new features:

```
App: acme-shop

Storefront:
  from image ghcr.io/acme/web:2.1
  open to the public on port 3000
  health check /healthz
  run 3 copies
  scale between 2 and 8 copies based on cpu 75%
  needs api
  graceful shutdown 30s

Api:
  from image ghcr.io/acme/api:2.1
  on port 8080 as http
  on port 9090 as grpc
  readiness check /ready
  liveness check /healthz
  run 3 copies
  needs db and cache
  needs permission to read secrets
  run as user 1000

Worker:
  from image ghcr.io/acme/worker:2.1
  run 2 copies
  needs db and cache

Nightly-Report:
  from image ghcr.io/acme/reports:2.1
  run every day at 2am
  needs db

Db:
  postgres 16, 50Gi, daily backups

Cache:
  redis 7
```

This shows: multi-port, auto-scaling, graceful shutdown, readiness vs liveness probes, RBAC, security context, CronJob, and the classic web+worker+db+cache pattern. 6 slices instead of 4.

Update `DEMO_PLAN` constant and the visual cards to match.

---

## 8. Guided Wizard for Custom Recipe

Replace the block-type picker with a question-based flow for non-engineers.

### Flow

```
Step 1: "What are you deploying?"
  → Web app / API backend / Background worker / Scheduled job / Database / Full stack app

Step 2 (varies by answer):
  Web app:     "What image?" + "What port?" + "Public?" + "Need a database?"
  API:         "What image?" + "Ports?" + "Need a database?" + "Need a cache?"
  Worker:      "What image?" + "Run forever or on a schedule?"
  Scheduled:   "What image?" + "How often?" (with presets)
  Database:    "PostgreSQL / MySQL / MongoDB" + "Storage size?"
  Full stack:  "How many services?" → mini-interview per service

Step 3: "Any extras?"
  → Database? Cache? Background workers? Scheduled jobs?
  (Each adds a service to the recipe)

Step 4: Review generated recipe → Configure → Deploy
```

### Implementation

Create `ui/src/components/deploy/GuidedWizard.tsx`:
- Question cards with big clickable options (like the source selector)
- Each answer adds services to an internal recipe being built
- At the end, show the generated recipe and transition to the existing configure/networking/target/deploy flow
- "I know what I'm doing → switch to advanced" escape hatch at any point

Wire into `DeployWizard.tsx` as a fourth source option, or replace "Custom Recipe" with this guided flow (keep "Custom Recipe" as an escape hatch within the guided wizard).

---

## 9. Implementation Order

### Wave 1 (no dependencies)
- Item 1: Landing page update (easy, standalone)
- Item 2: Private repo auth (easy, standalone)
- Item 3: Deploy progress streaming UI (medium, uses existing SSE)

### Wave 2 (depends on Wave 1)
- Item 4: Health waiting in K8s deployer (medium)
- Item 5: Post-deploy URL detection (medium, needs health waiting)
- Item 6: Build-from-source guidance (medium, extends importer)

### Wave 3 (depends on Wave 2)
- Item 7: Rollback (hard, needs version tracking + state snapshots)
- Item 8: Guided wizard (medium, standalone UI work)
