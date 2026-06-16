import type {
  InterviewQuestion,
  ServiceInterviewState,
  WizardTarget,
  TraitCombo,
  ResourceSize,
  ResourceDefaults,
  HealthCheckDefaults,
  DependencyConfig,
  Trait,
  WorkloadType,
  EnvVarEntry,
} from "./interview-types";

// ─── Step Definitions ───────────────────────────────────────

const STEPS: InterviewQuestion[] = [
  {
    id: "place-order",
    stepNumber: 1,
    title: "What are you ordering?",
    targetFilter: "both",
  },
  {
    id: "traits",
    stepNumber: 2,
    title: "What\u2019s on the menu?",
    targetFilter: "kitchen",
    skipWhen: (_state, target) => target === "local",
  },
  {
    id: "workload-inference",
    stepNumber: 3,
    title: "How should we prepare it?",
    targetFilter: "kitchen",
    skipWhen: (_state, target) => target === "local",
  },
  {
    id: "networking",
    stepNumber: 4,
    title: "Where\u2019s the entrance?",
    targetFilter: "both",
  },
  {
    id: "dependencies",
    stepNumber: 5,
    title: "Any sides with that?",
    targetFilter: "both",
  },
  {
    id: "configuration",
    stepNumber: 6,
    title: "Special instructions",
    targetFilter: "both",
  },
  {
    id: "anything-else",
    stepNumber: 7,
    title: "Anything else before we cook?",
    targetFilter: "both",
  },
  {
    id: "review",
    stepNumber: 8,
    title: "Review your order",
    targetFilter: "both",
  },
  {
    id: "deploy",
    stepNumber: 9,
    title: "Order placed!",
    targetFilter: "both",
  },
];

// ─── Engine ─────────────────────────────────────────────────

/**
 * Returns the list of active (non-skipped) steps for a given
 * service state and target.
 */
export function getActiveSteps(
  state: ServiceInterviewState,
  target: WizardTarget,
): InterviewQuestion[] {
  return STEPS.filter((s) => !s.skipWhen?.(state, target));
}

/**
 * Given the current step number, find the next non-skipped step.
 * Returns undefined if we're at the end.
 */
export function getNextStep(
  currentStep: number,
  state: ServiceInterviewState,
  target: WizardTarget,
): InterviewQuestion | undefined {
  const active = getActiveSteps(state, target);
  const idx = active.findIndex((s) => s.stepNumber === currentStep);
  return idx >= 0 && idx < active.length - 1 ? active[idx + 1] : undefined;
}

/**
 * Given the current step number, find the previous non-skipped step.
 * Returns undefined if we're at the beginning.
 */
export function getPreviousStep(
  currentStep: number,
  state: ServiceInterviewState,
  target: WizardTarget,
): InterviewQuestion | undefined {
  const active = getActiveSteps(state, target);
  const idx = active.findIndex((s) => s.stepNumber === currentStep);
  return idx > 0 ? active[idx - 1] : undefined;
}

/**
 * Get the step definition by step number.
 */
export function getStepByNumber(stepNumber: number): InterviewQuestion | undefined {
  return STEPS.find((s) => s.stepNumber === stepNumber);
}

/**
 * Get all step definitions (including potentially skipped ones).
 */
export function getAllSteps(): InterviewQuestion[] {
  return [...STEPS];
}

/**
 * Check if a given step is the last interactive step before deploy.
 * Step 8 (review) is where "Place Order" lives.
 */
export function isReviewStep(stepNumber: number): boolean {
  return stepNumber === 8;
}

/**
 * Check if we're on the deploy progress step.
 */
export function isDeployStep(stepNumber: number): boolean {
  return stepNumber === 9;
}

/**
 * Get the button text for the "Next" action at a given step.
 */
export function getNextButtonText(stepNumber: number): string {
  if (stepNumber === 1) return "Start your order \u2192";
  if (stepNumber === 8) return "Place Order";
  return "Next \u2192";
}

/**
 * Total number of steps (including deploy).
 */
export const TOTAL_STEPS = STEPS.length;

// ─── Fast Lane ──────────────────────────────────────────────

export const FAST_LANE_STEPS: InterviewQuestion[] = [
  {
    id: "fl-pick",
    stepNumber: 1,
    title: "What do you want to run?",
    targetFilter: "both",
  },
  {
    id: "fl-customize",
    stepNumber: 2,
    title: "How do you want it?",
    targetFilter: "both",
  },
  {
    id: "fl-review",
    stepNumber: 3,
    title: "Review your order",
    targetFilter: "both",
  },
];

// ─── Combo Presets ──────────────────────────────────────────

export interface ComboPreset {
  traits: Trait[];
  port: number;
  isPublic: boolean;
  workloadType: WorkloadType;
  healthCheck: HealthCheckDefaults;
  dependencies: DependencyConfig[];
}

export const TRAIT_COMBO_PRESETS: Record<TraitCombo, ComboPreset | null> = {
  website: {
    traits: ["web-facing"],
    port: 3000,
    isPublic: true,
    workloadType: "replicaset",
    healthCheck: { enabled: true, type: "http", path: "/health", port: 3000, intervalSeconds: 10 },
    dependencies: [
      { type: "database", enabled: false, engine: "postgres", version: "16", storage: "10Gi" },
      { type: "cache", enabled: false, engine: "redis", version: "7", storage: "" },
    ],
  },
  "api-database": {
    traits: ["web-facing", "stateful"],
    port: 8080,
    isPublic: true,
    workloadType: "replicaset",
    healthCheck: { enabled: true, type: "http", path: "/health", port: 8080, intervalSeconds: 10 },
    dependencies: [
      { type: "database", enabled: true, engine: "postgres", version: "16", storage: "10Gi" },
      { type: "cache", enabled: false, engine: "redis", version: "7", storage: "" },
    ],
  },
  "background-worker": {
    traits: ["worker"],
    port: 0,
    isPublic: false,
    workloadType: "replicaset",
    healthCheck: { enabled: false, type: "http", path: "/health", port: 8080, intervalSeconds: 10 },
    dependencies: [
      { type: "database", enabled: false, engine: "postgres", version: "16", storage: "10Gi" },
      { type: "cache", enabled: false, engine: "redis", version: "7", storage: "" },
    ],
  },
  "database-only": {
    traits: ["stateful"],
    port: 5432,
    isPublic: false,
    workloadType: "statefulset",
    healthCheck: { enabled: false, type: "tcp", path: "", port: 5432, intervalSeconds: 10 },
    dependencies: [
      { type: "database", enabled: true, engine: "postgres", version: "16", storage: "10Gi" },
      { type: "cache", enabled: false, engine: "redis", version: "7", storage: "" },
    ],
  },
  custom: null,
};

// ─── Resource T-Shirt Sizes ─────────────────────────────────

export const RESOURCE_SIZE_MAP: Record<ResourceSize, ResourceDefaults> = {
  S: { cpuRequest: "100m", cpuLimit: "200m", memoryRequest: "64Mi", memoryLimit: "128Mi" },
  M: { cpuRequest: "250m", cpuLimit: "500m", memoryRequest: "128Mi", memoryLimit: "256Mi" },
  L: { cpuRequest: "500m", cpuLimit: "1000m", memoryRequest: "256Mi", memoryLimit: "512Mi" },
  XL: { cpuRequest: "1000m", cpuLimit: "2000m", memoryRequest: "512Mi", memoryLimit: "1024Mi" },
};

// ─── Apply Combo Defaults ───────────────────────────────────

function buildDatabaseUrl(appName: string, engine: string): string {
  const name = appName || "app";
  switch (engine) {
    case "postgres":
      return `postgres://${name}-db:5432/${name}`;
    case "mysql":
      return `mysql://${name}-db:3306/${name}`;
    case "mongo":
      return `mongodb://${name}-db:27017/${name}`;
    default:
      return `postgres://${name}-db:5432/${name}`;
  }
}

export function applyComboDefaults(
  combo: TraitCombo,
  service: ServiceInterviewState,
  appName?: string,
): ServiceInterviewState {
  const preset = TRAIT_COMBO_PRESETS[combo];
  if (!preset) return service;

  const dbDep = preset.dependencies.find((d) => d.type === "database");
  const dbEnabled = dbDep?.enabled ?? false;
  const dbEngine = dbDep?.engine ?? "postgres";

  // Build env vars: remove old auto entries, add DATABASE_URL if db enabled
  const manualEnvVars = service.envVars.filter((e) => !e.auto);
  const envVars: EnvVarEntry[] = [...manualEnvVars];
  if (dbEnabled) {
    envVars.push({
      key: "DATABASE_URL",
      value: buildDatabaseUrl(appName ?? service.aliasName, dbEngine),
      isSecret: false,
      auto: true,
    });
  }

  return {
    ...service,
    traits: [...preset.traits],
    port: preset.port,
    isPublic: preset.isPublic,
    workloadType: preset.workloadType,
    healthCheck: { ...preset.healthCheck },
    dependencies: preset.dependencies.map((d) => ({ ...d })),
    envVars,
  };
}
