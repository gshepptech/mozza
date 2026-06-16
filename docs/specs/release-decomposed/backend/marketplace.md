---
domain: backend
file: marketplace
depends_on:
  - foundation/data-model
estimated_complexity: medium
---

## Purpose

Implement the recipe marketplace backend: fetch/cache recipe index from a GitHub repo, search/filter recipes, CLI commands for browsing and deploying community recipes.

## Scope

**Included:**
- `internal/marketplace/index.go` — fetch/cache recipe index from GitHub repo
- `internal/marketplace/search.go` — search by name, category, tags
- `internal/marketplace/install.go` — download recipe to local directory
- CLI commands: `mozza recipe search`, `mozza recipe info`, `mozza recipe install`, `mozza recipe deploy`, `mozza recipe update`
- API endpoints for dashboard integration
- Initial catalog: expand existing 10 templates to 15+ recipes with metadata

**Excluded:**
- Dashboard UI (see frontend/marketplace-ui.md)
- Community contribution workflow (post-launch)

## Requirements

- REQ-1: Marketplace recipes stored in a GitHub repo structure
- REQ-2: Each recipe has `.mozza` file + `metadata.yaml` (name, description, category, tags, author, source link)
- REQ-3: Categories: Web Frameworks, Databases, Monitoring, CMS, DevTools, Communication
- REQ-4: `mozza recipe search <query>` searches by name, category, tags
- REQ-5: `mozza recipe info <name>` shows recipe details and source link
- REQ-6: `mozza recipe install <name>` downloads recipe to current directory
- REQ-7: `mozza recipe deploy <name>` deploys directly without saving
- REQ-8: API endpoint `GET /api/v1/marketplace/recipes` with search, filter, pagination
- REQ-9: Marketplace index cached locally, refreshed on `mozza recipe update`
- REQ-10: Initial catalog: 15+ recipes (existing templates + framework recipes)
- REQ-11: Each recipe links to original source if applicable

## Explicit Behaviors

- Index fetched from GitHub API (public repo, no auth needed): list files in repo, download metadata.yaml for each
- Local cache in marketplace_cache table with content_hash for efficient refresh
- `mozza recipe update` forces re-fetch of entire index
- Search: fuzzy name matching + exact category/tag filtering
- `mozza recipe install <name>` writes `.mozza` file to current directory
- `mozza recipe deploy <name>` downloads recipe to temp dir, runs standard deploy flow
- Categories defined as constants, each recipe must have exactly one category
- Tags are free-form strings, max 5 per recipe
- Pagination: default 20 per page, max 100

## Dependencies

- foundation/data-model — marketplace_cache table

## Interfaces

**API Endpoints:**
```
GET  /api/v1/marketplace/recipes         — list/search (query, category, tags, page, per_page)
GET  /api/v1/marketplace/recipes/{name}  — recipe detail with full content
POST /api/v1/marketplace/recipes/{name}/deploy — deploy a marketplace recipe
```

**Internal:**
- `marketplace.Service` with `Search`, `Get`, `Install`, `Deploy`, `Refresh` methods
- `marketplace.Recipe{Name, Description, Category, Tags, Author, Source, Content}`
- `marketplace.SearchParams{Query, Category, Tags, Page, PerPage}`

## Constraints

- Index refresh should be incremental (content_hash comparison)
- No authentication needed for public marketplace repo
- Recipe content validated (parseable by recipe engine) before caching

## Edge Cases

- GitHub API rate limiting → cache aggressively, show cached results when rate-limited
- Recipe with invalid content → skip with warning during index refresh
- Recipe not found → clear error with suggestion to `mozza recipe update`
- Offline mode → serve from cache, warn about staleness
- Recipe name conflicts → use fully qualified `category/name`

## Acceptance Criteria

- [ ] `mozza recipe search <query>` returns matching recipes
- [ ] `mozza recipe info <name>` shows full recipe details
- [ ] `mozza recipe install <name>` downloads recipe file
- [ ] `mozza recipe deploy <name>` deploys recipe directly
- [ ] API endpoint returns paginated, filtered results
- [ ] Index cached and refreshable
- [ ] 15+ recipes in initial catalog
- [ ] `make test` passes

## Definition of Done

CLI search, info, install, deploy commands all work. API endpoint supports search/filter/pagination. 15+ recipes available with proper metadata. Cache works offline.

## Related Files

- frontend/marketplace-ui.md (dashboard for browsing recipes)

## Testing Strategy

- Mock GitHub API responses for index fetching
- Test search with various query/filter combinations
- Test cache invalidation and refresh logic
- Test recipe validation during indexing
- Run: `go test ./internal/marketplace/...`
