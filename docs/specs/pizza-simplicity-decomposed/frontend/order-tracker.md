---
domain: frontend
file: order-tracker
depends_on:
  - frontend/step3-review
estimated_complexity: medium
---

## Purpose

Build the pizza-style order tracker that replaces the current deploy progress view. Shows 5 friendly stages instead of raw logs, with order number, elapsed timer, and collapsible log section.

## Scope

**Included:**
- 5-stage horizontal pipeline tracker UI
- Order number display
- Elapsed timer
- Collapsible "View kitchen logs" section (reuses existing SSE/DeployLog)
- Success state with access URL card
- Failure state with friendly error message and fix actions
- "Place another order" and "View deployment" actions

**Excluded:**
- Backend deploy API changes
- SSE log streaming changes (reuse existing)
- Error intelligence changes (reuse existing deploy-error-intelligence.ts)

## Requirements

- REQ-1: After "Place Order" is clicked, the wizard view transitions to the order tracker. The 3-step indicator is replaced with the tracker header.
- REQ-2: **Order number** displayed prominently at top: "Order #[number]" — derive from deployment ID (use last 4 digits or a sequential counter from the deployment response's `order_number` field if available, else generate from timestamp).
- REQ-3: **5-stage tracker** displayed as a horizontal pipeline (on desktop) or vertical list (on mobile):
  1. "Taking your order..." — icon: clipboard/receipt
  2. "Prepping ingredients..." — icon: utensils/boxes
  3. "In the oven..." — icon: flame/fire
  4. "Quality check..." — icon: magnifying glass/checkmark
  5. "Ready for pickup!" — icon: party/check-circle (success) OR "Something burned..." — icon: alert-triangle (failure)
- REQ-4: **Stage states:**
  - Pending: muted/gray icon and text
  - Active: orange icon, pulsing animation (CSS pulse), bold text
  - Completed: green checkmark icon, normal text
  - Failed: red X icon, red text
- REQ-5: **Stage mapping from log parsing** — reuse the existing SSE log parsing from `StepDeployProgress`. Map step markers from the log stream to the 5 stages:
  - "Step 1" / recipe save → Stage 1
  - "Step 2" / manifest generation → Stage 2
  - "Step 3" / apply to cluster → Stage 3
  - "Step 4" / health check wait → Stage 4
  - "success" / "App accessible at" → Stage 5 (success)
  - Any error → mark current stage as failed
- REQ-6: **Elapsed timer** below the stage tracker: "Elapsed: Xs" updating every second. Starts when deploy begins, stops when complete or failed.
- REQ-7: **"View kitchen logs"** collapsible section below the timer. Collapsed by default. When expanded, shows the raw log stream using the existing `DeployLog` component (or the log display from `StepDeployProgress`). Auto-scrolls to bottom.
- REQ-8: **Success state:** When stage 5 completes successfully:
  - Stage 5 label changes to "Ready for pickup!"
  - Below the tracker, show a large card: "Your app is ready!" with the access URL as a clickable link (large, prominent)
  - Secondary actions: "View deployment" link (navigates to deployment detail page), "Place another order" button (resets wizard to Step 1)
- REQ-9: **Failure state:** When any stage fails:
  - Failed stage shows red X and label changes to "Something burned..."
  - Below the tracker, show the friendly error message from `deploy-error-intelligence.ts` (existing `classifyDeployError` function). Show: error title, description, suggested action.
  - Action buttons: "Try Again" (retry deploy with same recipe), "Edit Order" (go back to Step 2 with state preserved)
- REQ-10: The entire tracker has the same dark theme as the rest of the app. Orange (#ff6b35) for active/success elements, red for failures, gray for pending.

## Explicit Behaviors

- The tracker replaces the Step 3 view after "Place Order" is clicked. The user cannot go "back" from the tracker to the review step (deploy is in progress).
- "Try Again" does NOT go back to Step 3 — it immediately re-triggers the deploy with the same recipe (calls the deploy API again).
- "Edit Order" goes back to Step 2 (not Step 3) so the user can change settings. State is preserved.
- "Place another order" resets ALL wizard state and goes back to Step 1.
- If the deployment response includes an `order_number` field, use it. Otherwise, generate one from the deployment ID or timestamp: `#d-YYYYMMDD-NNN`.
- The elapsed timer counts wall-clock seconds, not server time. Simple `setInterval` increment.
- Console log errors from the deploy should NOT be shown in the main view — only in the expanded "View kitchen logs" section.

## Dependencies

- `frontend/step3-review` — "Place Order" triggers transition to this tracker

## Interfaces

**Consumes:**
```typescript
// From deploy execution (DeployWizard manages the API calls)
deployment: {
  id: string;
  order_number?: number;
  status: string;
  access_url?: string;
};

// SSE log stream (existing)
streamDeploymentLogs(deploymentId: string, onData: (line: string) => void, onDone: (status: string) => void): () => void;

// Error intelligence (existing)
classifyDeployError(logText: string): { title: string; message: string; action: string; actionLabel: string };

// Existing components
DeployLog: React.Component;  // or the log display from StepDeployProgress
```

**Produces:**
```typescript
// Callbacks to DeployWizard/OrderWizard
onRetry(): void;           // retry deploy with same recipe
onEditOrder(): void;       // go back to Step 2
onPlaceAnother(): void;    // reset wizard to Step 1
onViewDeployment(): void;  // navigate to deployment detail page
```

## Constraints

- Must reuse existing SSE log streaming — no new backend endpoints
- Must reuse existing `classifyDeployError` — no new error mapping logic
- Tracker must be responsive (horizontal on desktop, vertical on mobile)
- Pulse animation must be CSS only (no JS animation library)

## Edge Cases

- Deploy completes in <1 second → all stages flash to complete quickly. Timer shows "1s".
- Deploy fails on Stage 1 (recipe save) → only Stage 1 is red, stages 2-5 stay pending
- SSE connection drops mid-deploy → show "Connection lost" message with "Refresh" button
- Deploy has no access_url (e.g., worker/internal service) → success state shows "Your app is running!" without a URL link
- Very long deploy (>60s) → timer shows "1m 23s" format

## Acceptance Criteria

- [ ] Order tracker appears after "Place Order" with order number
- [ ] 5 stages display with correct labels and icons
- [ ] Active stage pulses with orange animation
- [ ] Completed stages show green checkmark
- [ ] Elapsed timer counts up during deploy
- [ ] "View kitchen logs" is collapsed by default
- [ ] Expanding shows raw log stream
- [ ] Success: shows "Ready for pickup!" with access URL card
- [ ] Success: "Place another order" and "View deployment" actions work
- [ ] Failure: shows friendly error message from error intelligence
- [ ] Failure: "Try Again" and "Edit Order" actions work
- [ ] No K8s jargon in tracker labels or error messages

## Definition of Done

Order tracker renders 5 pizza-themed stages, streams deploy progress, shows friendly success/failure states with actionable next steps. Reuses existing SSE and error intelligence.

## Related Files

- `frontend/step3-review.md` — triggers transition to tracker
- `foundation/deprecate-old-paths.md` — DeployWizard manages deploy API calls

## Testing Strategy

- Visual: click "Place Order" → verify tracker appears with 5 stages
- Visual: verify pulse animation on active stage
- Interaction: successful deploy → verify "Ready for pickup!" state with URL
- Interaction: failed deploy → verify friendly error message and action buttons
- Interaction: "Place another order" → verify wizard resets to Step 1
- Interaction: "Edit Order" → verify goes back to Step 2 with state preserved
- Timer: verify elapsed timer increments during deploy
- Logs: expand "View kitchen logs" → verify raw log stream visible
