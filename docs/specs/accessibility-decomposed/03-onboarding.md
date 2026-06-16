# Domain 3: Onboarding & First-Time UX

**Wave:** 3 (after concept explainers exist)
**Gaps covered:** E1–E7 (7 onboarding gaps)
**Depends on:** Domain 8 (shared components), Domain 1 (concept explainers)

## Purpose

A first-time user landing on the deploy wizard should immediately understand:
1. What Mozza does
2. What they need to get started
3. Which path to take (templates vs custom)

## Requirements

### FR-3.1: Welcome Overlay for First-Time Users (E1, E2)

**Where:** DeployWizard.tsx (wraps OrderWizard)

**Trigger:** Show when `localStorage.getItem("mozza-onboarded")` is null.

**Content:**
```
🍕 Welcome to Mozza!

Deploy your app in 3 simple steps:

  1. Pick — Choose your app from your menu, our templates, or a GitHub repo
  2. Customize — Tell us what kind of app it is
  3. Order — Review and deploy

[Get started]  [I've done this before — skip]
```

**Behavior:**
- Full-screen overlay with dark semi-transparent backdrop
- Centered card, max-width 480px
- "Get started" → dismiss, set `localStorage`, proceed to wizard
- "Skip" → dismiss, set `localStorage`, proceed to wizard
- Shown only once ever (unless localStorage cleared)
- Accessible: focus trap, Escape to dismiss

**Files:** New `OnboardingOverlay.tsx`, modified `DeployWizard.tsx`

### FR-3.2: Tab Guidance in Step 1 (E3)

**Where:** OrderWizard.tsx step 1

**Current:** Three tabs ("Your menu", "Classics", "From a repo") with no guidance on which to pick.

**Change:**
- Add subtitle under tabs:
  - "Your menu" tab: "Apps you've saved before"
  - "Classics" tab: "Ready-to-deploy popular apps"
  - "From a repo" tab: "Import from GitHub"
- When menu is empty AND it's a first visit, auto-switch to "Classics" tab
- Add visual indicator on "Classics" tab: "Start here" badge for first-time users

**Files:** OrderWizard.tsx:362-475

### FR-3.3: Empty Menu Smart CTA (E2, D2)

**Where:** OrderWizard.tsx empty state

**Current:** "Your menu is empty" with small "pick from our classics" link.

**Change:**
```
Your menu is empty — no worries!

[Browse ready-to-deploy apps →]     ← Primary CTA, styled as brand button
                                       Links to Classics tab

or

[Add your own app]                   ← Secondary, text link
                                       Opens inline create form

[Import from GitHub]                 ← Tertiary, text link
                                       Switches to repo tab
```

- The three options are ordered by ease-of-use (easiest first)
- "Browse ready-to-deploy apps" is prominent — this is the easiest path for non-engineers

**Files:** OrderWizard.tsx:389-402

### FR-3.4: Template Comparison Help (E4)

**Where:** TemplateCatalog.tsx

**Current:** Templates listed in a grid with name, icon, description, category badge.

**Change:**
- Add "What should I pick?" help section at top:
  - "Want a blog?" → WordPress, Ghost
  - "Want project management?" → Gitea
  - "Want analytics?" → Plausible
  - "Want automation?" → n8n
  - "Want monitoring?" → Uptime Kuma
  - "Just exploring?" → Start with Uptime Kuma (simplest)
- Add "Beginner friendly" badge to simplest templates (Uptime Kuma, Redis Commander)
- Add "Popular" badge to most-used templates
- Resource estimate tooltips: "S" = "Light, runs on small clusters" etc.

**Files:** TemplateCatalog.tsx

### FR-3.5: Deployment Status Legend (E5)

**Where:** DeploymentsPage.tsx

**Current:** Status badges (Running, Succeeded, Failed, Rolled Back, Partial) with no explanation.

**Change:**
- Add clickable "What do these mean?" link near status filters
- Shows legend:
  - **Running** — "Your deploy is in progress"
  - **Succeeded** — "Your app is live and healthy"
  - **Failed** — "Something went wrong — check the logs"
  - **Rolled Back** — "We undid the deploy and restored the previous version"
  - **Partial** — "Some parts deployed, some didn't — check details"
  - **Pending** — "Waiting to start"
- Legend uses same color coding as status badges

**Files:** DeploymentsPage.tsx

### FR-3.6: Post-Deploy Next Steps (E6)

**Where:** DeployWizard.tsx (success state), StepDeployProgress.tsx

**Current:** Shows access URL and "Place Another Order" / "View Deployments" buttons.

**Change:**
- Add "What's next?" section after success:
  ```
  ✓ Your app is live!

  → Visit your app: [https://myapp.cluster.example.com]
  → View logs: [View Logs button]
  → See all deployments: [View Deployments button]
  → Deploy again: [Place Another Order button]

  Tip: Bookmark your app's URL. Check the Deployments page
  to see status and roll back if needed.
  ```
- Each action is a clear button/link, not just text

**Files:** DeployWizard.tsx:82-149, StepDeployProgress.tsx (success card)

### FR-3.7: Failed Deploy Recovery Path (E7)

**Where:** DeploymentsPage.tsx, DeployWizard.tsx

**Current:** "Retry" button exists but is disabled ("Coming soon"). Only option is "Try Again" (re-deploys same recipe).

**Change:**
- Enable "Edit & Retry" button on failed deployments:
  - Opens the wizard pre-filled with the failed deployment's recipe
  - User can modify and re-deploy
- Add "What went wrong?" expandable section:
  - Shows the last error from deploy logs
  - Uses `analyzeDeployError()` to provide pizza-metaphor explanation
  - Includes "What to do" text from Domain 4 (error messages)
- Change "Try Again" to "Deploy Same Recipe Again" (clearer)

**Files:** DeploymentsPage.tsx:230-241, DeployWizard.tsx:104-127

## Acceptance Criteria

- [ ] First-time users see welcome overlay explaining 3-step flow
- [ ] Welcome overlay shown only once (localStorage persisted)
- [ ] Empty menu state has 3 clear CTAs ordered by ease
- [ ] First-time users see "Classics" tab auto-selected when menu is empty
- [ ] Template catalog has "What should I pick?" guidance
- [ ] Deployment statuses have a legend accessible from the page
- [ ] Success screen shows clear next steps with action buttons
- [ ] Failed deployments have "Edit & Retry" workflow

## Observable Truths

- OT-1: A first-time user understands what Mozza does within 5 seconds of the welcome overlay
- OT-2: A user with an empty menu can deploy a template without creating an alias first
- OT-3: A user who doesn't know which template to pick can find one via "What should I pick?"
- OT-4: A user seeing "Rolled Back" status can hover/click to understand what that means
- OT-5: A user whose deploy failed can edit the recipe and try again without starting over
- OT-6: A user who just deployed successfully knows exactly what to do next
