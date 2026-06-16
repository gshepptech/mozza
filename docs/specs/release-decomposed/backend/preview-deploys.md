---
domain: backend
file: preview-deploys
depends_on:
  - foundation/data-model
  - backend/git-deploy
  - infra/reverse-proxy-ssl
estimated_complexity: medium
---

## Purpose

Implement branch preview deployments: push to a non-default branch creates a unique URL (`{branch}.{app-domain}`) with isolated resources. Auto-cleanup after merge or inactivity.

## Scope

**Included:**
- `internal/gitdeploy/preview.go` — branch preview deploy management
- CLI commands: `mozza previews list`, `mozza previews clean`
- API endpoints for preview management
- Auto-cleanup logic (7-day inactivity)
- Integration with reverse proxy for preview domain routing

**Excluded:**
- PR comment posting with preview URL (nice-to-have, defer)
- Database sharing configuration (configurable isolation, defer to simple isolated mode)

## Requirements

- REQ-1: Push to non-default branch creates preview deployment at `{branch-name}.{app-domain}`
- REQ-2: Preview deploys use the same recipe as production but with isolated resources
- REQ-3: Dashboard shows list of active preview deploys per app
- REQ-4: Preview deploys auto-cleaned after branch merge or 7 days of inactivity
- REQ-5: `mozza previews list` shows active previews
- REQ-6: `mozza previews clean` removes stale previews
- REQ-7: Preview deploys get isolated database (simple mode — no production sharing)

## Explicit Behaviors

- Branch name sanitization: replace `/` with `-`, lowercase, strip special chars, max 63 chars for DNS compatibility
- Preview domain: `{sanitized-branch}.{app-domain}` — requires wildcard DNS or explicit DNS entry
- Each preview gets its own container namespace (e.g., `mozza-preview-{app}-{branch}`)
- Auto-cleanup runs as a background goroutine on a timer (every hour)
- Cleanup criteria: no webhook activity for 7 days OR branch deleted/merged on GitHub
- On cleanup: stop containers, remove from reverse proxy, delete preview_deploys record
- `last_activity` updated on: webhook push, manual redeploy, access via preview URL
- Preview deploys share the same build pipeline as production (reuse built images)

## Dependencies

- foundation/data-model — preview_deploys table
- backend/git-deploy — webhook handling triggers preview creation
- infra/reverse-proxy-ssl — domain routing for preview URLs

## Interfaces

**API Endpoints:**
```
GET    /api/v1/apps/{id}/previews        — list active previews for app
DELETE /api/v1/apps/{id}/previews/{branch} — clean specific preview
POST   /api/v1/apps/{id}/previews/{branch}/redeploy — manually redeploy preview
```

**Internal:**
- `preview.Manager` with `Create`, `List`, `Clean`, `CleanStale`, `Redeploy` methods
- Integrates with `gitdeploy.Service` for webhook-triggered creation
- Integrates with `proxy.Router` for domain registration

## Constraints

- Branch names must be DNS-safe after sanitization
- Max previews per app: 10 (prevent resource exhaustion)
- Preview containers get reduced resource limits (50% of production)

## Edge Cases

- Branch name with special chars (`feature/my-cool-thing!`) → sanitize to `feature-my-cool-thing`
- Very long branch names → truncate to 63 chars with hash suffix
- Preview for same branch pushed again → update existing preview (don't create duplicate)
- App deleted while previews exist → cascade cleanup
- Network partition during cleanup → retry on next cycle
- Two branches with same sanitized name → append short hash to disambiguate

## Acceptance Criteria

- [ ] Push to non-default branch creates preview at `{branch}.{app-domain}`
- [ ] Preview uses isolated resources (separate containers)
- [ ] Active previews listed via API and CLI
- [ ] Stale previews auto-cleaned after 7 days
- [ ] `mozza previews clean` removes stale previews manually
- [ ] Max 10 previews per app enforced
- [ ] `make test` passes

## Definition of Done

Full flow: push to feature branch → preview deployed at branch URL → accessible via browser → auto-cleaned after inactivity. CLI and API both work for listing and cleaning.

## Related Files

- backend/git-deploy.md (webhook triggers preview creation)
- infra/reverse-proxy-ssl.md (routes traffic to preview containers)

## Testing Strategy

- Test branch name sanitization with edge cases
- Test preview creation and cleanup lifecycle
- Test auto-cleanup timer logic
- Test max preview limit enforcement
- Run: `go test ./internal/gitdeploy/...`
