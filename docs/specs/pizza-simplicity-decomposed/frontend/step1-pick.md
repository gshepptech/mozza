---
domain: frontend
file: step1-pick
depends_on:
  - foundation/types-and-engine
  - foundation/deprecate-old-paths
estimated_complexity: high
---

## Purpose

Build Step 1 of the fast lane: "What do you want to run?" — a tabbed interface combining alias catalog, template catalog, and GitHub import into one unified picking experience.

## Scope

**Included:**
- Redesign OrderWizard Step 1 as a tabbed interface with 3 tabs
- Alias catalog tab: searchable card grid of saved aliases
- Template catalog tab: embed existing TemplateCatalog inline
- GitHub import tab: embed existing GitHubImport inline
- New 3-step indicator (Pick → Customize → Order)
- Empty state for alias catalog
- Inline alias creation (existing functionality, restyled)

**Excluded:**
- Step 2 and Step 3 UI (handled by other domain files)
- Backend alias/template API changes
- Recipe builder / advanced mode

## Requirements

- REQ-1: OrderWizard renders a 3-step indicator at top: "Pick" → "Customize" → "Order" as pills/dots. Step 1 is active. Replace the existing 8-step `StepIndicator`.
- REQ-2: Step 1 title: **"What do you want to run?"**
- REQ-3: Below the title, three tabs styled as pills or underlined text:
  - **"Your menu"** (default selected) — alias catalog
  - **"Classics"** — template catalog
  - **"From a repo"** — GitHub import
- REQ-4: **"Your menu" tab** displays aliases as a grid of cards (2-3 columns on desktop, 1 on mobile). Each card shows:
  - A circular icon with the first letter of the alias name (colored based on hash of name) as fallback, or a logo if available
  - Alias friendly name (bold, prominent)
  - The image reference in small muted text below (e.g., `ghcr.io/myorg/myapp:v1.2.3`)
  - Selected state: orange border glow, checkmark badge
- REQ-5: A search input above the grid filters aliases by name (client-side filter)
- REQ-6: An "Add new" card at the end of the grid opens the inline alias creation form (name + image URL), same as existing StepPlaceOrder inline creation but styled as a card
- REQ-7: **Empty state** when no aliases exist: "Your menu is empty" with subtext "Add your first app above, or pick from our classics →" with the "classics" word as a clickable link that switches to the template tab
- REQ-8: **"Classics" tab** renders TemplateCatalog component inline (import and embed it). When a template is selected, wizard state captures `source: "template"` and `selectedTemplateId`. Category filter pills shown horizontally above the grid.
- REQ-9: **"From a repo" tab** renders GitHubImport component inline. When import completes, wizard state captures `source: "github"` and the generated recipe data.
- REQ-10: Selecting any item (alias, template, or completing GitHub import) enables the "Next →" button at the bottom.
- REQ-11: **"I know what I'm doing → Advanced mode"** link at the very bottom, calls `onAdvancedMode` prop.
- REQ-12: The step indicator, tab interface, and "Next" button all use the existing design system: orange (#ff6b35) for active/selected states, dark backgrounds, rounded corners, muted text for secondary info.

## Explicit Behaviors

- Clicking an alias card selects it (orange border). Clicking again deselects. Only one alias can be selected at a time.
- Switching tabs does NOT clear the selection from another tab. But only the most recent selection counts (if user picks an alias, then switches to classics and picks a template, the template wins).
- The alias search is instant/debounced client-side filtering — no API calls on keystroke.
- Aliases are fetched via `listAliases(teamId)` on mount (existing API call from StepPlaceOrder).
- Templates are fetched via `listTemplates()` on mount (existing API call from TemplateCatalog).
- The "Next →" button is disabled until something is selected.
- Tab state is preserved if user goes back from Step 2.

## Dependencies

- `foundation/types-and-engine` — `FastLaneSource` type, `FAST_LANE_STEPS` definitions
- `foundation/deprecate-old-paths` — OrderWizard is the direct entry (no SourceSelector gate)

## Interfaces

**Consumes:**
```typescript
// Existing APIs
listAliases(teamId: string): Promise<ImageAlias[]>
listTemplates(category?: string): Promise<{ templates: Template[] }>

// New types from foundation
type FastLaneSource = "alias" | "template" | "github";
```

**Produces (via wizard state update):**
```typescript
{
  source: FastLaneSource;
  selectedAliasId: string | null;
  selectedTemplateId: string | null;
  aliasName: string;      // friendly name for Step 2 app name pre-fill
  aliasImage: string;     // image reference for recipe generation
}
```

## Constraints

- Must work with existing alias API (no backend changes)
- Must work with existing template API (no backend changes)
- Must embed TemplateCatalog and GitHubImport as children, not fork their code
- Grid must be responsive (2-3 cols desktop, 1 col mobile)

## Edge Cases

- Zero aliases AND zero templates → both tabs show empty states, "From a repo" is the only viable path
- Alias with very long name or image reference → truncate with ellipsis
- Template API fails → show error state in classics tab, other tabs unaffected
- User selects alias, goes to Step 2, comes back → alias should still be selected
- Team has 50+ aliases → grid should scroll, search becomes important

## Acceptance Criteria

- [ ] Step 1 shows "What do you want to run?" title
- [ ] 3-step indicator shows "Pick → Customize → Order" with Step 1 active
- [ ] Three tabs: "Your menu" (default), "Classics", "From a repo"
- [ ] Alias grid shows cards with name, icon, image reference
- [ ] Search input filters aliases by name
- [ ] Empty state shows when no aliases, with link to classics tab
- [ ] Selecting an alias enables "Next" button
- [ ] Selecting a template enables "Next" button
- [ ] Completing GitHub import enables "Next" button
- [ ] "Advanced mode" link at bottom works
- [ ] Tab state preserved on back navigation from Step 2

## Definition of Done

Step 1 renders as a tabbed interface with alias grid, template catalog, and GitHub import. User can pick from any source and proceed to Step 2. No SourceSelector gate.

## Related Files

- `frontend/step2-customize.md` — receives the selection from Step 1
- `frontend/step3-review.md` — shows what was picked

## Testing Strategy

- Visual: navigate to deploy → verify 3-step indicator, tab interface, alias grid
- Interaction: select alias → verify Next enabled, deselect → verify Next disabled
- Interaction: switch to Classics → select template → verify Next enabled
- Empty state: test with team that has no aliases
- Search: type in search → verify grid filters
- Navigation: select alias → Next → Back → verify alias still selected
