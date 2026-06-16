---
domain: frontend
file: landing-page
depends_on: []
estimated_complexity: high
---

## Purpose

Complete redesign of the Mozza landing page with premium pizza brand identity, custom SVGs, Stripe-level design quality. Dark theme, warm color palette, terminal animations, responsive.

## Scope

**Included:**
- Full landing page redesign (keep existing logo and slogan)
- Custom SVG illustrations throughout (no emojis, no stock icons)
- Sections: Hero with demo, How it works, Feature showcase, Recipe comparison, Marketplace preview, Community, Install CTA
- Terminal typing effect animations for CLI demos
- Responsive: desktop, tablet, mobile
- Static hosting ready (GitHub Pages)

**Excluded:**
- Logo redesign (keep existing)
- Domain acquisition (manual)
- GitHub Pages deployment configuration (manual)
- Custom mascot (defer — too design-heavy)

## Requirements

- REQ-1: Keep existing logo and "Deploy like you're ordering pizza" slogan
- REQ-2: Everything else redesigned from scratch
- REQ-3: Premium pizza brand identity — playful but professional
- REQ-4: Custom SVG illustrations throughout — no emojis, no stock icons, no pre-existing assets
- REQ-5: Sections: Hero, How it works, Feature showcase, Recipe comparison, Marketplace preview, Community, Install CTA
- REQ-6: Warm color palette anchored on existing orange (#ff6b35) with sophisticated typography
- REQ-7: Responsive: desktop, tablet, mobile
- REQ-8: Dark theme (#0a0a0a background or similar)
- REQ-9: Animations: subtle, purposeful — terminal typing effect for CLI demos
- REQ-10: Performance: < 3 second load, no external dependencies, static hosting ready
- REQ-11: Hosted on GitHub Pages

## Explicit Behaviors

- Hero: large headline + subhead, terminal animation showing `mozza init → mozza up`, CTA button "Get Started"
- How it works: 4-step visual flow with custom SVGs — Recipe → Detect → Build → Deploy → Monitor
- Feature showcase: alternating left/right sections with screenshots and descriptions
- Recipe comparison: side-by-side K8s YAML (200 lines, scrollable) vs Mozza recipe (10 lines, highlighted)
- Marketplace preview: 4-6 recipe cards with "Browse All" link
- Community: Discord invite + GitHub link + contributor count
- Install CTA: terminal block with `go install github.com/gshepptech/mozza/cmd/mozza@latest` and copy button
- Typography: Inter or similar modern sans-serif, multiple weights
- Color palette: primary orange #ff6b35, dark bg #0a0a0a, text #e0e0e0, accent variations
- Terminal typing animation: custom implementation using requestAnimationFrame, no library
- Animations trigger on scroll (Intersection Observer), play once
- Page built as React component within the existing UI (or standalone HTML — TBD based on hosting)
- No runtime JS dependencies beyond React (inline styles, no Tailwind)

## Dependencies

None — can be developed independently.

## Interfaces

**Output:**
- Static HTML/CSS/JS page (or React component)
- Custom SVG files in assets directory

## Constraints

- < 3 second load on 3G connection (NFR-7)
- No external dependencies (fonts loaded from local files)
- No emojis anywhere
- No stock icons or pre-existing illustration assets
- Static hosting compatible (no server-side rendering needed for landing page)

## Edge Cases

- Very slow connection → progressive loading, critical CSS inline
- Screen reader → proper ARIA labels and semantic HTML
- Print stylesheet → basic print-friendly version
- JavaScript disabled → content still visible (progressive enhancement)

## Acceptance Criteria

- [ ] New landing page with all 7 sections
- [ ] Custom SVG illustrations (no emojis, no stock)
- [ ] Dark theme with orange accent palette
- [ ] Terminal typing animation for CLI demos
- [ ] Recipe comparison section (K8s vs Mozza)
- [ ] Responsive on desktop, tablet, mobile
- [ ] Loads in < 3 seconds
- [ ] No external dependencies
- [ ] Copy button for install command works

## Definition of Done

Landing page is visually premium, responsive, fast-loading, and communicates Mozza's value proposition clearly. All sections present with custom illustrations.

## Related Files

- docs/readme-community.md (README links to landing page)

## Testing Strategy

- Visual review on desktop, tablet, mobile viewports
- Lighthouse performance audit (target > 90)
- Accessibility audit (ARIA, contrast ratios)
- Test terminal animation plays correctly
- Test copy button functionality
