# Changelog

All notable changes to Mozza will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Mozza is a deployment compiler with two paths: **pick it up** (`mozza compile` hands you the file)
and **get it delivered** (Mozza deploys it for you). The compiler and its two backends already
existed — this release names them and makes the compile-only path a first-class, documented
surface. See [docs/VISION.md](docs/VISION.md).

### Added

- `mozza compile [--target <name>] [-o <dir>]` — compile a recipe to deployment artifacts on disk
  without deploying. Contacts no cluster, reads no kubeconfig, records no deployment state, and
  needs no config file. Default target `kubernetes`, default output directory `.`. An unknown
  target fails before parsing and lists the valid ones
- `mozza targets` — list the registered compile targets with descriptions, marking the default
- `GET /api/v1/targets` — the registered targets as JSON, plus the default target name
- `POST /api/v1/compile` — compile recipe `source` to an optional `target` and return every
  generated file, the summary, and any capability warnings. Applies nothing, so it requires a
  session but not the `deployer` or `admin` role
- `internal/targets` — one place where every built-in target is registered, shared by the CLI and
  the HTTP API, so the two can never disagree about what targets exist
- `docs/VISION.md` — the product vision and design principles
- `docs/specs/multi-target-compiler.md` — the multi-target compiler milestone plan

### Changed

- README reframed around the deployment-compiler model, documenting both paths
- `docs/getting-started.md` covers the compile-only workflow as a first-class path, followed
  start to finish with no cluster and no Docker
- Capability warnings from a compile are now printed by every command that compiles, rather than
  each command deciding for itself whether to surface them
- Environment variable keys now follow the Kubernetes environment variable name rule — letters,
  digits, `_`, `-` and `.`, not starting with a digit — instead of uppercase and underscores only.
  The rule is taken from `k8s.io/apimachinery` rather than restated, so it cannot drift from what a
  cluster will accept, and a rejected key is now explained in words instead of as a regex
- Ports are now held to the range Kubernetes accepts — 1 to 65535 — everywhere a recipe or a
  hand-built plan can carry one: the slice's own port, each named `ports` entry, a sidecar's ports
  and a TCP health check's port. `on port 0 as http` used to validate and then compile to a
  `containerPort: 0` and a Service on `port: 0` that the API server refuses. A rejected port now
  names the offending value and states the range in words — with the line and column when it came
  from a recipe — and reports once instead of following the real complaint with a spurious
  `unknown directive`

### Deprecated

- `mozza deploy --export` — use `mozza compile --target kubernetes` instead. It still works and
  still writes to `./k8s/`, but prints a deprecation notice to stderr. It will be removed in a
  future minor version

### Removed

- The Prometheus recipe's `RETENTION` variable, which never took effect. Retention is set by the
  `--storage.tsdb.retention.time` command-line flag, which a recipe cannot yet express, so the
  control was inert. Retention stays at the Prometheus default of 15 days — the same value the
  catalog advertised — so behaviour is unchanged

### Fixed

- **Three bundled recipes could not be deployed at all.** `ghost`, `gitea` and `prometheus`
  installed cleanly and then failed to compile, because env-key validation was narrower than
  Kubernetes' own rule and rejected keys as ordinary as Ghost's `database__connection__user`. Every
  recipe in the catalog is now walked through parse, build and validate by `go test`, so a bundled
  recipe that cannot be compiled fails the build rather than the user's first deploy
- **Every HTTP health check compiled to port 80.** A `health check /path` directive names a path but
  no port, and the Kubernetes backend filled that gap with a literal `80` instead of the port the
  slice listens on — so `ghost` probed 80 while serving 2368, `gitea` 3000, `mattermost` 8065 and
  `prometheus` 9090. Liveness and readiness were both affected, so those four recipes rolled out pods
  that could never become ready and hung there. A probe now inherits the slice's port, including one
  declared as a named `ports` entry, and both backends resolve it by the same rule. A check that
  names no port — or nothing to check at all — is now rejected by `mozza validate` rather than
  compiled into a probe aimed at a guess
- **The local target replaced a recipe's probe timings with its own and dropped every check but one,
  without a word.** `health check /healthz every 30s, timeout 9s, wait 25s before starting` compiled
  to `interval: 10s`, `timeout: 5s` and no `start_period` — numbers the author never wrote — and a
  liveness, startup, TCP or command check was discarded outright, so a recipe with three probes
  compiled to one, exited 0 and printed nothing, while the README told the reader that no warning
  means nothing was lost. Compose allows one health check per service, so the local target now emits
  the first readiness, liveness or startup probe Compose can actually run — an HTTP or command check
  of a lesser kind is preferred over a TCP check of a senior one, which Compose cannot run at all
  since the daemon executes a command inside the container rather than opening a socket — and names
  every probe it could not carry in a warning. A check's interval, timeout and initial delay are now
  carried; `start_period` is Compose's closest equivalent to an initial delay rather than the same
  thing, because it keeps probing throughout the window and merely declines to count the failures
  toward `retries`. The 10s/5s/3 numbers survive as the documented defaults for a recipe that states
  no timings. Compose reports a container unhealthy but never restarts it, so a promoted liveness
  check does less locally than on Kubernetes and the warning says so. One deliberate behaviour change
  rides along: for a slice declaring two readiness probes the emitted check now comes from the last
  rather than the first, matching what Kubernetes already compiles. No bundled recipe or example
  declares two readiness probes, so none of them moves
- `mozza import` no longer writes a health check with no port to check. Rejecting unaimed checks
  would otherwise have turned a working import into a dead end: a Compose service with a healthcheck
  but no published `ports:` imported cleanly and then failed to compile. The importer now takes the
  port from the healthcheck URL when Compose publishes none, so a `curl` check on
  `http://localhost:3000/health` yields an `on port 3000` line. Where no port exists anywhere, the
  check is dropped rather than written — reported at import time and recorded as a comment in the
  recipe, both naming what to add to restore it
- **The CLI now writes to stdout.** Every command's output went to stderr and stdout was always
  empty, so `mozza targets | grep local` matched nothing and `mozza deploy --export > list.txt`
  wrote an empty file. Cobra's `Print*` helpers resolve through `OutOrStderr`, which walks the
  command tree for a writer and falls back to stderr when it finds none — and nothing ever set one.
  Streams are now split by intent: payload on stdout (the `targets` and `status` tables, `logs`
  output), progress and diagnostics on stderr (`compile`'s `wrote …` lines and summary, capability
  warnings, the `--export` notice). `mozza compile > files.txt` stays empty by design — for a
  command whose artifacts are files on disk, the chatter is not the output
- Banner and how-it-works graphics no longer describe Mozza as Kubernetes-only, matching the
  compiler reframe in the README
- Corrected stale README claims: the template catalog holds 15 recipes (documented as 10), and the
  minimum Go version is 1.25 (documented as 1.24, while `go.mod` requires 1.25.0)

## [1.0.0] — 2026-03-19

### Added

- 19 CLI commands: init, up, down, deploy, status, logs, rollback, promote, validate, doctor, serve, operator, import, recipe (search/info/install/deploy/update), connect/disconnect, previews, proxy, version
- Recipe language (.mozza files) with parser supporting full Kubernetes workload types
- Kubernetes compiler: Deployment, StatefulSet, DaemonSet, CronJob, Job, Service, Ingress, PVC, RBAC, HPA, PDB, NetworkPolicy
- Local compiler: Docker Compose generation
- Full deploy pipeline with image pre-validation, readiness polling, auto-rollback
- Web dashboard with 20 pages: overview, applications, deployments, environments, monitoring, marketplace, recipe builder, doctor, clusters, teams, profiles
- Deploy wizard with template catalog, GitHub import, Docker Compose import, framework detection
- Recipe review step — preview compiled manifests before deploying
- Order tracking with pizza-themed deployment status
- Auth with session cookies, bcrypt passwords, 3-role RBAC (viewer/deployer/admin)
- Team-based access control with image aliasing
- 15-recipe marketplace (WordPress, Ghost, Gitea, Uptime Kuma, etc.)
- Git deploy with GitHub webhooks and branch preview deployments
- Real-time monitoring with per-app time-series metrics
- Doctor with 11 diagnostic rules and auto-fix
- Reverse proxy with CertMagic auto-TLS, health-checked routing
- Framework auto-detection (Next.js, Django, Rails, Laravel, Go)
- Docker Compose, Helm, Dockerfile importers
- Environment promotion pipeline (dev → staging → production)

### Infrastructure

- SQLite + PostgreSQL dual-driver database
- AES-GCM encryption for stored secrets
- Prometheus metrics endpoint (opt-in)
- Embedded React UI via go:embed
- TLS support (cert files or self-signed)
