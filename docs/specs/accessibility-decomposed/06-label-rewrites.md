# Domain 6: Label & Copy Rewrites

**Wave:** 4 (parallel with Domains 4 and 5)
**Gaps covered:** B1–B10 (10 confusing labels and descriptions)
**Depends on:** Domain 8 (HelpTooltip for terms that can't be fully replaced)

## Purpose

Replace every confusing label, description, and copy with plain English that a
non-engineer understands on first read. When a technical term must stay, add a
tooltip. When it can be replaced entirely, replace it.

## Requirements

### FR-6.1: "Counter" vs "Kitchen" Labels (B1)

**Where:** OrderWizard.tsx:1094-1111

**Current:** Two buttons labeled "Local" and "Kitchen" with no explanation.

**Change:**
- "Local" → "My computer" with subtitle: "Run on your machine for testing"
- "Kitchen" → "Cloud server" with subtitle: "Deploy to your team's server"
- Add `<HelpTooltip term="deploy-target" />` next to section label
- Glossary entry: "Deploy target — where your app runs. 'My computer' is for trying things out. 'Cloud server' is for making it available to others."

**Files:** OrderWizard.tsx:1094-1111

### FR-6.2: Workload Type Labels (B2)

**Where:** StepWorkloadInference.tsx

**Current:** "Replicas", "Stateful", "Everywhere", "Scheduled"

**Change:**
| Current | New Label | New Subtitle |
|---------|-----------|-------------|
| Replicas | Identical copies | "Multiple copies for reliability. If one fails, others keep working." |
| Stateful | Copies with own storage | "Each copy remembers its own data. Good for databases." |
| Everywhere | One per server | "Puts one copy on every server in your system. Good for monitoring." |
| Scheduled | Runs on a timer | "Runs at specific times, then stops until the next run." |

- Keep "Chef's recommendation" badge on the recommended option
- Add `<HelpTooltip term="workload-type" />` next to section header

**Files:** StepWorkloadInference.tsx, recommendation-engine.ts (labels and subtitles)

### FR-6.3: "Each copy has its own storage" (B3)

**Where:** recommendation-engine.ts

**Current:** "Each copy has its own storage" — assumes user knows what "storage" and "copy" mean.

**Change:** "Each running instance of your app keeps its own separate data — like having separate notebooks for each employee."

**Files:** recommendation-engine.ts:121

### FR-6.4: "One copy on every node" (B4)

**Where:** recommendation-engine.ts

**Current:** "One copy on every node" — "node" is K8s jargon.

**Change:** "Puts one copy on every server in your system"

**Files:** recommendation-engine.ts:122

### FR-6.5: "persistent storage" (B5)

**Where:** recommendation-engine.ts

**Current:** "Each copy needs its own persistent storage to keep data safe."

**Change:** "Each copy keeps its own data that survives restarts — nothing gets lost if it stops and starts again."

**Files:** recommendation-engine.ts:29

### FR-6.6: "storage identity" (B6)

**Where:** recommendation-engine.ts

**Current:** "Each worker needs its own storage identity."

**Change:** "Each background worker has its own dedicated data space."

**Files:** recommendation-engine.ts:40

### FR-6.7: "Image alias" Label (B7)

**Where:** StepPlaceOrder.tsx:189

**Current:** "Image alias" in the autocomplete.

**Change:**
- Replace "Image alias" → "Saved app"
- Replace "Search by name or image URL..." → "Search your saved apps..."
- Replace "No aliases yet" → "No saved apps yet"

**Files:** StepPlaceOrder.tsx:189, 208, 314

### FR-6.8: "Listen on" (B8)

**Where:** StepNetworking.tsx

**Current:** "The port your app listens on inside the container"

**Change:** "The port number your app uses — usually printed when your app starts (like 'Server started on port 3000')"

**Files:** StepNetworking.tsx:75

### FR-6.9: Quick-Add Button Labels (B9)

**Where:** StepConfiguration.tsx:18-20

**Current:** Three buttons: `LOG_LEVEL`, `NODE_ENV`, `PORT` — no descriptions.

**Change:**
| Current | New Label | Added On-Click |
|---------|-----------|---------------|
| LOG_LEVEL | + Logging level | Sets LOG_LEVEL=info with hint "Controls how detailed your app's logs are" |
| NODE_ENV | + App mode | Sets NODE_ENV=production with hint "Tells your app whether it's in testing or live mode" |
| PORT | Remove entirely | Already set in the networking step — duplicate causes confusion |

**Files:** StepConfiguration.tsx:18-20

### FR-6.10: Cache Description (B10)

**Where:** StepDependencies.tsx:216-221

**Current:** "In-memory cache for fast lookups and sessions"

**Change:** "Fast temporary storage — makes your app respond quicker by remembering recent data"

**Files:** StepDependencies.tsx:221

## Acceptance Criteria

- [ ] "Counter" and "Kitchen" replaced with "My computer" and "Cloud server"
- [ ] All 4 workload type labels rewritten with plain-English names and subtitles
- [ ] "node", "persistent storage", "storage identity" never appear in user-facing text
- [ ] "Image alias" replaced with "Saved app" throughout
- [ ] "Listen on" replaced with user-friendly description
- [ ] Quick-add PORT button removed, LOG_LEVEL and NODE_ENV have descriptions
- [ ] Cache description uses no technical terms

## Observable Truths

- OT-1: A user choosing between "My computer" and "Cloud server" understands the difference
- OT-2: A user seeing workload type options can pick the right one from descriptions alone
- OT-3: The word "node" never appears in any user-facing label or description
- OT-4: The word "alias" never appears in any user-facing label or description
- OT-5: A user adding LOG_LEVEL understands it "controls how detailed logs are"
- OT-6: A user considering a cache understands it "makes your app respond quicker"
