# Spec: Pizza Simplicity — Make Deploy Feel Like Ordering Pizza

**Feature:** pizza-simplicity
**Date:** 2026-03-17
**Status:** DRAFT
**Depends on:** wizard-v2 (implemented), deploy-sources (implemented)

---

## Problem

The vision is "deploy apps like you order pizza." The current deploy experience doesn't deliver on that promise. A non-engineer hitting the Deploy page faces:

1. **A fork before the menu** — SourceSelector asks "Template Catalog / GitHub Import / Custom Recipe" before the user even understands what a recipe is.
2. **Container image jargon** — `nginx:alpine`, `postgres:16-alpine`, port numbers, cron expressions, `10Gi`, `250m` CPU.
3. **Too many steps** — The OrderWizard has 8 steps. Pizza ordering has 3: pick, customize, pay.
4. **Technical defaults surfaced** — Health check paths, CPU/memory limits, scaling targets visible by default.
5. **Multiple wizard codepaths** — GuidedWizard, OrderWizard, TemplateCatalog, GitHubImport all reachable from the same page.

The result: it feels like "deploy apps through a slightly friendlier Kubernetes form."

## Solution

Redesign the deploy flow around a single, opinionated path that mirrors pizza ordering:

```
PICK → CUSTOMIZE → ORDER → TRACK
```

Three questions to deploy. Everything else is optional, collapsed, expert-mode.

---

## Out of Scope

- Backend API changes (recipe format, deploy endpoint, alias API — all stay the same)
- LLM-assisted wizard
- New workload type support
- CLI changes
- Recipe builder / canvas editor changes

---

## Architecture Decision

**UI-only changes.** The OrderWizard already generates valid `.mozza` recipe source. The backend deploy pipeline is untouched. We're reshaping how we collect inputs and what we show by default.

**Key files affected:**
- `ui/src/components/deploy/DeployWizard.tsx` — main orchestrator, rewrite entry flow
- `ui/src/components/deploy/OrderWizard.tsx` — collapse to 3-step fast lane + optional expand
- `ui/src/components/deploy/order-wizard/*` — simplify individual steps
- `ui/src/components/deploy/SourceSelector.tsx` — demote to inline option inside wizard
- `ui/src/lib/interview-engine.ts` — add fast-lane step definitions
- `ui/src/lib/interview-types.ts` — add fast-lane state fields
- `ui/src/pages/DeployWizardPage.tsx` — minor copy changes

**Files NOT changed:**
- `ui/src/components/deploy/TemplateCatalog.tsx` — kept, moved inside wizard
- `ui/src/components/deploy/GitHubImport.tsx` — kept, moved inside wizard
- `ui/src/components/deploy/GuidedWizard.tsx` — deprecated, no longer rendered
- `ui/src/components/deploy/order-wizard/StepDeployProgress.tsx` — kept, enhanced
- Backend Go files — zero changes

---

## User Stories

### US-1: Single Entry Point — "What do you want to run?"

**As a** non-engineer, **I want** a single, clear starting point when I click Deploy **so that** I'm not confused by multiple paths before I even begin.

**Acceptance Criteria:**

- AC-1.1: The Deploy page shows ONE wizard. No SourceSelector fork. The OrderWizard is the default and only visible path.
- AC-1.2: Step 1 is titled **"What do you want to run?"** and presents three ways to answer, as tabs or cards within the same step (not separate pages):
  - **"Pick from the menu"** — shows alias catalog (searchable grid of friendly-named aliases with icons/logos). This is the default selected tab.
  - **"Choose a classic"** — shows the template catalog inline (WordPress, Ghost, Gitea, etc.)
  - **"I have a repo"** — shows the GitHub import field
- AC-1.3: The alias catalog shows cards with: friendly name, logo/icon (first letter fallback), short description ("Node.js web server"), and a subtle image tag underneath. No raw registry URLs as primary display.
- AC-1.4: If no aliases exist yet, the "Pick from the menu" tab shows an empty state: "Your menu is empty — add your first app or pick a classic" with arrow pointing to the other tabs.
- AC-1.5: Selecting an alias, template, or completing GitHub import all funnel into the same next step. The wizard state captures what was selected regardless of source.
- AC-1.6: The "I know what I'm doing" escape hatch remains at the bottom, linking to the recipe editor (advanced mode).
- AC-1.7: **GuidedWizard is no longer rendered anywhere.** Remove from DeployWizard.tsx import and source selector options. The file can stay but is dead code.

### US-2: Three-Question Fast Lane

**As a** non-engineer, **I want to** go from "what" to "deploying" in 3 steps **so that** deploying feels as simple as ordering pizza.

**Acceptance Criteria:**

- AC-2.1: The fast lane has exactly 3 visible steps:
  1. **"What do you want to run?"** (US-1) — pick alias/template/repo
  2. **"How do you want it?"** — the only customization screen
  3. **"Review & order"** — see the summary, press deploy
- AC-2.2: Step indicators at the top show 3 dots/pills, not 8. Labels: "Pick" → "Customize" → "Order"
- AC-2.3: Step 2 ("How do you want it?") shows only these controls, in this order:
  - **App name** — pre-filled from alias name or template name, editable
  - **Where to deploy** — "Local" or "Kitchen" toggle (same as current target selector)
  - **Trait cards** — "Website", "API + Database", "Background Worker" (simplified from current 3 separate traits into common combos). Each card is a single click that sets multiple traits + smart defaults at once.
  - **Extras** — toggle switches for "Add a database" (with engine picker if on), "Add a cache (Redis)". Same as current StepExtras/StepDependencies but inline.
- AC-2.4: Selecting a trait combo auto-sets ALL downstream defaults:
  - "Website" → port 3000, public, health check /health, ReplicaSet, 1 replica
  - "API + Database" → port 8080, public, health check /health, ReplicaSet, 1 replica, postgres enabled, DATABASE_URL auto-set
  - "Background Worker" → no port, private, no health check, ReplicaSet, 1 replica
- AC-2.5: Step 3 ("Review & order") shows a human-readable summary card (not raw recipe):
  - App name and type
  - Where it's deploying (Local / Kitchen name)
  - Services list with key details
  - "Your order" receipt-style layout
  - A collapsed "View recipe" section for those who want to see/edit the raw .mozza source
  - **"Place Order"** button (orange, prominent, pizza-themed)
- AC-2.6: The recipe is generated automatically from the wizard state. The user never needs to see it unless they choose to.

### US-3: Smart Defaults — Zero Jargon by Default

**As a** non-engineer, **I want** all technical details handled automatically **so that** I don't need to know what ports, replicas, CPU limits, or cron expressions are.

**Acceptance Criteria:**

- AC-3.1: The following are NEVER shown in the fast lane (hidden behind "Customize more" expand):
  - Port number (auto-detected from alias/trait)
  - Replica count (default 1)
  - CPU/memory requests and limits
  - Health check path, interval, timeout
  - Scaling min/max/target
  - Workload type name (ReplicaSet, StatefulSet, etc.)
  - Cron expressions (show "Every hour / day / week" buttons instead)
  - Storage sizes in Gi notation (show "Small 5GB / Medium 20GB / Large 50GB" if shown at all)
- AC-3.2: Domain names — if public, auto-generate a domain. Don't ask unless user clicks "Custom domain" link.
- AC-3.3: Environment variables — don't show the env var editor in the fast lane. Auto-generated ones (DATABASE_URL, REDIS_URL) are set silently. User can add custom ones via "Customize more."
- AC-3.4: The "Customize more" expand section (available on Step 2) reveals:
  - Port override
  - Replica count
  - Environment variables editor
  - Domain override
  - Schedule picker (for workers)
  - Health check config
  - Resource limits (as T-shirt sizes: S/M/L/XL, not raw values)
  - Scaling config
- AC-3.5: T-shirt size mapping for resources:
  - **S**: 100m/200m CPU, 64Mi/128Mi memory
  - **M**: 250m/500m CPU, 128Mi/256Mi memory (default)
  - **L**: 500m/1000m CPU, 256Mi/512Mi memory
  - **XL**: 1000m/2000m CPU, 512Mi/1024Mi memory

### US-4: Pizza-Style Order Tracker

**As a** user, **I want to** see my deploy progress as a pizza order tracker **so that** I understand what's happening without reading logs.

**Acceptance Criteria:**

- AC-4.1: After clicking "Place Order", the view transitions to an order tracker with these stages:
  1. "Taking your order..." (saving recipe + validating)
  2. "Prepping ingredients..." (generating manifests/compose)
  3. "In the oven..." (applying to cluster/docker)
  4. "Quality check..." (waiting for health checks)
  5. "Ready for pickup!" (success) or "Something burned..." (failure)
- AC-4.2: Each stage shows as a horizontal pipeline with icons. Active stage pulses. Completed stages get a checkmark. Failed stage gets an X.
- AC-4.3: An **order number** is displayed prominently: "Order #1234" (derived from deployment ID or sequence).
- AC-4.4: Below the stage tracker, an **estimated time** or elapsed timer shows.
- AC-4.5: A "View kitchen logs" collapsible section shows the raw log stream (existing DeployLog/SSE functionality). Collapsed by default.
- AC-4.6: On success: show the access URL prominently as a big clickable card: "Your app is ready! → Open [url]". Plus secondary actions: "View deployment", "Place another order".
- AC-4.7: On failure: show the friendly error message from deploy-error-intelligence.ts (already implemented) with suggested fix action. "Edit order" goes back to Step 2 with state preserved.
- AC-4.8: This replaces the current StepDeployProgress content but reuses its SSE log parsing logic.

### US-5: Template & GitHub as First-Class Tabs

**As a** non-engineer, **I want to** deploy a well-known app (WordPress, Gitea) or paste a GitHub URL **without** leaving the pizza ordering flow **so that** everything feels like one experience.

**Acceptance Criteria:**

- AC-5.1: Templates selected in Step 1 flow into Step 2 with pre-filled values. Template variables become the "customize" fields in Step 2 instead of the trait cards.
- AC-5.2: GitHub repos imported in Step 1 generate a recipe that pre-fills Step 2. The user sees the same "How do you want it?" screen with inferred values.
- AC-5.3: When a template is selected, Step 2 heading changes to: "Customize your [template name]" and shows template-specific variables (e.g., "Admin password", "Storage size") styled as the same toggle/input cards.
- AC-5.4: The step indicator remains 3 steps regardless of source. Templates don't add extra steps.

### US-6: Deprecate Old Paths

**As a** developer, **I want** unused wizard code paths removed **so that** the codebase is clean.

**Acceptance Criteria:**

- AC-6.1: `SourceSelector.tsx` is no longer rendered as a gate/step. It may be kept as a component but is not in the main flow.
- AC-6.2: `GuidedWizard.tsx` is no longer imported or rendered in `DeployWizard.tsx`.
- AC-6.3: The `DeployWizard` component's internal step machine is simplified — no more `source` state that gates which sub-wizard to show.
- AC-6.4: The old 8-step OrderWizard step definitions in `interview-engine.ts` remain but are only used when user clicks "Customize more" to access advanced options. The fast lane has its own 3-step flow.

---

## UX Flow Diagram

```
User clicks "Deploy" in sidebar
         │
         ▼
┌─────────────────────────────────────┐
│  STEP 1: "What do you want to run?" │
│                                     │
│  [Pick from menu] [Classic] [Repo]  │
│                                     │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐  │
│  │myapp│ │nginx│ │redis│ │ + ← │  │
│  └─────┘ └─────┘ └─────┘ └─────┘  │
│                                     │
│            [Next →]                 │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  STEP 2: "How do you want it?"      │
│                                     │
│  App name: [my-blog        ]        │
│  Deploy to: [Local] [Kitchen]       │
│                                     │
│  What kind?                         │
│  ┌──────────┐ ┌──────────┐         │
│  │ Website  │ │API + DB  │ ...     │
│  └──────────┘ └──────────┘         │
│                                     │
│  Extras:                            │
│  [x] Add a database (PostgreSQL ▼)  │
│  [ ] Add a cache                    │
│                                     │
│  ▸ Customize more...                │
│                                     │
│            [Next →]                 │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  STEP 3: "Review your order"        │
│                                     │
│  ┌─ Your Order ──────────────────┐  │
│  │ my-blog (Website)             │  │
│  │ Deploying to: Local           │  │
│  │ + PostgreSQL database         │  │
│  │ Public: yes                   │  │
│  └───────────────────────────────┘  │
│                                     │
│  ▸ View recipe (collapsed)          │
│                                     │
│       [◀ Back]  [🍕 Place Order]    │
└─────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│  ORDER TRACKER                      │
│  Order #1234                        │
│                                     │
│  ● Taking order                     │
│  ● Prepping ━━━━━ (active)          │
│  ○ In the oven                      │
│  ○ Quality check                    │
│  ○ Ready!                           │
│                                     │
│  ▸ View kitchen logs                │
│                                     │
│  Elapsed: 12s                       │
└─────────────────────────────────────┘
```

---

## Trait Combo Cards (Step 2)

Replace the three individual trait toggles with pre-built combos:

| Card | Label | Traits Set | Auto-Defaults |
|------|-------|-----------|---------------|
| Website | "A site people visit" | web-facing | port 3000, public, http health |
| API + Database | "Backend with storage" | web-facing, stateful | port 8080, public, postgres on, DATABASE_URL |
| Background Worker | "Runs behind the scenes" | worker | no port, private, no health check |
| Database Only | "Just a database" | stateful | postgres, 10Gi, private |
| Custom | "Something else" | (none, user picks) | Falls through to expanded trait toggles |

When "Custom" is selected, the original three trait toggles appear below.

---

## Component Mapping

| New UX Element | Existing Component | Change |
|---|---|---|
| Step 1 alias catalog | StepPlaceOrder | Redesign as browsable grid instead of autocomplete input |
| Step 1 template tab | TemplateCatalog | Embed inline, remove category sidebar |
| Step 1 repo tab | GitHubImport | Embed inline |
| Step 2 trait combos | StepTraits | Replace 3 toggles with combo cards |
| Step 2 extras | StepDependencies | Inline toggles, not separate step |
| Step 2 "Customize more" | StepNetworking, StepConfiguration, StepAnythingElse | Collapsed accordion |
| Step 3 summary | StepReview | Receipt-style card instead of raw recipe |
| Order tracker | StepDeployProgress | Visual pipeline stages |
| Step indicator | StepIndicator | 3 dots instead of 8 |

---

## Migration Notes

- The existing 8-step interview engine is not deleted. It powers the "Customize more" expanded view. The fast lane is a new 3-step flow layered on top.
- `ServiceInterviewState` gains optional fields for the fast lane (selected combo, expanded flag). No breaking changes.
- Recipe generation uses the same `generateOrderRecipe()` function. The fast lane just pre-fills more state before calling it.
- Deploy API calls are identical. Only the UI path to get there changes.

---

## Testing Checklist

- [ ] Non-engineer can deploy a template in 3 clicks (pick template → next → place order)
- [ ] Non-engineer can deploy their own app in 3 steps (pick alias → set type → place order)
- [ ] No Kubernetes vocabulary visible in fast lane (no ports, replicas, CPU, cron, Gi)
- [ ] "Customize more" reveals all advanced options
- [ ] Template variables appear as Step 2 fields when template selected
- [ ] GitHub import flows into Step 2 with inferred values
- [ ] Order tracker shows pizza stages during deploy
- [ ] Error messages use pizza metaphors (already implemented, just verify)
- [ ] Recipe is generated correctly with smart defaults
- [ ] GuidedWizard is no longer reachable from any UI path
- [ ] "I know what I'm doing" escape hatch still works
