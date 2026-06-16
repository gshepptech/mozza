# Domain: Integration Wiring

**Source spec:** docs/specs/wizard-v2.md — Technical Notes (Components to Modify)

## Purpose

Wire OrderWizard into the existing app. Replace GuidedWizard with OrderWizard
in DeployWizard's guided source path.

## Requirements

- DeployWizard.tsx: swap GuidedWizard for OrderWizard in the guided source path
- OrderWizard receives same props interface as GuidedWizard (onComplete, onAdvancedMode)
- Existing template, GitHub import, and advanced recipe paths remain unchanged
- Route: `/app/deploy` continues to work
- DeployProgress.tsx: add pizza metaphor stage names alongside existing ones

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/DeployWizard.tsx` | Swap GuidedWizard → OrderWizard import |
| `ui/src/components/deploy/DeployProgress.tsx` | Add pizza metaphor stage name mapping |

## Changes to DeployWizard.tsx

1. Replace `import { GuidedWizard }` with `import { OrderWizard }`
2. In the guided source rendering path, replace `<GuidedWizard ... />` with `<OrderWizard ... />`
3. Same props: `onComplete`, `onAdvancedMode`
4. No other changes to DeployWizard — template, GitHub, advanced paths unchanged

## Changes to DeployProgress.tsx

1. Add optional `usePizzaMetaphor` prop (default false)
2. When enabled, map step labels to pizza metaphor versions:
   - "Parsing recipe" → "Taking your order..."
   - "Building plan" → "Prepping ingredients..."
   - "Validating plan" → "Prepping ingredients..."
   - "Applying to cluster" → "Firing up the oven..."
   - "Waiting for pods" → "Checking the crust..."
3. Success: "Order served!" / Failure: "Order burned!"

## Verification

- Navigate to `/app/deploy` → select Custom → Guided mode
- OrderWizard should render instead of GuidedWizard
- Complete a full wizard flow → recipe generated → deploy triggered
- Template and GitHub import paths still work unchanged
