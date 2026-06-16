# Deploy Sources — Template Marketplace + GitHub Import

> **Status:** Draft
> **Date:** 2026-03-16
> **Scope:** Template catalog, GitHub repo import, auto-recipe generation, one-click deploy

---

## 1. Problem

Mozza requires users to bring pre-built container images and write recipes manually. A non-engineer who wants to deploy HobbyFarm doesn't have images — they have a GitHub URL. The wizard assumes too much knowledge.

## 2. Solution: Two Deploy Sources

### Source A: Template Marketplace

Pre-built recipes for popular apps. User picks from a catalog, configures a few settings, clicks deploy.

### Source B: GitHub Import

Paste a repo URL. Mozza scans the repo, auto-generates a recipe, user confirms, deploys.

---

## 3. Template Marketplace

### 3.1 Data Model

```go
// Template represents a pre-built deployable app template.
type Template struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Icon        string            `json:"icon"`        // URL or emoji
    Category    string            `json:"category"`     // "cms", "devtools", "databases", "monitoring", "learning", "ecommerce"
    Tags        []string          `json:"tags"`
    Source      string            `json:"source"`       // .mozza recipe content
    Variables   []TemplateVar     `json:"variables"`    // user-configurable settings
    Repo        string            `json:"repo"`         // source repo URL (for reference)
    Official    bool              `json:"official"`     // curated by Mozza team
    MinK8sVer   string            `json:"min_k8s_ver"`  // minimum K8s version
    EstResources string           `json:"est_resources"` // "1 CPU, 2GB RAM" estimate
}

// TemplateVar is a user-configurable setting in a template.
type TemplateVar struct {
    Key         string `json:"key"`
    Label       string `json:"label"`       // "Admin Password"
    Description string `json:"description"` // "Password for the admin user"
    Type        string `json:"type"`        // "string", "password", "number", "boolean", "select"
    Default     string `json:"default"`
    Required    bool   `json:"required"`
    Options     []string `json:"options,omitempty"` // for "select" type
}
```

### 3.2 Built-in Templates (ship with binary)

Embed a `templates/` directory with these starter templates:

| Template | Category | Services | Description |
|---|---|---|---|
| **WordPress** | CMS | wordpress + mysql | Classic blog/CMS |
| **Ghost** | CMS | ghost + mysql | Modern publishing |
| **Gitea** | DevTools | gitea + postgres | Self-hosted Git |
| **Uptime Kuma** | Monitoring | uptime-kuma | Status monitoring |
| **Plausible** | Analytics | plausible + postgres + clickhouse | Privacy-first analytics |
| **Minio** | Storage | minio | S3-compatible object storage |
| **Redis Commander** | DevTools | redis-commander + redis | Redis GUI |
| **Postgres + pgAdmin** | Databases | postgres + pgadmin | Database with admin UI |
| **n8n** | Automation | n8n + postgres | Workflow automation |
| **HobbyFarm** | Learning | gargantua + ui + shell + controllers + postgres | K8s learning platform |

Each template is a .mozza file with `{{VARIABLE}}` placeholders that get replaced with user inputs.

### 3.3 Template Recipe Format

```mozza
# WordPress — Classic blog/CMS
# Template variables:
#   SITE_TITLE: Your site title (default: "My Blog")
#   ADMIN_PASSWORD: WordPress admin password (required, password)
#   STORAGE_SIZE: Database storage (default: "10Gi")

App: wordpress

Wordpress:
  from image wordpress:6
  open to the public on port 80
  health check /wp-admin/install.php
  run 1 copy
  needs db
  set WORDPRESS_DB_HOST to "db:3306"
  set WORDPRESS_DB_USER to "wordpress"
  set WORDPRESS_DB_PASSWORD to "{{ADMIN_PASSWORD}}"
  set WORDPRESS_DB_NAME to "wordpress"

Db:
  mysql 8, {{STORAGE_SIZE}}, daily backups
  set MYSQL_ROOT_PASSWORD to "{{ADMIN_PASSWORD}}"
  set MYSQL_DATABASE to "wordpress"
  set MYSQL_USER to "wordpress"
  set MYSQL_PASSWORD to "{{ADMIN_PASSWORD}}"
```

### 3.4 API Endpoints

```
GET    /api/v1/templates                    — list all templates (filterable by category, tags)
GET    /api/v1/templates/{id}               — get template details + variables
POST   /api/v1/templates/{id}/deploy        — deploy template with variable values
```

### 3.5 Deploy from Template Flow

```
POST /api/v1/templates/{id}/deploy
{
    "team_id": "...",
    "target": "kubernetes",
    "cluster_id": "...",
    "namespace": "wordpress",
    "variables": {
        "ADMIN_PASSWORD": "supersecret",
        "STORAGE_SIZE": "20Gi",
        "SITE_TITLE": "My Blog"
    }
}
```

Server-side:
1. Load template source
2. Replace `{{VAR}}` placeholders with user values
3. Parse as recipe
4. Build plan + validate
5. Deploy (same pipeline as regular deploy)
6. Return deployment with recipe snapshot

### 3.6 Wizard UI: Template Step

Add a new first step to the deploy wizard — "Choose how to deploy":

```
┌─────────────────────────────────────────────┐
│  How would you like to deploy?               │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Template  │  │  Import  │  │  Custom  │   │
│  │ Catalog   │  │  GitHub  │  │  Recipe  │   │
│  │           │  │          │  │          │   │
│  │ Pick from │  │ Paste a  │  │ Write or │   │
│  │ ready-to- │  │ repo URL │  │ select a │   │
│  │ deploy    │  │ and we   │  │ recipe   │   │
│  │ apps      │  │ figure   │  │ manually │   │
│  │           │  │ it out   │  │          │   │
│  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────┘
```

### 3.7 Template Catalog UI

Grid of template cards, filterable by category:

```
┌─────────────────────────────────────────────┐
│  Categories: [All] [CMS] [DevTools] [DB]    │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ 📝       │  │ 👻       │  │ 🔧       │  │
│  │ WordPress │  │ Ghost    │  │ Gitea    │  │
│  │ Blog/CMS  │  │ Publish  │  │ Self-    │  │
│  │ wp + mysql│  │ ghost+db │  │ hosted   │  │
│  │           │  │          │  │ Git      │  │
│  │ [Deploy]  │  │ [Deploy] │  │ [Deploy] │  │
│  └──────────┘  └──────────┘  └──────────┘  │
│                                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ 📊       │  │ 🗄️       │  │ 🎓       │  │
│  │ Plausible │  │ Postgres │  │ HobbyFarm│  │
│  │ Analytics │  │ + pgAdmin│  │ Learning │  │
│  └──────────┘  └──────────┘  └──────────┘  │
└─────────────────────────────────────────────┘
```

After selecting a template, show variable configuration form:

```
┌─────────────────────────────────────────────┐
│  Configure WordPress                         │
│                                              │
│  Site Title:     [My Blog          ]         │
│  Admin Password: [••••••••         ] 🔒      │
│  Storage Size:   [10Gi             ]         │
│                                              │
│  Estimated resources: 1 CPU, 2GB RAM         │
│                                              │
│  [Deploy to My Computer]  [Deploy to K8s]    │
└─────────────────────────────────────────────┘
```

---

## 4. GitHub Import

### 4.1 Flow

```
User pastes: https://github.com/hobbyfarm/gargantua
                        ↓
Mozza fetches repo metadata (GitHub API)
                        ↓
Scans for: Dockerfile, docker-compose.yml, Helm chart, k8s manifests
                        ↓
Auto-generates a .mozza recipe from what it finds
                        ↓
User reviews/edits the generated recipe
                        ↓
Deploy
```

### 4.2 Repo Scanner

```go
// ScanResult describes what was found in a repository.
type ScanResult struct {
    RepoURL     string          `json:"repo_url"`
    RepoName    string          `json:"repo_name"`
    Description string          `json:"description"`
    Sources     []DetectedSource `json:"sources"`
    Generated   *GeneratedRecipe `json:"generated,omitempty"`
    Warnings    []string        `json:"warnings"`
}

// DetectedSource is something deployable found in the repo.
type DetectedSource struct {
    Type     string `json:"type"`     // "dockerfile", "compose", "helm", "k8s-manifests", "buildpack"
    Path     string `json:"path"`     // "Dockerfile", "docker-compose.yml", "charts/myapp"
    Priority int    `json:"priority"` // higher = preferred
}

// GeneratedRecipe is the auto-generated recipe from repo scan.
type GeneratedRecipe struct {
    Source   string   `json:"source"`   // .mozza content
    Method  string   `json:"method"`   // "from-compose", "from-helm", "from-dockerfile", "from-manifests"
    Editable bool    `json:"editable"` // always true — user can modify before deploy
}
```

### 4.3 Scan Strategies (priority order)

#### Strategy 1: docker-compose.yml → .mozza

If repo has `docker-compose.yml` or `docker-compose.yaml`:
- Parse each service → map to a recipe slice
- Map `image:` → `from image`
- Map `ports:` → `open to the public on port` or `on port`
- Map `depends_on:` → `needs`
- Map `environment:` → `set KEY to VALUE`
- Map `volumes:` with named volumes → storage directives
- Map `restart:` → `restart` directive
- Detect well-known images (postgres, redis, mysql) → engine shorthand

#### Strategy 2: Helm chart → .mozza

If repo has `Chart.yaml` or `charts/` directory:
- Parse `values.yaml` for image refs, replica counts, ports
- Parse templates for service types, ingress rules
- Map to recipe slices
- Extract configurable values as template variables

#### Strategy 3: Kubernetes manifests → .mozza

If repo has `k8s/`, `manifests/`, `deploy/` directories with `.yaml` files:
- Parse Deployment/StatefulSet/DaemonSet → slices
- Parse Services → port config
- Parse Ingress → public + domain
- Parse ConfigMaps/Secrets → env vars

#### Strategy 4: Dockerfile → .mozza

If repo has `Dockerfile` (no compose):
- Single service slice
- Detect EXPOSE port from Dockerfile
- Image = needs to be built (warn user: "This repo needs to be built first. Push the image to a registry, then deploy.")
- Or: if GitHub Container Registry has packages, use `ghcr.io/{owner}/{repo}:latest`

### 4.4 API Endpoints

```
POST   /api/v1/import/scan           — scan a repo URL, return ScanResult
POST   /api/v1/import/deploy         — deploy from scan result (with optional recipe edits)
```

### 4.5 Scan Request/Response

```
POST /api/v1/import/scan
{ "url": "https://github.com/hobbyfarm/gargantua" }

Response:
{
    "repo_url": "https://github.com/hobbyfarm/gargantua",
    "repo_name": "gargantua",
    "description": "Backend API for HobbyFarm",
    "sources": [
        {"type": "helm", "path": "charts/hobbyfarm", "priority": 1},
        {"type": "dockerfile", "path": "Dockerfile", "priority": 2}
    ],
    "generated": {
        "source": "App: gargantua\n\nApi:\n  from image hobbyfarm/gargantua:latest\n  ...",
        "method": "from-helm",
        "editable": true
    },
    "warnings": ["Helm chart detected — using values.yaml defaults"]
}
```

### 4.6 GitHub Import UI

```
┌─────────────────────────────────────────────┐
│  Import from GitHub                          │
│                                              │
│  Repository URL:                             │
│  [https://github.com/hobbyfarm/gargantua  ] │
│                                              │
│  [Scan Repository]                           │
│                                              │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│                                              │
│  ✓ Found: Helm chart at charts/hobbyfarm     │
│  ✓ Generated recipe (9 services)             │
│                                              │
│  ┌─── Generated Recipe ─────────────────┐    │
│  │ App: gargantua                       │    │
│  │                                      │    │
│  │ Api:                                 │    │
│  │   from image hobbyfarm/gargantua     │    │
│  │   on port 8080 as http               │    │
│  │   ...                                │    │
│  │                                [Edit]│    │
│  └──────────────────────────────────────┘    │
│                                              │
│  [Use This Recipe]                           │
└─────────────────────────────────────────────┘
```

After clicking "Use This Recipe", the generated recipe is saved and the wizard continues to configure/networking/target/deploy steps.

---

## 5. Implementation

### 5.1 Backend: Templates

| File | Purpose |
|---|---|
| `internal/template/template.go` | Template struct, variable substitution |
| `internal/template/catalog.go` | Load embedded templates, list/filter |
| `internal/template/embed.go` | go:embed for templates/ directory |
| `templates/*.mozza` | 10 template recipe files with {{VAR}} placeholders |
| `templates/catalog.json` | Template metadata (name, description, category, variables, icon) |
| `internal/server/template_handlers.go` | API handlers: list, get, deploy-from-template |

### 5.2 Backend: GitHub Import

| File | Purpose |
|---|---|
| `internal/importer/scanner.go` | Repo scanner orchestrator |
| `internal/importer/github.go` | GitHub API client (fetch repo metadata, file listing, raw content) |
| `internal/importer/compose.go` | docker-compose.yml → .mozza converter |
| `internal/importer/helm.go` | Helm chart → .mozza converter |
| `internal/importer/manifests.go` | K8s manifests → .mozza converter |
| `internal/importer/dockerfile.go` | Dockerfile → .mozza converter |
| `internal/server/import_handlers.go` | API handlers: scan, deploy-from-import |

### 5.3 Frontend: Wizard Source Selection

| File | Purpose |
|---|---|
| `ui/src/components/deploy/SourceSelector.tsx` | "How to deploy?" — 3 cards (Template, GitHub, Custom) |
| `ui/src/components/deploy/TemplateCatalog.tsx` | Template grid with category filters |
| `ui/src/components/deploy/TemplateConfig.tsx` | Variable configuration form for selected template |
| `ui/src/components/deploy/GitHubImport.tsx` | URL input + scan results + recipe preview/edit |
| `ui/src/api/client.ts` | New API functions: listTemplates, getTemplate, deployTemplate, scanRepo, deployImport |

### 5.4 Wizard Flow Change

Current: Step 1 (Select Recipe) → Step 2 (Configure) → ...

New: Step 0 (Choose Source) → branches:
- **Template** → Template Catalog → Configure Variables → Target → Deploy
- **GitHub** → Paste URL → Scan → Review Recipe → Configure → Target → Deploy
- **Custom Recipe** → existing flow (select/create recipe → configure → ...)

### 5.5 Routes

```go
// Templates
r.Get("/templates", s.handleListTemplates())
r.Get("/templates/{id}", s.handleGetTemplate())
r.Post("/templates/{id}/deploy", s.handleDeployTemplate())

// Import
r.Post("/import/scan", s.handleScanRepo())
r.Post("/import/deploy", s.handleDeployImport())
```

---

## 6. Template Recipes

### 6.1 WordPress

```mozza
App: wordpress

Wordpress:
  from image wordpress:6-apache
  open to the public on port 80
  health check /
  run 1 copy
  needs db
  set WORDPRESS_DB_HOST to "db:3306"
  set WORDPRESS_DB_USER to "wordpress"
  set WORDPRESS_DB_PASSWORD to "{{DB_PASSWORD}}"
  set WORDPRESS_DB_NAME to "wordpress"
  limit cpu to "500m"
  limit memory to "512Mi"

Db:
  mysql 8, {{STORAGE_SIZE}}
  set MYSQL_ROOT_PASSWORD to "{{DB_PASSWORD}}"
  set MYSQL_DATABASE to "wordpress"
  set MYSQL_USER to "wordpress"
  set MYSQL_PASSWORD to "{{DB_PASSWORD}}"
```

Variables: DB_PASSWORD (password, required), STORAGE_SIZE (string, default "10Gi")

### 6.2 Ghost

```mozza
App: ghost

Ghost:
  from image ghost:5-alpine
  open to the public on port 2368
  health check /ghost/api/v4/admin/site/
  run 1 copy
  needs db
  set url to "http://{{DOMAIN}}:2368"
  set database__client to "mysql"
  set database__connection__host to "db"
  set database__connection__user to "ghost"
  set database__connection__password to "{{DB_PASSWORD}}"
  set database__connection__database to "ghost"

Db:
  mysql 8, 10Gi
  set MYSQL_ROOT_PASSWORD to "{{DB_PASSWORD}}"
  set MYSQL_DATABASE to "ghost"
  set MYSQL_USER to "ghost"
  set MYSQL_PASSWORD to "{{DB_PASSWORD}}"
```

### 6.3 Gitea

```mozza
App: gitea

Gitea:
  from image gitea/gitea:latest
  open to the public on port 3000
  on port 22 as ssh using tcp
  health check /api/healthz
  run 1 copy
  needs db
  set GITEA__database__DB_TYPE to "postgres"
  set GITEA__database__HOST to "db:5432"
  set GITEA__database__NAME to "gitea"
  set GITEA__database__USER to "gitea"
  set GITEA__database__PASSWD to "{{DB_PASSWORD}}"

Db:
  postgres 16, 20Gi, daily backups
  set POSTGRES_USER to "gitea"
  set POSTGRES_PASSWORD to "{{DB_PASSWORD}}"
  set POSTGRES_DB to "gitea"
```

### 6.4 Uptime Kuma

```mozza
App: uptime-kuma

Kuma:
  from image louislam/uptime-kuma:1
  open to the public on port 3001
  health check /
  run 1 copy
  mount file "data" at /app/data
  limit cpu to "250m"
  limit memory to "256Mi"
```

### 6.5 Plausible Analytics

```mozza
App: plausible

Plausible:
  from image ghcr.io/plausible/community-edition:v2
  open to the public on port 8000
  health check /api/health
  run 1 copy
  needs db and events-db
  set BASE_URL to "http://{{DOMAIN}}"
  set SECRET_KEY_BASE to "{{SECRET_KEY}}"
  set DATABASE_URL to "postgres://plausible:{{DB_PASSWORD}}@db:5432/plausible"
  set CLICKHOUSE_DATABASE_URL to "http://events-db:8123/plausible_events"

Db:
  postgres 16, 10Gi
  set POSTGRES_USER to "plausible"
  set POSTGRES_PASSWORD to "{{DB_PASSWORD}}"
  set POSTGRES_DB to "plausible"

Events-Db:
  from image clickhouse/clickhouse-server:24-alpine
  on port 8123
  each copy needs its own storage of 20Gi
  set CLICKHOUSE_DB to "plausible_events"
```

### 6.6 MinIO

```mozza
App: minio

Minio:
  from image minio/minio:latest
  open to the public on port 9000
  on port 9001 as console
  health check /minio/health/live
  run 1 copy
  set MINIO_ROOT_USER to "{{ACCESS_KEY}}"
  set MINIO_ROOT_PASSWORD to "{{SECRET_KEY}}"
  mount file "data" at /data
```

### 6.7 Postgres + pgAdmin

```mozza
App: postgres-admin

Db:
  postgres 16, {{STORAGE_SIZE}}, daily backups
  set POSTGRES_USER to "{{DB_USER}}"
  set POSTGRES_PASSWORD to "{{DB_PASSWORD}}"
  set POSTGRES_DB to "{{DB_NAME}}"

Pgadmin:
  from image dpage/pgadmin4:latest
  open to the public on port 80
  run 1 copy
  needs db
  set PGADMIN_DEFAULT_EMAIL to "{{ADMIN_EMAIL}}"
  set PGADMIN_DEFAULT_PASSWORD to "{{DB_PASSWORD}}"
```

### 6.8 n8n

```mozza
App: n8n

N8n:
  from image n8nio/n8n:latest
  open to the public on port 5678
  health check /healthz
  run 1 copy
  needs db
  set DB_TYPE to "postgresdb"
  set DB_POSTGRESDB_HOST to "db"
  set DB_POSTGRESDB_PORT to "5432"
  set DB_POSTGRESDB_DATABASE to "n8n"
  set DB_POSTGRESDB_USER to "n8n"
  set DB_POSTGRESDB_PASSWORD to "{{DB_PASSWORD}}"

Db:
  postgres 16, 10Gi
  set POSTGRES_USER to "n8n"
  set POSTGRES_PASSWORD to "{{DB_PASSWORD}}"
  set POSTGRES_DB to "n8n"
```

### 6.9 Redis Commander

```mozza
App: redis-tools

Redis-Commander:
  from image rediscommander/redis-commander:latest
  open to the public on port 8081
  run 1 copy
  needs cache
  set REDIS_HOSTS to "local:cache:6379"

Cache:
  redis 7
```

### 6.10 HobbyFarm

Use the existing `examples/hobbyfarm.mozza` as the template source.

---

## 7. Compose → Recipe Converter Rules

| Compose field | Recipe directive |
|---|---|
| `image:` | `from image` |
| `ports: ["8080:80"]` | `open to the public on port 80` |
| `ports: ["8080"]` (no host) | `on port 8080` |
| `depends_on:` | `needs` |
| `environment:` map | `set KEY to "VALUE"` |
| `environment:` list | `set KEY to "VALUE"` (split on `=`) |
| `volumes: [named:/path]` | engine shorthand storage or `mount` |
| `restart: always` | `restart always` |
| `deploy.replicas:` | `run N copies` |
| `image: postgres:*` | `postgres VERSION, SIZE` engine shorthand |
| `image: redis:*` | `redis VERSION` engine shorthand |
| `image: mysql:*` | `mysql VERSION, SIZE` engine shorthand |
| `healthcheck.test:` | `health check PATH` (extract from CMD) |

---

## 8. Testing

- All 10 template recipes must parse and validate via `mozza validate`
- Template variable substitution must handle missing required vars (error), defaults, and all types
- Compose converter must handle: basic compose, compose with volumes, compose with env files, compose with depends_on chains
- GitHub scan must handle: public repos, repos with compose, repos with helm, repos with just Dockerfile, repos with nothing deployable
- UI: template catalog renders, variable form validates required fields, GitHub URL input triggers scan
