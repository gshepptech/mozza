# Domain: OrderWizard Shell

**Source spec:** docs/specs/wizard-v2.md — Technical Notes

## Purpose

Top-level `OrderWizard` component that replaces `GuidedWizard`. Provides the layout
shell: step navigation, sidebar cart area, step content area, and bottom navigation bar.

## Requirements

- New `OrderWizard` component replacing GuidedWizard
- Props: `onComplete: (recipeName: string, recipeSource: string) => void`, `onAdvancedMode: () => void`
- Same props interface as GuidedWizard for drop-in replacement
- Wizard title: "Place your order" (pizza metaphor)
- Step indicator showing progress through the interview
- Back/Next navigation at the bottom
- Step titles from pizza metaphor reference table
- Layout: sidebar (cart) on left, main content area on right
- For single-service: sidebar collapsed (thin strip) — AC-8.8
- Sidebar visible during steps 2-7, hidden step 1, read-only steps 8-9 — AC-8.7

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/OrderWizard.tsx` | Main wizard component — layout, step rendering, navigation |
| `ui/src/components/deploy/order-wizard/StepIndicator.tsx` | Step progress dots/bar at top |
| `ui/src/components/deploy/order-wizard/StepNavigation.tsx` | Back/Next buttons at bottom |

## Layout

```
┌──────────────────────────────────────────────────────┐
│  🍕 Place your order          Step 3 of 8            │
│  ● ● ● ○ ○ ○ ○ ○                                    │
├──────────┬───────────────────────────────────────────┤
│ Cart     │  Step Content                              │
│ sidebar  │  "How should we prepare it?"               │
│          │                                            │
│ Your     │  [Step-specific component renders here]    │
│ order    │                                            │
│ (2 items)│                                            │
│          │                                            │
│ ┌──────┐ │                                            │
│ │myapp │ │                                            │
│ │×3    │ │                                            │
│ ├──────┤ │                                            │
│ │pg    │ │                                            │
│ │(side)│ │                                            │
│ └──────┘ │                                            │
│          │                                            │
│ [+ Add]  │                                            │
├──────────┴───────────────────────────────────────────┤
│  ← Back                            Start your order →│
└──────────────────────────────────────────────────────┘
```

## Props Interface

```typescript
interface OrderWizardProps {
  onComplete: (recipeName: string, recipeSource: string) => void;
  onAdvancedMode: () => void;
}
```

## State Management

- Uses InterviewEngine from domain 01
- WizardState managed via useState at this level
- Step components receive state + update callbacks
- Cart sidebar receives full services array

## Step Routing

Map step number to component:
1. StepPlaceOrder (US-1)
2. StepTraits (US-2)
3. StepWorkloadInference (US-3)
4. StepNetworking (US-4)
5. StepDependencies (US-5)
6. StepConfiguration (US-6)
7. StepAnythingElse (US-7)
8. StepReview (US-9)
9. StepDeployProgress (US-10)

## Button Text

- Step 1 Next: "Start your order →" (AC-1.6)
- Steps 2-7 Next: "Next →"
- Step 8: "Place Order" (AC-9.7) — bg-brand, Rocket icon
- Back: "← Back"
