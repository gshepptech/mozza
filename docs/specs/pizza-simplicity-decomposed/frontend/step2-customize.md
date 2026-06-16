---
domain: frontend
file: step2-customize
depends_on:
  - foundation/types-and-engine
  - frontend/step1-pick
estimated_complexity: high
---

## Purpose

Build Step 2 of the fast lane: "How do you want it?" — the single customization screen with trait combo cards, extras, and a collapsible "Customize more" section for advanced options.

## Scope

**Included:**
- Step 2 layout with app name, target toggle, trait combo cards, extras
- Trait combo card selection with auto-default application
- Extras section (database toggle + engine picker, cache toggle)
- "Customize more" collapsible section with advanced options
- T-shirt size resource selector
- Template-specific variable mode (when source is template)
- Schedule picker for workers (human-readable, not cron)

**Excluded:**
- Step 1 and Step 3 UI
- Recipe generation (existing function reused)
- Deploy execution

## Requirements

- REQ-1: Step 2 title: **"How do you want it?"** (or **"Customize your [template name]"** when source is template)
- REQ-2: **App name** input at top, pre-filled from alias name or template name. Editable. Label: "Name your app"
- REQ-3: **Deploy target** toggle: "Local" and "Kitchen" as two pill buttons side by side. Default: Kitchen if clusters exist, Local if not. Same logic as existing target selector.
- REQ-4: **Trait combo cards** — 5 cards in a grid (2 columns on desktop):
  - "Website" with Globe icon — "A site people visit"
  - "API + Database" with Server+Database icon — "Backend with storage"
  - "Background Worker" with Cog icon — "Runs behind the scenes"
  - "Database Only" with Database icon — "Just a database"
  - "Custom" with Settings icon — "Something else"
  Only one combo can be selected at a time. Selected card gets orange border glow.
- REQ-5: When a combo is selected, call `applyComboDefaults(combo, service)` to set ALL downstream defaults (port, isPublic, healthCheck, workloadType, dependencies, etc.). This happens immediately on click.
- REQ-6: **Extras section** (below combo cards):
  - "Add a database" toggle switch. When ON, show engine picker (PostgreSQL / MySQL / MongoDB as 3 small pill buttons). Pre-ON if combo already includes database.
  - "Add a cache (Redis)" toggle switch. Pre-ON if combo already includes cache.
  - These modify `service.dependencies` directly.
- REQ-7: When database is toggled ON, auto-add `DATABASE_URL` to env vars (with `auto: true`). When toggled OFF, remove it.
- REQ-8: **"Customize more"** — a collapsible section (chevron + "Customize more..." text). Collapsed by default. When expanded, sets `fastLane: false` in wizard state. Contains:
  - **Port** number input (label: "Port")
  - **Replicas** number stepper (label: "How many copies?", min 1, max 100)
  - **Environment variables** editor (existing key/value/secret UI from StepConfiguration)
  - **Custom domain** input (label: "Custom address", placeholder: "app.example.com")
  - **Schedule picker** (only visible if worker combo selected) — "Every hour / Every day / Every week / Custom" buttons. Same as existing schedule presets but NO raw cron input in the main view.
  - **Health check** toggle + type selector (HTTP/TCP) + path input
  - **Resources** as T-shirt size selector: S / M / L / XL buttons (maps via `RESOURCE_SIZE_MAP`)
  - **Scaling** toggle + min/max stepper (label: "Auto-scale?")
- REQ-9: When "Custom" combo is selected, the original 3 trait toggles appear below the combo cards (web-facing, stateful, worker) so user can pick individual traits.
- REQ-10: **Template variable mode**: When `source === "template"`, hide trait combo cards and extras. Instead, show the template's variables as input fields (text, password, number, select based on variable type). Each variable gets a label and optional description. This replaces the combo cards section.
- REQ-11: **GitHub import mode**: When `source === "github"`, show trait combo cards with the inferred combo pre-selected (based on what the GitHub import detected). User can override.

## Explicit Behaviors

- Selecting a new combo replaces ALL defaults from the previous combo. It does NOT merge. `applyComboDefaults` replaces traits, port, isPublic, healthCheck, workloadType, dependencies entirely.
- If user selects "API + Database" (which enables postgres), then manually toggles database OFF in extras, the combo visually stays "API + Database" but the database dependency is off. This is fine — the combo card is a quick-start, not a constraint.
- "Customize more" is a one-way expansion in the current session — once expanded, it stays expanded (no re-collapse needed). It adds fields but doesn't remove the combo cards above.
- T-shirt sizes only appear inside "Customize more". Default is "M" (250m/500m CPU, 128Mi/256Mi memory). Selecting a size updates `service.resources` via `RESOURCE_SIZE_MAP`.
- Schedule picker buttons set `service.schedule` to the corresponding cron expression. "Custom" reveals a text input for raw cron. Only visible for worker/scheduled combos.

## Dependencies

- `foundation/types-and-engine` — `TraitCombo`, `TRAIT_COMBO_PRESETS`, `applyComboDefaults`, `RESOURCE_SIZE_MAP`
- `frontend/step1-pick` — provides `source`, `selectedTemplateId`, `aliasName` in wizard state

## Interfaces

**Consumes:**
```typescript
// From wizard state (set by Step 1)
source: FastLaneSource;
selectedTemplateId: string | null;
aliasName: string;

// From foundation
applyComboDefaults(combo: TraitCombo, service: ServiceInterviewState): ServiceInterviewState;
TRAIT_COMBO_PRESETS: Record<TraitCombo, ComboPreset | null>;
RESOURCE_SIZE_MAP: Record<ResourceSize, ResourceDefaults>;

// Existing APIs
listClusters(): Promise<{ clusters: Cluster[] }>;
getTemplate(id: string): Promise<Template>;
```

**Produces (via wizard state update):**
```typescript
{
  selectedCombo: TraitCombo;
  fastLane: boolean;     // false when "Customize more" expanded
  // Plus all service fields updated by applyComboDefaults
}
```

## Constraints

- Must NOT show port, replicas, CPU, memory, health check, scaling, cron expressions, storage Gi notation, workload type names, or domain input in the fast lane (above the "Customize more" fold)
- Must show "Every hour / day / week" buttons instead of cron expressions
- Must show S/M/L/XL instead of raw CPU/memory values
- Template variables must be styled consistently with the rest of Step 2

## Edge Cases

- No clusters registered → Kitchen option disabled or hidden, Local pre-selected
- Template with 0 variables → show "No configuration needed" message, Step 2 is just app name + target
- Template with 10+ variables → scroll within the step
- User selects "Database Only" combo → hide extras section (database is already the main thing)
- User on mobile → combo cards stack to 1 column, extras are full-width
- Switching combos while "Customize more" is expanded → expanded fields update to match new combo

## Acceptance Criteria

- [ ] Step 2 shows "How do you want it?" title
- [ ] App name pre-filled from Step 1 selection
- [ ] Target toggle shows Local/Kitchen
- [ ] 5 trait combo cards render in grid
- [ ] Clicking "Website" sets port 3000, public, web-facing trait
- [ ] Clicking "API + Database" sets port 8080, public, postgres on, DATABASE_URL auto-added
- [ ] Clicking "Background Worker" sets no port, private, worker trait
- [ ] Database toggle in extras works, engine picker appears when ON
- [ ] Cache toggle in extras works
- [ ] "Customize more" is collapsed by default
- [ ] Expanding "Customize more" reveals port, replicas, env vars, domain, health check, resources, scaling
- [ ] Resources show as S/M/L/XL buttons, not raw values
- [ ] Template source shows template variables instead of combo cards
- [ ] No K8s jargon visible above the "Customize more" fold

## Definition of Done

Step 2 renders combo cards, extras, and collapsed advanced section. Selecting a combo auto-sets all defaults. Template mode shows variables. No jargon in the fast lane.

## Related Files

- `frontend/step1-pick.md` — provides source and selection data
- `frontend/step3-review.md` — consumes the customized service state

## Testing Strategy

- Visual: navigate through Step 1 → Step 2 → verify combo cards, extras, collapsed section
- Interaction: click each combo → verify defaults applied correctly (check port, isPublic, dependencies)
- Interaction: toggle database in extras → verify DATABASE_URL env var added/removed
- Interaction: expand "Customize more" → verify all advanced fields appear
- Interaction: select T-shirt size → verify resources updated
- Template mode: select template in Step 1 → verify Step 2 shows template variables
- Jargon check: visually inspect — no ports, replicas, CPU, cron visible in collapsed mode
