# Accessibility Spec: Deploy Like Ordering Pizza — For Real

**Goal:** A person who has never deployed software should be able to go from "I have a thing" to "it's running" without encountering a single term they don't understand.

**Scope:** 54 gaps across the deploy wizard, error messages, onboarding, and deployment tracking. Organized into 8 domains.

## Principles

1. **No jargon without explanation** — every technical term gets a plain-English tooltip, inline help, or is replaced entirely
2. **Smart defaults over configuration** — if we can infer it, don't ask
3. **Examples over descriptions** — show, don't tell
4. **Progressive disclosure** — simple first, details on demand
5. **Every dead end has a next step** — no screen should leave the user wondering "now what?"
6. **Errors are actionable** — every error tells you what went wrong AND what to do about it

## Domains

| Domain | Spec File | Gap Count | Priority |
|--------|-----------|-----------|----------|
| 1. Concept Explainers | `01-concept-explainers.md` | 18 | P0 — blocks everything |
| 2. Field Guidance | `02-field-guidance.md` | 10 | P0 — users can't fill forms without this |
| 3. Onboarding & First Run | `03-onboarding.md` | 7 | P0 — first impression |
| 4. Error Messages | `04-error-messages.md` | 6 | P1 — encountered after deploy |
| 5. Dead Ends & Feedback | `05-dead-ends.md` | 10 | P1 — frustration points |
| 6. Label & Copy Rewrites | `06-label-rewrites.md` | 10 | P1 — confusion points |
| 7. Deployment Tracking UX | `07-deployment-tracking.md` | 3 | P2 — post-deploy experience |
| 8. Shared Components | `08-shared-components.md` | N/A | Foundation — tooltip, help system |

## Implementation Order

```
Wave 1: Domain 8 (shared components) — build the tooltip/help system first
Wave 2: Domains 1 + 2 (concept explainers + field guidance) — in parallel
Wave 3: Domain 3 (onboarding) — depends on explainers existing
Wave 4: Domains 4 + 5 + 6 (errors, dead ends, labels) — in parallel
Wave 5: Domain 7 (deployment tracking) — last, depends on everything
```

## Files Changed

All changes are UI-only (React/TypeScript). No backend changes required.

### New Files
- `ui/src/components/ui/HelpTooltip.tsx` — shared help icon + tooltip
- `ui/src/components/ui/InlineHelp.tsx` — expandable inline explanation
- `ui/src/components/deploy/OnboardingOverlay.tsx` — first-time welcome
- `ui/src/lib/glossary.ts` — centralized plain-English definitions
- `ui/src/lib/field-help.ts` — per-field help text and examples

### Modified Files
- `ui/src/components/deploy/OrderWizard.tsx`
- `ui/src/components/deploy/DeployWizard.tsx`
- `ui/src/components/deploy/order-wizard/StepPlaceOrder.tsx`
- `ui/src/components/deploy/order-wizard/StepTraits.tsx`
- `ui/src/components/deploy/order-wizard/StepWorkloadInference.tsx`
- `ui/src/components/deploy/order-wizard/StepNetworking.tsx`
- `ui/src/components/deploy/order-wizard/StepDependencies.tsx`
- `ui/src/components/deploy/order-wizard/StepConfiguration.tsx`
- `ui/src/components/deploy/order-wizard/StepAnythingElse.tsx`
- `ui/src/components/deploy/order-wizard/StepReview.tsx`
- `ui/src/components/deploy/order-wizard/StepDeployProgress.tsx`
- `ui/src/components/deploy/TemplateCatalog.tsx`
- `ui/src/components/deploy/GitHubImport.tsx`
- `ui/src/components/deploy/GuidedWizard.tsx`
- `ui/src/components/deploy/DeployProgress.tsx`
- `ui/src/lib/deploy-error-intelligence.ts`
- `ui/src/lib/recommendation-engine.ts`
- `ui/src/pages/DeploymentsPage.tsx`

## Success Criteria

- A non-technical user can complete a deploy using only templates with zero questions
- Every field in the wizard has help text accessible via ? icon or inline
- Every error message has a "What to do" section
- No Kubernetes term appears without a plain-English explanation
- Every empty state has a clear call-to-action
- First-time users see a welcome overlay explaining the 3-step flow
