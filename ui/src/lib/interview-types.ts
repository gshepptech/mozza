// ─── Wizard v2 Types ────────────────────────────────────────

export type WizardTarget = "local" | "kitchen";
export type Trait = "web-facing" | "stateful" | "worker";
export type WorkloadType = "replicaset" | "statefulset" | "daemonset" | "cronjob";

// ─── Fast Lane Types ────────────────────────────────────────

export type TraitCombo = "website" | "api-database" | "background-worker" | "database-only" | "custom";
export type FastLaneSource = "alias" | "template" | "github";
export type ResourceSize = "S" | "M" | "L" | "XL";

// ─── Dependency Config ──────────────────────────────────────

export interface DependencyConfig {
  type: "database" | "cache" | "queue";
  enabled: boolean;
  engine: string;    // postgres, mysql, mongo, redis, rabbitmq, nats
  version: string;
  storage: string;   // e.g. "10Gi"
}

// ─── Env Var Entry ──────────────────────────────────────────

export interface EnvVarEntry {
  key: string;
  value: string;
  isSecret: boolean;
  auto?: boolean;    // true for auto-generated entries (e.g. DATABASE_URL)
}

// ─── Health / Resources / Scaling Defaults ──────────────────

export interface HealthCheckDefaults {
  enabled: boolean;
  type: "http" | "tcp";
  path: string;
  port: number;
  intervalSeconds: number;
}

export interface ResourceDefaults {
  cpuRequest: string;
  cpuLimit: string;
  memoryRequest: string;
  memoryLimit: string;
}

export interface ScalingDefaults {
  enabled: boolean;
  minReplicas: number;
  maxReplicas: number;
  cpuTarget: number;
}

// ─── Per-Service Interview State ────────────────────────────

export interface ServiceInterviewState {
  // Identity
  serviceName: string;       // unique name for this service in the recipe
  needsServices: string[];   // names of other services this one depends on
  // Step 1
  aliasName: string;
  aliasImage: string;
  replicas: number;
  // Step 2
  traits: Trait[];
  // Step 3
  workloadType: WorkloadType;
  workloadOverridden: boolean;
  schedule: string;
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

// ─── Wizard-Level State ─────────────────────────────────────

export interface WizardState {
  services: ServiceInterviewState[];
  activeServiceIndex: number;
  currentStep: number;
  target: WizardTarget;
  clusterId: string;
  // Fast lane fields
  fastLane?: boolean;
  selectedCombo?: TraitCombo | null;
  source?: FastLaneSource;
  selectedTemplateId?: string | null;
  selectedAliasId?: string | null;
  resourceSize?: ResourceSize;
}

// ─── Interview Question Definition ──────────────────────────

export interface InterviewQuestion {
  id: string;
  stepNumber: number;
  title: string;
  targetFilter: "local" | "kitchen" | "both";
  skipWhen?: (state: ServiceInterviewState, target: WizardTarget) => boolean;
}

// ─── Recommendation ─────────────────────────────────────────

export interface Recommendation {
  workloadType: WorkloadType;
  confidence: "high" | "medium";
  reasoning: string;
  explanation: string;
}

// ─── Defaults ───────────────────────────────────────────────

export function createDefaultServiceState(): ServiceInterviewState {
  return {
    serviceName: "",
    needsServices: [],
    aliasName: "",
    aliasImage: "",
    replicas: 1,
    traits: [],
    workloadType: "replicaset",
    workloadOverridden: false,
    schedule: "",
    port: 8080,
    isPublic: false,
    domain: "",
    dependencies: [
      { type: "database", enabled: false, engine: "postgres", version: "16", storage: "10Gi" },
      { type: "cache", enabled: false, engine: "redis", version: "7", storage: "" },
    ],
    envVars: [],
    healthCheck: {
      enabled: true,
      type: "http",
      path: "/health",
      port: 8080,
      intervalSeconds: 10,
    },
    resources: {
      cpuRequest: "250m",
      cpuLimit: "500m",
      memoryRequest: "128Mi",
      memoryLimit: "256Mi",
    },
    scaling: {
      enabled: false,
      minReplicas: 1,
      maxReplicas: 5,
      cpuTarget: 80,
    },
  };
}
