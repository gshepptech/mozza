<div align="center">

<img src=".github/assets/banner.svg" alt="Mozza: describe your app in plain English, Mozza writes the deployment files" width="100%"/>

[![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-green?style=flat-square)](LICENSE)

</div>

# Mozza

A deployment compiler: short, plain-English `.mozza` recipes compile to Kubernetes manifests or a
Docker Compose file.

Most apps need the same handful of things: run this image, expose this port, keep two copies,
give the database a disk, connect the API to it. Kubernetes spells that out in a couple of hundred
lines of YAML per app, and Compose spells it a different way. Mozza lets you write the intent once,
in about ten lines, and generates the files. `mozza compile` is offline. It reads no kubeconfig,
needs no account, and writes plain files you can commit and apply with your own pipeline. When you
want Mozza to do the applying too, `mozza up` and `mozza deploy` use the same compiler.

## Features

- **A small recipe language** with a hand-written lexer and parser. Errors point at the line and
  column and say what was expected.
- **Two compile targets from one recipe.** `kubernetes` emits Deployments, StatefulSets, Services,
  Ingress, PVCs, Jobs, CronJobs, DaemonSets, HPAs, PDBs, NetworkPolicies and RBAC. `local` emits a
  `docker-compose.yml`.
- **Capability warnings.** When a target cannot express something, such as autoscaling in Compose,
  the compiler prints a warning instead of dropping it silently.
- **Dependency wiring.** `needs db and cache` orders startup and injects `DATABASE_URL` and
  `REDIS_URL` with values that connect to the generated database and cache containers.
- **Importers.** `mozza import` turns a `docker-compose.yml` into a recipe. The web UI can infer
  one from a GitHub repository (Next.js, Django, Rails, Laravel, Go).
- **Local stacks.** `mozza stack up` builds a whole environment on a throwaway kind or k3d
  cluster for apps that already ship their own Helm charts.
- **Optional dashboard.** `mozza serve` runs a web UI with a guided recipe wizard, deploy history,
  rollback and a catalog of 14 app templates.

## Quick start

Requires Go 1.25 or newer.

```bash
git clone https://github.com/gshepptech/mozza.git
cd mozza
go build -o bin/mozza ./cmd/mozza
```

`examples/full-stack.mozza` describes a frontend, an API, PostgreSQL and Redis:

```mozza
# Full Stack: web frontend, API backend, database, and cache

App: full-stack

Images:
  frontend: myorg/frontend:latest
  backend: myorg/backend:latest

Frontend:
  from frontend
  open to the public on port 3000
  health check /health
  run 2 copies
  needs backend

Backend:
  from backend
  on port 8080
  health check /healthz
  run 2 copies
  needs db and cache

Db:
  postgres 16, 10Gi

Cache:
  redis 7
```

Compile it to Kubernetes manifests:

```console
$ ./bin/mozza compile --recipe examples/full-stack.mozza -o out
  wrote out/k8s/frontend-deployment.yaml
  wrote out/k8s/backend-deployment.yaml
  wrote out/k8s/db-statefulset.yaml
  wrote out/k8s/cache-deployment.yaml
  wrote out/k8s/frontend-service.yaml
  wrote out/k8s/backend-service.yaml
  wrote out/k8s/db-service.yaml
  wrote out/k8s/cache-service.yaml
  wrote out/k8s/frontend-ingress.yaml
generated 9 Kubernetes manifests for "full-stack"
```

The `Backend` slice becomes a Deployment with its probes, and the connection strings come from
`needs db and cache`:

```yaml
# out/k8s/backend-deployment.yaml (excerpt)
      containers:
      - env:
        - name: DATABASE_URL
          value: postgres://postgres:mozza@db:5432/db
        - name: REDIS_URL
          value: redis://cache:6379
        image: myorg/backend:latest
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
        name: backend
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
```

Apply the manifests with `kubectl create namespace full-stack && kubectl apply -f out/k8s`.

The same recipe compiles to Compose:

```console
$ ./bin/mozza compile --recipe examples/full-stack.mozza --target local -o out
  wrote out/docker-compose.yml
Generated docker-compose.yml for "full-stack" with 4 service(s), 1 volume(s), 1 network(s).
```

When a target loses something, the compiler says so:

```console
$ ./bin/mozza compile --recipe examples/microservices.mozza --target local -o out
  wrote out/docker-compose.yml
Generated docker-compose.yml for "microservices-demo" with 5 service(s), 1 volume(s), 1 network(s).

3 warning(s):
  ! api-gateway: Auto-scaling is Kubernetes-only. Running a fixed replica count locally.
  ! auth-service: Permissions are Kubernetes-only. Skipped for local target.
  ! auth-service: Compose runs one health check per service, so the liveness check on /healthz was dropped. Only the readiness check on /ready is checked locally.
```

Every recipe in `examples/` compiles to both targets, and CI checks that on each push.

## How it works

```
             ┌───────────┐    ┌─────────────┐    ┌───────────────┐     ┌─▶ k8s compiler   ─▶ k8s/*.yaml
 app.mozza ─▶│   lexer   │──▶ │   parser    │──▶ │  plan.Build   │ ────┤
             │ (tokens)  │    │   (AST)     │    │  + Validate   │     └─▶ local compiler ─▶ docker-compose.yml
             └───────────┘    └─────────────┘    └───────────────┘
                                                  target-neutral
                                                  plan, deps wired
```

1. **Lex and parse** (`internal/recipe`). The lexer is line-oriented: a section header such as
   `Backend:` opens a slice, and each indented line is one directive. The parser is
   recursive-descent and returns typed errors with positions.
2. **Build a plan** (`internal/plan`). The AST becomes a target-neutral `AppPlan`. Engine
   shorthands like `postgres 16, 10Gi` expand to an image, port and volume, workload kinds are
   inferred (`run every hour` is a CronJob, `run on every node` is a DaemonSet), and `needs`
   builds the dependency graph and the connection variables.
3. **Validate.** Names, ports, quantities and the dependency graph are checked against what the
   targets accept, using the Kubernetes API machinery's own rules where they exist.
4. **Compile** (`internal/k8s`, `internal/local`). Each target implements one `Compiler`
   interface and returns files plus capability warnings. `internal/targets` registers them once
   for the CLI and the HTTP API.

`mozza up` and `mozza deploy` compile the same way and then apply the result to Docker or a
cluster: image pre-checks, ordered apply, readiness waits and automatic rollback on failure.

## Recipe reference

A recipe starts with `App: <name>` and an optional `Namespace:`. Each `Name:` block is a slice.

| Directive | Example |
|---|---|
| Image | `from image nginx:1.27` or `from <alias>` with an `Images:` block |
| Ports | `on port 8080`, `open to the public on port 3000` |
| Replicas | `run 3 copies`, `scale between 2 and 8 copies based on cpu 75%` |
| Health | `health check /healthz` |
| Dependencies | `needs db and cache` |
| Engines | `postgres 16, 10Gi`, `redis 7`, `mysql 8`, `mongo 7` |
| Environment | `set LOG_LEVEL to "debug"` |
| Limits | `limit cpu to "500m"`, `limit memory to "512Mi"` |
| Schedules | `run every hour`, `run on every node` |
| Domain | `domain "shop.example.com"` |

[docs/getting-started.md](docs/getting-started.md) walks through the full language, the CLI and
the dashboard. [docs/stacks.md](docs/stacks.md) covers `mozza stack`, and
[docs/production-guide.md](docs/production-guide.md) covers running the dashboard for a team.

### Main commands

| Command | What it does |
|---|---|
| `mozza targets` | List compile targets |
| `mozza compile [--target kubernetes\|local] [-o dir]` | Compile the recipe to files |
| `mozza validate` | Check a recipe without compiling |
| `mozza import docker-compose.yml` | Turn a Compose file into a recipe |
| `mozza up` / `mozza down` | Run the app locally with Docker Compose |
| `mozza deploy` / `mozza rollback` | Apply to a Kubernetes cluster, or undo the last deploy |
| `mozza stack up webapp` | Bring up the built-in web + PostgreSQL + Redis stack on kind |
| `mozza serve` | Start the dashboard on `localhost:8080` |

Every command takes `--recipe <path>` (default `app.mozza`). Configuration comes from
`.mozza.yaml` or `MOZZA_*` environment variables. `compile`, `targets` and `validate` need none.

## Development

```bash
make build    # bin/mozza, with version, commit and date stamped in
make test     # go test -race ./...
make check    # gofmt, go vet and the tests
make ui       # rebuild the React dashboard into internal/ui/dist
```

The Go module builds on its own: the dashboard is prebuilt into `internal/ui/dist` and embedded
with `go:embed`, so Node.js is only needed when you change `ui/`. Tests that drive Docker Compose
skip themselves when Docker is not available. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
workflow.

| Path | Contents |
|---|---|
| `cmd/mozza` | CLI entry point |
| `internal/recipe`, `internal/plan` | Lexer, parser, AST and the target-neutral plan |
| `internal/k8s`, `internal/local`, `internal/targets` | The two compilers and their registry |
| `internal/deploy`, `internal/stack` | Applying to Docker and Kubernetes, and local cluster stacks |
| `internal/importer`, `internal/detect` | Compose, Dockerfile and repository import |
| `internal/server`, `internal/store`, `ui/` | Dashboard API, SQLite/PostgreSQL store and React UI |

## Work in progress

Two features are in the tree but not finished:

- **Git push-to-deploy and branch previews** (`internal/gitdeploy`, `mozza connect`,
  `mozza disconnect`, `mozza previews`). The webhook handler, build queue and preview
  bookkeeping exist, but `mozza serve` does not construct the service yet, so the repo,
  build, webhook and preview endpoints answer 503.
- **App metrics collector** (`internal/monitor`). It samples container CPU, memory and
  network and polls health endpoints, but nothing registers apps with it yet, so the
  per-app metrics, health and time-series endpoints return empty data. The dashboard's
  monitoring page shows pod health from Kubernetes only.

## License

Apache License 2.0. See [LICENSE](LICENSE).
