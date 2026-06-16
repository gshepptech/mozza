# Domain: Step 7 — Anything Else Before We Cook?

**Source spec:** docs/specs/wizard-v2.md — US-7

## Purpose

Review and optionally customize health checks, resource limits, and scaling.
Collapsed by default showing auto-set defaults.

## Acceptance Criteria

- AC-7.1: Summary card showing auto-set defaults based on workload type:
  - Health check: HTTP GET /health (web-facing) or TCP port check (others)
  - Resources: CPU 250m-500m, Memory 128Mi-256Mi (web), 500m-1cpu, 256Mi-512Mi (DB)
  - Scaling: OFF by default. If ON: 1-5 replicas, 80% CPU target.
- AC-7.2: Each section is expandable — collapsed by default showing the summary, click to customize.
- AC-7.3: Pre-filled values match workload type. User only changes what they want.
- AC-7.4: Step title: "Anything else before we cook?"
- AC-7.5: For Local target: skip health check and scaling sections (not applicable to Docker Compose). Show only resource limits.

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/components/deploy/order-wizard/StepAnythingElse.tsx` | Step 7 component |

## Types

```typescript
interface HealthCheckDefaults {
  enabled: boolean;
  type: "http" | "tcp";
  path: string;         // "/health" for HTTP
  port: number;
  intervalSeconds: number;
}

interface ResourceDefaults {
  cpuRequest: string;   // "250m"
  cpuLimit: string;     // "500m"
  memoryRequest: string; // "128Mi"
  memoryLimit: string;   // "256Mi"
}

interface ScalingDefaults {
  enabled: boolean;
  minReplicas: number;  // 1
  maxReplicas: number;  // 5
  cpuTarget: number;    // 80
}
```

## Component Props

```typescript
interface StepAnythingElseProps {
  traits: Trait[];
  target: WizardTarget;
  workloadType: WorkloadType;
  port: number;
  healthCheck: HealthCheckDefaults;
  resources: ResourceDefaults;
  scaling: ScalingDefaults;
  onUpdate: (updates: Partial<ServiceInterviewState>) => void;
}
```

## UI Elements

1. **Health Check section** (K8s only) — "How should we check the crust?"
   - Collapsed: "HTTP GET /health every 10s" or "TCP port 8080 every 10s"
   - Expanded: type selector (HTTP/TCP), path input, port, interval
   - Pre-filled: HTTP /health for web-facing, TCP port for others

2. **Resources section** — "Oven temperature"
   - Collapsed: "CPU: 250m-500m, Memory: 128Mi-256Mi"
   - Expanded: CPU request/limit inputs, Memory request/limit inputs
   - Pre-filled based on workload type

3. **Scaling section** (K8s only) — "Need more ovens?"
   - Collapsed: "Off" or "1-5 replicas at 80% CPU"
   - Expanded: enable toggle, min/max replicas, CPU target percentage
   - Default: OFF

## Default Inference

| Workload | CPU | Memory | Health |
|----------|-----|--------|--------|
| Web/API  | 250m-500m | 128Mi-256Mi | HTTP /health |
| Database | 500m-1cpu | 256Mi-512Mi | TCP port |
| Worker   | 250m-500m | 128Mi-256Mi | TCP port |
| Cache    | 250m-500m | 128Mi-256Mi | TCP port |

## Validation

Resource values must be valid K8s resource formats. Health check path must start with "/" for HTTP type.
