# Domain: Review Your Order (Step 8)

**Source spec:** docs/specs/wizard-v2.md — US-9, FR-5

## Purpose

Review the generated recipe, optionally edit inline, toggle YAML view,
and press "Place Order" to deploy.

## Acceptance Criteria

- AC-9.1: Primary view: .mozza recipe in a syntax-highlighted code editor (Monaco or CodeMirror).
- AC-9.2: Recipe is editable inline. Changes validated on blur (parse the recipe, show errors inline).
- AC-9.3: Secondary toggle: "View the blueprint" shows compiled Kubernetes YAML (read-only for K8s target) or docker-compose.yml (for Local target).
- AC-9.4: Summary card above the editor showing: service count, total replicas, target, dependencies.
- AC-9.5: If user edits the recipe, show a "Modified" badge. "Reset" button reverts to generated version.
- AC-9.6: Step title: "Review your order"
- AC-9.7: "Place Order" button: bg-brand, prominent, with Rocket icon. Disabled until recipe parses successfully.
- AC-9.8: Save recipe option: "Save this recipe for later" toggle. If ON, saves to team's recipe list.

## Inline Recipe Editing (FR-5)

- Code editor component with .mozza syntax highlighting
- Real-time parse validation on blur
- Error markers on invalid lines
- Reset to generated version

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepReview.tsx` | Step 8 component |

## Component Props

```typescript
interface StepReviewProps {
  recipeSource: string;
  generatedRecipeSource: string;  // Original for reset
  target: WizardTarget;
  services: ServiceInterviewState[];
  teamId: string;
  onRecipeChange: (source: string) => void;
  onPlaceOrder: () => void;
}
```

## UI Elements

1. **Summary card** — above the editor
   - Service count: "3 services"
   - Total replicas: "7 total copies"
   - Target: "Kitchen (Kubernetes)" or "Counter (Local)"
   - Dependencies: "PostgreSQL, Redis"

2. **Recipe editor** — syntax-highlighted textarea
   - Use a `<textarea>` with monospace font (no heavy editor dependency)
   - Syntax highlighting via CSS classes on a overlay div (optional)
   - Or use a lightweight editor like CodeMirror if already a dependency
   - Validate on blur via `api.validateRecipe(source)`
   - Show validation errors below editor

3. **Modified badge + Reset** — shown when recipe differs from generated
   - "Modified" badge next to title
   - "Reset" button to revert to `generatedRecipeSource`

4. **YAML toggle** — "View the blueprint"
   - Toggle button switches between .mozza and compiled output
   - For K8s: show Kubernetes YAML (read-only)
   - For Local: show docker-compose.yml (read-only)
   - Note: this requires a compile endpoint or client-side generation

5. **Save recipe toggle** — "Save this recipe for later"
   - Toggle switch
   - If ON: saves to team recipes on deploy via `api.createRecipe()`

6. **Place Order button** — prominent
   - Text: "Place Order" with rocket icon
   - Style: bg-brand (orange), large
   - Disabled if recipe has validation errors

## Validation

Recipe must parse successfully (no errors from validate endpoint).
