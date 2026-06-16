---
domain: frontend
file: deploy-terminal
depends_on:
  - backend/git-deploy
estimated_complexity: medium
---

## Purpose

Implement real-time deploy terminal in the web UI. Shows live output during deployment — pulling images, building, creating services, health checks — like watching pizza being made.

## Scope

**Included:**
- `ui/src/components/deploy/DeployTerminal.tsx` — real-time deploy output (WebSocket)
- WebSocket connection for real-time streaming
- Step progress indicators (pending → in progress → success → failed)
- Build log viewer with syntax highlighting
- Deploy progress persistence across page refreshes

**Excluded:**
- Backend WebSocket implementation (see backend/git-deploy.md)
- Build pipeline logic (backend handles this)

## Requirements

- REQ-1: Terminal-style panel shows live output during deploy
- REQ-2: WebSocket connection for real-time streaming (not polling)
- REQ-3: Each step has visual indicator: pending → in progress (spinner) → success (checkmark) → failed (X)
- REQ-4: Build logs shown in scrollable, syntax-highlighted terminal component
- REQ-5: Clickable "View full logs" expands to full-screen terminal
- REQ-6: Deploy progress persists across page refreshes (reconnect to running deploy)
- REQ-7: On failure: error highlighted in red with plain-English explanation + suggested fix
- REQ-8: On success: celebration moment + direct link to deployed app

## Explicit Behaviors

- WebSocket URL: `ws://localhost:8080/api/v1/builds/{id}/logs` (or wss:// with TLS)
- Messages are line-by-line build output with metadata: `{type: "step"|"log"|"error"|"complete", content: string, step?: number}`
- Steps displayed as a vertical timeline:
  1. Cloning repository
  2. Detecting framework
  3. Building image
  4. Creating containers
  5. Health check
  6. Routing traffic
- Each step expands to show detailed logs when clicked
- Terminal component uses monospace font, dark background, green text (classic terminal look)
- Auto-scroll to bottom of logs (with "scroll lock" toggle to pause)
- On page refresh: GET `/api/v1/builds/{id}` to check if deploy is still running, reconnect WebSocket if so
- Failure display: step turns red, error message shown in a callout box with:
  - What happened (technical)
  - What it means (plain English)
  - What to try (actionable suggestion)
- Success display: confetti animation (subtle), deployed URL as a clickable link, "Open App" button

## Dependencies

- backend/git-deploy — WebSocket endpoint for build log streaming

## Interfaces

**WebSocket Protocol:**
```
Connect: ws://host/api/v1/builds/{id}/logs
Messages: {type: "step"|"log"|"error"|"complete", content: string, step?: number, timestamp: number}
```

**React Components:**
- `DeployTerminal` — main terminal component
- `DeployStep` — individual step in the timeline
- `LogViewer` — scrollable, syntax-highlighted log output
- `DeploySuccess` — success celebration with app link
- `DeployError` — error display with explanation and suggestions

## Constraints

- Must work without WebSocket fallback (WebSocket is required)
- Terminal should handle 10,000+ log lines without performance issues (virtual scrolling)
- Must not block UI thread during rapid log streaming

## Edge Cases

- WebSocket connection drops → show "Reconnecting..." indicator, auto-reconnect
- Deploy completes while page is being loaded → show final state from API
- Very rapid log output → batch rendering (requestAnimationFrame)
- Build with no output for 30+ seconds → show "Still working..." indicator
- Multiple concurrent deploys → show active deploy for the current app

## Acceptance Criteria

- [ ] Real-time deploy output shown in terminal panel
- [ ] WebSocket streaming works
- [ ] Step indicators show progress (pending/active/success/failed)
- [ ] Build logs scrollable and syntax-highlighted
- [ ] Full-screen log view available
- [ ] Progress persists across page refresh
- [ ] Failures show clear error with suggestion
- [ ] Success shows celebration and app link

## Definition of Done

Full deploy experience: click deploy → watch live terminal output → see step progress → celebration on success OR clear error on failure.

## Related Files

- backend/git-deploy.md (WebSocket endpoint)
- frontend/compose-wizard.md (triggers deploy)
- frontend/framework-wizard.md (triggers deploy)

## Testing Strategy

- Mock WebSocket for component tests
- Test step state transitions
- Test reconnection logic
- Test error display rendering
- Test log performance with large output
