---
domain: frontend
file: framework-wizard
depends_on:
  - backend/framework-detector
estimated_complexity: medium
---

## Purpose

Implement per-framework guided wizard flow in the web UI. Detects framework from repo URL or uploaded files, asks framework-specific questions, generates complete recipe with dependencies.

## Scope

**Included:**
- `ui/src/components/deploy/FrameworkDetect.tsx` — framework detection result display
- `ui/src/components/deploy/FrameworkWizard.tsx` — per-framework guided flow
- API endpoint: `POST /api/v1/detect` — upload project files or provide repo URL
- API endpoint: `POST /api/v1/detect/generate` — generate recipe from detection result
- Per-framework question flows for Next.js, Django, Rails, Laravel, Go

**Excluded:**
- CLI framework detection (see backend/framework-detector.md)
- Dockerfile generation logic (backend handles this)

## Requirements

- REQ-1: Wizard detects framework from GitHub repo URL or uploaded project files
- REQ-2: Per-framework guided flow with smart questions:
  - Next.js: "Do you need a database? Which one?"
  - Django: "Your app uses PostgreSQL. Want me to set that up too?"
  - Rails: "Your app has Sidekiq. Want a background worker?"
  - Laravel: "Using Octane? We'll optimize for that."
  - Go: "Static binary or need CGO?"
- REQ-3: Each framework flow generates complete recipe including dependencies
- REQ-4: Pre-filled sensible defaults per framework
- REQ-5: Wizard explains each decision in non-technical terms
- REQ-6: Generated recipe shown for review before deploy

## Explicit Behaviors

- Detection trigger: user provides GitHub repo URL OR uploads key project files (package.json, go.mod, etc.)
- Framework detection result shown as a card: "We detected a Next.js app (HIGH confidence)"
- Per-framework questions presented as a stepped form (1-3 questions per framework)
- Questions are contextual: if Django has PostgreSQL in requirements.txt, ask "Want us to set up PostgreSQL?" not "Which database?"
- Defaults pre-filled: "We recommend port 3000 for Next.js" with option to change
- Non-technical explanations: "This is like choosing your pizza toppings — each one adds a dependency to your recipe"
- Recipe includes detected dependencies: database (Postgres/MySQL/SQLite), cache (Redis), worker (Sidekiq/Celery), etc.
- Review screen: syntax-highlighted recipe + Dockerfile preview + estimated resources

## Dependencies

- backend/framework-detector — provides detection API and recipe generation

## Interfaces

**API Endpoints (backend provides):**
```
POST /api/v1/detect          — {repo_url?: string, files?: File[]} → {framework, confidence, details}
POST /api/v1/detect/generate — {detection_result, user_choices} → {recipe, dockerfile}
```

**React Components:**
- `FrameworkDetect` — shows detection result with confidence
- `FrameworkWizard` — per-framework question flow
- Integrates into existing deploy wizard

## Constraints

- Follow existing UI patterns
- Responsive design
- Framework questions must feel conversational, not like a form

## Edge Cases

- No framework detected → suggest manual recipe creation or compose import
- Multiple frameworks detected → show options, let user pick
- Repo URL inaccessible → error with suggestion to upload files instead
- Framework detected with LOW confidence → show result with caveat

## Acceptance Criteria

- [ ] Framework detection works from repo URL and file upload
- [ ] Per-framework guided questions displayed
- [ ] Sensible defaults pre-filled
- [ ] Complete recipe generated with dependencies
- [ ] Non-technical explanations for each choice
- [ ] Recipe review before deploy
- [ ] All 5 frameworks have working wizard flows

## Definition of Done

Full flow: provide repo URL → detect framework → answer guided questions → review recipe → deploy. All 5 frameworks have specific question flows.

## Related Files

- backend/framework-detector.md (detection engine)
- frontend/compose-wizard.md (similar wizard pattern)

## Testing Strategy

- Component tests with mock detection results
- Test each framework's question flow
- Test error states
- Test recipe preview rendering
