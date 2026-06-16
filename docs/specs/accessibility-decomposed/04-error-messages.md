# Domain 4: Error Messages

**Wave:** 4 (parallel with Domains 5 and 6)
**Gaps covered:** F1–F6 (6 error messages exposing K8s internals)
**Depends on:** Domain 8 (glossary for expandable technical details)

## Purpose

Every error message should answer three questions:
1. What happened? (in plain English)
2. Why did it happen? (likely cause)
3. What should I do? (specific next step)

No Kubernetes term should appear in the primary error message. Technical details
go in an expandable "For your developer" section.

## Requirements

### FR-4.1: ImagePullBackOff → "Can't find your app package" (F1)

**Current:**
```
title: "Can't find that ingredient"
message: "We couldn't pull the container image. Check that the image name is correct
and that the registry is accessible."
```

**Change:**
```
title: "Can't find your app"
message: "Mozza couldn't download the app package you specified."

What to do:
  • Double-check the app package URL you entered
  • Make sure the package exists and is publicly accessible
  • If it's a private package, check that your credentials are set up

▸ Technical details (for your developer)
  Error: ImagePullBackOff — the container runtime couldn't pull the specified
  image from the registry. Verify the image ref, tag, and pull secrets.
```

**Files:** deploy-error-intelligence.ts:20-28

### FR-4.2: RBAC/Permission → "Permission denied" (F2)

**Current:**
```
title: "The kitchen doesn't have permission"
message: "The deployment was rejected due to insufficient permissions.
Check that the service account has the required RBAC roles."
```

**Change:**
```
title: "Permission denied"
message: "Mozza doesn't have permission to deploy to this environment."

What to do:
  • Ask your system administrator to grant deployment access
  • Share this error with them — they'll know what to do

▸ Technical details (for your developer)
  RBAC error — the service account used by Mozza lacks the required
  ClusterRole/Role bindings for this namespace. Check RBAC policies.
```

**Files:** deploy-error-intelligence.ts:30-38

### FR-4.3: CrashLoopBackOff → "Your app crashed on startup" (F3)

**Current:**
```
title: "The oven keeps shutting off"
message: "Your container is crashing on startup. Check the app logs
for errors, ensure the health check endpoint is correct, and verify
the app starts without errors locally."
```

**Change:**
```
title: "Your app crashed on startup"
message: "Mozza started your app but it immediately stopped. This usually
means there's an error in your app's code or configuration."

What to do:
  • Check your app's logs (click "View Logs" below)
  • Make sure your app runs without errors on your computer first
  • Check that all required settings (environment variables) are set
  • If you set a health check, make sure the URL path exists

▸ Technical details (for your developer)
  CrashLoopBackOff — the container starts, exits with non-zero, and
  Kubernetes restarts it in a back-off loop. Check: entrypoint, env
  vars, DB connectivity, health probe path.
```

**Files:** deploy-error-intelligence.ts:68-78

### FR-4.4: OOMKilled → "Your app ran out of memory" (F4)

**Current:** Lumped into the CrashLoopBackOff pattern.

**Change:** Add as separate pattern:
```
pattern: /OOMKilled/i

title: "Your app ran out of memory"
message: "Your app tried to use more memory than was allocated to it."

What to do:
  • Try a larger resource size (go back and pick L or XL in Customize)
  • If your app processes large files or datasets, it may need even more
  • Check for memory leaks if this happens repeatedly

▸ Technical details (for your developer)
  OOMKilled — the container exceeded its memory limit and was terminated
  by the kernel OOM killer. Current limit: {memoryLimit}. Increase
  resources.limits.memory or optimize application memory usage.
```

**Files:** deploy-error-intelligence.ts (new pattern, between crash and quota)

### FR-4.5: Resource Quota → "Not enough room" (F5)

**Current:**
```
title: "The kitchen is full"
message: "Your cluster doesn't have enough resources (CPU or memory)
to run this. Try reducing replicas or resource requests."
```

**Change:**
```
title: "Not enough room"
message: "The system doesn't have enough space to run your app right now."

What to do:
  • Try a smaller resource size (S or M instead of L or XL)
  • Reduce the number of copies to 1 or 2
  • Ask your admin if more capacity can be added

▸ Technical details (for your developer)
  Resource quota exceeded or insufficient cluster resources. The
  requested CPU/memory exceeds available capacity. Options: reduce
  requests/limits, scale down replicas, add nodes, or adjust
  ResourceQuota/LimitRange.
```

**Files:** deploy-error-intelligence.ts:58-66

### FR-4.6: Timeout → "Taking too long" (F6)

**Current:**
```
title: "Order is taking too long"
message: "The deployment is taking too long to become ready. The cluster
might be overloaded or the containers might be failing to start."
```

**Change:**
```
title: "Taking too long"
message: "Your app hasn't started within the expected time."

What to do:
  • Check your app's logs for errors (click "View Logs")
  • Your app might need more time to start — this is common for large apps
  • The system might be busy — try again in a few minutes
  • If this keeps happening, try a larger resource size

▸ Technical details (for your developer)
  Deployment readiness timeout. Pods are not reaching Ready state
  within the configured timeout. Check: readiness probes, resource
  contention, image pull times, init containers, dependency availability.
```

**Files:** deploy-error-intelligence.ts:48-56

### FR-4.7: Error Display Component Update

**Where:** StepDeployProgress.tsx, DeployProgress.tsx

Update the error display to use the new 3-part structure:

```tsx
<div className="error-card">
  <h3>{error.title}</h3>
  <p>{error.message}</p>

  <div className="what-to-do">
    <h4>What to do:</h4>
    <ul>{error.steps.map(s => <li>{s}</li>)}</ul>
  </div>

  <details className="technical-details">
    <summary>Technical details (for your developer)</summary>
    <pre>{error.technicalDetail}</pre>
  </details>
</div>
```

**Files:** StepDeployProgress.tsx, DeployProgress.tsx, deploy-error-intelligence.ts (type update)

## Acceptance Criteria

- [ ] All 6 error patterns have 3-part structure (what/why/what-to-do)
- [ ] No Kubernetes term in primary title or message
- [ ] Technical details in expandable section only
- [ ] OOMKilled is a separate pattern (not lumped into CrashLoop)
- [ ] Error display component uses new structure
- [ ] "View Logs" button is included in relevant error cards
- [ ] Every "What to do" has at least 2 actionable steps

## Observable Truths

- OT-1: A non-engineer who sees ImagePullBackOff understands they entered a wrong URL
- OT-2: A non-engineer who sees RBAC errors knows to contact their admin
- OT-3: A non-engineer who sees OOMKilled knows to pick a larger size
- OT-4: Technical details are hidden by default, accessible to developers
- OT-5: Every error has a "What to do" section with specific next steps
