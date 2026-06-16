# Domain: Interview Engine

**Source spec:** docs/specs/wizard-v2.md — FR-1, FR-2

## Purpose

Core state machine that drives the pizza-ordering wizard. Manages question sequencing,
skip logic, back navigation, and per-service interview state. All step components
consume this engine.

## Requirements

### Interview Engine (FR-1)
- Questions defined as a sequence with skip conditions
- Each question has: ID, title (pizza metaphor), traits required, target filter (local/k8s/both), pre-fill logic, validation
- Skip logic evaluates after each answer to determine next question
- Back navigation preserves all answers

### Recommendation Engine (FR-2)
- Maps trait combinations to workload types with confidence levels
- Generates human-readable reasoning for each recommendation
- Pre-selects recommended options in the UI
- Recommendations are overridable — user's choice always wins

### Inference Logic (AC-3.1)
- web-facing + NOT stateful → ReplicaSet (recommended)
- stateful (with or without web-facing) → StatefulSet (recommended)
- worker/batch only → ReplicaSet for long-running, CronJob if scheduled
- all three → StatefulSet (data trumps)

## File Plan

| File | Purpose |
|------|---------|
| `ui/src/lib/interview-engine.ts` | InterviewEngine class: question registry, sequencing, skip logic, state |
| `ui/src/lib/recommendation-engine.ts` | RecommendationEngine: trait→workload mapping, reasoning, confidence |
| `ui/src/lib/interview-types.ts` | Shared types: InterviewQuestion, InterviewState, WizardTarget, Trait, etc. |

## Types

```typescript
type WizardTarget = "local" | "kitchen";  // Local = Docker Compose, Kitchen = K8s
type Trait = "web-facing" | "stateful" | "worker";
type WorkloadType = "replicaset" | "statefulset" | "daemonset" | "cronjob";

interface InterviewQuestion {
  id: string;
  title: string;          // Pizza metaphor title
  stepNumber: number;
  targetFilter: "local" | "kitchen" | "both";
  skipWhen?: (state: ServiceInterviewState) => boolean;
  validate: (state: ServiceInterviewState) => boolean;
}

interface ServiceInterviewState {
  // Step 1
  aliasName: string;
  aliasImage: string;
  replicas: number;
  target: WizardTarget;
  // Step 2
  traits: Trait[];
  // Step 3
  workloadType: WorkloadType;
  workloadOverridden: boolean;
  // Step 4
  port: number;
  isPublic: boolean;
  domain: string;
  // Step 5
  dependencies: DependencyConfig[];
  // Step 6
  envVars: EnvVarEntry[];
  // Step 7
  healthCheck: HealthCheckDefaults;
  resources: ResourceDefaults;
  scaling: ScalingDefaults;
}

interface Recommendation {
  workloadType: WorkloadType;
  confidence: "high" | "medium";
  reasoning: string;      // Human-readable: "Because your app stores data..."
  explanation: string;     // Pizza metaphor explanation
}

interface WizardState {
  services: ServiceInterviewState[];
  activeServiceIndex: number;
  currentStep: number;
  target: WizardTarget;   // Set once, shared across services
}
```

## Behavior

### Skip Logic
- Step 2 (Traits): skip for Local target (AC-2.4)
- Step 3 (Workload Inference): skip for Local target (AC-3.7)
- Step 7 (Anything Else): for Local, skip health check and scaling sections (AC-7.5)

### Navigation
- `next()`: evaluate skip conditions, advance to next applicable step
- `back()`: return to previous applicable step, preserving answers
- `goToStep(n)`: jump to step N (for cart click-to-edit)
- `getCurrentStep()`: returns current InterviewQuestion

### Multi-Service
- Each service has independent `ServiceInterviewState`
- Target is shared (set in first service's Step 1)
- Adding a service starts from Step 1 but skips target selection
