# Getting Started with Mozza

Mozza turns a simple recipe file into production-ready deployments. This guide walks you from zero to a running application in minutes.

## Prerequisites

- **Docker** installed and running (required for local deployment)
- **Go 1.24+** (optional, only for building from source)
- **kubectl** (optional, only for Kubernetes deployment)

## Installation

### Option 1: Go Install

```bash
go install github.com/gshepptech/mozza/cmd/mozza@latest
```

### Option 2: Docker

```bash
docker run --rm -v "$PWD":/app -w /app \
  -v /var/run/docker.sock:/var/run/docker.sock \
  github.com/gshepptech/mozza mozza up
```

### Option 3: Build from Source

```bash
git clone https://github.com/gshepptech/mozza.git
cd mozza
make build
```

The binary is at `bin/mozza`. Move it somewhere on your `$PATH`:

```bash
sudo mv bin/mozza /usr/local/bin/
```

Verify the installation:

```bash
mozza version
```

## Your First Recipe

Mozza reads a recipe file (by default `app.mozza` in the current directory) that describes your application in plain English. Let's build one step by step.

### Step 1: Scaffold a Project

The quickest way to start is with `mozza init`:

```bash
mkdir my-app && cd my-app
mozza init my-app
```

This creates an `app.mozza` file with a starter recipe. But let's write one from scratch to understand the format.

### Step 2: Name Your App

Create a file called `app.mozza` and start with the app name:

```
App: my-app
```

Every recipe begins with `App:` followed by a DNS-compatible name (lowercase letters, numbers, and hyphens).

### Step 3: Add a Web Service

Add a slice (Mozza's term for a service) below the app name:

```
App: my-app

Api:
  from image myorg/api:1.0.0
  open to the public on port 8080
  health check /healthz
  run 2 copies
```

Here is what each line does:

- **`Api:`** names this slice. The name is up to you.
- **`from image`** tells Mozza which Docker image to run.
- **`open to the public on port`** exposes the port and marks it as publicly accessible.
- **`health check`** sets the HTTP path Mozza uses to verify the service is healthy.
- **`run 2 copies`** starts two replicas for availability.

### Step 4: Add a Database

Mozza has shorthand for common data stores. Add a database below your API:

```
App: my-app

Api:
  from image myorg/api:1.0.0
  open to the public on port 8080
  health check /healthz
  run 2 copies
  needs db

Db:
  postgres 16, 10Gi
```

- **`needs db`** declares a dependency. Mozza ensures the database starts before the API.
- **`postgres 16, 10Gi`** provisions PostgreSQL version 16 with 10 GiB of storage.

You can also use `redis` and `mysql` with the same shorthand syntax.

### Step 5: Add Environment Variables

Pass configuration to your services with `set`:

```
App: my-app

Api:
  from image myorg/api:1.0.0
  open to the public on port 8080
  health check /healthz
  run 2 copies
  set DATABASE_URL to "postgres://db:5432/app"
  set LOG_LEVEL to "info"
  needs db

Db:
  postgres 16, 10Gi
```

Each `set KEY to "value"` line becomes an environment variable inside the container.

### Step 6: Resource Limits and Domains (Optional)

For production readiness, add resource limits and a custom domain:

```
App: my-app

Api:
  from image myorg/api:1.0.0
  open to the public on port 8080
  health check /healthz
  run 2 copies
  set DATABASE_URL to "postgres://db:5432/app"
  set LOG_LEVEL to "info"
  limit cpu to "500m"
  limit memory to "256Mi"
  restart always
  domain "api.example.com"
  needs db

Db:
  postgres 16, 10Gi
```

- **`limit cpu`** and **`limit memory`** set Kubernetes-style resource limits.
- **`restart always`** configures the restart policy.
- **`domain`** assigns a custom domain for ingress routing.

## Local Deploy

With your `app.mozza` ready, run these commands in order:

### Validate the Recipe

```bash
mozza validate
```

This parses and checks your recipe without deploying anything. You should see:

```
Recipe "my-app" is valid: 2 slice(s) defined.
```

If there are syntax errors, Mozza tells you exactly where the problem is.

### Start Locally

```bash
mozza up
```

Mozza compiles your recipe into a `docker-compose.yml` and runs `docker compose up -d`. You should see:

```
  wrote docker-compose.yml
Starting services...
Services started successfully.
```

### Check Status

```bash
mozza status
```

This shows a table of all running services:

```
NAME    STATUS   IMAGE                 PORTS
api     running  myorg/api:1.0.0       0.0.0.0:8080->8080/tcp
db      running  postgres:16           5432/tcp
```

### View Logs

```bash
mozza logs
```

This streams logs from all services. Use the `--tail` flag to limit output:

```bash
mozza logs --tail 50
```

### Stop Everything

```bash
mozza down
```

This tears down all containers and networks created by `mozza up`.

## Web Dashboard

Mozza includes a built-in web dashboard for managing deployments visually.

```bash
mozza serve
```

The dashboard starts on `http://localhost:8080` by default. You can change the port:

```bash
mozza serve --port 9090
mozza serve --host 0.0.0.0 --no-auth    # bind to all interfaces (use with caution)
```

The dashboard provides:

- **Guided Wizard** -- walk through creating a recipe step by step, no CLI knowledge needed.
- **Template Marketplace** -- browse and deploy pre-built recipes with one click.
- **Deployment History** -- view past deployments with order numbers and status tracking.
- **Recipe Editor** -- edit your `app.mozza` file directly in the browser.
- **Health Diagnostics** -- run `mozza doctor` checks from the UI.

## Using Templates

Mozza ships with a catalog of ready-to-use recipes for popular applications. Browse them in the web dashboard under the Template Marketplace, or explore the built-in templates:

| Template | Category | Description |
|----------|----------|-------------|
| **WordPress** | CMS | Classic blog and content management system |
| **Ghost** | CMS | Modern publishing and blog platform |
| **Gitea** | DevTools | Self-hosted Git service with web UI |
| **Uptime Kuma** | Monitoring | Self-hosted status and uptime monitoring |
| **Plausible Analytics** | Analytics | Privacy-first web analytics |
| **MinIO** | Storage | S3-compatible object storage |
| **Postgres + pgAdmin** | Databases | PostgreSQL database with pgAdmin web UI |
| **n8n** | Automation | Workflow automation platform |
| **Redis Commander** | DevTools | Redis management GUI with data browser |
| **HobbyFarm** | Learning | Kubernetes-based interactive learning platform |

Templates support variables for passwords, domains, and sizing. When you deploy a template through the dashboard, the guided wizard prompts you for each required variable.

## Deploy to Kubernetes

Once you are happy with your recipe locally, deploy it to a Kubernetes cluster:

```bash
mozza deploy
```

Mozza compiles your recipe into Kubernetes manifests (Deployments, Services, ConfigMaps, PVCs) and applies them via server-side apply.

### Target a Specific Cluster

```bash
mozza deploy --context my-cluster
```

The `--context` flag selects a kubeconfig context. Without it, Mozza uses your current kubectl context.

### What Happens During Deploy

1. **Image validation** -- Mozza verifies all container images are accessible.
2. **Manifest generation** -- your recipe compiles into Kubernetes resources.
3. **Server-side apply** -- manifests are applied to the cluster with progress reporting.
4. **Health waiting** -- Mozza waits for pods to become ready (default timeout: 5 minutes).
5. **URL detection** -- if ingress is configured, Mozza reports the application URL.

Each deployment gets an **order number** (deploy ID) for tracking. You will see output like:

```
Validating images...
[1/3] Creating namespace my-app...
[2/3] Applying 4 resources...
[3/3] Waiting for rollout...
Deploy complete (d-20260317-001) in 42s
  Deployment/api: running
  Service/api: active
  Deployment/db: running
  Service/db: active
```

### Export Without Applying

To generate Kubernetes manifests without deploying:

```bash
mozza deploy --export
```

This writes YAML files to disk for review or use in a GitOps workflow.

### Check Kubernetes Status

```bash
mozza status --target kubernetes
```

This queries the cluster for live pod status, replica counts, and restart counts.

## Rollback

If a deployment goes wrong, revert to the previous successful deployment:

```bash
mozza rollback
```

Mozza retrieves the recipe from the last successful deploy and re-applies it. The rollback is recorded as a new deployment in the history, so you have a complete audit trail.

## Next Steps

- Run `mozza doctor` to diagnose environment issues (Docker connectivity, resource availability, recipe problems).
- Run `mozza serve --help` to see all dashboard options.
- Explore the full workload taxonomy -- Mozza supports cron jobs (`run every hour`), daemon sets (`run on every node`), stateful services, init containers, sidecars, auto-scaling, and more.
- Set up team access through the web dashboard.
- Use `mozza deploy --export` to integrate with your CI/CD pipeline.
