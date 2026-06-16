# Domain: Step 3 — Workload Inference + Confirm

**Source spec:** docs/specs/wizard-v2.md — US-3

## Purpose

Show Mozza's recommendation for workload type based on traits. User confirms
or overrides. Skipped for Local target.

## Acceptance Criteria

- AC-3.1: Inference logic:
  - web-facing + NOT stateful → ReplicaSet (recommended)
  - stateful (with or without web-facing) → StatefulSet (recommended)
  - worker/batch only → ReplicaSet for long-running, CronJob if scheduled
  - all three → StatefulSet (data trumps)
- AC-3.2: Display format: "Based on your answers, this looks like a **[type]**. We recommend **[workload]** with **[N] copies**." Followed by reasoning.
- AC-3.3: Recommendation is pre-selected. User sees 3 option cards:
  - "Replicas — identical copies for availability" ("Like having multiple ovens running the same recipe")
  - "Stateful — each copy has its own storage" ("Like assigning a dedicated chef to each station")
  - "Everywhere — one copy on every node" ("Like putting a menu in every window")
- AC-3.4: Recommended option has glow border + "(Chef's recommendation)" badge.
- AC-3.5: CronJob option appears only if worker trait is selected: "Runs on a schedule — pick when" with schedule picker.
- AC-3.6: Step title: "How should we prepare it?"
- AC-3.7: Skipped for Local target.

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepWorkloadInference.tsx` | Step 3 component |

## Component Props

```typescript
interface StepWorkloadInferenceProps {
  traits: Trait[];
  replicas: number;
  workloadType: WorkloadType;
  workloadOverridden: boolean;
  schedule: string;
  onUpdate: (updates: Partial<ServiceInterviewState>) => void;
}
```

## UI Elements

1. **Recommendation banner** — text block with inference result
   - Uses RecommendationEngine to get recommendation + reasoning
   - Format: "Based on your answers, this looks like a **web service**. We recommend **Replicas** with **3 copies**."
   - Reasoning: "Because your app is web-facing and doesn't store data, identical copies give you the best availability."

2. **Workload cards** — 3-4 selectable cards
   - ReplicaSet: pizza oven icon, "Replicas", subtitle, pizza analogy
   - StatefulSet: chef icon, "Stateful", subtitle, pizza analogy
   - DaemonSet: menu icon, "Everywhere", subtitle, pizza analogy
   - CronJob (conditional on worker trait): clock icon, "Scheduled", with schedule picker

3. **Chef's recommendation badge** — on the recommended card
   - Glow border (e.g., `box-shadow: 0 0 12px rgba(255, 107, 53, 0.4)`)
   - "(Chef's recommendation)" text badge

4. **Schedule picker** (if CronJob selected)
   - Preset options: every minute, hourly, daily, weekly
   - Custom cron input

## Validation

A workload type must be selected. If CronJob, schedule must be non-empty.

## Skip Condition

`target === "local"` → skip entirely.
