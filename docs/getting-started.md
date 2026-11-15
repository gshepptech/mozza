# Getting Started with Mozza

Mozza is a **deployment compiler**. You describe your app once, in plain English, and Mozza either
hands you the file your platform needs or deploys it for you.

Those are the two paths, and you can stop at either one:

| Path | What you run | What you get | What you need |
|---|---|---|---|
| 🥡 **Pick it up** | `mozza compile` | Files on disk (Kubernetes manifests or a Compose file) | Nothing but the binary |
| 🛵 **Get it delivered** | `mozza up` / `mozza deploy` | A running app | Docker, or a cluster |

This guide covers both. If you have no cluster and no Docker, start at
[Compile Only](#compile-only-pick-it-up) — it works offline.

## Prerequisites

- **Nothing** for `mozza compile`, `mozza validate`, and `mozza targets`. No Docker, no cluster,
  no config file, no login, no network.
- **Docker** installed and running — only for `mozza up` (local deployment).
- **kubectl** and a cluster — only for `mozza deploy`.
- **Go 1.25+** — only for building from source.

## Installation

### Option 1: Go Install

```bash
go install github.com/gshepptech/mozza/cmd/mozza@latest
```

### Option 2: Docker

The published image's entrypoint is `mozza serve`, so it runs the dashboard:

```bash
docker build -t mozza .
docker run -p 8080:8080 -v mozza-data:/data mozza
```

To use the image as a compiler instead, override the entrypoint and mount your project:

```bash
docker run --rm -v "$PWD":/app -w /app \
  --entrypoint mozza mozza compile --target local -o ./out
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

```mozza ignore
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

One directive per line, and one line per directive: a directive starts at the beginning of a line
and ends at the end of it, so you cannot put two on one line, and you cannot wrap a long one across
two.

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

## Validate the Recipe

Whichever path you take, start here:

```bash
mozza validate
```

This parses and checks your recipe without deploying anything. You should see:

```
Recipe "my-app" is valid: 2 slice(s) defined.
```

If there are syntax errors, Mozza tells you exactly where the problem is.

## Compile Only: Pick It Up

This is the path where Mozza hands you files and gets out of the way. It contacts no cluster,
reads no kubeconfig, records no deployment state, and needs no config file. It is a compiler you
invoke, like `tsc` or `gcc`.

### Step 1: See What You Can Compile To

```bash
mozza targets
```

```
  TARGET      DESCRIPTION
* kubernetes  Kubernetes manifests (Deployment, Service, Ingress, PVC, RBAC, ...)
  local       Docker Compose file for running on one machine

* default target, used when --target is omitted
```

The `*` marks the default. `kubernetes` is the default because it is the reference semantics for
the recipe language. The other targets are projections of it, and they warn about anything they
cannot express.

`mozza targets` is always the list of what the binary in your hand can compile to.

### Step 2: Compile

`mozza compile` takes two flags:

| Flag | Default | What it does |
|---|---|---|
| `--target <name>` | `kubernetes` | Which target to compile to |
| `-o`, `--output <dir>` | `.` | Directory to write artifacts into |

Compile to Docker Compose:

```bash
mozza compile --target local -o ./out
```

```
  wrote out/docker-compose.yml
Generated docker-compose.yml for "my-app" with 2 service(s), 1 volume(s), 1 network(s).
```

Compile to Kubernetes manifests (the default target, so `--target` can be omitted):

```bash
mozza compile -o ./deploy
```

```
  wrote deploy/k8s/web-deployment.yaml
  wrote deploy/k8s/db-statefulset.yaml
  wrote deploy/k8s/web-service.yaml
  wrote deploy/k8s/db-service.yaml
  wrote deploy/k8s/web-ingress.yaml
generated 5 Kubernetes manifests for "my-app"
```

Note the layout difference: the `kubernetes` target nests its manifests in a `k8s/` subdirectory
of your output directory, while the `local` target writes `docker-compose.yml` at the top of it.

`mozza compile` takes no positional arguments. The recipe comes from the global `--recipe` flag,
which defaults to `app.mozza`.

### Step 3: Read the Warnings

Not every idea exists on every platform. When a target cannot express something in your recipe,
Mozza says so rather than dropping it quietly. Take a recipe that asks for autoscaling:

```
Web:
  from image myorg/web:1.0.0
  open to the public on port 3000
  scale between 2 and 8 copies based on cpu 75%
```

The Kubernetes target expresses it as a HorizontalPodAutoscaler:

```console
$ mozza compile -o ./k8sout
  wrote k8sout/k8s/web-deployment.yaml
  wrote k8sout/k8s/web-service.yaml
  wrote k8sout/k8s/web-horizontalpodautoscaler.yaml
  wrote k8sout/k8s/web-ingress.yaml
generated 4 Kubernetes manifests for "my-app"
```

The local target cannot, and names exactly what it could not carry across:

```console
$ mozza compile --target local -o ./out
  wrote out/docker-compose.yml
Generated docker-compose.yml for "my-app" with 1 service(s), 1 network(s).

1 warning(s):
  ! web: Auto-scaling is Kubernetes-only. Running a fixed replica count locally.
```

Silence means nothing was dropped. Warnings do not fail the build (the exit code stays `0`), so
read them rather than relying on exit status to catch a lost setting.

A typo in `--target` fails immediately, before any parsing, and tells you the valid options:

```console
$ mozza compile --target nomad -o ./x
runCompile: unknown target "nomad" (available: kubernetes, local)
```

That command exits non-zero, so it is safe to put in a script.

### Step 4: Ship the Files

The output is standard, boring, portable files. Nothing in them depends on Mozza:

```bash
mozza compile -o ./deploy
kubectl apply -f ./deploy/k8s/          # or let your CI do it
```

```bash
mozza compile --target local -o ./out
docker compose -f ./out/docker-compose.yml up -d
```

Commit the generated files if you want a GitOps trail, but the file worth reviewing is
`app.mozza`. That is the source; everything under `./deploy` is a build artifact.

In CI, the whole thing is two lines and needs no credentials:

```yaml
- run: mozza validate
- run: mozza compile --target kubernetes -o ./deploy
```

Mozza splits its streams the way a compiler does: payload on stdout, progress and diagnostics on
stderr. Piping and redirecting both behave predictably:

| Stream | What goes there |
|---|---|
| **stdout** | Whatever the command's result *is*: the `mozza targets` table, the `mozza status` table, `mozza logs` output |
| **stderr** | Progress and diagnostics: `mozza compile`'s `wrote …` lines and summary, capability warnings, the `--export` deprecation notice |

For `mozza compile` the artifacts *are* the files on disk, so its chatter goes to stderr and stdout
stays clean:

```bash
mozza compile --target kubernetes -o ./deploy > /dev/null   # progress still visible on stderr
mozza targets | grep -q local && echo "compose target available"
```

`mozza compile > files.txt` therefore produces an empty file: compile writes its files to disk,
not to stdout. Drive scripts off the exit code (`0` on success, non-zero on failure) and the files on
disk, and pipe `mozza targets` or `mozza status` when you want to grep the result.

## Local Deploy: Get It Delivered

With your `app.mozza` ready and Docker running:

### Start Locally

```bash
mozza up
```

Mozza compiles your recipe into a `docker-compose.yml` and runs `docker compose up -d`:

```
  wrote docker-compose.yml
Generated docker-compose.yml for "my-app" with 2 service(s), 1 volume(s), 1 network(s).
Starting services...
...
Services started successfully.
```

The first two lines are exactly what `mozza compile --target local` would have printed. `up` is
the same compile, followed by an apply.

### Check Status

```bash
mozza status
```

This shows a table of all running services:

```
NAME  STATUS   IMAGE               PORTS
api   running  nginx:alpine        8080->8080/tcp
db    running  postgres:16-alpine  5432->5432/tcp
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

```
Stopping services...
...
Services stopped successfully.
```

This tears down the containers and networks created by `mozza up`. Named volumes survive, so your
database data is still there next time you run `mozza up`.

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

`mozza serve` reads `app.mozza` from the working directory at startup, so run it inside a project.

The dashboard provides:

- **Guided Wizard**: walk through creating a recipe step by step, no CLI knowledge needed.
- **Template Marketplace**: browse and deploy pre-built recipes with one click.
- **Deployment History**: view past deployments with order numbers and status tracking.
- **Recipe Editor**: edit your `app.mozza` file directly in the browser.
- **Health Diagnostics**: run `mozza doctor` checks from the UI.

### Compiling over HTTP

Both paths are available through the API, so the dashboard and your own tooling use the same
compiler as the CLI. Both endpoints require a session cookie, but neither requires the `deployer`
or `admin` role. Compiling changes nothing, so a `viewer` can do it.

**`GET /api/v1/targets`** — the same list `mozza targets` prints:

```json
{
  "default": "kubernetes",
  "targets": [
    {
      "name": "kubernetes",
      "description": "Kubernetes manifests (Deployment, Service, Ingress, PVC, RBAC, ...)",
      "default": true
    },
    {
      "name": "local",
      "description": "Docker Compose file for running on one machine",
      "default": false
    }
  ]
}
```

**`POST /api/v1/compile`** — compile recipe source and get the files back. It applies nothing.

Request body: `source` is the recipe text and is required; `target` is optional and defaults to
`kubernetes`.

```json
{
  "target": "local",
  "source": "App: my-app\n\nWeb:\n  from image myorg/web:1.0.0\n  open to the public on port 3000\n  needs db\n\nDb:\n  postgres 16, 10Gi\n"
}
```

The response carries the target that was used, every generated file with its content as text, the
summary line, and any capability warnings. `warnings` is always an array, never `null`:

```json
{
  "target": "local",
  "files": [{ "path": "docker-compose.yml", "content": "services:\n  ..." }],
  "summary": "Generated docker-compose.yml for \"my-app\" with 2 service(s), 1 volume(s), 1 network(s).",
  "warnings": []
}
```

Bad input comes back as `400` with a plain-English reason, such as an unknown target, a missing `source`
or a recipe that will not parse:

```json
{ "error": "unknown target \"nomad\"; valid targets: kubernetes, local" }
{ "error": "source is required" }
{ "error": "recipe parse failed: line 1, col 1: unexpected token Ident" }
```

## Using Templates

Mozza ships with a catalog of 14 ready-to-use recipes for popular applications. Browse them in the web dashboard under the Template Marketplace, or from the CLI with `mozza recipe search`:

| Template | Category | Description |
|----------|----------|-------------|
| **WordPress** | CMS | Classic blog and content management system |
| **Ghost** | CMS | Modern publishing and blog platform |
| **Nextcloud** | CMS | Self-hosted file sync and share platform |
| **Gitea** | DevTools | Self-hosted Git service with web UI |
| **Redis Commander** | DevTools | Redis management GUI with data browser |
| **Uptime Kuma** | Monitoring | Self-hosted status and uptime monitoring |
| **Grafana** | Monitoring | Observability dashboards and visualization platform |
| **Prometheus** | Monitoring | Metrics collection and alerting toolkit |
| **Plausible Analytics** | Analytics | Privacy-first web analytics |
| **MinIO** | Storage | S3-compatible object storage |
| **Postgres + pgAdmin** | Databases | PostgreSQL database with pgAdmin web UI |
| **n8n** | Automation | Workflow automation platform |
| **RabbitMQ** | Communication | Message broker with management UI |
| **Mattermost** | Communication | Team messaging and collaboration platform |

Templates support variables for passwords, domains, and sizing. When you deploy a template through the dashboard, the guided wizard prompts you for each required variable.

All 14 are embedded in the binary, so `mozza recipe search` works offline. The CLI also tries to
merge in a remote community index; when that is unreachable it says so and falls back to the
embedded catalog rather than failing:

```console
$ mozza recipe update
Refreshing marketplace index...
Warning: remote index refresh failed: fetchIndex: HTTP 404 from https://raw.githubusercontent.com/gshepptech/mozza-recipes/main/index.json
Using embedded catalog only.
```

Because templates are just recipes, you can also pick it up rather than get it delivered:

```bash
mozza recipe install ghost
mozza compile --target kubernetes -o ./deploy
```

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

1. **Image validation**: Mozza verifies all container images are accessible.
2. **Manifest generation**: your recipe compiles into Kubernetes resources.
3. **Server-side apply**: manifests are applied to the cluster with progress reporting.
4. **Health waiting**: Mozza waits for pods to become ready (default timeout: 5 minutes, `--timeout` to change).
5. **URL detection**: if ingress is configured, Mozza reports the application URL.

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

### Generate Manifests Without Applying

Use `mozza compile`:

```bash
mozza compile --target kubernetes -o ./deploy
```

> **Deprecated:** `mozza deploy --export` does the same thing and still works, but prints a
> deprecation notice and will be removed. It has no output-directory flag and always writes to
> `./k8s/` relative to the working directory.
>
> ```console
> $ mozza deploy --export
> Warning: --export is deprecated; use `mozza compile --target kubernetes` instead.
> ```

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
- Explore the full workload taxonomy. Mozza supports cron jobs (`run every hour`), daemon sets (`run on every node`), stateful services, init containers, sidecars, auto-scaling, and more.
- Set up team access through the web dashboard.
- Use `mozza compile` in CI to keep generated manifests in version control.
- See the [production guide](production-guide.md) for hardening a real deployment.
