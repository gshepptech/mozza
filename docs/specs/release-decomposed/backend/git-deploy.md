---
domain: backend
file: git-deploy
depends_on:
  - foundation/data-model
  - backend/framework-detector
estimated_complexity: high
---

## Purpose

Implement Git push-to-deploy: GitHub webhook receiver, OAuth flow for repo connection, auto-build engine that clones repos, detects frameworks, builds Docker images, and deploys. Covers US-5 (Webhook Receiver) and US-6 (Auto-Build).

## Scope

**Included:**
- `internal/gitdeploy/webhook.go` — GitHub webhook receiver, signature validation
- `internal/gitdeploy/oauth.go` — GitHub OAuth flow for repo connection
- `internal/gitdeploy/builder.go` — clone repo, detect framework, build Docker image
- `internal/gitdeploy/queue.go` — build queue with concurrency limit (max 2)
- CLI commands: `mozza connect github.com/user/repo`, `mozza disconnect`
- API endpoints for webhook, repo management, build history
- Server route registration

**Excluded:**
- Branch preview deploys (see backend/preview-deploys.md)
- Real-time terminal UI (see frontend/deploy-terminal.md)

## Requirements

- REQ-1: `mozza connect github.com/user/repo` sets up a webhook on the GitHub repo
- REQ-2: OAuth flow for GitHub authentication (or PAT as fallback)
- REQ-3: Webhook endpoint at `/api/v1/webhooks/github` receives push events
- REQ-4: Validate GitHub signature (X-Hub-Signature-256) on every webhook
- REQ-5: Push to default branch triggers deploy
- REQ-6: On webhook trigger: clone repo → detect framework → generate Dockerfile if needed → build image → deploy
- REQ-7: If repo has Dockerfile, use it directly
- REQ-8: If repo has no Dockerfile, auto-detect framework and generate one
- REQ-9: Build happens on the Mozza server using local Docker
- REQ-10: Built image tagged with `commit_sha-branch_name`
- REQ-11: Previous image retained for rollback
- REQ-12: Build timeout: 10 minutes default, configurable per app
- REQ-13: Concurrent builds queued (max 2 simultaneous)
- REQ-14: Build failures reported clearly with plain-English explanation
- REQ-15: Dashboard shows connected repos with status
- REQ-16: `mozza disconnect` removes webhook from GitHub
- REQ-17: Handle webhook delivery failures gracefully (retry with backoff)
- REQ-18: Build output streamable via WebSocket for real-time display

## Explicit Behaviors

- OAuth flow: redirect to GitHub → callback at `/api/v1/auth/github/callback` → store encrypted token
- PAT fallback: `mozza connect --token <pat> github.com/user/repo`
- Webhook setup: use GitHub API to create webhook on the repo, store webhook_id in connected_repos
- Webhook validation: compute HMAC-SHA256 of request body with webhook secret, compare to X-Hub-Signature-256 header
- Build process: `git clone --depth 1 --branch <branch> <repo_url>` → `detect.Scan()` → `docker build` → `docker tag`
- Build queue uses a buffered channel (cap 2) as semaphore
- Build status transitions: pending → building → success/failed
- Build logs captured from docker build output via io.Reader
- Build logs stored in builds table, streamable via WebSocket at `/api/v1/builds/{id}/logs`
- On successful build: trigger standard deploy flow with built image
- On failure: store error, notify via build status (no external notifications yet)
- Image naming: `mozza-{app-name}:{commit-sha-short}`

## Dependencies

- foundation/data-model — connected_repos, builds tables
- backend/framework-detector — detect.Scan() for repos without Dockerfiles

## Interfaces

**API Endpoints:**
```
POST   /api/v1/repos/connect           — {repo_url, auth_method: "oauth"|"pat", token?}
DELETE /api/v1/repos/{id}/disconnect    — remove webhook, delete connection
GET    /api/v1/repos                    — list connected repos with status
POST   /api/v1/webhooks/github         — webhook receiver (GitHub calls this)
GET    /api/v1/auth/github/callback    — OAuth callback
GET    /api/v1/builds                   — list builds with filtering
GET    /api/v1/builds/{id}              — build detail
GET    /api/v1/builds/{id}/logs         — WebSocket: stream build logs
```

**Internal:**
- `gitdeploy.Service` struct with `Connect`, `Disconnect`, `HandleWebhook`, `GetBuilds` methods
- `gitdeploy.Builder` interface for testability
- `gitdeploy.Queue` for build concurrency management

## Constraints

- Max 2 concurrent builds to avoid resource exhaustion
- Build timeout: 10 minutes default
- Git push to running deploy in < 3 minutes excluding build time (NFR-4)
- OAuth tokens stored encrypted in database
- Webhook secret stored securely (not in plain text)

## Edge Cases

- Webhook received for disconnected repo → ignore with 200 (don't error)
- Concurrent webhooks for same repo → queue, don't duplicate
- Build fails mid-clone (network issue) → clear error, retry suggestion
- Repo has no supported framework AND no Dockerfile → clear error with guidance
- OAuth token expired → refresh flow or re-auth prompt
- Very large repo clone → consider shallow clone + sparse checkout
- Webhook signature mismatch → 401, log attempt (potential security event)
- Build succeeds but deploy fails → report build success, deploy failure separately

## Acceptance Criteria

- [ ] `mozza connect github.com/user/repo` creates webhook on GitHub
- [ ] OAuth flow completes and stores encrypted token
- [ ] PAT fallback works for repo connection
- [ ] Webhook endpoint validates GitHub signatures
- [ ] Push to default branch triggers clone → detect → build → deploy
- [ ] Repos with Dockerfile use it directly
- [ ] Repos without Dockerfile get auto-generated one
- [ ] Build queue limits concurrent builds to 2
- [ ] Build timeout enforced at 10 minutes
- [ ] Build failures show clear error messages
- [ ] `mozza disconnect` removes webhook
- [ ] Build logs streamable via WebSocket
- [ ] `make test` passes

## Definition of Done

Full flow works: connect repo → push code → webhook fires → build starts → image created → app deployed. Build history visible via API. Disconnect cleanly removes webhook.

## Related Files

- backend/preview-deploys.md (branch pushes trigger preview deploys)
- frontend/deploy-terminal.md (UI for build log streaming)

## Testing Strategy

- Mock GitHub API for webhook creation/deletion
- Test webhook signature validation with known payloads
- Test build queue concurrency (goroutine-based tests)
- Test build timeout enforcement
- Integration test: full clone → detect → build flow (with test repo)
- Run: `go test ./internal/gitdeploy/...`
