---
domain: foundation
file: data-model
depends_on: []
estimated_complexity: medium
---

## Purpose

Define all new database tables and migrations required by v1 launch features. This is the data foundation that all other domains build upon.

## Scope

**Included:**
- New SQLite tables for: connected repos, builds, preview deploys, metrics, marketplace cache, certificates
- Migration files in `internal/store/migrations/` and `internal/store/migrations_pg/`
- Store layer functions (CRUD) for each new table

**Excluded:**
- Business logic using these tables (handled by respective backend domains)
- Existing table modifications (unless required for new features)

## Requirements

- REQ-1: Create `connected_repos` table — repo_url, webhook_id, oauth_token_encrypted, app_id, created_at, updated_at
- REQ-2: Create `builds` table — repo_url, commit_sha, branch, status (pending/building/success/failed), logs (TEXT), duration_ms, image_tag, created_at
- REQ-3: Create `preview_deploys` table — app_id, branch, domain, deploy_id, created_at, last_activity
- REQ-4: Create `metrics` table — app_id, timestamp, cpu_percent, memory_bytes, network_rx, network_tx, request_count
- REQ-5: Create `marketplace_cache` table — name, category, tags (JSON), content_hash, recipe_content, fetched_at
- REQ-6: Create `certificates` table — domain, issued_at, expires_at, provider, cert_path, key_path, status
- REQ-7: Provide Store methods for each table: Create, Get, List (with filtering/pagination), Update, Delete
- REQ-8: Add proper indexes for query performance (app_id+timestamp on metrics, domain on certificates, etc.)
- REQ-9: Metrics table should support efficient time-range queries and automatic pruning (7-day retention)

## Explicit Behaviors

- Migration numbering continues from existing sequence (check latest migration number)
- All tables include `id INTEGER PRIMARY KEY AUTOINCREMENT`, `created_at`, `updated_at`
- `oauth_token_encrypted` uses the existing encryption pattern if one exists, otherwise stores as encrypted blob
- `metrics` table uses INTEGER for timestamp (Unix epoch) for efficient range queries
- `builds.logs` is TEXT type to store full build output
- `marketplace_cache.tags` stored as JSON array string
- `preview_deploys.last_activity` updated on every access to support auto-cleanup
- Store methods follow existing patterns in `internal/store/` (use existing query helpers, error wrapping)

## Dependencies

None — this is a foundation domain.

## Interfaces

**Output (consumed by other domains):**
- `store.ConnectedRepo` struct + CRUD methods
- `store.Build` struct + CRUD methods + `ListByRepo`, `GetLatestByBranch`
- `store.PreviewDeploy` struct + CRUD methods + `ListByApp`, `CleanStale(maxAge)`
- `store.Metric` struct + `Record`, `Query(appID, start, end, resolution)`, `Prune(maxAge)`
- `store.MarketplaceRecipe` struct + CRUD methods + `Search(query, category, tags)`
- `store.Certificate` struct + CRUD methods + `GetByDomain`, `ListExpiring(before)`

## Constraints

- SQLite compatible (primary) + PostgreSQL compatible (migrations_pg/)
- Metrics pruning must be efficient — DELETE with timestamp range, not row-by-row
- No foreign key constraints to existing tables that would break existing tests
- Migration files must be idempotent (IF NOT EXISTS)

## Edge Cases

- Concurrent metric writes from multiple goroutines (ensure thread safety)
- Very large build logs (>1MB) — consider TEXT with no length limit
- Marketplace cache with stale data — content_hash enables efficient refresh checks
- Preview deploy cleanup when app is deleted — cascade or orphan handling
- Certificate table with duplicate domains (should be unique constraint)

## Acceptance Criteria

- [ ] All 6 new migration files created and numbered correctly
- [ ] PostgreSQL variants created in migrations_pg/
- [ ] Store structs defined with proper JSON/DB tags
- [ ] CRUD methods implemented for all 6 tables
- [ ] Metrics time-range query returns data grouped by resolution
- [ ] Metrics pruning deletes records older than 7 days
- [ ] All methods have proper error wrapping with context
- [ ] `make test` passes with store tests

## Definition of Done

All migrations run without error on fresh database. Store methods pass unit tests with table-driven test cases covering happy path, not-found, and constraint violations.

## Related Files

- backend/git-deploy.md (uses connected_repos, builds)
- backend/preview-deploys.md (uses preview_deploys)
- backend/monitoring.md (uses metrics)
- backend/marketplace.md (uses marketplace_cache)
- infra/reverse-proxy-ssl.md (uses certificates)

## Testing Strategy

- Table-driven tests for each Store method
- Test migration up/down
- Test metrics time-range queries with known data
- Test concurrent writes to metrics table
- Run: `go test ./internal/store/...`
