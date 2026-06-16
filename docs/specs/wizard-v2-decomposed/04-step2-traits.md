# Domain: Step 2 — Traits Interview

**Source spec:** docs/specs/wizard-v2.md — US-2

## Purpose

Three trait toggles as cards with icons. User selects what their app does.
Skipped entirely for Local target.

## Acceptance Criteria

- AC-2.1: Three trait toggles as cards with icons:
  - "Do people visit this in a browser?" (Globe icon) → web-facing
  - "Does it store or manage data?" (Database icon) → stateful
  - "Does it run tasks in the background?" (Cog icon) → worker/batch
- AC-2.2: Multiple traits can be true simultaneously (multi-select toggles)
- AC-2.3: At least one trait must be selected to proceed
- AC-2.4: For Local target: skip this step entirely — Docker Compose doesn't differentiate workload types. Jump to Networking (US-4).
- AC-2.5: Step title: "What's on the menu?"

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepTraits.tsx` | Step 2 component |

## Component Props

```typescript
interface StepTraitsProps {
  traits: Trait[];
  onUpdate: (traits: Trait[]) => void;
}
```

## UI Elements

Three cards in a row, each toggleable:
- Card selected state: highlighted border + checkmark
- Card unselected: muted border
- Each card: icon (top), question text (middle), trait name badge (bottom)

## Validation

At least one trait selected (AC-2.3).

## Skip Condition

`target === "local"` → skip to Step 4 (Networking).
