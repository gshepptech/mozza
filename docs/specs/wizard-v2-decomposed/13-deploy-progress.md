# Domain: Place Order + Tracking (Step 9)

**Source spec:** docs/specs/wizard-v2.md — US-10, FR-6

## Purpose

Deploy with one click, show pizza-metaphor progress, handle errors with
actionable intelligence.

## Acceptance Criteria

- AC-10.1: On "Place Order" click: save recipe → compile → deploy. Show progress stepper.
- AC-10.2: Progress stages (pizza metaphor):
  - "Taking your order..." (saving recipe)
  - "Prepping ingredients..." (compiling manifests)
  - "Firing up the oven..." (applying to cluster / docker compose up)
  - "Checking the crust..." (health waiting)
  - "Order served!" (deploy complete) or "Order burned!" (deploy failed)
- AC-10.3: Order number assigned and displayed: "Order #d-20260317-001"
- AC-10.4: On success: "Your order has been served!" with links to status page and "Place another order" button.
- AC-10.5: On failure: contextual error with fix suggestion:
  - Image not found → "We can't find that ingredient. Check the image name in your alias." + link back to alias.
  - Permission denied → "The kitchen doesn't have permission to cook this. Your cluster needs [specific RBAC]."
  - Cluster unreachable → "Can't reach the kitchen. Check your cluster connection." + link to /app/clusters.
  - Timeout → "Order is taking too long to prepare. The oven might be overloaded." + retry button.
- AC-10.6: "Try Again" button on failure. "Edit Order" button drops back to review step with error context.

## Deploy Error Intelligence (FR-6)

- Parse common K8s errors into human-friendly messages
- Map errors to wizard steps (image error → alias step, RBAC → cluster page)
- Provide actionable fix suggestions, not just error text

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepDeployProgress.tsx` | Step 9 component — wraps DeployProgress with pizza metaphor |
| `ui/src/lib/deploy-error-intelligence.ts` | Error parser: K8s errors → human-friendly pizza messages |

## Component Props

```typescript
interface StepDeployProgressProps {
  recipeSource: string;
  recipeName: string;
  target: WizardTarget;
  teamId: string;
  clusterId: string;
  saveRecipe: boolean;
  onRetry: () => void;
  onEditOrder: () => void;
  onNewOrder: () => void;
}
```

## Deploy Flow

1. Save recipe (if `saveRecipe` toggle ON) → `api.createRecipe()`
2. If not saving, use existing recipe or create a temporary one
3. Trigger deploy → `api.triggerDeploy(recipeId, teamId, target, "production", clusterId)`
4. Stream logs via `api.streamDeploymentLogs(deploymentId)`
5. Map log lines to pizza metaphor stages
6. Show final status

## Pizza Metaphor Stage Mapping

Map existing deploy log patterns to pizza stages:
- "Step 1/5: Parsing recipe" → "Taking your order..."
- "Step 2/5: Building plan" → "Prepping ingredients..."
- "Step 3/5: Validating plan" → "Prepping ingredients..."
- "Step 4/5: Applying" → "Firing up the oven..."
- "Step 5/5: Waiting for pods" → "Checking the crust..."
- Success → "Order served! 🍕"
- Failure → "Order burned!"

## Error Intelligence

```typescript
interface DeployError {
  pattern: RegExp;
  title: string;       // Pizza metaphor title
  message: string;     // Human-friendly explanation
  action: { label: string; href?: string; onClick?: () => void };
}
```

Error mapping:
- `ImagePullBackOff` / `ErrImagePull` → "Can't find that ingredient"
- `Forbidden` / `RBAC` → "Kitchen doesn't have permission"
- `connection refused` / `unreachable` → "Can't reach the kitchen"
- `deadline exceeded` / `timeout` → "Order taking too long"

## UI Elements

1. **Progress stepper** — vertical steps with pizza stage names
2. **Log display** — terminal-style streaming logs
3. **Order number** — displayed prominently when deploy starts
4. **Success card** — "Order served!" with app URL link + "Place Another Order" button
5. **Failure card** — error title, human-friendly message, action buttons (Try Again, Edit Order)
