# Domain: Step 4 — Networking

**Source spec:** docs/specs/wizard-v2.md — US-4

## Purpose

Configure how the app is accessed: port, public/private, optional domain.

## Acceptance Criteria

- AC-4.1: Port input with smart default: 8080 for APIs, 3000 for web-facing, 5432 for postgres, etc. Pre-filled based on traits.
- AC-4.2: "Should this be open to the public?" toggle. Pre-selected ON if web-facing trait, OFF otherwise. Label: "Open the front door?"
- AC-4.3: If public: optional domain field. Label: "Custom address? (optional)" Placeholder: "app.example.com"
- AC-4.4: For Local target: only show port field. No public/domain (Docker Compose handles this differently).
- AC-4.5: Step title: "Where's the entrance?"

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepNetworking.tsx` | Step 4 component |

## Component Props

```typescript
interface StepNetworkingProps {
  traits: Trait[];
  target: WizardTarget;
  port: number;
  isPublic: boolean;
  domain: string;
  onUpdate: (updates: Partial<ServiceInterviewState>) => void;
}
```

## UI Elements

1. **Port input** — number field
   - Smart default based on traits:
     - web-facing → 3000
     - stateful (database) → 5432
     - worker → 8080
     - default → 8080
   - Also leverage existing `getPortRecommendation()` from `networking-recommendations.ts` if image is known

2. **Public toggle** (K8s only) — "Open the front door?"
   - Toggle switch with label
   - Pre-selected ON if web-facing trait
   - Pre-selected OFF otherwise

3. **Domain field** (K8s + public only) — "Custom address? (optional)"
   - Text input, placeholder "app.example.com"
   - Only shown when isPublic is true AND target is Kitchen

## Validation

Port must be > 0 and < 65536. Domain is optional.

## Target-Specific Behavior

- **Local:** Only port field shown. No public toggle, no domain.
- **Kitchen:** All fields shown.
