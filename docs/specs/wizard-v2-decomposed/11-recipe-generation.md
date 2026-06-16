# Domain: Recipe Generation

**Source spec:** docs/specs/wizard-v2.md — FR-3

## Purpose

Generate a valid `.mozza` recipe from all wizard answers across all services.

## Requirements (FR-3)

- Generates valid .mozza recipe from wizard answers
- Includes all configured services, dependencies, env vars, health checks
- Recipe validates against the existing parser (no syntax errors)
- Supports all existing recipe DSL features: `needs`, `set`, `limit`, `health check`, `domain`, `open to the public`, etc.

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/lib/recipe-generator.ts` | Generate .mozza recipe text from WizardState |

## Function Signature

```typescript
function generateOrderRecipe(state: WizardState): string
```

## Recipe Format Reference

From existing `generateRecipeSource()` in GuidedWizard and `generateRecipePreview()` in DeployWizard:

```
App: myapp

Images:
  myapp = registry.example.com/org/myapp:latest

web:
  from image myapp
  run 3 copies
  on port 3000
  open to the public
  domain app.example.com
  needs db
  set NODE_ENV to "production"
  set DATABASE_URL to "postgres://db:5432/myapp"
  health check /health
  limit cpu 250m to 500m
  limit memory 128Mi to 256Mi

db:
  from image postgres:16
  on port 5432
  with 10Gi storage
  set POSTGRES_DB to "myapp"
```

## Generation Rules

1. **App name**: from first service alias name
2. **Images section**: one alias per unique image
3. **Per service**:
   - `from image {alias}` or `from image {full_image}`
   - `run {N} copies` if replicas > 1
   - `on port {port}`
   - `open to the public` if isPublic
   - `domain {domain}` if domain set
   - `needs {dep1} and {dep2}` if dependencies
   - `set {KEY} to "{VALUE}"` for each env var
   - `health check {path}` for HTTP health checks
   - `limit cpu {req} to {limit}` for resources
   - `limit memory {req} to {limit}` for resources
   - `run every "{schedule}"` for CronJobs
   - `with {size} storage` for databases/stateful
4. **Dependency slices**: auto-generated for enabled DB/cache
   - DB: image from engine (postgres:16, mysql:8, mongo:7), port, storage, env vars
   - Cache: redis:7, port 6379

## Validation

After generation, validate via `api.validateRecipe(source)` to ensure the recipe
parses correctly against the backend parser.

## Secrets Handling

Env vars marked as secret generate `secret {KEY}` directives instead of `set {KEY}`.
