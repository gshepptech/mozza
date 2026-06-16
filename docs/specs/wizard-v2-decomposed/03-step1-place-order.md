# Domain: Step 1 — Place Order

**Source spec:** docs/specs/wizard-v2.md — US-1, US-11

## Purpose

First step of the wizard: user picks an image alias, sets replica count, and
chooses a deploy target (Local or Kitchen).

## Acceptance Criteria

- AC-1.1: Autocomplete field searches saved aliases by name. Typing "my" shows all aliases starting with "my".
- AC-1.2: Replica count is a number stepper, default 1, min 1, max 100.
- AC-1.3: Target toggle: "Local" (Docker Compose) or "Kitchen" (Kubernetes). Default: Local if no clusters registered, Kitchen if clusters exist.
- AC-1.4: If user types an alias name that doesn't exist, an inline form expands: "We don't have myapp on the menu yet — what's the full image?" with a registry URL input. On submit, alias is created via POST /api/aliases and selected.
- AC-1.5: If target is Kitchen (K8s) and no clusters are registered, show: "You need to connect a kitchen first" with link to /app/clusters.
- AC-1.6: "Next" button text: "Start your order →"
- AC-11.1: Alias autocomplete searches by name and image URL.
- AC-11.2: Inline alias creation saves to DB immediately.
- AC-11.3: Existing aliases show image URL as subtitle in autocomplete dropdown.
- AC-11.5: Recently used aliases appear first in autocomplete.

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepPlaceOrder.tsx` | Step 1 component |

## Step Title

"What are you ordering?"

## Component Props

```typescript
interface StepPlaceOrderProps {
  state: ServiceInterviewState;
  onUpdate: (updates: Partial<ServiceInterviewState>) => void;
  teamId: string;
  isAdditionalService: boolean;  // Skip target selection for 2nd+ service
}
```

## UI Elements

1. **Alias Autocomplete** — text input with dropdown
   - Fetches aliases via `api.listAliases(teamId)`
   - Filters by typed text (name AND image URL)
   - Shows: alias name (bold) + image URL (subtitle)
   - Recently used first (sort by usage or creation date)
   - No match → inline creation form

2. **Inline Alias Creation** — conditionally shown
   - Message: "We don't have {name} on the menu yet — what's the full image?"
   - Registry URL input with placeholder: "registry.example.com/org/image:tag"
   - "Add to menu" button → POST /api/v1/aliases, then select it

3. **Replica Stepper** — number input with +/- buttons
   - Default: 1, Min: 1, Max: 100
   - Label: "How many copies?"

4. **Target Toggle** — two-option toggle
   - "Counter" (Local/Docker Compose) — house icon
   - "Kitchen" (Kubernetes) — server icon
   - Default: Kitchen if clusters exist, Counter if not
   - If Kitchen selected and no clusters → warning message + link

## Validation

- Alias must be selected (name + image both non-empty)
- Replicas must be 1-100
- If Kitchen target, at least one cluster must exist (or show warning)

## API Calls

- `GET /api/v1/aliases?team_id=X` — list aliases for autocomplete
- `POST /api/v1/aliases` — create inline alias
- `GET /api/v1/clusters` — check if clusters exist for target default
