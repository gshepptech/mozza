# Domain 7: Deployment Tracking UX

**Wave:** 5 (last — depends on everything else)
**Gaps covered:** E5 (status legend), E6 (post-deploy), E7 (recovery)
**Depends on:** Domains 3 (onboarding), 4 (error messages)

## Purpose

After deploying, the user needs to understand what happened, what's happening now,
and what they can do about it. The deployment tracking page should be as accessible
as the wizard.

## Requirements

### FR-7.1: Status Legend (E5, expanded)

**Where:** DeploymentsPage.tsx

Partially covered in Domain 3 (FR-3.5). This adds detail.

**Add status legend as a permanent, dismissable help card at the top of the page:**

```
Understanding your deployments:

🟢 Succeeded — Your app is live and working
🔵 Running   — Deploy in progress, almost there
🟡 Pending   — Waiting in line to start
🟠 Partial   — Some parts are running, some aren't (check details)
🔴 Failed    — Something went wrong (click for details)
⏪ Rolled Back — We restored your previous version
```

- Shown by default for users with <3 total deployments
- Dismissable with "Got it" button (remembers via localStorage)
- Always accessible via "?" icon in page header

**Files:** DeploymentsPage.tsx

### FR-7.2: Deployment Detail Enrichment

**Where:** DeploymentsPage.tsx (expanded deployment view)

**Current:** Shows log, status badge, environment, recipe. Sparse for non-engineers.

**Change — add plain-English summary at top of expanded view:**

For succeeded:
```
✓ Deployed successfully in {elapsed}

Your app "{name}" is running with {replica_count} copy/copies.
Access it at: {access_url}

Last deployed: {timestamp} by {user}
```

For failed:
```
✗ Deploy failed after {elapsed}

{error_title} — {error_message}
[What to do] [View Logs] [Edit & Retry]
```

For running:
```
⟳ Deploying now... ({elapsed} so far)

Step {current}/{total}: {current_step_description}
```

For rolled back:
```
⏪ Rolled back to previous version

The deploy at {timestamp} was unsuccessful, so we restored the
version that was running before. Your app is back to normal.

[View what changed] [Try again with edits]
```

**Files:** DeploymentsPage.tsx

### FR-7.3: Deployment Actions Clarity

**Where:** DeploymentsPage.tsx (action buttons on each deployment)

**Current:** Buttons: Refresh, Rollback, Delete with minimal context.

**Change:**
- **Rollback** → "Undo this deploy" with confirmation: "This will restore the version that was running before this deploy. Your app will briefly restart."
- **Delete** → "Remove from history" with confirmation: "This removes the deployment record. It does NOT stop your running app."
- **Refresh** → Keep as-is (icon button)
- Add **"View Logs"** button that links to log viewer
- Add **"Edit & Redeploy"** button on failed deployments (from Domain 3, FR-3.7)
- Tooltip on each button explaining what it does

**Files:** DeploymentsPage.tsx

### FR-7.4: Empty Deployments Page

**Where:** DeploymentsPage.tsx

**Current state unknown — check if there's an empty state.**

**Add if missing:**
```
No deployments yet

You haven't deployed anything yet. Head to the Deploy page to get started.

[Deploy your first app →]
```

**Files:** DeploymentsPage.tsx

## Acceptance Criteria

- [ ] Status legend shown by default for new users (<3 deployments)
- [ ] Status legend dismissable and re-accessible via ? icon
- [ ] Every deployment detail view has a plain-English summary
- [ ] Failed deployments have "What to do" section + "Edit & Retry"
- [ ] Rolled back deployments explain what happened and offer next steps
- [ ] Rollback button says "Undo this deploy" with confirmation dialog
- [ ] Delete button clarifies it removes history, not the running app
- [ ] Empty deployments page has CTA to deploy

## Observable Truths

- OT-1: A user with their first deployment understands all status colors without guessing
- OT-2: A user whose deploy failed can see exactly what went wrong and retry with edits
- OT-3: A user clicking "Rollback" understands they're restoring the previous version
- OT-4: A user clicking "Delete" understands their running app won't be affected
- OT-5: A user with no deployments has a clear path to their first deploy
