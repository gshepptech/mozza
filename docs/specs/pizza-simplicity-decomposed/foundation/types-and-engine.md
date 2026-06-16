---
domain: foundation
file: types-and-engine
depends_on: []
estimated_complexity: medium
---

## Purpose

Add fast-lane types, step definitions, and trait combo preset logic to the interview engine. This is the foundation that all 3 fast-lane steps build on.

## Scope

**Included:**
- New TypeScript types for fast-lane state (combo selection, expanded flag, source type)
- Fast-lane 3-step definitions in `interview-engine.ts`
- Trait combo presets with auto-default mappings
- T-shirt size resource mappings
- Smart default inference logic

**Excluded:**
- UI components (handled by step1/step2/step3 domain files)
- Recipe generation changes (existing `generateOrderRecipe()` is reused)
- Backend changes

## Requirements

- REQ-1: Add a `TraitCombo` type with values: `"website"`, `"api-database"`, `"background-worker"`, `"database-only"`, `"custom"`
- REQ-2: Add a `FastLaneSource` type with values: `"alias"`, `"template"`, `"github"`
- REQ-3: Add a `ResourceSize` type with values: `"S"`, `"M"`, `"L"`, `"XL"`
- REQ-4: Extend `WizardState` (or create parallel `FastLaneState`) with fields:
  - `fastLane: boolean` (true by default, false when "Customize more" is expanded)
  - `selectedCombo: TraitCombo | null`
  - `source: FastLaneSource`
  - `selectedTemplateId: string | null`
  - `selectedAliasId: string | null`
  - `resourceSize: ResourceSize` (default "M")
- REQ-5: Add fast-lane step definitions to `interview-engine.ts`:
  - Step FL-1: "What do you want to run?" (id: `fl-pick`)
  - Step FL-2: "How do you want it?" (id: `fl-customize`)
  - Step FL-3: "Review your order" (id: `fl-review`)
- REQ-6: Create a `TRAIT_COMBO_PRESETS` constant mapping each combo to its auto-defaults:
  - `website`: `{ traits: ["web-facing"], port: 3000, isPublic: true, healthCheck: { enabled: true, type: "http", path: "/health", port: 3000, intervalSeconds: 10 }, workloadType: "replicaset", dependencies: [{ type: "database", enabled: false }, { type: "cache", enabled: false }] }`
  - `api-database`: `{ traits: ["web-facing", "stateful"], port: 8080, isPublic: true, healthCheck: { enabled: true, type: "http", path: "/health", port: 8080, intervalSeconds: 10 }, workloadType: "replicaset", dependencies: [{ type: "database", enabled: true, engine: "postgres", version: "16", storage: "10Gi" }, { type: "cache", enabled: false }] }`
  - `background-worker`: `{ traits: ["worker"], port: 0, isPublic: false, healthCheck: { enabled: false, type: "http", path: "/health", port: 8080, intervalSeconds: 10 }, workloadType: "replicaset", dependencies: [{ type: "database", enabled: false }, { type: "cache", enabled: false }] }`
  - `database-only`: `{ traits: ["stateful"], port: 5432, isPublic: false, healthCheck: { enabled: false, type: "tcp", path: "", port: 5432, intervalSeconds: 10 }, workloadType: "statefulset", dependencies: [{ type: "database", enabled: true, engine: "postgres", version: "16", storage: "10Gi" }, { type: "cache", enabled: false }] }`
  - `custom`: null (falls through to expanded trait toggles)
- REQ-7: Create a `RESOURCE_SIZE_MAP` constant:
  - `S`: `{ cpuRequest: "100m", cpuLimit: "200m", memoryRequest: "64Mi", memoryLimit: "128Mi" }`
  - `M`: `{ cpuRequest: "250m", cpuLimit: "500m", memoryRequest: "128Mi", memoryLimit: "256Mi" }`
  - `L`: `{ cpuRequest: "500m", cpuLimit: "1000m", memoryRequest: "256Mi", memoryLimit: "512Mi" }`
  - `XL`: `{ cpuRequest: "1000m", cpuLimit: "2000m", memoryRequest: "512Mi", memoryLimit: "1024Mi" }`
- REQ-8: Create an `applyComboDefaults(combo: TraitCombo, service: ServiceInterviewState): ServiceInterviewState` function that merges preset defaults into a service state. When combo is `"custom"`, return service unchanged.
- REQ-9: Ensure `DATABASE_URL` env var is auto-generated when database dependency is enabled (format: `postgres://{appname}-db:5432/{appname}` for postgres, similar for mysql/mongo)

## Explicit Behaviors

- When `applyComboDefaults` is called with `"api-database"`, the returned service MUST have `dependencies[0].enabled = true` and `envVars` must include a `DATABASE_URL` entry with `auto: true`
- When switching combos, ALL previous combo defaults are replaced (not merged additively)
- The fast-lane step definitions must coexist with the existing 8-step definitions — both must be importable from `interview-engine.ts`
- `ResourceSize` defaults to `"M"` and is only visible when "Customize more" is expanded

## Dependencies

None — this is the foundation.

## Interfaces

**Exports from `interview-types.ts`:**
```typescript
export type TraitCombo = "website" | "api-database" | "background-worker" | "database-only" | "custom";
export type FastLaneSource = "alias" | "template" | "github";
export type ResourceSize = "S" | "M" | "L" | "XL";
```

**Exports from `interview-engine.ts`:**
```typescript
export const FAST_LANE_STEPS: InterviewQuestion[];
export const TRAIT_COMBO_PRESETS: Record<TraitCombo, ComboPreset | null>;
export const RESOURCE_SIZE_MAP: Record<ResourceSize, ResourceDefaults>;
export function applyComboDefaults(combo: TraitCombo, service: ServiceInterviewState): ServiceInterviewState;
```

## Constraints

- No breaking changes to existing `ServiceInterviewState` or `WizardState` — add fields as optional
- Existing 8-step flow must still work for "Customize more" fallback
- All new types must be exported for use by step components

## Edge Cases

- User selects "api-database" combo, then toggles database OFF in extras → combo should switch to "custom" or decouple
- Switching combos rapidly should not leave stale state (e.g., DATABASE_URL from a previous combo)
- Template source selected → combo cards are irrelevant (template provides its own config)

## Acceptance Criteria

- [ ] `TraitCombo`, `FastLaneSource`, `ResourceSize` types are exported from `interview-types.ts`
- [ ] `TRAIT_COMBO_PRESETS` maps each combo to correct defaults
- [ ] `RESOURCE_SIZE_MAP` maps S/M/L/XL to correct CPU/memory values
- [ ] `applyComboDefaults("api-database", defaultService)` returns service with postgres enabled and DATABASE_URL set
- [ ] `applyComboDefaults("custom", service)` returns service unchanged
- [ ] `FAST_LANE_STEPS` exports 3 step definitions with correct ids and titles
- [ ] Existing `getActiveSteps()` / `getNextStep()` / `getPreviousStep()` still work for 8-step flow

## Definition of Done

All new types, constants, and functions are implemented in `interview-types.ts` and `interview-engine.ts`. Existing interview engine functionality is unbroken. Fast-lane steps are importable.

## Related Files

- `frontend/step2-customize.md` — consumes `TRAIT_COMBO_PRESETS` and `applyComboDefaults`
- `frontend/step3-review.md` — consumes `RESOURCE_SIZE_MAP`

## Testing Strategy

- Unit-test `applyComboDefaults` for each combo: verify port, isPublic, traits, dependencies, envVars
- Unit-test `RESOURCE_SIZE_MAP` values match spec
- Verify `FAST_LANE_STEPS` has 3 entries with correct stepNumbers
- Verify existing 8-step `getActiveSteps()` returns same results as before
