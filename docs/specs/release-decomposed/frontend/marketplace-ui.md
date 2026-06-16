---
domain: frontend
file: marketplace-ui
depends_on:
  - backend/marketplace
estimated_complexity: medium
---

## Purpose

Implement the recipe marketplace dashboard UI: grid view with recipe cards, category filtering, search, recipe detail page with deploy button.

## Scope

**Included:**
- `ui/src/components/marketplace/RecipeCatalog.tsx` — grid view with search/filter
- `ui/src/components/marketplace/RecipeCard.tsx` — individual recipe card
- `ui/src/components/marketplace/RecipeDetail.tsx` — full recipe page with deploy
- Dashboard page: "Marketplace" / "Recipe Catalog"
- Category filter sidebar
- Search bar with instant results
- Popular/featured recipes section

**Excluded:**
- Backend marketplace logic (see backend/marketplace.md)
- Community contribution UI (post-launch)

## Requirements

- REQ-1: Dashboard has "Marketplace" / "Recipe Catalog" page
- REQ-2: Grid view with recipe cards: icon, name, description, category badge, one-click deploy
- REQ-3: Category filter sidebar
- REQ-4: Search bar with instant results
- REQ-5: Recipe detail page: full description, recipe preview (syntax highlighted), deploy button, source link
- REQ-6: "Deploy this recipe" launches deploy wizard with recipe pre-loaded
- REQ-7: Popular / featured recipes section at top
- REQ-8: Recipe cards use custom SVG icons per category (not emojis)

## Explicit Behaviors

- Grid layout: 3 columns on desktop, 2 on tablet, 1 on mobile
- Recipe cards: dark card with orange accent border on hover, category badge color-coded
- Category icons (SVG): Web Frameworks (globe), Databases (cylinder), Monitoring (chart), CMS (document), DevTools (wrench), Communication (chat bubble)
- Search: debounced (300ms), filters as you type, searches name + description + tags
- Category filter: toggleable sidebar buttons, multiple categories selectable
- Featured section: curated list (flag in recipe metadata), horizontal scroll on mobile
- Recipe detail: modal or dedicated page, shows full .mozza recipe with syntax highlighting
- "Deploy" on recipe detail → redirects to deploy wizard with recipe content pre-loaded
- Empty state: "No recipes found. Try a different search term or category."
- Loading state: skeleton cards during API fetch

## Dependencies

- backend/marketplace — API endpoints for recipe listing, search, detail

## Interfaces

**API (consumed):**
```
GET /api/v1/marketplace/recipes         — list with search/filter/pagination
GET /api/v1/marketplace/recipes/{name}  — recipe detail
POST /api/v1/marketplace/recipes/{name}/deploy — deploy recipe
```

**React Components:**
- `RecipeCatalog` — page component with grid, search, filters
- `RecipeCard` — card component
- `RecipeDetail` — detail modal/page
- Add route to existing router

## Constraints

- Follow existing UI design (dark theme, orange accent #ff6b35)
- Responsive: desktop, tablet, mobile
- No external icon libraries — custom SVGs only

## Edge Cases

- No recipes match search → show empty state
- API error → show error with retry button
- Very long recipe description → truncate with "Read more"
- Recipe with no icon → fallback to category icon

## Acceptance Criteria

- [ ] Marketplace page accessible from dashboard navigation
- [ ] Grid view with recipe cards
- [ ] Category filtering works
- [ ] Search returns instant results
- [ ] Recipe detail page shows full info + recipe preview
- [ ] "Deploy" button launches deploy wizard
- [ ] Featured/popular section at top
- [ ] Custom SVG icons per category
- [ ] Responsive on all screen sizes

## Definition of Done

Browse marketplace → filter by category → search → view recipe detail → deploy from marketplace. Full flow works with responsive design.

## Related Files

- backend/marketplace.md (API backend)
- frontend/compose-wizard.md (deploy wizard integration)

## Testing Strategy

- Component tests for cards, catalog, detail
- Test search debounce behavior
- Test category filter combinations
- Test responsive layout breakpoints
- Test deploy button integration
