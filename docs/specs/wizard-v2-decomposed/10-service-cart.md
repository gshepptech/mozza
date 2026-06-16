# Domain: Service Cart (Sidebar)

**Source spec:** docs/specs/wizard-v2.md — US-8, FR-4

## Purpose

Left sidebar showing all services in the current order. Supports multi-service
stacks with add/remove/edit.

## Acceptance Criteria

- AC-8.1: Left sidebar shows all services as cards: name, type icon, replica count.
- AC-8.2: Active service is highlighted. Click a service to edit it.
- AC-8.3: "+" button at the bottom: "Add another item to your order". Clicking starts a new service interview from US-1 (alias + count, skipping target since it's already set).
- AC-8.4: Dependency slices (auto-added DB, cache) shown with a "side" badge and lighter styling.
- AC-8.5: Delete button (trash icon) on each service. Deleting a service removes its dependencies too (with confirmation).
- AC-8.6: Cart header shows total services: "Your order (3 items)"
- AC-8.7: Sidebar is always visible during steps 2-7. Hidden during step 1 (first service not yet started) and visible but read-only during steps 8-9 (review + deploy).
- AC-8.8: For single-service orders, sidebar is collapsed (just a thin strip showing the one service) to maximize interview area.

## Multi-Service Support (FR-4)

- Sidebar cart tracks all services in the current order
- Each service has independent interview state
- Dependencies are shared across services (one DB serves all that need it)
- Recipe generation produces a single recipe with all services

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/ServiceCart.tsx` | Sidebar cart component |

## Component Props

```typescript
interface ServiceCartProps {
  services: ServiceInterviewState[];
  activeServiceIndex: number;
  currentStep: number;
  onSelectService: (index: number) => void;
  onAddService: () => void;
  onDeleteService: (index: number) => void;
  readOnly: boolean;  // true during steps 8-9
}
```

## UI Elements

1. **Cart header** — "Your order (N items)"
   - Total count includes main services only (not dependency "sides")

2. **Service cards** — one per service
   - Name (alias name)
   - Type icon (based on workload type or traits)
   - Replica count: "×3"
   - Active service: highlighted border
   - Click to edit (navigates to that service's current step)
   - Delete button (trash icon) with confirmation dialog

3. **Dependency "side" items** — lighter styling
   - Shown with "side" badge
   - Auto-added DB/cache shown with engine icon
   - Not directly clickable (editing via parent service's Step 5)

4. **Add button** — "+ Add another item to your order"
   - At bottom of cart
   - Hidden in read-only mode (steps 8-9)

5. **Collapsed mode** — for single-service orders
   - Thin strip showing service name + icon
   - Expands on hover or click

## Visibility Rules

| Step | Sidebar State |
|------|--------------|
| 1 (first service) | Hidden |
| 1 (additional service) | Visible |
| 2-7 | Visible |
| 8-9 | Visible, read-only |
