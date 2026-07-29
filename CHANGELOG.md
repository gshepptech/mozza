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
