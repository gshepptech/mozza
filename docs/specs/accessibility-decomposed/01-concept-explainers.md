# Domain 1: Concept Explainers

**Wave:** 2 (after shared components built)
**Gaps covered:** A1–A18 (18 unexplained concepts)
**Depends on:** Domain 8 (HelpTooltip, glossary)

## Purpose

Add a `HelpTooltip` next to every technical term shown to the user. The user should
never see a word they don't understand without a `?` icon that explains it.

## Requirements

### FR-1.1: Container Image Explainer

**Where:** OrderWizard.tsx (inline alias creation), StepPlaceOrder.tsx, GuidedWizard.tsx

- Add `<HelpTooltip term="container-image" />` next to every "Image" label
- Replace label "Image URL" → "App package URL" with tooltip
- Replace placeholder `"e.g. ghcr.io/org/app:latest"` → `"Paste your app's package URL (ask your developer for this)"`

**Files:** OrderWizard.tsx:437, StepPlaceOrder.tsx:189, GuidedWizard.tsx:409

### FR-1.2: Port Explainer

**Where:** StepNetworking.tsx, OrderWizard.tsx (customize more)

- Add `<HelpTooltip term="port" />` next to "Port" label
- Add `<InlineHelp>` below with framework-specific defaults
- Keep smart defaults from `smartDefaultPort()` — just explain them

**Files:** StepNetworking.tsx:54, OrderWizard.tsx:748

### FR-1.3: Replica Explainer

**Where:** StepPlaceOrder.tsx, OrderWizard.tsx (customize more)

- Replace label "Replicas" → "Running copies"
- Add `<HelpTooltip term="replica" />` next to label
- Add hint: "More copies = handles more traffic, but costs more"

**Files:** StepPlaceOrder.tsx:392, OrderWizard.tsx:768

### FR-1.4: Cluster Explainer

**Where:** StepNetworking.tsx, error messages

- Add `<HelpTooltip term="cluster" />` next to any mention of "cluster"
- Replace "Only reachable inside the cluster" → "Only reachable by other apps in your system"
- Replace "Kitchen (kubernetes)" → "Kitchen (cloud server)" with tooltip

**Files:** StepNetworking.tsx:121, OrderWizard.tsx:1106

### FR-1.5: Pod Explainer

**Where:** DeployProgress.tsx (non-pizza mode only)

- Replace "Waiting for pods" → "Waiting for your app to start"
- In pizza mode this is already "Quality check" — no change needed

**Files:** DeployProgress.tsx:28

### FR-1.6: Environment Variable Explainer

**Where:** StepConfiguration.tsx

- Replace "Add environment variables your app needs" → "Add settings your app needs to run"
- Add `<HelpTooltip term="environment-variable" />` next to header
- Add `<InlineHelp>`: "Environment variables are like settings your app reads when it starts. For example, DATABASE_URL tells your app where to find the database."

**Files:** StepConfiguration.tsx:73

### FR-1.7: Secret Explainer

**Where:** StepConfiguration.tsx

- Replace "Toggle the lock for secrets" → "Mark as sensitive (passwords, API keys)"
- Add `<HelpTooltip term="secret" />` next to lock icon

**Files:** StepConfiguration.tsx (lock toggle area)

### FR-1.8: Health Check Explainer

**Where:** StepAnythingElse.tsx, OrderWizard.tsx

- Add `<HelpTooltip term="health-check" />` next to health check toggle
- Add hint: "Mozza will periodically check if your app is still running"

**Files:** StepAnythingElse.tsx:134, OrderWizard.tsx:882

### FR-1.9: HTTP vs TCP Explainer

**Where:** StepAnythingElse.tsx

- Replace "HTTP" / "TCP" buttons with "Web check (HTTP)" / "Connection check (TCP)"
- Add `<HelpTooltip term="http-vs-tcp" />` next to the toggle
- Hint: "Web check visits a URL. Connection check just verifies the port is open."

**Files:** StepAnythingElse.tsx:140

### FR-1.10: CPU & Memory Explainer

**Where:** StepAnythingElse.tsx, OrderWizard.tsx

- In old 9-step flow: replace raw inputs with T-shirt size selector (already done in OrderWizard)
- Add `<HelpTooltip term="cpu-memory" />` next to "Resources" / "Oven temperature" label
- Replace "250m CPU, 128Mi memory" → "Small: handles light traffic"

**Files:** StepAnythingElse.tsx:198, OrderWizard.tsx:846

### FR-1.11: Auto-Scaling Explainer

**Where:** StepAnythingElse.tsx

- Replace "Auto-scaling" → "Automatic copies"
- Add `<HelpTooltip term="auto-scaling" />`
- Replace "CPU target %" → "Add more copies when load reaches..." with tooltip
- Add hint: "Mozza will automatically add copies when your app gets busy, and remove them when traffic drops"

**Files:** StepAnythingElse.tsx:256

### FR-1.12: Storage Size Explainer

**Where:** StepDependencies.tsx

- Replace placeholder "10Gi" → "10" with "GB" unit label next to input
- Internally convert: user enters "10", we store "10Gi"
- Add hint: "How much disk space for your database. 10 GB is enough for most small apps."
- Add `<HelpTooltip term="storage-gi" />`

**Files:** StepDependencies.tsx:160

### FR-1.13: DATABASE_URL Explainer

**Where:** StepConfiguration.tsx

- Add `<HelpTooltip term="database-url" />` next to auto-generated DATABASE_URL
- Add hint: "This tells your app where to find the database. Generated automatically — you usually don't need to change this."

**Files:** StepConfiguration.tsx:34

### FR-1.14: Cache / Redis Explainer

**Where:** StepDependencies.tsx, OrderWizard.tsx

- Replace "Need a cache (Redis)?" → "Add fast storage (for sessions, counters, etc.)"
- Add `<HelpTooltip term="cache-redis" />`
- Add hint: "A cache stores frequently used data in memory so your app responds faster"

**Files:** StepDependencies.tsx:216, OrderWizard.tsx:725

### FR-1.15: Docker/Helm/Dockerfile Explainer

**Where:** GitHubImport.tsx

- Replace "Looking for Docker Compose, Helm charts, and Dockerfiles" → "Scanning for deployment files..."
- Add expandable `<InlineHelp>`: "We look for files that describe how to build and run your app (Dockerfile, docker-compose.yml, Helm charts). If none are found, you may need to add one to your repo."

**Files:** GitHubImport.tsx:135

### FR-1.16: Registry Explainer

**Where:** deploy-error-intelligence.ts

- Replace "registry is accessible" → "the place where your app package is stored is accessible"
- Add `<HelpTooltip term="registry" />` in error display component

**Files:** deploy-error-intelligence.ts:25

### FR-1.17: RBAC / Service Account Explainer

**Where:** deploy-error-intelligence.ts

- Replace entire message: "Check that the service account has the required RBAC roles" → "Your system administrator needs to grant permission for this deployment. Share this error with them."
- Remove all K8s jargon from user-facing text; put technical details in expandable section

**Files:** deploy-error-intelligence.ts:32-35

### FR-1.18: Namespace Explainer

**Where:** DeployWizard.tsx (API call)

- Namespace is passed as "development" but never shown to user — no change needed unless we surface it later
- If/when shown, use "environment" instead of "namespace"

**Files:** No UI change needed currently

## Acceptance Criteria

- [ ] Every term from A1–A18 has a HelpTooltip or has been replaced with plain English
- [ ] No glossary term appears in the UI without an adjacent ? icon
- [ ] All replacement labels are understandable without tech background
- [ ] InlineHelp is used for concepts needing more than one sentence
- [ ] No new jargon is introduced by the replacements

## Observable Truths

- OT-1: A user seeing "Port" for the first time can hover the ? and understand what to enter
- OT-2: A user seeing "Container image" can understand it's "a packaged version of your app"
- OT-3: The word "pod" never appears in user-facing text (replaced with "app" or "copy")
- OT-4: The word "namespace" never appears in user-facing text (replaced with "environment")
- OT-5: RBAC errors tell the user to contact their admin, not to check Kubernetes roles
- OT-6: Storage input shows "GB" not "Gi"
