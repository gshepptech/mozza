# Domain: Step 5 — Dependencies

**Source spec:** docs/specs/wizard-v2.md — US-5

## Purpose

Add databases, caches, and other services. Presented as pizza toppings toggle cards.

## Acceptance Criteria

- AC-5.1: Toggle cards (like pizza toppings):
  - "Database" → expands: PostgreSQL / MySQL / MongoDB picker + storage size
  - "Cache" → adds Redis slice
  - "Message Queue" → adds RabbitMQ or NATS (future, hidden for now)
- AC-5.2: Each added dependency appears in the sidebar cart.
- AC-5.3: Dependencies auto-generate `needs` directives in the recipe.
- AC-5.4: Database toggle pre-fills: engine = postgres, storage = 10Gi, version = 16.
- AC-5.5: For stateful trait apps: database toggle is pre-selected ON with note: "Since your app stores data, you probably need a database."
- AC-5.6: Step title: "Any sides with that?"
- AC-5.7: Env vars for database connection (DATABASE_URL, etc.) auto-generated when a DB dependency is added.

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepDependencies.tsx` | Step 5 component |

## Types

```typescript
interface DependencyConfig {
  type: "database" | "cache" | "queue";
  enabled: boolean;
  engine: string;    // postgres, mysql, mongo, redis, rabbitmq, nats
  version: string;
  storage: string;   // e.g. "10Gi"
}
```

## Component Props

```typescript
interface StepDependenciesProps {
  traits: Trait[];
  dependencies: DependencyConfig[];
  onUpdate: (deps: DependencyConfig[]) => void;
}
```

## UI Elements

1. **Database card** — toggle card
   - Icon: database
   - When enabled, expands to show:
     - Engine picker: PostgreSQL / MySQL / MongoDB (3 buttons)
     - Storage size input (default "10Gi")
     - Version input (default "16" for postgres)
   - Pre-selected if stateful trait with note

2. **Cache card** — toggle card
   - Icon: lightning bolt / zap
   - When enabled: adds Redis (no config needed)

3. **Message Queue card** — hidden for now (future)
   - AC-5.1 says "hidden for now"

## Auto-Generation

When DB is added:
- Auto-generate env var: `DATABASE_URL` pointing to the DB slice
- Auto-add `needs` directive in recipe generation

## Validation

No required selections — step can be skipped with no dependencies.
