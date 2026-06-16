// API response types for the Mozza backend.

export interface User {
  id: string;
  email: string;
  name: string;
  role: string;
}

export interface Team {
  id: string;
  name: string;
  slug: string;
  created_by: string;
}

export interface TeamMember {
  user_id: string;
  role: string;
  email?: string;
  name?: string;
}

export interface Recipe {
  id: string;
  team_id: string;
  name: string;
  source: string;
  canvas: string;
  created_by: string;
}

export interface Deployment {
  id: string;
  recipe_id: string;
  team_id: string;
  target: string;
  environment: string;
  status: string;
  started_by: string;
  log: string;
  started_at: string;
  finished_at?: string;
  order_number?: number;
  version?: number;
  has_previous_state?: boolean;
  access_url?: string;
}

export interface ImageAlias {
  id: string;
  team_id: string;
  name: string;
  image_ref: string;
  created_at: string;
}

export interface Slice {
  name: string;
  kind: string;
  image: string;
  port: number;
  public: boolean;
  replicas: number;
  health_path?: string;
}

export interface Ingredient {
  from: string;
  to: string;
}

export interface PlanData {
  name: string;
  slices: Slice[];
  ingredients: Ingredient[];
}

export interface ValidationResult {
  valid: boolean;
  errors: string[];
}

export interface DoctorFinding {
  severity: "error" | "warning" | "info" | "ok";
  rule: string;
  message: string;
  fix?: string;
}

export interface DoctorReport {
  findings: DoctorFinding[];
}

export interface ContainerStatus {
  name: string;
  state: string;
  image: string;
  ports: string;
  health?: string;
}

export interface StatusReport {
  containers: ContainerStatus[];
}

export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
}

// Cluster API types (real K8s data).

export interface ClusterNode {
  name: string;
  status: string;
  roles: string;
  age: string;
  version: string;
  cpu: string;
  memory: string;
  internal_ip: string;
}

export interface ClusterInfo {
  connected: boolean;
  nodes: ClusterNode[];
}

export interface ClusterPod {
  name: string;
  namespace: string;
  status: string;
  ready: string;
  restarts: number;
  age: string;
  node: string;
  ip?: string;
  app?: string;
}

export interface ClusterDeployment {
  name: string;
  namespace: string;
  ready: string;
  up_to_date: number;
  available: number;
  age: string;
  image?: string;
  labels?: Record<string, string>;
}

export interface ClusterNamespace {
  name: string;
  status: string;
  age: string;
}

export interface ClusterService {
  name: string;
  namespace: string;
  type: string;
  cluster_ip: string;
  ports: string;
  age: string;
}

export interface ClusterEvent {
  type: string;
  reason: string;
  message: string;
  object: string;
  namespace: string;
  age: string;
  count: number;
}

export interface ClusterMetrics {
  nodes: number;
  cpu_cores: number;
  cpu_percent: number;
  memory_gb: number;
  memory_percent: number;
  total_pods: number;
  running_pods: number;
  pending_pods: number;
  failed_pods: number;
  uptime: string;
}

// Registered cluster (management API).
export interface RegisteredCluster {
  id: string;
  name: string;
  created_at: string;
}

// Canvas model for the visual builder.
export interface CanvasBlock {
  id: string;
  kind: "web" | "worker" | "database" | "cache" | "api" | "gateway" | "task" | "scheduled" | "stateful" | "daemon";
  name: string;
  image: string;
  port: number;
  public: boolean;
  replicas: number;
  health_path: string;
  x: number;
  y: number;
  schedule?: string;
  parallelism?: number;
  retries?: number;
  engine?: string;
  version?: string;
  storage?: string;
  backups?: boolean;
  stateful_storage?: string;
  ordered_startup?: boolean;
}

export interface CanvasConnection {
  from: string;
  to: string;
}

export interface CanvasState {
  blocks: CanvasBlock[];
  connections: CanvasConnection[];
}

// Template marketplace types.

export interface Template {
  id: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  tags: string[];
  source: string;
  variables: TemplateVar[];
  repo?: string;
  official: boolean;
  est_resources?: string;
}

export interface TemplateVar {
  key: string;
  label: string;
  description: string;
  type: "string" | "password" | "number" | "boolean" | "select";
  default: string;
  required: boolean;
  options?: string[];
}

export interface ScanResult {
  repo_url: string;
  repo_name: string;
  description: string;
  sources: { type: string; path: string; priority: number }[];
  generated?: { source: string; method: string; editable: boolean; needs_build: boolean; build_instructions?: string };
  warnings: string[];
}

// Compose import types.

export interface ComposeServiceInfo {
  name: string;
  image: string;
  engine?: string;
  version?: string;
  port?: number;
  public?: boolean;
  env?: Record<string, string>;
}

export interface ComposeWarning {
  feature: string;
  message: string;
  severity: "info" | "warn" | "error";
}

export interface ComposeParseResult {
  services: ComposeServiceInfo[];
  recipe: string;
  warnings: ComposeWarning[];
}

// Framework detection types.
export interface DetectResult {
  framework: string;
  language: string;
  confidence: string;
  port: number;
  build_cmd?: string;
  start_cmd?: string;
  base_image: string;
  health_path: string;
  dockerfile: string;
  recipe: string;
  details?: Record<string, string>;
}

export interface GenerateResult {
  recipe: string;
  dockerfile: string;
}

// Marketplace types.

export interface MarketplaceSearchResult {
  template: Template;
  score: number;
}

export interface MarketplaceListResult {
  recipes: MarketplaceSearchResult[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}
