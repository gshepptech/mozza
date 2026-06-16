// Typed API client for the Mozza backend.

import type {
  User,
  Team,
  TeamMember,
  Recipe,
  Deployment,
  PlanData,
  ValidationResult,
  DoctorReport,
  StatusReport,
  VersionInfo,
  ImageAlias,
  ClusterInfo,
  ClusterPod,
  ClusterDeployment,
  ClusterNamespace,
  ClusterService,
  ClusterEvent,
  ClusterMetrics,
  Template,
  ScanResult,
  ComposeParseResult,
  DetectResult,
  GenerateResult,
  MarketplaceListResult,
} from "./types";

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    credentials: "same-origin",
    ...opts,
    headers: {
      "Content-Type": "application/json",
      ...opts?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error || res.statusText);
  }

  return res.json();
}

// Auth
export async function register(email: string, name: string, password: string): Promise<User> {
  return request<User>("/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, name, password }),
  });
}

export async function login(email: string, password: string): Promise<User> {
  return request<User>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
}

export async function logout(): Promise<void> {
  await request("/auth/logout", { method: "POST" });
}

export async function getMe(): Promise<User> {
  return request<User>("/auth/me");
}

export async function updateProfile(name: string): Promise<User> {
  return request<User>("/auth/me", {
    method: "PUT",
    body: JSON.stringify({ name }),
  });
}

// Plan
export async function getPlan(): Promise<PlanData> {
  return request<PlanData>("/plan");
}

// Teams
export async function createTeam(name: string, slug: string): Promise<Team> {
  return request<Team>("/teams", {
    method: "POST",
    body: JSON.stringify({ name, slug }),
  });
}

export async function listTeams(): Promise<{ teams: Team[] }> {
  return request<{ teams: Team[] }>("/teams");
}

export async function getTeam(id: string): Promise<Team> {
  return request<Team>(`/teams/${id}`);
}

export async function deleteTeam(id: string): Promise<void> {
  await request(`/teams/${id}`, { method: "DELETE" });
}

export async function listTeamMembers(teamId: string): Promise<{ members: TeamMember[] }> {
  return request<{ members: TeamMember[] }>(`/teams/${teamId}/members`);
}

export async function addTeamMember(teamId: string, email: string, role: string): Promise<void> {
  await request(`/teams/${teamId}/members`, {
    method: "POST",
    body: JSON.stringify({ email, role }),
  });
}

export async function removeTeamMember(teamId: string, userId: string): Promise<void> {
  await request(`/teams/${teamId}/members/${userId}`, { method: "DELETE" });
}

// Recipes
export async function createRecipe(teamId: string, name: string, source: string, canvas: string): Promise<Recipe> {
  return request<Recipe>("/recipes", {
    method: "POST",
    body: JSON.stringify({ team_id: teamId, name, source, canvas }),
  });
}

export async function listRecipes(teamId: string): Promise<{ recipes: Recipe[] }> {
  return request<{ recipes: Recipe[] }>(`/recipes?team_id=${teamId}`);
}

export async function getRecipe(id: string): Promise<Recipe> {
  return request<Recipe>(`/recipes/${id}`);
}

export async function updateRecipe(id: string, name: string, source: string, canvas: string): Promise<Recipe> {
  return request<Recipe>(`/recipes/${id}`, {
    method: "PUT",
    body: JSON.stringify({ name, source, canvas }),
  });
}

export async function deleteRecipe(id: string): Promise<void> {
  await request(`/recipes/${id}`, { method: "DELETE" });
}

export async function validateRecipe(source: string): Promise<ValidationResult> {
  return request<ValidationResult>("/recipes/validate", {
    method: "POST",
    body: JSON.stringify({ source }),
  });
}

// Deployments
export async function triggerDeploy(
  recipeId: string,
  teamId: string,
  target: string,
  environment: string,
  clusterId?: string,
): Promise<Deployment> {
  const payload: Record<string, string> = {
    recipe_id: recipeId,
    team_id: teamId,
    target,
    environment,
  };
  if (clusterId) payload.cluster_id = clusterId;
  return request<Deployment>("/deployments", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function listDeployments(teamId: string, limit = 50): Promise<{ deployments: Deployment[] }> {
  return request<{ deployments: Deployment[] }>(`/deployments?team_id=${teamId}&limit=${limit}`);
}

export async function getDeployment(id: string): Promise<Deployment> {
  return request<Deployment>(`/deployments/${id}`);
}

export async function deleteDeployment(id: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/deployments/${id}`, { method: "DELETE" });
}

export async function rollbackDeployment(id: string): Promise<Deployment> {
  return request<Deployment>(`/deployments/${id}/rollback`, { method: "POST" });
}

// SSE log stream for a deployment.
export function streamDeploymentLogs(
  deploymentId: string,
  onData: (line: string) => void,
  onDone: (status: string) => void,
): () => void {
  const es = new EventSource(`/api/v1/deployments/${deploymentId}/logs`);

  es.onmessage = (e) => {
    onData(e.data);
  };

  es.addEventListener("done", (e) => {
    onDone((e as MessageEvent).data);
    es.close();
  });

  es.onerror = () => {
    es.close();
  };

  return () => es.close();
}

// Deploy Preview
export interface DeployPreviewSlice {
  name: string;
  kind: string;
  image: string;
  port: number;
  replicas: number;
  public: boolean;
}

export interface DeployPreviewSummary {
  services: number;
  databases: number;
  caches: number;
  workers: number;
  total: number;
}

export interface DeployPreviewResponse {
  name: string;
  slices: DeployPreviewSlice[];
  summary: DeployPreviewSummary;
  manifests?: string;
}

export async function previewDeploy(source: string): Promise<DeployPreviewResponse> {
  return request<DeployPreviewResponse>("/deploy/preview", {
    method: "POST",
    body: JSON.stringify({ source }),
  });
}

// Doctor
export async function getDoctor(): Promise<DoctorReport> {
  return request<DoctorReport>("/doctor");
}

// Status
export async function getStatus(): Promise<StatusReport> {
  return request<StatusReport>("/status");
}

// Version
export async function getVersion(): Promise<VersionInfo> {
  return request<VersionInfo>("/version");
}

// Image Aliases
export async function listAliases(teamId: string): Promise<ImageAlias[]> {
  const data = await request<{ aliases: ImageAlias[] }>(`/aliases?team_id=${teamId}`);
  return data.aliases;
}

export async function createAlias(teamId: string, name: string, imageRef: string): Promise<void> {
  await request("/aliases", {
    method: "POST",
    body: JSON.stringify({ team_id: teamId, name, image_ref: imageRef }),
  });
}

export async function deleteAlias(teamId: string, name: string): Promise<void> {
  await request(`/aliases/${encodeURIComponent(name)}?team_id=${teamId}`, { method: "DELETE" });
}

// Registered Clusters (management)
export async function createCluster(name: string, kubeconfig: string): Promise<any> {
  return request("/clusters", { method: "POST", body: JSON.stringify({ name, kubeconfig }) });
}

export async function listClusters(): Promise<{ clusters: any[] }> {
  return request("/clusters");
}

export async function deleteCluster(id: string): Promise<void> {
  await request(`/clusters/${id}`, { method: "DELETE" });
}

// Cluster (real K8s data)
export async function getClusterNodes(): Promise<ClusterInfo> {
  return request<ClusterInfo>("/cluster/nodes");
}

export async function getClusterPods(namespace?: string): Promise<{ pods: ClusterPod[] }> {
  const qs = namespace ? `?namespace=${namespace}` : "";
  return request<{ pods: ClusterPod[] }>(`/cluster/pods${qs}`);
}

export async function getClusterDeployments(namespace?: string): Promise<{ deployments: ClusterDeployment[] }> {
  const qs = namespace ? `?namespace=${namespace}` : "";
  return request<{ deployments: ClusterDeployment[] }>(`/cluster/deployments${qs}`);
}

export async function getClusterNamespaces(): Promise<{ namespaces: ClusterNamespace[] }> {
  return request<{ namespaces: ClusterNamespace[] }>("/cluster/namespaces");
}

export async function getClusterServices(namespace?: string): Promise<{ services: ClusterService[] }> {
  const qs = namespace ? `?namespace=${namespace}` : "";
  return request<{ services: ClusterService[] }>(`/cluster/services${qs}`);
}

export async function getClusterEvents(): Promise<{ events: ClusterEvent[] }> {
  return request<{ events: ClusterEvent[] }>("/cluster/events");
}

export async function getClusterMetrics(): Promise<ClusterMetrics> {
  return request<ClusterMetrics>("/cluster/metrics");
}

export async function getClusterPodLogs(namespace: string, pod: string): Promise<string> {
  const res = await fetch(`/api/v1/cluster/pods/logs?namespace=${namespace}&pod=${pod}`, {
    credentials: "same-origin",
  });
  if (!res.ok) throw new ApiError(res.status, "failed to get logs");
  return res.text();
}

// Templates
export async function listTemplates(category?: string): Promise<{ templates: Template[] }> {
  const qs = category ? `?category=${encodeURIComponent(category)}` : "";
  return request<{ templates: Template[] }>(`/templates${qs}`);
}

export async function getTemplate(id: string): Promise<Template> {
  return request<Template>(`/templates/${id}`);
}

export async function deployTemplate(
  id: string,
  data: { team_id: string; target: string; cluster_id?: string; namespace: string; variables: Record<string, string> },
): Promise<Deployment> {
  return request<Deployment>(`/templates/${id}/deploy`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// GitHub Import
export async function scanRepo(url: string, token?: string): Promise<ScanResult> {
  const payload: Record<string, string> = { url };
  if (token) payload.token = token;
  return request<ScanResult>("/import/scan", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function deployImport(
  data: { team_id: string; target: string; cluster_id?: string; namespace: string; source: string },
): Promise<Deployment> {
  return request<Deployment>("/import/deploy", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Compose Import
export async function parseCompose(content: string): Promise<ComposeParseResult> {
  return request<ComposeParseResult>("/import/compose", {
    method: "POST",
    body: JSON.stringify({ content }),
  });
}

export async function deployCompose(
  data: { content: string; team_id: string; target: string; cluster_id?: string; adjustments?: Record<string, string> },
): Promise<Deployment> {
  return request<Deployment>("/import/compose/deploy", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Framework Detection
export async function detectFramework(repoUrl?: string): Promise<DetectResult> {
  const payload: Record<string, string> = {};
  if (repoUrl) payload.repo_url = repoUrl;
  return request<DetectResult>("/detect", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export async function generateFromDetection(
  data: { framework: string; language: string; app_name: string; port: number; user_choices: Record<string, string> },
): Promise<GenerateResult> {
  return request<GenerateResult>("/detect/generate", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Marketplace
export async function listMarketplaceRecipes(params?: {
  query?: string;
  category?: string;
  tags?: string[];
  page?: number;
  per_page?: number;
}): Promise<MarketplaceListResult> {
  const qs = new URLSearchParams();
  if (params?.query) qs.set("query", params.query);
  if (params?.category) qs.set("category", params.category);
  if (params?.tags?.length) qs.set("tags", params.tags.join(","));
  if (params?.page) qs.set("page", String(params.page));
  if (params?.per_page) qs.set("per_page", String(params.per_page));
  const q = qs.toString();
  return request<MarketplaceListResult>(`/marketplace/recipes${q ? `?${q}` : ""}`);
}

export async function getMarketplaceRecipe(name: string): Promise<Template> {
  return request<Template>(`/marketplace/recipes/${encodeURIComponent(name)}`);
}

export async function deployMarketplaceRecipe(
  name: string,
  data: { team_id: string; target: string; cluster_id?: string; variables?: Record<string, string> },
): Promise<Deployment> {
  return request<Deployment>(`/marketplace/recipes/${encodeURIComponent(name)}/deploy`, {
    method: "POST",
    body: JSON.stringify(data),
  });
}

// Monitoring Time Series
export interface TimeSeriesPoint {
  timestamp: string;
  cpu: number;
  memory: number;
  network_in: number;
  network_out: number;
}

export interface TimeSeriesResponse {
  app_id: string;
  period: string;
  points: TimeSeriesPoint[];
}

export async function getAppTimeSeries(
  appId: string,
  period: string = "1h",
): Promise<TimeSeriesResponse> {
  return request<TimeSeriesResponse>(`/apps/${appId}/timeseries?period=${period}`);
}

// Cluster operations
export async function restartDeployment(namespace: string, name: string): Promise<{ status: string }> {
  return request<{ status: string }>(`/cluster/namespaces/${namespace}/deployments/${name}/restart`, {
    method: "POST",
  });
}

// Registry scanning
export interface RegistryImage {
  name: string;
  full_ref: string;
  description?: string;
  last_updated?: string;
}

export interface ScanNamespaceResult {
  namespace: string;
  images: RegistryImage[];
  total: number;
}

export async function scanNamespace(namespace: string): Promise<ScanNamespaceResult> {
  return request<ScanNamespaceResult>(`/registry/scan?namespace=${encodeURIComponent(namespace)}`);
}

export { ApiError };
