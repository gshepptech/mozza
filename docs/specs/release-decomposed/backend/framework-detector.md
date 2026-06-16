---
domain: backend
file: framework-detector
depends_on: []
estimated_complexity: high
---

## Purpose

Implement a framework auto-detection engine that scans a project directory, identifies the framework/language, and generates an optimized Mozza recipe and Dockerfile.

## Scope

**Included:**
- `internal/detect/detect.go` — main detection engine
- `internal/detect/frameworks/nextjs.go` — Next.js detection + customization
- `internal/detect/frameworks/django.go` — Django detection + customization
- `internal/detect/frameworks/rails.go` — Rails detection + customization
- `internal/detect/frameworks/laravel.go` — Laravel detection + customization
- `internal/detect/frameworks/go.go` — Go detection + customization
- `internal/detect/templates/` — Dockerfile templates per framework
- CLI command: `mozza init --detect`, `mozza init --detect --framework <name>`

**Excluded:**
- Web UI wizard flow (see frontend/framework-wizard.md)
- API endpoint (see frontend/framework-wizard.md)

## Requirements

- REQ-1: `mozza init --detect` scans current directory for project markers
- REQ-2: Detect package.json → identify Next.js, React, Vue, Express, Fastify
- REQ-3: Detect requirements.txt/pyproject.toml → identify Django, Flask, FastAPI
- REQ-4: Detect Gemfile → identify Rails
- REQ-5: Detect composer.json → identify Laravel
- REQ-6: Detect go.mod → identify Go project
- REQ-7: For each framework, generate recipe with correct: base image, port, health check endpoint, build command, start command, resource limits
- REQ-8: Next.js smart detection: standalone vs export mode, SSR vs static
- REQ-9: Django smart detection: gunicorn/uvicorn, static file serving, collectstatic
- REQ-10: Rails smart detection: Puma config, asset pipeline, database adapter
- REQ-11: Laravel smart detection: Octane, queue workers, scheduler
- REQ-12: Go smart detection: build tags, CGO requirements, static linking
- REQ-13: Generate Dockerfile if repo doesn't have one (framework-specific templates)
- REQ-14: `mozza init --detect --framework next` forces specific framework detection
- REQ-15: Show detection results with confidence level before generating
- REQ-16: Detection completes in < 5 seconds (NFR-2)

## Explicit Behaviors

- Detection works by scanning for marker files: package.json, go.mod, Gemfile, requirements.txt, pyproject.toml, composer.json
- Confidence levels: HIGH (framework-specific config found, e.g., next.config.js), MEDIUM (package.json with framework dep), LOW (generic project structure)
- Next.js: read `next.config.js` for `output: 'standalone'` vs `output: 'export'`; check for `getServerSideProps` usage for SSR detection
- Django: check `manage.py`, `settings.py`; scan requirements for gunicorn/uvicorn; detect STATIC_ROOT config
- Rails: check `Gemfile.lock` for puma; detect `config/database.yml` adapter; check for webpacker/esbuild
- Laravel: check `composer.json` for laravel/octane; detect queue config; check `.env` for APP_PORT
- Go: check for main package, build tags in files, CGO imports, static linking flags in Makefile
- Dockerfile templates use multi-stage builds where appropriate (Go, Next.js)
- Generated recipe includes sensible defaults per framework:
  - Next.js: port 3000, health check /, node:20-alpine base
  - Django: port 8000, health check /health, python:3.12-slim base
  - Rails: port 3000, health check /up, ruby:3.3-slim base
  - Laravel: port 8000, health check /, php:8.3-fpm-alpine base
  - Go: port 8080, health check /healthz, scratch base (static binary)

## Dependencies

None — framework detector is self-contained. Uses existing recipe AST.

## Interfaces

**Input:**
- Directory path to scan
- Optional forced framework name
- Optional repo URL (for remote detection)

**Output:**
- `detect.Result{Framework, Confidence, Details map[string]interface{}, Recipe *recipe.Recipe, Dockerfile string}`

**API (for wizard integration):**
- `detect.Scan(dirPath string) (*Result, error)`
- `detect.ScanWithFramework(dirPath, framework string) (*Result, error)`
- `detect.GenerateDockerfile(result *Result) (string, error)`
- `detect.GenerateRecipe(result *Result) (*recipe.Recipe, error)`

## Constraints

- Detection must complete in < 5 seconds (NFR-2)
- No network calls during detection (local files only)
- Must not modify any files during detection (read-only scan)
- Generated Dockerfiles must produce images < 500MB where possible

## Edge Cases

- Project with multiple frameworks (e.g., Go backend + React frontend) → detect primary, mention secondary
- Monorepo with multiple services → detect at specified path, not root
- No framework detected → suggest manual recipe creation with helpful template
- Corrupted or incomplete package.json → handle gracefully, reduce confidence
- Framework detected but missing critical config (e.g., Django without manage.py) → warn
- Project already has a Dockerfile → skip generation, suggest using existing
- Project with custom build system (Makefile, Taskfile) → detect and incorporate

## Acceptance Criteria

- [ ] `mozza init --detect` correctly identifies all 5 framework families
- [ ] Smart customization works for: Next.js (standalone/export), Django (gunicorn), Rails (Puma), Laravel (Octane), Go (static)
- [ ] Dockerfile templates generated for all 5 frameworks
- [ ] Confidence levels shown to user before generating
- [ ] `--framework` flag forces specific detection
- [ ] Detection completes in < 5 seconds
- [ ] Generated recipes are valid and parseable
- [ ] `make test` passes

## Definition of Done

Running `mozza init --detect` in example project directories for each framework correctly identifies the framework, generates a valid recipe with framework-specific optimizations, and optionally generates a Dockerfile. All 5 frameworks have working detection and generation.

## Related Files

- frontend/framework-wizard.md (web UI for framework wizard)
- backend/compose-importer.md (similar pattern)
- backend/git-deploy.md (uses detector during auto-build)

## Testing Strategy

- Create test fixture directories for each framework with realistic file structures
- Table-driven tests for detection accuracy
- Test confidence levels with varying project completeness
- Test Dockerfile generation output
- Test recipe generation output validation
- Run: `go test ./internal/detect/...`
