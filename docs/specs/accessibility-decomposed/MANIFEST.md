# Accessibility Spec — Decomposed Manifest

**Parent spec:** `docs/specs/accessibility.md`
**Total gaps:** 54
**Total domains:** 8
**Total requirements:** 53 FRs across all domains

## Domain Files

| # | File | Domain | Gaps | FRs | Wave | Depends On |
|---|------|--------|------|-----|------|------------|
| 8 | `08-shared-components.md` | Shared Components | — | 4 | 1 | None |
| 1 | `01-concept-explainers.md` | Concept Explainers | A1–A18 | 18 | 2 | Domain 8 |
| 2 | `02-field-guidance.md` | Field Guidance | C1–C10 | 10 | 2 | Domain 8 |
| 3 | `03-onboarding.md` | Onboarding & First Run | E1–E7 | 7 | 3 | Domains 1, 8 |
| 4 | `04-error-messages.md` | Error Messages | F1–F6 | 7 | 4 | Domain 8 |
| 5 | `05-dead-ends.md` | Dead Ends & Feedback | D1–D10 | 10 | 4 | Domain 8 |
| 6 | `06-label-rewrites.md` | Label & Copy Rewrites | B1–B10 | 10 | 4 | Domain 8 |
| 7 | `07-deployment-tracking.md` | Deployment Tracking | E5–E7 | 4 | 5 | Domains 3, 4 |

## Wave Execution Plan

```
Wave 1:  [Domain 8]                          Foundation — tooltips, glossary, help system
           │
Wave 2:  [Domain 1] ∥ [Domain 2]            Content — explainers + field guidance (parallel)
           │
Wave 3:  [Domain 3]                          Onboarding — welcome overlay, first-time flows
           │
Wave 4:  [Domain 4] ∥ [Domain 5] ∥ [Domain 6]  Polish — errors, dead ends, labels (parallel)
           │
Wave 5:  [Domain 7]                          Post-deploy — tracking page improvements
```

## File Change Matrix

| File | D1 | D2 | D3 | D4 | D5 | D6 | D7 | D8 |
|------|----|----|----|----|----|----|----|----|
| **NEW: glossary.ts** | | | | | | | | ✓ |
| **NEW: field-help.ts** | | | | | | | | ✓ |
| **NEW: HelpTooltip.tsx** | | | | | | | | ✓ |
| **NEW: InlineHelp.tsx** | | | | | | | | ✓ |
| **NEW: OnboardingOverlay.tsx** | | | ✓ | | | | | |
| OrderWizard.tsx | ✓ | ✓ | ✓ | | ✓ | ✓ | | |
| DeployWizard.tsx | | | ✓ | | ✓ | | | |
| StepPlaceOrder.tsx | ✓ | ✓ | | | | ✓ | | |
| StepTraits.tsx | ✓ | | | | | | | |
| StepWorkloadInference.tsx | ✓ | | | | | ✓ | | |
| StepNetworking.tsx | ✓ | ✓ | | | | ✓ | | |
| StepDependencies.tsx | ✓ | ✓ | | | ✓ | ✓ | | |
| StepConfiguration.tsx | ✓ | ✓ | | | ✓ | ✓ | | |
| StepAnythingElse.tsx | ✓ | ✓ | | | ✓ | | | |
| StepReview.tsx | | | | | ✓ | | | |
| StepDeployProgress.tsx | | | ✓ | ✓ | | | | |
| DeployProgress.tsx | ✓ | | | ✓ | | | | |
| TemplateCatalog.tsx | | | ✓ | | | | | |
| GitHubImport.tsx | ✓ | | | | ✓ | | | |
| GuidedWizard.tsx | ✓ | | | | | | | |
| deploy-error-intelligence.ts | ✓ | | | ✓ | | | | |
| recommendation-engine.ts | | | | | | ✓ | | |
| DeploymentsPage.tsx | | | ✓ | | | | ✓ | |

## Gap → Domain Mapping

| Gap ID | Description | Domain |
|--------|-------------|--------|
| A1–A18 | Unexplained concepts (image, port, pod, etc.) | 1 |
| B1–B10 | Confusing labels (Counter/Kitchen, Replicas, etc.) | 6 |
| C1–C10 | Fields with no guidance (port, storage, env vars, etc.) | 2 |
| D1–D10 | Dead ends (silent errors, empty states, etc.) | 5 |
| E1–E7 | Missing onboarding (first-time UX, status legend, etc.) | 3 + 7 |
| F1–F6 | K8s error messages exposed (ImagePull, RBAC, OOM, etc.) | 4 |

## Cross-Domain Dependencies

- Domain 1 uses `HelpTooltip` from Domain 8
- Domain 2 uses `InlineHelp` and `fieldHelp` from Domain 8
- Domain 3 uses concept explainers from Domain 1 (welcome overlay references terms)
- Domain 4 uses glossary from Domain 8 (expandable technical details)
- Domain 7 uses error messages from Domain 4 and onboarding patterns from Domain 3

## Success Criteria (from parent spec)

- [ ] A non-technical user can complete a deploy using only templates with zero questions
- [ ] Every field in the wizard has help text accessible via ? icon or inline
- [ ] Every error message has a "What to do" section
- [ ] No Kubernetes term appears without a plain-English explanation
- [ ] Every empty state has a clear call-to-action
- [ ] First-time users see a welcome overlay explaining the 3-step flow
