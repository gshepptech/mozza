# Domain 2: Field Guidance

**Wave:** 2 (parallel with Domain 1)
**Gaps covered:** C1–C10 (10 fields with no guidance)
**Depends on:** Domain 8 (InlineHelp, fieldHelp module)

## Purpose

Every input field in the wizard should tell the user what to enter, why it matters,
and what a good default looks like. No field should leave the user staring at an
empty input wondering "what goes here?"

## Requirements

### FR-2.1: Image URL Field Guidance (C1)

**Where:** OrderWizard.tsx inline alias creation form

**Current:** Two bare inputs — "App name" and "Image URL" with no explanation.

**Change:**
- Label: "App name" → keep
- Label: "Image URL" → "App package URL"
- Add hint below image input: "This is the address of your packaged app. Your developer or CI pipeline can provide this."
- Add `<InlineHelp>`: "It looks like: `ghcr.io/yourcompany/app:v1.0` or `docker.io/nginx:latest`. If you don't have one, try the Classics tab for pre-built apps."
- Add basic validation: must contain at least one `/` or be a known short name (nginx, postgres, etc.)

**Files:** OrderWizard.tsx:431-440

### FR-2.2: Port Number Guidance (C2)

**Where:** StepNetworking.tsx:54, OrderWizard.tsx:747

**Current:** Input with placeholder "8080" and no context.

**Change:**
- Add hint: "Check your app's startup logs or config. Common ports:"
- Show framework-specific examples as chips: `3000 Node.js` `8080 Go/Java` `5000 Python` `4000 Phoenix`
- Clicking a chip fills the input
- Add `<InlineHelp>`: "If your app prints 'Listening on port XXXX' when it starts, that's the number to use."

**Files:** StepNetworking.tsx:54-76, OrderWizard.tsx:747-765

### FR-2.3: Storage Size Guidance (C3)

**Where:** StepDependencies.tsx:160

**Current:** Input with placeholder "10Gi" — unit unexplained.

**Change:**
- Replace text input with preset buttons: `5 GB` `10 GB` `25 GB` `50 GB` + custom input
- Add hint: "How much disk space for your database"
- Add size guidance:
  - 5 GB: "Small projects, prototypes"
  - 10 GB: "Most apps"
  - 25 GB: "Growing apps with lots of data"
  - 50 GB: "Large datasets"
- Internally convert to Gi format for the recipe

**Files:** StepDependencies.tsx:160-170

### FR-2.4: Database Engine Guidance (C4)

**Where:** StepDependencies.tsx:130, OrderWizard.tsx:706-724

**Current:** Three buttons (PostgreSQL, MySQL, MongoDB) with no comparison.

**Change:**
- Add one-line description under each:
  - PostgreSQL: "Most popular. Great for everything."
  - MySQL: "Common with PHP and WordPress."
  - MongoDB: "For document-style data (JSON)."
- Add "Not sure? PostgreSQL is the safe choice." hint below
- Mark PostgreSQL as "Recommended" with subtle badge

**Files:** StepDependencies.tsx:130-154, OrderWizard.tsx:706-724

### FR-2.5: Environment Variable Guidance (C5)

**Where:** StepConfiguration.tsx

**Current:** Empty table with KEY/VALUE columns. Quick-add buttons (LOG_LEVEL, NODE_ENV, PORT) with no explanation.

**Change:**
- Add intro text: "Settings your app reads when it starts. Most apps need at least a few."
- Quick-add buttons get descriptions:
  - LOG_LEVEL → "Logging detail" (with hint: "Set to 'info' for normal, 'debug' for troubleshooting")
  - NODE_ENV → "App mode" (with hint: "Set to 'production' for live, 'development' for testing")
  - PORT → Remove this button (already set in networking step, confusing duplicate)
- Add framework-specific presets section:
  - "Common for Node.js: NODE_ENV, LOG_LEVEL"
  - "Common for Python: FLASK_ENV, DEBUG"
  - "Common for Go: GIN_MODE, LOG_FORMAT"

**Files:** StepConfiguration.tsx:18-20, 73

### FR-2.6: Health Check Path Guidance (C6)

**Where:** StepAnythingElse.tsx

**Current:** Input with no guidance on what path to use.

**Change:**
- Add hint: "The URL Mozza visits to check if your app is alive"
- Show common paths as clickable chips: `/health` `/healthz` `/ready` `/ping` `/`
- Clicking fills the input
- Add `<InlineHelp>`: "Most web frameworks have a built-in health endpoint. If yours doesn't, `/` (the homepage) works as a basic check."

**Files:** StepAnythingElse.tsx (health check section)

### FR-2.7: CPU/Memory Guidance (C7)

**Where:** StepAnythingElse.tsx (old 9-step flow)

**Current:** Four raw inputs (CPU request, CPU limit, Memory request, Memory limit) with millicores/mebibytes.

**Change:**
- Replace all four inputs with T-shirt size selector (same as OrderWizard already has)
- Show what each size handles:
  - S: "Light traffic, simple apps"
  - M: "Moderate traffic, most apps" (default)
  - L: "Heavy traffic, complex processing"
  - XL: "Very heavy traffic, data crunching"
- Raw values shown as small print below: "M = 250m CPU, 128Mi memory"
- Keep raw inputs accessible via "Show exact values" toggle for power users

**Files:** StepAnythingElse.tsx:198-246

### FR-2.8: Cron Schedule Guidance (C8)

**Where:** OrderWizard.tsx (customize more, workers only)

**Current:** Three preset buttons (Every hour, Every day, Every week) — decent but no custom option explanation.

**Change:**
- Keep presets, add descriptions:
  - Every hour: "Runs once per hour, every hour"
  - Every day: "Runs once per day at midnight"
  - Every week: "Runs once per week on Monday at midnight"
- Add "Custom" button that shows a simple time picker:
  - "Run every __ hours" (number input)
  - "Run every day at __:__" (time picker)
  - "Run on [weekday picker] at __:__"
- DO NOT show raw cron syntax to the user; generate it internally

**Files:** OrderWizard.tsx:809-842

### FR-2.9: Custom Domain Guidance (C9)

**Where:** StepNetworking.tsx:160, OrderWizard.tsx:790

**Current:** "Custom address? (optional)" with placeholder "e.g. api.myapp.com"

**Change:**
- Add hint: "Leave blank and we'll give you one automatically (like myapp.your-cluster.example.com)"
- Add `<InlineHelp>`: "To use your own domain (like app.yourcompany.com), enter it here. You'll need to update your DNS settings to point to your cluster — Mozza will show you how after deploying."

**Files:** StepNetworking.tsx:160-177, OrderWizard.tsx:790-806

### FR-2.10: Replica Count Guidance (C10)

**Where:** StepPlaceOrder.tsx:392, OrderWizard.tsx:768

**Current:** +/- stepper with range 1-100, no context.

**Change:**
- Add hint below stepper:
  - 1: "Single copy — fine for development"
  - 2-3: "Recommended for production — stays up if one copy fails"
  - 4+: "High availability — handles heavy traffic"
- Show warning at >10: "High replica count increases resource usage"
- Show warning at >50: "Are you sure? This will use significant resources."

**Files:** StepPlaceOrder.tsx:392-446, OrderWizard.tsx:768-787

## Acceptance Criteria

- [ ] Every input field has either a hint, placeholder, or InlineHelp
- [ ] No field uses raw Kubernetes units (m, Mi, Gi) in user-facing text
- [ ] Framework-specific examples are shown where applicable
- [ ] Preset buttons have descriptions explaining what they do
- [ ] Cron expressions are never shown to the user (generated internally)
- [ ] Storage input uses GB not Gi
- [ ] Port field has clickable framework examples

## Observable Truths

- OT-1: A user who doesn't know their port number can click "3000 Node.js" to fill it
- OT-2: A user choosing a database engine can read one-line descriptions to decide
- OT-3: A user sees "10 GB" not "10Gi" when configuring storage
- OT-4: A user scheduling a worker can pick "Every day at 9am" without seeing cron syntax
- OT-5: A user with 1 replica sees "fine for development" — no guessing
- OT-6: A user who doesn't know what env vars to add sees framework-specific suggestions
