# Domain: Step 6 — Configuration (Env Vars)

**Source spec:** docs/specs/wizard-v2.md — US-6

## Purpose

Add environment variables and secrets for the app.

## Acceptance Criteria

- AC-6.1: Key-value pair list with add/remove buttons.
- AC-6.2: Secret toggle per entry (eye icon). Secrets use K8s Secrets, non-secrets use ConfigMap.
- AC-6.3: Auto-populated entries from dependencies: if DB was added, DATABASE_URL is pre-filled pointing to the DB slice.
- AC-6.4: "Common configs" quick-add buttons: LOG_LEVEL, NODE_ENV, PORT (pre-filled from networking step).
- AC-6.5: Step title: "Special instructions"

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepConfiguration.tsx` | Step 6 component |

## Types

```typescript
interface EnvVarEntry {
  key: string;
  value: string;
  isSecret: boolean;
}
```

## Component Props

```typescript
interface StepConfigurationProps {
  envVars: EnvVarEntry[];
  dependencies: DependencyConfig[];
  port: number;
  onUpdate: (envVars: EnvVarEntry[]) => void;
}
```

## UI Elements

1. **Env var list** — rows of key/value/secret
   - Key input (text)
   - Value input (text, masked if secret)
   - Secret toggle (eye icon)
   - Remove button (trash icon)
   - Add button at bottom: "+ Add variable"

2. **Auto-populated entries** — pre-filled from dependencies
   - If DB added: `DATABASE_URL = postgres://{service}-db:5432/{appname}`
   - These are editable but marked with "auto" badge

3. **Quick-add buttons** — common configs
   - LOG_LEVEL (default: "info")
   - NODE_ENV (default: "production")
   - PORT (pre-filled from networking step)
   - Each button adds the entry if not already present

## Validation

Keys must be non-empty and valid env var names (uppercase, underscores).
No duplicate keys allowed.
