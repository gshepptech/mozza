---
domain: backend
file: compose-importer
depends_on: []
estimated_complexity: high
---

## Purpose

Implement a Docker Compose file importer that converts docker-compose.yml files into Mozza recipe format. Covers the full Docker Compose v3.x spec with clean mapping to Mozza's recipe DSL.

## Scope

**Included:**
- `internal/importer/compose.go` — parse and convert compose files to recipe AST
- CLI commands: `mozza import ./docker-compose.yml`, `mozza import --dry-run`
- Full Docker Compose spec mapping: services, images, build contexts, ports, volumes, env vars, depends_on, healthchecks, restart policies, networks, configs, secrets, profiles, extends
- Preview mode with confirmation before writing
- Unsupported feature warnings

**Excluded:**
- Web UI wizard for compose import (see frontend/compose-wizard.md)
- API endpoint for compose upload (see frontend/compose-wizard.md)

## Requirements

- REQ-1: `mozza import ./docker-compose.yml` reads and parses a local compose file
- REQ-2: Map compose `services` → recipe Slices
- REQ-3: Map `image` → `from image "<name>"`
- REQ-4: Map `ports` → `on port <N>` / `open to the public`
- REQ-5: Map named `volumes` → `storage` declarations
- REQ-6: Map `environment` → `set <KEY> to "<VALUE>"` declarations
- REQ-7: Map `depends_on` → `needs "<service>"` declarations preserving dependency order
- REQ-8: Map `healthcheck` → `health check <path>` declarations
- REQ-9: Map `deploy.replicas` → `run <N> copies`
- REQ-10: Handle `build` contexts — warn "needs pre-built image" with instructions to build first
- REQ-11: Map `networks` → NetworkPolicy mapping where applicable
- REQ-12: Map `configs`/`secrets` → Secret references
- REQ-13: Show clear warnings for unsupported features: "Your compose file uses X — here's how Mozza handles this: Y"
- REQ-14: `mozza import --dry-run` prints generated recipe to stdout without writing
- REQ-15: Preview mode shows generated recipe and asks for confirmation before writing
- REQ-16: Handle multi-service compose files (5+ services)
- REQ-17: Output must be a valid, parseable `.mozza` recipe file

## Explicit Behaviors

- Parse using a proper YAML library (gopkg.in/yaml.v3 or similar)
- Support compose file versions 2.x and 3.x (detect version field)
- Restart policies: `always` → `restart always`, `on-failure` → `restart on-failure`, `unless-stopped` → `restart always`
- Port mapping: `"8080:80"` → `on port 80` with host port annotation
- Named volumes get `storage` blocks; bind mounts get warnings about host dependency
- Environment variables from `env_file` should be read and inlined
- `extends` should be resolved before conversion
- `profiles` should be documented in recipe comments
- Service names become slice names (sanitized to valid identifiers)
- Output recipe uses clean, idiomatic DSL — not a literal 1:1 translation

## Dependencies

None — compose importer is self-contained. Uses existing recipe AST from `internal/recipe/`.

## Interfaces

**Input:**
- File path to docker-compose.yml
- CLI flags: `--dry-run`, `--output <path>`

**Output:**
- `.mozza` recipe file written to disk (or stdout with --dry-run)
- Warnings printed to stderr for unsupported features

**API (for wizard integration):**
- `importer.ComposeToRecipe(yamlContent []byte) (*recipe.Recipe, []Warning, error)`
- `Warning{Feature string, Message string, Severity string}`

## Constraints

- Must handle compose files up to 1000 lines (NFR-3)
- No external dependencies beyond YAML parser
- Must preserve service dependency order
- Generated recipe must pass `mozza validate`

## Edge Cases

- Empty compose file → clear error "No services found"
- Compose file with only build contexts (no images) → error with guidance
- Services with circular depends_on → detect and error
- Very long environment variable values (multi-line)
- Compose file using variable interpolation (${VAR:-default}) → resolve or warn
- Services with both `image` and `build` → prefer image, note build context
- Duplicate port mappings across services → warn about conflicts
- Compose file with `version: "2"` vs `version: "3.8"` differences

## Acceptance Criteria

- [ ] `mozza import ./docker-compose.yml` produces valid .mozza recipe
- [ ] Full compose spec elements mapped: services, images, ports, volumes, env vars, depends_on, healthchecks, restart policies
- [ ] Unsupported features show clear warning messages
- [ ] `--dry-run` prints to stdout without writing
- [ ] Preview mode asks confirmation before writing
- [ ] Handles 5+ service compose files correctly
- [ ] Service dependency order preserved via `needs` declarations
- [ ] `make test` passes with import test cases
- [ ] `make lint` passes

## Definition of Done

Running `mozza import` on the example compose files in `examples/` produces valid, deployable recipes. All compose spec elements are either mapped or warned about. Tests cover happy path, edge cases, and error cases.

## Related Files

- frontend/compose-wizard.md (web UI for compose import)
- backend/framework-detector.md (similar pattern — analyze input, generate recipe)

## Testing Strategy

- Table-driven tests with various compose file inputs
- Test each mapping (ports, volumes, env, etc.) individually
- Test multi-service compose files
- Test error cases (invalid YAML, empty file, circular deps)
- Test warning generation for unsupported features
- Run: `go test ./internal/importer/...`
