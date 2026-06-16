---
domain: frontend
file: compose-wizard
depends_on:
  - backend/compose-importer
estimated_complexity: medium
---

## Purpose

Add Docker Compose import flow to the web UI deploy wizard: paste/upload compose file, see step-by-step breakdown of detected services with pizza metaphor, adjust configuration, preview recipe, deploy.

## Scope

**Included:**
- `ui/src/components/deploy/ComposeImport.tsx` — compose file upload/paste + preview
- API endpoint: `POST /api/v1/import/compose` — upload compose file, get recipe preview
- API endpoint: `POST /api/v1/import/compose/deploy` — import and deploy
- Integration with existing deploy wizard flow
- Service breakdown display with pizza metaphor

**Excluded:**
- CLI compose import (see backend/compose-importer.md)
- Full compose spec parsing (backend handles this)

## Requirements

- REQ-1: Deploy wizard has "I have a docker-compose.yml" option alongside existing options
- REQ-2: User can paste compose file content or upload a file
- REQ-3: Wizard shows step-by-step breakdown of detected services with pizza metaphor ("We found 3 slices in your recipe: Web, API, Database")
- REQ-4: Each detected service shows its configuration with option to adjust
- REQ-5: Preview generated recipe before deploying
- REQ-6: Deploy button triggers standard deploy flow
- REQ-7: Error states handled: invalid YAML, empty file, compose file with only build contexts

## Explicit Behaviors

- "Import Docker Compose" option appears as a card in the deploy wizard's method selection
- Paste mode: textarea with syntax highlighting (or monospace font)
- Upload mode: file input accepting `.yml`, `.yaml` files, max 1MB
- On submit: POST to `/api/v1/import/compose` → returns parsed services + generated recipe + warnings
- Service breakdown: each service shown as a card with icon, name, image, ports, env vars
- Per-service adjustments: toggle ports open/closed, edit env vars, change image tag
- Warnings shown as dismissible banners (e.g., "Build context detected — you'll need to pre-build this image")
- Recipe preview: syntax-highlighted read-only view of the generated .mozza file
- "Deploy" button triggers POST to `/api/v1/import/compose/deploy`
- Pizza metaphor: "We found 3 slices in your pizza: Web Server, API Backend, PostgreSQL Database"

## Dependencies

- backend/compose-importer — provides `ComposeToRecipe` function and API endpoint

## Interfaces

**API Endpoints (backend provides these):**
```
POST /api/v1/import/compose        — {content: string} → {services: [], recipe: string, warnings: []}
POST /api/v1/import/compose/deploy — {content: string, adjustments: {}} → deploy result
```

**React Components:**
- `ComposeImport` — main component, manages paste/upload/preview/deploy flow
- Integrates into existing `DeployWizard` component as a new method option

## Constraints

- Must follow existing UI design patterns (dark theme, orange accent)
- Must be responsive (mobile-friendly)
- No external dependencies beyond what's already in the UI

## Edge Cases

- Very large compose file (>1000 lines) → show loading state during parsing
- Invalid YAML → show parse error with line number if possible
- File with only build contexts → show error explaining need for pre-built images
- Network error during import → retry with error message
- User adjusts service config then re-pastes → reset adjustments

## Acceptance Criteria

- [ ] "Import Docker Compose" option visible in deploy wizard
- [ ] Paste and upload both work
- [ ] Service breakdown shown with pizza metaphor
- [ ] Per-service configuration adjustable
- [ ] Recipe preview shows generated .mozza content
- [ ] Deploy button works
- [ ] Error states handled gracefully
- [ ] Responsive on mobile

## Definition of Done

Full wizard flow: paste compose file → see service breakdown → adjust config → preview recipe → deploy. Errors handled with clear messaging.

## Related Files

- backend/compose-importer.md (backend parsing logic)
- frontend/framework-wizard.md (similar wizard flow pattern)

## Testing Strategy

- Component tests with mock API responses
- Test paste and upload modes
- Test error state rendering
- Test service adjustment flow
- Visual regression tests
