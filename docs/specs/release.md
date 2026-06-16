# Mozza v1 Launch Specification

## Overview

Prepare Mozza for a public Show HN launch as a premium, fully-featured self-hosted deployment platform. No shortcuts, no MVPs. Every feature is production-grade. The goal: when strangers encounter Mozza for the first time, they immediately understand the value, can install it in one command, and deploy their first app in under 60 seconds.

**Positioning:** "Deploy like you're ordering pizza." The deployment tool for people who hate deployment. Self-hosted. Premium. Pizza-themed brand identity with Stripe-level design quality.

**Target persona:** Indie hackers, solo founders, small dev teams — people technical enough to build apps but who resent the deployment tax.

---

## Scope

### In Scope

**Core Features (P0)**
1. Docker Compose import — full spec support, CLI + wizard
2. Framework auto-detect + smart scaffolding with per-framework wizard (Next.js, Django, Rails, Laravel, Go)

**Core Features (P1)**
3. Git push deploy — webhook receiver, auto-build on Mozza server, Dockerfile generation for repos without one
4. One-line install script + cross-platform binary releases (GoReleaser)

**Core Features (P2)**
5. Auto-SSL via embedded certmagic (Caddy's Go library) — automatic Let's Encrypt, zero config
6. Built-in reverse proxy — route traffic by domain to deployed apps, all on ports 80/443
7. Recipe marketplace — GitHub repo source + CLI search + dashboard browse

**Launch Infrastructure**
8. Move to GitHub (primary repo)
9. README rewrite (authentic pain-first voice, no marketing speak)
10. Complete landing page redesign (custom SVGs, premium pizza brand identity, no pre-existing assets)
11. Hosted landing page (GitHub Pages, free)
12. 60-second demo (side-by-side K8s YAML vs Mozza recipe)
13. Discord + GitHub Discussions community channels

**Bonus Features (make it sick)**
14. Real-time deploy terminal in web UI — live output while deploying, like watching pizza being made
15. App previews / branch deploys — push to branch, get unique URL (branch-name.app.example.com)
16. Built-in monitoring dashboard — CPU, memory, network per app, simple graphs for non-engineers

**Enhancements**
17. Doctor upgrade — better error messages and plain-English suggestions (not full auto-remediation yet)

### Out of Scope
- mozza.cloud hosted offering (post-launch, after traction)
- Pricing / business model decisions (deferred)
- CI/CD pipeline (manual GoReleaser releases)
- Mobile app
- Nomad / Docker Swarm targets
- Full doctor auto-remediation (v1.1)
- Slack/Discord deploy notifications (post-launch)

---

## User Stories

### US-1: Docker Compose Import (CLI)
**Description:** As a developer with an existing docker-compose.yml, I want to convert it to a Mozza recipe so I can deploy with Mozza without rewriting my config.

**Acceptance Criteria:**
- [ ] `mozza import ./docker-compose.yml` reads a local compose file
- [ ] Full Docker Compose spec supported: services, images, build contexts (converted to image requirement), ports, volumes, env vars, depends_on, healthchecks, restart policies, networks, configs, secrets, profiles, extends
- [ ] Unsupported features show clear warning: "Your compose file uses X — here's how Mozza handles this: Y"
- [ ] Output: valid `.mozza` recipe file with clean, idiomatic DSL (not a literal translation — maps cleanly to recipe concepts)
- [ ] Preview mode: shows generated recipe and asks for confirmation before writing
- [ ] `mozza import --dry-run` prints recipe to stdout without writing
- [ ] Handles multi-service compose files (5+ services)
- [ ] Preserves service dependency order via `needs` declarations
- [ ] `make test` passes with import test cases
- [ ] `make lint` passes

### US-2: Docker Compose Import (Wizard)
**Description:** As a non-engineer using the web UI, I want to import my docker-compose.yml through the wizard so I don't have to use the command line.

**Acceptance Criteria:**
- [ ] Deploy wizard has "I have a docker-compose.yml" option alongside "Start from scratch" and "Use a template"
- [ ] User can paste compose file content or upload a file
- [ ] Wizard shows a step-by-step breakdown of detected services with the pizza metaphor ("We found 3 slices in your recipe: Web, API, Database")
- [ ] Each detected service shows its configuration with option to adjust
- [ ] Preview generated recipe before deploying
- [ ] Deploy button triggers the standard deploy flow
- [ ] Error states handled: invalid YAML, empty file, compose file with only build contexts and no images

### US-3: Framework Auto-Detect Engine
**Description:** As a developer, I want Mozza to detect my project's framework and generate an optimized recipe automatically.

**Acceptance Criteria:**
- [ ] `mozza init --detect` scans current directory for project markers
- [ ] Detects: `package.json` (Next.js, React, Vue, Express, Fastify), `requirements.txt`/`pyproject.toml` (Django, Flask, FastAPI), `Gemfile` (Rails), `composer.json` (Laravel), `go.mod` (Go)
- [ ] For each framework, generates a customized recipe with correct: base image, port, health check endpoint, build command, start command, resource limits
- [ ] Smart customization per framework:
  - Next.js: detects standalone vs export mode, SSR vs static
  - Django: detects if using gunicorn/uvicorn, static file serving, collectstatic
  - Rails: detects Puma config, asset pipeline, database adapter
  - Laravel: detects Octane, queue workers, scheduler
  - Go: detects build tags, CGO requirements, static linking
- [ ] Generates Dockerfile if repo doesn't have one (templates with smart customization)
- [ ] `mozza init --detect --framework next` forces specific framework detection
- [ ] Detection results shown to user with confidence level before generating

### US-4: Per-Framework Wizard Flow
**Description:** As a non-engineer, I want the deploy wizard to walk me through deploying my specific framework with guided questions.

**Acceptance Criteria:**
- [ ] Wizard detects framework from GitHub repo URL or uploaded project files
- [ ] Per-framework guided flow:
  - "I see you have a Next.js app. Do you need a database? Which one?"
  - "Your Django app uses PostgreSQL. Want me to set that up too?"
  - "Your Rails app has Sidekiq. Want a background worker?"
- [ ] Each framework flow generates complete recipe including dependencies (database, cache, worker)
- [ ] Pre-filled sensible defaults per framework (e.g., Next.js gets port 3000, Django gets 8000)
- [ ] Wizard explains each decision in non-technical terms ("This is like choosing your pizza toppings")
- [ ] Generated recipe shown for review before deploy

### US-5: Git Push Deploy — Webhook Receiver
**Description:** As a developer, I want to connect my GitHub repo so that pushing code automatically deploys my app.

**Acceptance Criteria:**
- [ ] `mozza connect github.com/user/repo` sets up a webhook on the GitHub repo
- [ ] OAuth flow for GitHub authentication (or personal access token as fallback)
- [ ] Webhook endpoint at `/api/v1/webhooks/github` receives push events
- [ ] Push to default branch triggers deploy
- [ ] Push to other branches triggers preview deploy (see US-7)
- [ ] Webhook validates GitHub signature (X-Hub-Signature-256)
- [ ] Dashboard shows connected repos with status
- [ ] `mozza disconnect` removes webhook
- [ ] Handles webhook delivery failures gracefully (retry with backoff)

### US-6: Git Push Deploy — Auto-Build
**Description:** As a developer who pushed code, I want Mozza to automatically build and deploy my app without me configuring a CI pipeline.

**Acceptance Criteria:**
- [ ] On webhook trigger: clone repo, detect framework, generate Dockerfile if needed, build image, deploy
- [ ] If repo has Dockerfile: use it directly
- [ ] If repo has no Dockerfile: auto-detect framework (US-3) and generate one
- [ ] Build happens on the Mozza server using local Docker
- [ ] Build output streamed to dashboard in real-time (US-14)
- [ ] Built image tagged with commit SHA + branch name
- [ ] Previous image retained for rollback
- [ ] Build failures reported clearly: "Build failed at step 3: npm install returned error — missing dependency X"
- [ ] Build timeout: 10 minutes default, configurable per app
- [ ] Concurrent builds queued (max 2 simultaneous to avoid resource exhaustion)

### US-7: Branch Preview Deploys
**Description:** As a developer, I want to preview changes on a branch before merging to main by getting a unique URL for each branch.

**Acceptance Criteria:**
- [ ] Push to non-default branch creates preview deployment at `{branch-name}.{app-domain}`
- [ ] Preview deploys use the same recipe as production but with isolated resources
- [ ] Dashboard shows list of active preview deploys per app
- [ ] Preview deploys auto-cleaned after branch merge or 7 days of inactivity
- [ ] PR comment posted (optional) with preview URL when preview deploy succeeds
- [ ] `mozza previews list` shows active previews
- [ ] `mozza previews clean` removes stale previews
- [ ] Preview deploys share the same database as production (read-only) OR get isolated database (configurable)

### US-8: Auto-SSL via Certmagic
**Description:** As a user deploying an app with a custom domain, I want HTTPS to work automatically without configuring certificates.

**Acceptance Criteria:**
- [ ] When recipe contains `domain "app.example.com"`, Mozza auto-provisions a Let's Encrypt TLS certificate
- [ ] Uses certmagic (Caddy's Go library) embedded in the Mozza binary — no external dependencies
- [ ] ACME HTTP-01 challenge handled automatically (requires port 80 accessible)
- [ ] Certificate auto-renewed before expiry
- [ ] Certificate storage in Mozza's data directory (survives restarts)
- [ ] HTTP automatically redirects to HTTPS
- [ ] Multiple domains supported (one per app/slice)
- [ ] `mozza serve --domain admin.example.com` provisions TLS for the dashboard itself
- [ ] Graceful fallback: if cert provisioning fails, serve HTTP with clear warning
- [ ] Self-signed cert mode for local/development use

### US-9: Built-in Reverse Proxy
**Description:** As a user running multiple apps on one server, I want traffic routed to the right app by domain name without managing a separate reverse proxy.

**Acceptance Criteria:**
- [ ] Mozza listens on ports 80 and 443 for incoming traffic
- [ ] Routes requests to the correct deployed app based on the `domain` field in the recipe
- [ ] Health-check-aware routing: unhealthy backends removed from rotation
- [ ] WebSocket support (passthrough)
- [ ] Request logging (access logs) with configurable verbosity
- [ ] Custom headers (X-Forwarded-For, X-Real-IP, X-Forwarded-Proto)
- [ ] Rate limiting per domain (configurable, sensible defaults)
- [ ] Dashboard accessible at a configured admin domain (e.g., `admin.mozza.example.com`)
- [ ] Automatic HTTPS for all routed domains (via US-8)
- [ ] `mozza proxy status` shows routing table

### US-10: Recipe Marketplace — Backend
**Description:** As a user, I want to browse, search, and deploy community recipes from a curated collection.

**Acceptance Criteria:**
- [ ] Marketplace recipes stored in a GitHub repo (e.g., `github.com/mozza/recipes`)
- [ ] Each recipe: `.mozza` file + `metadata.yaml` (name, description, category, tags, author, source link)
- [ ] Categories: Web Frameworks, Databases, Monitoring, CMS, DevTools, Communication
- [ ] CLI: `mozza recipe search <query>` searches by name, category, tags
- [ ] CLI: `mozza recipe info <name>` shows recipe details and source link
- [ ] CLI: `mozza recipe install <name>` downloads recipe to current directory
- [ ] CLI: `mozza recipe deploy <name>` deploys directly without saving
- [ ] API endpoint: `GET /api/v1/marketplace/recipes` with search, filter, pagination
- [ ] Marketplace index cached locally, refreshed on `mozza recipe update`
- [ ] Initial catalog: 15+ recipes (existing 10 templates + 5 framework recipes)
- [ ] Each recipe links to original source (Helm chart, compose file) if applicable

### US-11: Recipe Marketplace — Dashboard UI
**Description:** As a non-engineer, I want to browse and deploy recipes from the web dashboard with a visual catalog.

**Acceptance Criteria:**
- [ ] Dashboard has "Marketplace" / "Recipe Catalog" page
- [ ] Grid view with recipe cards: icon, name, description, category badge, one-click deploy
- [ ] Category filter sidebar
- [ ] Search bar with instant results
- [ ] Recipe detail page: full description, recipe preview (syntax highlighted), deploy button, source link
- [ ] "Deploy this recipe" button launches the deploy wizard with the recipe pre-loaded
- [ ] Popular / featured recipes section at top
- [ ] Recipe cards use custom SVG icons per category (not emojis)

### US-12: One-Line Install + Binary Releases
**Description:** As a new user, I want to install Mozza with a single command on any platform.

**Acceptance Criteria:**
- [ ] GoReleaser config produces binaries for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- [ ] Install script at `github.com/gshepptech/mozza/releases` (or similar) detects OS/arch and downloads correct binary
- [ ] `go install github.com/gshepptech/mozza/cmd/mozza@latest` installs Mozza to `/usr/local/bin/mozza`
- [ ] Script verifies checksum after download
- [ ] Script handles: existing installation (upgrade), permission issues (suggests sudo), missing dependencies (warns about Docker)
- [ ] Docker image: `docker run -v /var/run/docker.sock:/var/run/docker.sock mozza/mozza serve`
- [ ] `go install` path still works for Go developers
- [ ] Homebrew tap: `brew install mozza/tap/mozza` (nice to have)
- [ ] GitHub Releases page with changelog, checksums, and install instructions
- [ ] `mozza version` shows version, commit SHA, build date

### US-13: README Rewrite
**Description:** Rewrite the README with authentic, pain-first voice that resonates with the target market.

**Acceptance Criteria:**
- [ ] Opens with real developer frustration — not marketing speak, not "YAML walls" cliché. Something like a real person venting about deployment complexity, then: "...so I built Mozza."
- [ ] 60-second demo GIF/terminal recording embedded prominently
- [ ] Side-by-side: K8s YAML (200 lines) vs Mozza recipe (10 lines) for the same app
- [ ] Quick start: one-line install → `mozza init` → `mozza up` → running app
- [ ] Feature list with real screenshots (not bullet points)
- [ ] "How it works" section with visual flow: Recipe → Detect → Build → Deploy → Monitor
- [ ] Comparison table: Mozza vs Coolify vs Dokku vs Render vs Railway
- [ ] Links to: docs, Discord, GitHub Discussions, marketplace, landing page
- [ ] Contributing section
- [ ] License section (BSL/SSPL — TBD, use placeholder)
- [ ] No emojis. Custom badges/shields only.

### US-14: Real-Time Deploy Terminal in Web UI
**Description:** As a user deploying from the web UI, I want to see live output of the deployment process — like watching my pizza being made.

**Acceptance Criteria:**
- [ ] During deploy, a terminal-style panel shows live output: pulling images, building, creating services, health checks
- [ ] WebSocket connection for real-time streaming (not polling)
- [ ] Each step has a visual indicator: pending → in progress (spinner) → success (checkmark) → failed (X)
- [ ] Build logs shown in a scrollable, syntax-highlighted terminal component
- [ ] Clickable "View full logs" expands to full-screen terminal
- [ ] Deploy progress persists across page refreshes (reconnects to running deploy)
- [ ] On failure: error highlighted in red with plain-English explanation + suggested fix
- [ ] On success: celebration moment + direct link to the deployed app

### US-15: Built-in Monitoring Dashboard
**Description:** As a user, I want to see basic metrics for my deployed apps without setting up Prometheus/Grafana.

**Acceptance Criteria:**
- [ ] Dashboard shows per-app: CPU usage, memory usage, network I/O, request count
- [ ] Simple time-series graphs (last 1h, 6h, 24h, 7d)
- [ ] Metrics collected from Docker stats API (local) or Kubernetes metrics API
- [ ] No external dependencies (no Prometheus, no Grafana)
- [ ] Health status indicator per app (healthy/degraded/down) based on health check results
- [ ] Alert indicators: "Memory usage above 80%" shown as a warning badge
- [ ] Response time metrics if reverse proxy is handling traffic
- [ ] Lightweight: metrics collection adds < 1% CPU overhead
- [ ] Data retention: 7 days of metrics stored in SQLite

### US-16: Landing Page Redesign
**Description:** Complete redesign of the Mozza landing page with premium pizza brand identity, custom SVGs, no pre-existing assets.

**Acceptance Criteria:**
- [ ] Keep existing logo and "Deploy like you're ordering pizza" slogan
- [ ] Everything else redesigned from scratch
- [ ] Premium pizza brand identity: playful but professional, Stripe-level design quality
- [ ] Custom SVG illustrations throughout — no emojis, no stock icons, no pre-existing assets
- [ ] Custom mascot or character that embodies the brand (pizza-themed, clean, modern — not cartoonish)
- [ ] Sections: Hero with demo, "How it works" visual flow, Feature showcase, Recipe comparison, Marketplace preview, Community, Install CTA
- [ ] Warm color palette anchored on existing orange (#ff6b35) with sophisticated typography
- [ ] Responsive: desktop, tablet, mobile
- [ ] Dark theme (current #0a0a0a background or similar)
- [ ] Animations: subtle, purposeful (not gratuitous). Terminal typing effect for CLI demos.
- [ ] Performance: < 3 second load, no external dependencies, static hosting ready
- [ ] Hosted on GitHub Pages (free)

### US-17: Doctor Enhancement
**Description:** Upgrade doctor with better error messages and plain-English suggestions.

**Acceptance Criteria:**
- [ ] Each doctor finding includes a plain-English explanation: "Your app doesn't have a health check. This means Mozza can't tell if your app is actually working. Add `health check /ready` to your recipe."
- [ ] Suggestions are actionable: show the exact recipe line to add/change
- [ ] Group findings by severity with clear labels: "Must fix before deploy" / "Recommended" / "Nice to have"
- [ ] `mozza doctor --fix` auto-applies safe fixes (e.g., adds missing health check defaults)
- [ ] Doctor runs automatically before first deploy with a summary: "Found 2 issues. Fix them? [Y/n]"

### US-18: GitHub Repo Setup
**Description:** Move the Mozza repository to GitHub and set up the community infrastructure.

**Acceptance Criteria:**
- [ ] Repository created on GitHub with appropriate org name
- [ ] Full git history preserved from GitLab
- [ ] GitHub Discussions enabled (categories: Q&A, Show and Tell, Ideas, General)
- [ ] Issue templates: Bug Report, Feature Request, Recipe Submission
- [ ] CONTRIBUTING.md with guidelines
- [ ] CODE_OF_CONDUCT.md
- [ ] Security policy (SECURITY.md)
- [ ] GitHub Actions workflow for: build, test, lint (basic — no full CI/CD yet)
- [ ] Branch protection on main (require PR, require passing checks)

### US-19: Discord Community
**Description:** Set up a Discord server for the Mozza community.

**Acceptance Criteria:**
- [ ] Discord server created with channels: #general, #help, #show-and-tell, #recipes, #feature-requests, #announcements
- [ ] Welcome message with getting-started links
- [ ] Invite link in README, landing page, and docs
- [ ] Basic moderation bot configured
- [ ] Roles: Maintainer, Contributor, Community

---

## Technical Design

### Architecture Overview

```
User → mozza CLI or Web UI
         ↓
    Recipe Engine (parse .mozza / import compose / detect framework)
         ↓
    Plan Builder (validate, dependency DAG, resource allocation)
         ↓
    Compiler (target: K8s manifests or Docker Compose)
         ↓
    Deployer (apply to K8s or run docker compose)
         ↓
    Reverse Proxy (certmagic TLS, domain routing)
         ↓
    Monitor (Docker stats / K8s metrics → SQLite)
```

### New Components

#### 1. Compose Importer (`internal/importer/compose.go`)
- Parse full Docker Compose spec (v3.x)
- Map compose concepts to Mozza recipe AST:
  - `services` → Slices
  - `image` → `from image`
  - `ports` → `on port` / `open to the public`
  - `volumes` (named) → `storage`
  - `environment` → `set X to Y`
  - `depends_on` → `needs`
  - `healthcheck` → `health check`
  - `deploy.replicas` → `run N copies`
  - `build` → warn: "needs pre-built image" with instructions
  - `networks` → NetworkPolicy mapping
  - `configs`/`secrets` → Secret references
- Validate output recipe is parseable and deployable

#### 2. Framework Detector (`internal/detect/`)
- `detect.go` — scan directory for project markers
- `frameworks/nextjs.go`, `django.go`, `rails.go`, `laravel.go`, `go.go`
- Each framework module: detect → analyze → generate recipe + Dockerfile
- Dockerfile templates in `internal/detect/templates/`
- Smart customization: read config files (next.config.js, settings.py, etc.) to adjust template

#### 3. Git Deploy (`internal/gitdeploy/`)
- `webhook.go` — GitHub webhook receiver, signature validation
- `oauth.go` — GitHub OAuth flow for repo connection
- `builder.go` — clone repo, detect framework, build Docker image
- `queue.go` — build queue with concurrency limit (max 2)
- `preview.go` — branch preview deploy management

#### 4. Reverse Proxy (`internal/proxy/`)
- `proxy.go` — HTTP/HTTPS reverse proxy using certmagic for TLS
- `router.go` — domain-based routing table, updated on deploy/undeploy
- `health.go` — backend health checking, remove unhealthy targets
- Integrated into `mozza serve` — single process handles dashboard + proxy + TLS

#### 5. Monitoring (`internal/monitor/`)
- `collector.go` — periodic Docker stats / K8s metrics collection
- `store.go` — time-series data in SQLite (7-day retention, automatic pruning)
- `api.go` — REST endpoints for dashboard graphs

#### 6. Marketplace (`internal/marketplace/`)
- `index.go` — fetch/cache recipe index from GitHub repo
- `search.go` — search by name, category, tags
- `install.go` — download recipe to local directory
- CLI commands: `mozza recipe search|info|install|deploy|update`

### Data Model Changes

New tables in `internal/store/migrations/`:
- `connected_repos` — GitHub repo connections (repo_url, webhook_id, oauth_token_encrypted, app_id)
- `builds` — build history (repo_url, commit_sha, branch, status, logs, duration, image_tag)
- `preview_deploys` — branch previews (app_id, branch, domain, deploy_id, created_at, last_activity)
- `metrics` — time-series metrics (app_id, timestamp, cpu_percent, memory_bytes, network_rx, network_tx, request_count)
- `marketplace_cache` — cached recipe index (name, category, tags, content_hash, fetched_at)
- `certificates` — TLS certificate metadata (domain, issued_at, expires_at, provider)

### API Endpoints (New)

```
# Compose Import
POST   /api/v1/import/compose          — upload compose file, get recipe preview
POST   /api/v1/import/compose/deploy    — import and deploy in one step

# Framework Detection
POST   /api/v1/detect                   — upload project files or provide repo URL, get detection result
POST   /api/v1/detect/generate          — generate recipe from detection result

# Git Deploy
POST   /api/v1/repos/connect            — connect a GitHub repo (starts OAuth)
DELETE /api/v1/repos/{id}/disconnect     — disconnect repo, remove webhook
GET    /api/v1/repos                     — list connected repos
POST   /api/v1/webhooks/github          — webhook receiver (called by GitHub)
GET    /api/v1/builds                    — list builds
GET    /api/v1/builds/{id}/logs          — stream build logs (WebSocket upgrade)

# Preview Deploys
GET    /api/v1/apps/{id}/previews       — list active previews
DELETE /api/v1/apps/{id}/previews/{branch} — clean specific preview

# Monitoring
GET    /api/v1/apps/{id}/metrics        — get metrics (query params: period, resolution)
GET    /api/v1/apps/{id}/health          — get health status

# Marketplace
GET    /api/v1/marketplace/recipes       — list/search recipes
GET    /api/v1/marketplace/recipes/{name} — recipe detail
POST   /api/v1/marketplace/recipes/{name}/deploy — deploy a marketplace recipe

# Proxy
GET    /api/v1/proxy/routes              — current routing table
GET    /api/v1/proxy/certificates         — TLS certificate status
```

### UI Components (New)

```
ui/src/components/
  deploy/
    ComposeImport.tsx        — compose file upload/paste + preview
    FrameworkDetect.tsx       — framework detection result display
    FrameworkWizard.tsx       — per-framework guided flow
    DeployTerminal.tsx        — real-time deploy output (WebSocket)
  repos/
    ConnectRepo.tsx           — GitHub repo connection flow
    RepoList.tsx              — connected repos with status
    BuildHistory.tsx          — build list with status badges
    PreviewDeploys.tsx        — branch preview management
  monitoring/
    MetricsChart.tsx          — time-series graph component
    AppHealth.tsx             — health status indicator
    MonitoringDashboard.tsx   — per-app metrics overview
  marketplace/
    RecipeCatalog.tsx         — grid view with search/filter
    RecipeCard.tsx            — individual recipe card
    RecipeDetail.tsx          — full recipe page with deploy button
```

---

## Non-Functional Requirements

- **NFR-1:** One-line install completes in < 30 seconds on a 50Mbps connection
- **NFR-2:** Framework detection completes in < 5 seconds for any project
- **NFR-3:** Docker Compose import handles files up to 1000 lines
- **NFR-4:** Git push to running deploy in < 3 minutes (excluding build time)
- **NFR-5:** Reverse proxy adds < 5ms latency to proxied requests
- **NFR-6:** Monitoring collection adds < 1% CPU overhead
- **NFR-7:** Landing page loads in < 3 seconds on 3G connection
- **NFR-8:** TLS certificate provisioning completes in < 30 seconds
- **NFR-9:** Dashboard responsive down to 375px width (mobile)
- **NFR-10:** All API endpoints return within 500ms (excluding deploy/build operations)
- **NFR-11:** Single Mozza binary < 50MB
- **NFR-12:** Supports 50 concurrent deployed apps on a single server (8GB RAM, 4 CPU)

---

## Implementation Phases

### Phase 1: Foundation — Import + Detect + Install (Week 1-2)
- [ ] Docker Compose importer (full spec → recipe conversion)
- [ ] Framework detection engine (5 frameworks)
- [ ] Dockerfile template generation (5 frameworks)
- [ ] GoReleaser config + install script
- [ ] CLI commands: `mozza import`, `mozza init --detect`, `mozza recipe`
- **Verification:** `mozza import ./examples/docker-compose.yml` produces valid recipe. `mozza init --detect` in a Next.js project generates correct recipe. `make test` passes.

### Phase 2: Git Deploy + Build (Week 3-4)
- [ ] GitHub webhook receiver + OAuth flow
- [ ] Auto-build engine (clone → detect → build → deploy)
- [ ] Build queue with concurrency control
- [ ] Real-time build log streaming (WebSocket)
- [ ] Branch preview deploys
- [ ] Dashboard: connected repos, build history, preview management
- **Verification:** Push to connected GitHub repo triggers build and deploy. Preview URL accessible for branch push.

### Phase 3: Reverse Proxy + Auto-SSL (Week 5)
- [ ] Certmagic integration for automatic TLS
- [ ] Domain-based reverse proxy routing
- [ ] Health-check-aware backend management
- [ ] Integration with deploy flow (auto-register/deregister routes)
- [ ] `mozza serve` handles dashboard + proxy in single process
- **Verification:** Deploy app with `domain` field → HTTPS works automatically. Multiple apps routed by domain on ports 80/443.

### Phase 4: Marketplace + Monitoring (Week 6)
- [ ] Recipe marketplace GitHub repo with 15+ recipes
- [ ] CLI search + install + deploy commands
- [ ] Dashboard marketplace UI (catalog, search, one-click deploy)
- [ ] Metrics collector (Docker stats / K8s metrics)
- [ ] Monitoring dashboard (graphs, health indicators)
- [ ] Doctor enhancement (plain-English suggestions, `--fix` flag)
- **Verification:** `mozza recipe search django` returns results. Dashboard shows CPU/memory graphs for running app.

### Phase 5: UI Polish + Deploy Terminal (Week 7)
- [ ] Compose import wizard flow
- [ ] Per-framework wizard flow
- [ ] Real-time deploy terminal component
- [ ] Deploy progress with step indicators
- [ ] Build log viewer
- [ ] All new UI pages responsive
- **Verification:** Full wizard flow: paste compose file → see preview → deploy → watch terminal output → app running.

### Phase 6: Landing Page + Launch Prep (Week 8)
- [ ] Complete landing page redesign (custom SVGs, premium pizza brand, no pre-existing assets)
- [ ] README rewrite (authentic voice, pain-first, demo GIF, comparison table)
- [ ] GitHub repo setup (move from GitLab, Discussions, issue templates, CONTRIBUTING.md)
- [ ] Discord server setup
- [ ] 60-second demo recording
- [ ] Domain acquisition + GitHub Pages deployment
- [ ] Final testing: full flow from install → first deploy
- **Verification:** Landing page live at domain. README compelling. Fresh install → `mozza up` works in < 60 seconds.

---

## Definition of Done

This feature is complete when:
- [ ] All 19 user stories' acceptance criteria pass
- [ ] All 6 implementation phases verified
- [ ] Tests pass: `make test`
- [ ] Lint passes: `make lint`
- [ ] Build succeeds: `make build`
- [ ] Binary < 50MB
- [ ] Fresh install (`curl | sh`) → `mozza init --detect` → `mozza up` → running app in < 60 seconds
- [ ] Landing page live and loading < 3 seconds
- [ ] README rewritten with demo GIF
- [ ] Discord server live with invite links
- [ ] GitHub repo public with Discussions enabled
- [ ] 15+ marketplace recipes available
- [ ] Auto-SSL provisions cert for a real domain
- [ ] Git push triggers deploy from a connected repo

---

## Risk Register

| Risk | Mitigation |
|------|-----------|
| Certmagic integration complexity | Start with HTTP-01 challenge only, add DNS challenge later |
| Docker Compose full spec is enormous | Map 80% of features cleanly, provide clear warnings for the rest |
| Framework detection false positives | Confidence scoring + user confirmation before generating |
| Build queue resource exhaustion | Hard limit of 2 concurrent builds, configurable |
| Marketplace cold start (no community) | Ship 15+ recipes ourselves, make contributing trivial |
| Landing page design scope creep | Lock design direction early, iterate after launch |

---

## Next Steps

```bash
# Recommended execution path (large scope, parallel execution + audit)
/decompose docs/specs/release.md
/marathon --mill-ui http://localhost:8080 --spec docs/specs/release.md
```
