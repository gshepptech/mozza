# Changelog

All notable changes to Mozza will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
