# Spec: Deploy Wizard v2 — Pizza-Ordering Flow

**Feature:** wizard-v2
**Date:** 2026-03-17
**Status:** APPROVED

---

## Overview

Replace the current GuidedWizard with a pizza-ordering-style progressive interview. The user starts with "I want 3 myapp", gets walked through adaptive questions with pre-selected recommendations, reviews the generated recipe, and presses "Place Order" to deploy. The entire UI uses pizza/restaurant metaphor language — no stone unturned.

## Out of Scope

- LLM-assisted wizard (future enhancement)
- Nomad target support
- Helm chart integration
- Multi-cluster deploy in one order
- GitOps / CD pipeline integration

---

## User Stories

### US-1: Place an Order (Step 1)

**As a** user, **I want to** pick an image alias, set a replica count, and choose a deploy target **so that** the wizard knows what I'm ordering.

**Acceptance Criteria:**
- AC-1.1: Autocomplete field searches saved aliases by name. Typing "my" shows all aliases starting with "my".
- AC-1.2: Replica count is a number stepper, default 1, min 1, max 100.
- AC-1.3: Target toggle: "Local" (Docker Compose) or "Kitchen" (Kubernetes). Default: Local if no clusters registered, Kitchen if clusters exist.
- AC-1.4: If user types an alias name that doesn't exist, an inline form expands: "We don't have myapp on the menu yet — what's the full image?" with a registry URL input. On submit, alias is created via POST /api/aliases and selected.
- AC-1.5: If target is Kitchen (K8s) and no clusters are registered, show: "You need to connect a kitchen first" with link to /app/clusters.
- AC-1.6: "Next" button text: "Start your order →"

### US-2: Traits Interview (Step 2)

**As a** user, **I want to** answer simple yes/no questions about my app's behavior **so that** Mozza can infer the right workload type.

**Acceptance Criteria:**
- AC-2.1: Three trait toggles presented as cards with icons:
  - "Do people visit this in a browser?" (Globe icon) → web-facing
  - "Does it store or manage data?" (Database icon) → stateful
  - "Does it run tasks in the background?" (Cog icon) → worker/batch
- AC-2.2: Multiple traits can be true simultaneously (multi-select toggles).
- AC-2.3: At least one trait must be selected to proceed.
- AC-2.4: For **Local target**: skip this step entirely — Docker Compose doesn't differentiate workload types. Jump to Networking (US-4).
- AC-2.5: Step title: "What's on the menu?"

### US-3: Workload Inference + Confirm (Step 3, K8s only)

**As a** user, **I want to** see Mozza's recommendation for my workload type with reasoning **so that** I can confirm or override it.

**Acceptance Criteria:**
- AC-3.1: Inference logic:
  - web-facing + NOT stateful → ReplicaSet (recommended)
  - stateful (with or without web-facing) → StatefulSet (recommended)
  - worker/batch only → ReplicaSet for long-running, CronJob if scheduled
  - all three → StatefulSet (data trumps)
- AC-3.2: Display format: "Based on your answers, this looks like a **[type]**. We recommend **[workload]** with **[N] copies**." Followed by reasoning: "Because [explanation in plain language]."
- AC-3.3: Recommendation is **pre-selected**. User sees 3 option cards:
  - "Replicas — identical copies for availability" (with pizza analogy: "Like having multiple ovens running the same recipe")
  - "Stateful — each copy has its own storage" ("Like assigning a dedicated chef to each station")
  - "Everywhere — one copy on every node" ("Like putting a menu in every window")
- AC-3.4: Recommended option has glow border + "(Chef's recommendation)" badge.
- AC-3.5: CronJob option appears only if worker trait is selected: "Runs on a schedule — pick when" with schedule picker.
- AC-3.6: Step title: "How should we prepare it?"
- AC-3.7: Skipped for Local target.

### US-4: Networking (Step 4)

**As a** user, **I want to** configure how my app is accessed **so that** it's reachable where it needs to be.

**Acceptance Criteria:**
- AC-4.1: Port input with smart default: 8080 for APIs, 3000 for web-facing, 5432 for postgres, etc. Pre-filled based on traits.
- AC-4.2: "Should this be open to the public?" toggle. Pre-selected ON if web-facing trait, OFF otherwise. Label: "Open the front door?"
- AC-4.3: If public: optional domain field. Label: "Custom address? (optional)" Placeholder: "app.example.com"
- AC-4.4: For Local target: only show port field. No public/domain (Docker Compose handles this differently).
- AC-4.5: Step title: "Where's the entrance?"

### US-5: Dependencies (Step 5)

**As a** user, **I want to** add databases, caches, and other services my app needs **so that** they deploy together as a complete stack.

**Acceptance Criteria:**
- AC-5.1: Presented as toggle cards (like pizza toppings):
  - "Database" → expands: PostgreSQL / MySQL / MongoDB picker + storage size
  - "Cache" → adds Redis slice
  - "Message Queue" → adds RabbitMQ or NATS (future, hidden for now)
- AC-5.2: Each added dependency appears in the sidebar cart (see US-8).
- AC-5.3: Dependencies auto-generate `needs` directives in the recipe.
- AC-5.4: Database toggle pre-fills: engine = postgres, storage = 10Gi, version = 16.
- AC-5.5: For stateful trait apps: database toggle is pre-selected ON with note: "Since your app stores data, you probably need a database."
- AC-5.6: Step title: "Any sides with that?"
- AC-5.7: Env vars for database connection (DATABASE_URL, etc.) auto-generated when a DB dependency is added.

### US-6: Configuration (Step 6)

**As a** user, **I want to** add environment variables and secrets **so that** my app has the config it needs at runtime.

**Acceptance Criteria:**
- AC-6.1: Key-value pair list with add/remove buttons.
- AC-6.2: Secret toggle per entry (eye icon). Secrets use K8s Secrets, non-secrets use ConfigMap (for K8s target).
- AC-6.3: Auto-populated entries from dependencies: if DB was added, DATABASE_URL is pre-filled pointing to the DB slice.
- AC-6.4: "Common configs" quick-add buttons: LOG_LEVEL, NODE_ENV, PORT (pre-filled from networking step).
- AC-6.5: Step title: "Special instructions"

### US-7: Anything Else? (Step 7)

**As a** user, **I want to** review and optionally customize health checks, resource limits, and scaling **so that** my deployment is production-ready.

**Acceptance Criteria:**
- AC-7.1: Summary card showing auto-set defaults based on workload type:
  - Health check: HTTP GET /health (web-facing) or TCP port check (others)
  - Resources: CPU 250m-500m, Memory 128Mi-256Mi (web), 500m-1cpu, 256Mi-512Mi (DB)
  - Scaling: OFF by default. If ON: 1-5 replicas, 80% CPU target.
- AC-7.2: Each section is **expandable** — collapsed by default showing the summary, click to customize.
- AC-7.3: Pre-filled values match workload type. User only changes what they want.
- AC-7.4: Step title: "Anything else before we cook?"
- AC-7.5: For Local target: skip health check and scaling sections (not applicable to Docker Compose). Show only resource limits (Docker resource constraints).

### US-8: Service Cart (Sidebar)

**As a** user, **I want to** see all services in my order as I build them **so that** I can manage a multi-service stack.

**Acceptance Criteria:**
- AC-8.1: Left sidebar shows all services in the current order as cards: name, type icon, replica count.
- AC-8.2: Active service is highlighted. Click a service to edit it.
- AC-8.3: "+" button at the bottom: "Add another item to your order". Clicking starts a new service interview from US-1 (alias + count, skipping target since it's already set).
- AC-8.4: Dependency slices (auto-added DB, cache) shown with a "side" badge and lighter styling — they're part of the order but auto-configured.
- AC-8.5: Delete button (trash icon) on each service. Deleting a service removes its dependencies too (with confirmation).
- AC-8.6: Cart header shows total services: "Your order (3 items)"
- AC-8.7: Sidebar is always visible during steps 2-7. Hidden during step 1 (first service not yet started) and visible but read-only during steps 8-9 (review + deploy).
- AC-8.8: For single-service orders, sidebar is collapsed (just a thin strip showing the one service) to maximize interview area.

### US-9: Review Your Order (Step 8)

**As a** user, **I want to** review the generated recipe and optionally edit it **so that** I'm confident in what's being deployed.

**Acceptance Criteria:**
- AC-9.1: Primary view: `.mozza` recipe in a syntax-highlighted code editor (Monaco or CodeMirror).
- AC-9.2: Recipe is **editable inline**. Changes are validated on blur (parse the recipe, show errors inline).
- AC-9.3: Secondary toggle: "View the blueprint" shows compiled Kubernetes YAML (read-only for K8s target) or docker-compose.yml (for Local target).
- AC-9.4: Summary card above the editor showing: service count, total replicas, target, dependencies.
- AC-9.5: If user edits the recipe, show a "Modified" badge. "Reset" button reverts to generated version.
- AC-9.6: Step title: "Review your order"
- AC-9.7: "Place Order" button: `bg-brand`, prominent, with Rocket icon. Disabled until recipe parses successfully.
- AC-9.8: Save recipe option: "Save this recipe for later" toggle. If ON, saves to team's recipe list.

### US-10: Place Order + Tracking (Step 9)

**As a** user, **I want to** deploy with one click and see live progress **so that** I know my order is being fulfilled.

**Acceptance Criteria:**
- AC-10.1: On "Place Order" click: save recipe → compile → deploy. Show progress stepper (existing DeployProgress component).
- AC-10.2: Progress stages (pizza metaphor):
  - "Taking your order..." (saving recipe)
  - "Prepping ingredients..." (compiling manifests)
  - "Firing up the oven..." (applying to cluster / docker compose up)
  - "Checking the crust..." (health waiting)
  - "Order served!" (deploy complete) or "Order burned!" (deploy failed)
- AC-10.3: Order number assigned and displayed: "Order #d-20260317-001"
- AC-10.4: On success: "Your order has been served!" with links to status page and "Place another order" button.
- AC-10.5: On failure: contextual error with fix suggestion. Parse K8s error → human-friendly message + fix action:
  - Image not found → "We can't find that ingredient. Check the image name in your alias." + link back to alias.
  - Permission denied → "The kitchen doesn't have permission to cook this. Your cluster needs [specific RBAC]."
  - Cluster unreachable → "Can't reach the kitchen. Check your cluster connection." + link to /app/clusters.
  - Timeout → "Order is taking too long to prepare. The oven might be overloaded." + retry button.
- AC-10.6: "Try Again" button available on failure. "Edit Order" button drops back to review step with error context.

### US-11: Image Alias Management

**As a** user, **I want to** manage my image aliases **so that** I can quickly reference images by friendly names.

**Acceptance Criteria:**
- AC-11.1: Alias autocomplete in Step 1 searches by name and image URL.
- AC-11.2: Inline alias creation (see AC-1.4) saves to DB immediately.
- AC-11.3: Existing aliases show image URL as subtitle in autocomplete dropdown.
- AC-11.4: If an alias resolves to a private registry, Mozza prompts for pull secret name (K8s) or registry login (local).
- AC-11.5: Recently used aliases appear first in autocomplete.

---

## Functional Requirements

### FR-1: Progressive Interview Engine
- Questions are defined as a sequence with skip conditions
- Each question has: ID, title (pizza metaphor), traits required, target filter (local/k8s/both), pre-fill logic, validation
- Skip logic evaluates after each answer to determine next question
- Back navigation preserves all answers

### FR-2: Recommendation Engine
- Maps trait combinations to workload types with confidence levels
- Generates human-readable reasoning for each recommendation
- Pre-selects recommended options in the UI
- Recommendations are overridable — user's choice always wins

### FR-3: Recipe Generation
- Generates valid .mozza recipe from wizard answers
- Includes all configured services, dependencies, env vars, health checks
- Recipe validates against the existing parser (no syntax errors)
- Supports all existing recipe DSL features: `needs`, `set`, `limit`, `health check`, `domain`, `open to the public`, etc.

### FR-4: Multi-Service Support
- Sidebar cart tracks all services in the current order
- Each service has independent interview state
- Dependencies are shared across services (one DB serves all that need it)
- Recipe generation produces a single recipe with all services

### FR-5: Inline Recipe Editing
- Code editor component with .mozza syntax highlighting
- Real-time parse validation on blur
- Error markers on invalid lines
- Reset to generated version

### FR-6: Deploy Error Intelligence
- Parse common K8s errors into human-friendly messages
- Map errors to wizard steps (image error → alias step, RBAC → cluster page)
- Provide actionable fix suggestions, not just error text

---

## UI Copy — Pizza Metaphor Reference

| Context | Copy |
|---------|------|
| Wizard title | "Place your order" |
| Step 1 title | "What are you ordering?" |
| Step 2 title | "What's on the menu?" |
| Step 3 title | "How should we prepare it?" |
| Step 4 title | "Where's the entrance?" |
| Step 5 title | "Any sides with that?" |
| Step 6 title | "Special instructions" |
| Step 7 title | "Anything else before we cook?" |
| Step 8 title | "Review your order" |
| Step 9 title | "Order placed!" |
| Add service | "Add another item to your order" |
| Cart header | "Your order (N items)" |
| Deploy button | "Place Order" |
| Deploy progress 1 | "Taking your order..." |
| Deploy progress 2 | "Prepping ingredients..." |
| Deploy progress 3 | "Firing up the oven..." |
| Deploy progress 4 | "Checking the crust..." |
| Deploy success | "Order served!" |
| Deploy failed | "Order burned!" |
| Retry button | "Try Again" |
| Edit after fail | "Edit Order" |
| New deploy | "Place Another Order" |
| Recommendation badge | "Chef's recommendation" |
| Public access toggle | "Open the front door?" |
| Dependencies header | "Sides & extras" |
| Env vars | "Special instructions" |
| Health check | "How should we check the crust?" |
| Scaling | "Need more ovens?" |
| K8s target | "Kitchen" |
| Local target | "Counter" (local/dev) |
| Cluster | "Kitchen" |
| ReplicaSet explanation | "Like having multiple ovens running the same recipe" |
| StatefulSet explanation | "Like assigning a dedicated chef to each station" |
| DaemonSet explanation | "Like putting a menu in every window" |
| YAML toggle | "View the blueprint" |
| Save recipe | "Save this recipe for later" |

---

## Implementation Phases

### Phase 1: Foundation — Interview Engine + Single Service
- New `OrderWizard` component replacing GuidedWizard
- Interview engine with question sequencing, skip logic, back navigation
- Steps 1-4: alias picker, traits, inference, networking
- Single service only (no cart sidebar yet)
- Recipe generation from wizard answers
- Basic review step (recipe view, no inline editing yet)
- **Verify:** Wizard produces valid .mozza recipe for a single service. Recipe compiles and deploys.

### Phase 2: Dependencies + Config + Defaults
- Steps 5-7: dependencies, env vars, "anything else?"
- Auto-add DB/cache slices with smart defaults
- Auto-populate env vars (DATABASE_URL, etc.)
- Recommendation engine with pre-selected defaults
- **Verify:** Wizard generates multi-slice recipe (service + DB + cache). All env vars and health checks present.

### Phase 3: Multi-Service Cart
- Sidebar service cart (US-8)
- Add/remove services
- Independent interview state per service
- Shared dependencies across services
- **Verify:** Can build a 3-service stack (web + api + DB) in one wizard run. Cart shows all services.

### Phase 4: Review + Edit + Deploy
- Inline recipe editor with syntax highlighting and validation
- YAML toggle view
- Deploy with pizza-metaphor progress messages
- Error intelligence (contextual errors + fix suggestions)
- Save recipe option
- **Verify:** Can edit generated recipe inline, deploy succeeds, errors show human-friendly messages.

### Phase 5: Polish + Alias UX
- Inline alias creation flow
- Autocomplete with recent aliases
- Private registry handling
- Responsive design + animations
- Full pizza metaphor copy pass
- **Verify:** Complete flow from new alias creation to deployed stack, all UI copy uses metaphor.

---

## Technical Notes

### Components to Create
- `OrderWizard.tsx` — main wizard component (replaces GuidedWizard)
- `InterviewEngine.ts` — question sequencing, skip logic, state management
- `RecommendationEngine.ts` — trait → workload type mapping + reasoning
- `ServiceCart.tsx` — sidebar cart component
- `RecipeEditor.tsx` — inline recipe editor with validation
- `OrderProgress.tsx` — deploy progress with pizza metaphor stages (wraps existing DeployProgress)

### Components to Modify
- `DeployWizard.tsx` — swap GuidedWizard for OrderWizard in the guided source path
- `DeployProgress.tsx` — add pizza metaphor stage names

### API Endpoints (existing, no changes needed)
- POST /api/aliases — create alias (for inline creation)
- GET /api/aliases — list aliases (for autocomplete)
- POST /api/deploy — trigger deploy
- GET /api/clusters — list clusters (for target selection)

### Recipe Generation
- Reuse `generateRecipeSource()` pattern from existing GuidedWizard
- Extend to support all workload types, health checks, resource limits, scaling
- Validate generated recipe against `internal/recipe` parser via existing validate endpoint
