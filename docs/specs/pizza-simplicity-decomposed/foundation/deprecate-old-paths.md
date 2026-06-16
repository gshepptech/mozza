---
domain: foundation
file: deprecate-old-paths
depends_on: []
estimated_complexity: low
---

## Purpose

Remove the SourceSelector gate and GuidedWizard from the deploy flow. Simplify DeployWizard.tsx so OrderWizard is the single entry point.

## Scope

**Included:**
- Remove GuidedWizard import and rendering from DeployWizard.tsx
- Remove SourceSelector as a gating step in DeployWizard.tsx
- Simplify DeployWizard internal state machine (no more `source` state)
- Pass OrderWizard as the direct child of DeployWizard
- Update DeployWizardPage.tsx copy

**Excluded:**
- Deleting GuidedWizard.tsx or SourceSelector.tsx files (keep as dead code)
- Changing OrderWizard internals (handled by other domain files)

## Requirements

- REQ-1: `DeployWizard.tsx` must NOT import or render `GuidedWizard`
- REQ-2: `DeployWizard.tsx` must NOT render `SourceSelector` as a gate/step before the wizard
- REQ-3: `DeployWizard.tsx` must render `OrderWizard` directly as the primary deploy experience
- REQ-4: Remove the `source` state variable and any conditional rendering based on it (the code that switches between template/github/custom/guided paths)
- REQ-5: Keep the deploy execution logic (recipe save, triggerDeploy, log streaming) in DeployWizard — OrderWizard calls `onComplete` with recipe data, DeployWizard handles the API calls
- REQ-6: `DeployWizardPage.tsx` heading should read "Deploy" with subtitle "Place your order"

## Explicit Behaviors

- When user navigates to `/app/deploy`, they see OrderWizard immediately — no intermediate selection screen
- The `onComplete` callback from OrderWizard receives `(recipeName, recipeSource)` and triggers the deploy flow
- The `onAdvancedMode` callback still exists, linking to the recipe editor for power users
- Template catalog and GitHub import are NOT removed — they move inside OrderWizard Step 1 (handled by `frontend/step1-pick.md`)

## Dependencies

None — can be done in parallel with types-and-engine.

## Interfaces

**DeployWizard props (unchanged):**
```typescript
interface DeployWizardProps {
  teamId: string;
  onDeployComplete?: (deployment: Deployment) => void;
}
```

**OrderWizard props (unchanged):**
```typescript
interface OrderWizardProps {
  onComplete: (recipeName: string, recipeSource: string) => void;
  onAdvancedMode: () => void;
  teamId?: string;
}
```

## Constraints

- Do NOT delete any files — only remove imports and rendering
- Keep all deploy API logic (createRecipe, triggerDeploy, streamDeploymentLogs) in DeployWizard.tsx
- Do NOT break the deploy execution pipeline — recipe save → deploy → log stream must still work

## Edge Cases

- If OrderWizard's `onComplete` is called with empty recipe source, DeployWizard should handle gracefully (existing validation)
- Advanced mode escape hatch must still function

## Acceptance Criteria

- [ ] Navigating to `/app/deploy` shows OrderWizard immediately, no SourceSelector
- [ ] `GuidedWizard` is not imported in `DeployWizard.tsx`
- [ ] No `source` state variable or source-based conditional rendering in `DeployWizard.tsx`
- [ ] Deploy execution (API calls) still works after simplification
- [ ] "I know what I'm doing" advanced mode link still functions
- [ ] Page heading says "Deploy" with subtitle "Place your order"

## Definition of Done

DeployWizard.tsx is simplified to directly render OrderWizard. No SourceSelector gate. No GuidedWizard. Deploy pipeline (API calls) unchanged.

## Related Files

- `frontend/step1-pick.md` — OrderWizard Step 1 absorbs template catalog and GitHub import
- `frontend/order-tracker.md` — deploy progress display

## Testing Strategy

- Navigate to `/app/deploy` → verify OrderWizard renders directly
- Verify no SourceSelector appears
- Complete a deploy flow → verify recipe save + deploy API calls work
- Click "I know what I'm doing" → verify advanced mode opens
