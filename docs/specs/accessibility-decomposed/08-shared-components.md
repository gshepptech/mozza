# Domain 8: Shared Components (Foundation)

**Wave:** 1 (build first — all other domains depend on this)
**Files:** New components + glossary

## Purpose

Before adding help text to 50+ places, build the reusable components and centralized
content that every other domain will use.

## Requirements

### FR-8.1: Glossary Module (`ui/src/lib/glossary.ts`)

A single file mapping every technical term to a plain-English definition. Every tooltip
and inline help pulls from this — one place to update, consistent language everywhere.

```typescript
export const glossary: Record<string, { term: string; plain: string; example?: string }> = {
  "container-image": {
    term: "Container image",
    plain: "A packaged version of your app that's ready to run. Think of it like a zip file that contains your app and everything it needs.",
    example: "ghcr.io/mycompany/my-api:v1.2.0",
  },
  "port": {
    term: "Port",
    plain: "A numbered door on your app that traffic comes through. Your app picks a door number when it starts up — we need to know which one.",
    example: "A Node.js app usually uses 3000. A Go API usually uses 8080.",
  },
  // ... every term from gaps A1–A18
};
```

**Required entries (minimum):**
- container-image, port, replica, cluster, pod, environment-variable, secret,
  health-check, http-vs-tcp, cpu-memory, auto-scaling, storage-gi, database-url,
  cache-redis, docker-compose, helm, dockerfile, registry, service-account, rbac,
  namespace, node, statefulset, replicaset, daemonset, cronjob, domain, ingress

**Rules:**
- `plain` must be understandable by a 14-year-old
- `plain` must be under 200 characters
- `example` is optional but strongly preferred
- No term may reference another unexplained term in its definition

### FR-8.2: HelpTooltip Component (`ui/src/components/ui/HelpTooltip.tsx`)

A `?` icon that shows a tooltip on hover/click with the glossary definition.

```tsx
<HelpTooltip term="port" />
// Renders: (?) icon → hover → "Port: A numbered door on your app..."
```

**Behavior:**
- Renders a small `?` circle icon (14px, muted color)
- On hover (desktop): shows tooltip with `plain` text + optional `example`
- On click (mobile): shows tooltip, click again to dismiss
- Tooltip max-width 280px, dark background, light text
- Positioned via Radix Popover (already in deps) or CSS
- Does NOT block interaction with adjacent elements
- Accessible: `aria-label`, `role="tooltip"`, keyboard focusable

### FR-8.3: InlineHelp Component (`ui/src/components/ui/InlineHelp.tsx`)

A small expandable text block for longer explanations below a field.

```tsx
<InlineHelp>
  Your app listens on a specific port number when it starts. If you're not sure,
  check your app's config file or ask whoever built it. Common defaults: Node.js
  uses 3000, Python Flask uses 5000, Go uses 8080.
</InlineHelp>
```

**Behavior:**
- Collapsed by default, shows "Learn more" link
- Expands to show help text (max 3 lines recommended)
- Collapse animation (200ms ease)
- Text styled as `text-xs text-muted-foreground`
- No icon — the link IS the affordance

### FR-8.4: Field Help Module (`ui/src/lib/field-help.ts`)

Per-field help text and examples, separate from the glossary (which is per-concept).

```typescript
export const fieldHelp: Record<string, {
  label: string;        // Plain-English field label
  hint?: string;        // One-line hint below the field
  help?: string;        // Expandable "Learn more" text
  placeholder?: string; // Example value
  examples?: string[];  // Multiple examples
}> = {
  "port": {
    label: "Port number",
    hint: "The door number your app uses for traffic",
    help: "Your app listens on a specific port when it starts. Common defaults: Node.js uses 3000, Python uses 5000, Go uses 8080. Check your app's Dockerfile or startup command if unsure.",
    placeholder: "3000",
    examples: ["3000 (Node.js)", "8080 (Go, Java)", "5000 (Python Flask)", "4000 (Elixir Phoenix)"],
  },
  // ... every field from gaps C1–C10
};
```

## Acceptance Criteria

- [ ] `glossary.ts` has entries for all 28+ terms listed above
- [ ] Every glossary entry's `plain` text passes a readability check (no nested jargon)
- [ ] `HelpTooltip` renders on hover and click, dismisses correctly
- [ ] `HelpTooltip` is keyboard accessible (Tab → Enter to open)
- [ ] `InlineHelp` expands and collapses with animation
- [ ] `fieldHelp` has entries for all 10 fields from domain 02
- [ ] Components use existing design system colors (brand orange, muted foreground)
- [ ] No new dependencies added (use Radix Popover already in deps)

## Observable Truths

- OT-1: A developer can import `HelpTooltip` and pass any glossary term to get a working tooltip
- OT-2: A developer can import `InlineHelp` to wrap any help text and get expand/collapse behavior
- OT-3: All glossary definitions are understandable without prior tech knowledge
- OT-4: Tooltips work on mobile (tap to open, tap elsewhere to close)
- OT-5: Screen readers announce tooltip content when focused
