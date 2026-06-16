import { useState, useEffect, useCallback } from "react";
import {
  getClusterNamespaces, getClusterPods, getClusterDeployments,
  getClusterMetrics, listDeployments, triggerDeploy,
} from "@/api/client";
import type { ClusterNamespace, ClusterMetrics, Deployment } from "@/api/types";
import { useNavigate } from "react-router-dom";
import {
  Layers, ArrowRight, CheckCircle, AlertTriangle, Server,
  Cpu, HardDrive, Clock, Box, ChevronRight, RefreshCw, Loader,
  History,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { ClusterBanner } from "@/components/ClusterBanner";
import { useClusterStatus } from "@/context/ClusterContext";
import { MozzaLogoLarge } from "@/components/custom/MozzaLogo";

// Assign a visual style based on namespace name patterns
function getEnvStyle(name: string): { gradient: string; border: string; icon: string; label: string } {
  const n = name.toLowerCase();
  if (n.includes("prod")) {
    return { gradient: "from-error/5 to-transparent", border: "border-error/20", icon: "text-error", label: "Production" };
  }
  if (n.includes("stag") || n.includes("preview")) {
    return { gradient: "from-warning/5 to-transparent", border: "border-warning/20", icon: "text-warning", label: "Staging" };
  }
  if (n.includes("dev") || n.includes("default")) {
    return { gradient: "from-info/5 to-transparent", border: "border-info/20", icon: "text-info", label: "Development" };
  }
  return { gradient: "from-muted/5 to-transparent", border: "border-border/50", icon: "text-muted-foreground", label: name };
}

interface NamespaceData {
  namespace: ClusterNamespace;
  podCount: number;
  runningPods: number;
  deploymentCount: number;
  latestDeploy?: Deployment;
}

// Promotion history entry derived from deployment records.
interface PromotionEntry {
  id: string;
  fromEnv: string;
  toEnv: string;
  version?: number;
  startedBy: string;
  timestamp: string;
}

export default function EnvironmentsPage() {
  const navigate = useNavigate();
  const [namespaces, setNamespaces] = useState<NamespaceData[]>([]);
  const [metrics, setMetrics] = useState<ClusterMetrics | null>(null);
  const [promotionHistory, setPromotionHistory] = useState<PromotionEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [promoting, setPromoting] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [nsRes, podsRes, depsRes, metricsRes] = await Promise.all([
        getClusterNamespaces(),
        getClusterPods(),
        getClusterDeployments(),
        getClusterMetrics(),
      ]);

      const allPods = podsRes.pods || [];
      const allDeps = depsRes.deployments || [];
      const nsList = nsRes.namespaces || [];

      // Fetch Mozza deployments to correlate with namespaces.
      let allMozzaDeploys: Deployment[] = [];
      try {
        const deploysRes = await listDeployments("", 100);
        allMozzaDeploys = deploysRes.deployments || [];
      } catch {
        // Deployments may not be available — non-blocking.
      }
      const enriched: NamespaceData[] = nsList.map(ns => {
        const nsPods = allPods.filter(p => p.namespace === ns.name);
        const nsDeps = allDeps.filter(d => d.namespace === ns.name);
        // Find latest Mozza deploy for this environment.
        const envDeploys = allMozzaDeploys
          .filter(d => d.environment === ns.name)
          .sort((a, b) => (b.started_at || "").localeCompare(a.started_at || ""));
        return {
          namespace: ns,
          podCount: nsPods.length,
          runningPods: nsPods.filter(p => p.status === "Running").length,
          deploymentCount: nsDeps.length,
          latestDeploy: envDeploys[0],
        };
      });

      // Build promotion history from consecutive environment deploys.
      const envOrder = nsList.map(ns => ns.name);
      const history: PromotionEntry[] = [];
      for (let i = 1; i < envOrder.length; i++) {
        const toDeploys = allMozzaDeploys
          .filter(d => d.environment === envOrder[i] && d.status === "completed")
          .sort((a, b) => (b.started_at || "").localeCompare(a.started_at || ""));
        for (const d of toDeploys.slice(0, 3)) {
          history.push({
            id: d.id,
            fromEnv: envOrder[i - 1] ?? "unknown",
            toEnv: envOrder[i] ?? d.environment,
            version: d.version,
            startedBy: d.started_by,
            timestamp: d.started_at,
          });
        }
      }
      history.sort((a, b) => (b.timestamp || "").localeCompare(a.timestamp || ""));
      setPromotionHistory(history.slice(0, 10));

      setNamespaces(enriched);
      setMetrics(metricsRes);
    } catch {
      setError("Failed to load environment data. Connect a Docker host or Kubernetes cluster first.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const handlePromote = useCallback(async (fromNs: NamespaceData, toNs: NamespaceData) => {
    const deploy = fromNs.latestDeploy;
    if (!deploy?.recipe_id) return;
    setPromoting(toNs.namespace.name);
    try {
      await triggerDeploy(
        deploy.recipe_id,
        deploy.team_id,
        deploy.target,
        toNs.namespace.name,
      );
      await fetchData();
    } catch {
      setError(`Failed to promote to ${toNs.namespace.name}.`);
    } finally {
      setPromoting(null);
    }
  }, [fetchData]);

  const { status: clusterStatus, loading: clusterLoading } = useClusterStatus();
  const notConnected = !clusterLoading && !clusterStatus.reachable;

  // --- Friendly setup experience when no environment is connected ---
  if (notConnected && !loading) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Environments</h1>
            <p className="text-sm text-muted-foreground mt-0.5">Manage your deployment targets and promotion pipeline</p>
          </div>
        </div>

        <Card className="oven-card bg-card border-border/50">
          <CardContent className="flex flex-col items-center py-16 px-8">
            <MozzaLogoLarge className="text-brand mb-6" size={72} />
            <h2 className="text-xl font-bold text-foreground mb-2">Let's set up your first environment</h2>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-8">
              Mozza deploys to Docker or Kubernetes. Connect where you want your apps to run and you'll see your environments here.
            </p>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 w-full max-w-lg mb-8">
              {/* Docker option */}
              <button
                onClick={() => navigate("/app/clusters")}
                className="group flex flex-col items-center gap-3 p-6 rounded-xl border border-border/50 bg-card hover:border-brand/30 hover:bg-brand/5 transition-all text-left"
              >
                <div className="p-3 rounded-lg bg-blue-500/10 group-hover:bg-blue-500/15 transition-colors">
                  <Server className="h-6 w-6 text-blue-400" />
                </div>
                <div className="text-center">
                  <div className="text-sm font-semibold text-foreground">Docker</div>
                  <div className="text-xs text-muted-foreground mt-1">Local or remote Docker host</div>
                </div>
              </button>

              {/* Kubernetes option */}
              <button
                onClick={() => navigate("/app/clusters")}
                className="group flex flex-col items-center gap-3 p-6 rounded-xl border border-border/50 bg-card hover:border-brand/30 hover:bg-brand/5 transition-all text-left"
              >
                <div className="p-3 rounded-lg bg-brand/10 group-hover:bg-brand/15 transition-colors">
                  <Layers className="h-6 w-6 text-brand" />
                </div>
                <div className="text-center">
                  <div className="text-sm font-semibold text-foreground">Kubernetes</div>
                  <div className="text-xs text-muted-foreground mt-1">Any K8s cluster with a kubeconfig</div>
                </div>
              </button>
            </div>

            <p className="text-xs text-muted-foreground">
              You can add more environments later. Most people start with Docker locally.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div>
      <ClusterBanner />
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Environments</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Manage your deployment targets and promotion pipeline</p>
        </div>
        <div className="flex gap-3">
          <Button variant="outline" size="sm" onClick={fetchData} disabled={loading}>
            {loading
              ? <Loader className="mr-2 h-3.5 w-3.5 animate-spin" />
              : <RefreshCw className="mr-2 h-3.5 w-3.5" />
            }
            Refresh
          </Button>
          <Button
            className="shadow-[0_0_12px_rgba(255,107,53,0.15)]"
            onClick={() => navigate("/app/clusters")}
          >
            <Layers className="mr-2 h-4 w-4" />
            New Environment
          </Button>
        </div>
      </div>

      {error && !notConnected && (
        <div className="mb-6 rounded-lg border border-error/30 bg-error/5 p-4 text-sm text-error">
          {error}
        </div>
      )}

      {/* Cluster metrics summary */}
      {!loading && metrics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <Card className="oven-card bg-card border-border/50">
            <CardContent className="p-4">
              <div className="flex items-center gap-1.5 mb-1">
                <Server className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs text-muted-foreground">Nodes</span>
              </div>
              <p className="text-2xl font-bold text-foreground">{metrics.nodes}</p>
            </CardContent>
          </Card>
          <Card className="oven-card bg-card border-border/50">
            <CardContent className="p-4">
              <div className="flex items-center gap-1.5 mb-1">
                <Cpu className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs text-muted-foreground">CPU ({Math.round(metrics.cpu_percent)}%)</span>
              </div>
              <div className="h-2 rounded-full bg-muted overflow-hidden mt-2">
                <div
                  className={cn("h-full rounded-full", metrics.cpu_percent > 80 ? "bg-error" : metrics.cpu_percent > 60 ? "bg-warning" : "bg-brand")}
                  style={{ width: `${metrics.cpu_percent}%` }}
                />
              </div>
              <p className="text-xs text-muted-foreground mt-1">{Math.round(metrics.cpu_cores)} cores</p>
            </CardContent>
          </Card>
          <Card className="oven-card bg-card border-border/50">
            <CardContent className="p-4">
              <div className="flex items-center gap-1.5 mb-1">
                <HardDrive className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs text-muted-foreground">Memory ({Math.round(metrics.memory_percent)}%)</span>
              </div>
              <div className="h-2 rounded-full bg-muted overflow-hidden mt-2">
                <div
                  className={cn("h-full rounded-full", metrics.memory_percent > 80 ? "bg-error" : metrics.memory_percent > 60 ? "bg-warning" : "bg-brand")}
                  style={{ width: `${metrics.memory_percent}%` }}
                />
              </div>
              <p className="text-xs text-muted-foreground mt-1">{metrics.memory_gb.toFixed(1)} GB</p>
            </CardContent>
          </Card>
          <Card className="oven-card bg-card border-border/50">
            <CardContent className="p-4">
              <div className="flex items-center gap-1.5 mb-1">
                <Box className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs text-muted-foreground">Pods</span>
              </div>
              <p className="text-2xl font-bold text-foreground">
                {metrics.running_pods}
                <span className="text-sm font-normal text-muted-foreground ml-1">/ {metrics.total_pods}</span>
              </p>
              {metrics.pending_pods > 0 && (
                <p className="text-xs text-warning mt-1">{metrics.pending_pods} pending</p>
              )}
              {metrics.failed_pods > 0 && (
                <p className="text-xs text-error mt-1">{metrics.failed_pods} failed</p>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Loading state */}
      {loading && (
        <div className="space-y-4">
          <Skeleton className="h-40 w-full rounded-xl" />
          <Skeleton className="h-32 w-full rounded-xl" />
          <Skeleton className="h-32 w-full rounded-xl" />
        </div>
      )}

      {/* Promotion pipeline */}
      {!loading && namespaces.length > 0 && (
        <Card className="oven-card bg-card mb-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <ArrowRight className="h-4 w-4 text-brand" />
              Promotion Pipeline
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-0">
              {namespaces.map((ns, i) => {
                const style = getEnvStyle(ns.namespace.name);
                const isHealthy = ns.runningPods === ns.podCount && ns.podCount > 0;
                const deploy = ns.latestDeploy;
                const nextNs = i < namespaces.length - 1 ? namespaces[i + 1] : null;
                const canPromote = deploy?.recipe_id && nextNs;
                const isPromoting = promoting === nextNs?.namespace.name;
                return (
                  <div key={ns.namespace.name} className="flex items-center flex-1">
                    <div className={cn(
                      "flex-1 rounded-lg border p-4 bg-gradient-to-b",
                      style.gradient,
                      style.border,
                    )}>
                      <div className="flex items-center gap-2 mb-2">
                        <div className={cn("h-2 w-2 rounded-full", {
                          "bg-success": isHealthy,
                          "bg-warning": !isHealthy && ns.podCount > 0,
                          "bg-muted-foreground": ns.podCount === 0,
                        })} />
                        <span className="text-sm font-semibold text-foreground">{ns.namespace.name}</span>
                      </div>
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between">
                          <span className="text-[10px] text-muted-foreground">Apps</span>
                          <span className="text-xs font-semibold">{ns.deploymentCount}</span>
                        </div>
                        {deploy?.version != null && (
                          <div className="flex items-center justify-between">
                            <span className="text-[10px] text-muted-foreground">Version</span>
                            <Badge variant="outline" className="text-[10px] h-4 px-1.5">
                              v{deploy.version}
                            </Badge>
                          </div>
                        )}
                        <div className="flex items-center justify-between">
                          <span className="text-[10px] text-muted-foreground">Pods</span>
                          <span className="text-xs">
                            <span className="text-success font-semibold">{ns.runningPods}</span>
                            {ns.podCount - ns.runningPods > 0 && (
                              <span className="text-error font-semibold"> / {ns.podCount - ns.runningPods}</span>
                            )}
                          </span>
                        </div>
                        {deploy?.started_at && (
                          <div className="flex items-center justify-between">
                            <span className="text-[10px] text-muted-foreground">Last deploy</span>
                            <span className="text-[10px] font-mono text-muted-foreground">
                              {formatRelativeTime(deploy.started_at)}
                            </span>
                          </div>
                        )}
                      </div>
                    </div>
                    {i < namespaces.length - 1 && (
                      <div className="flex flex-col items-center mx-2 shrink-0">
                        <Button
                          variant="ghost"
                          size="icon"
                          className={cn(
                            "h-7 w-7",
                            canPromote
                              ? "text-brand hover:bg-brand/10"
                              : "text-muted-foreground/30 cursor-not-allowed",
                          )}
                          disabled={!canPromote || !!isPromoting}
                          onClick={() => canPromote && nextNs && handlePromote(ns, nextNs)}
                        >
                          {isPromoting
                            ? <Loader className="h-4 w-4 animate-spin" />
                            : <ChevronRight className="h-4 w-4" />
                          }
                        </Button>
                        <span className="text-[9px] text-muted-foreground">
                          {isPromoting ? "promoting..." : "promote"}
                        </span>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Promotion history */}
      {!loading && promotionHistory.length > 0 && (
        <Card className="oven-card bg-card mb-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <History className="h-4 w-4 text-muted-foreground" />
              Promotion History
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {promotionHistory.map(entry => (
                <div key={entry.id} className="flex items-center justify-between py-2 px-3 rounded-lg bg-muted/30">
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-[10px]">{entry.fromEnv}</Badge>
                    <ArrowRight className="h-3 w-3 text-muted-foreground" />
                    <Badge variant="outline" className="text-[10px]">{entry.toEnv}</Badge>
                    {entry.version != null && (
                      <span className="text-[10px] text-muted-foreground font-mono">v{entry.version}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-[10px] text-muted-foreground">{entry.startedBy}</span>
                    <span className="text-[10px] text-muted-foreground font-mono">
                      {formatRelativeTime(entry.timestamp)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Empty state */}
      {!loading && namespaces.length === 0 && !error && (
        <Card className="oven-card bg-card border-border/50">
          <CardContent className="flex flex-col items-center py-16">
            <div className="p-4 rounded-xl bg-brand/10 mb-4">
              <Layers className="h-8 w-8 text-brand" />
            </div>
            <p className="text-base font-semibold text-foreground">No environments found</p>
            <p className="text-sm text-muted-foreground mt-1.5">Connect a Docker host or Kubernetes cluster to get started</p>
          </CardContent>
        </Card>
      )}

      {/* Environment detail cards */}
      {!loading && namespaces.length > 0 && (
        <div className="space-y-4">
          {namespaces.map(ns => {
            const style = getEnvStyle(ns.namespace.name);
            const isHealthy = ns.runningPods === ns.podCount && ns.podCount > 0;
            const unhealthyPods = ns.podCount - ns.runningPods;
            return (
              <Card key={ns.namespace.name} className={cn("bg-card border", style.border)}>
                <CardContent className="pt-5 pb-4 px-5">
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                      <div className={cn("p-2 rounded-lg bg-gradient-to-b", style.gradient)}>
                        <Layers className={cn("h-5 w-5", style.icon)} />
                      </div>
                      <div>
                        <h3 className="text-base font-semibold text-foreground">{ns.namespace.name}</h3>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-xs text-muted-foreground">Age: {ns.namespace.age}</span>
                          <span className="text-xs text-muted-foreground">&middot;</span>
                          <span className="text-xs text-muted-foreground">{ns.namespace.status}</span>
                        </div>
                      </div>
                    </div>
                    <Badge variant="outline" className={cn("text-xs gap-1", isHealthy ? "text-success border-success/30" : unhealthyPods > 0 ? "text-warning border-warning/30" : "text-muted-foreground border-border")}>
                      {isHealthy ? <CheckCircle className="h-3 w-3" /> : <AlertTriangle className="h-3 w-3" />}
                      {isHealthy ? "healthy" : ns.podCount === 0 ? "empty" : "degraded"}
                    </Badge>
                  </div>

                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    {/* Deployments */}
                    <div>
                      <div className="flex items-center gap-1.5 mb-1">
                        <Box className="h-3 w-3 text-muted-foreground" />
                        <span className="text-[10px] text-muted-foreground">Deployments</span>
                      </div>
                      <p className="text-lg font-bold text-foreground">{ns.deploymentCount}</p>
                    </div>
                    {/* Pods */}
                    <div>
                      <div className="flex items-center gap-1.5 mb-1">
                        <Server className="h-3 w-3 text-muted-foreground" />
                        <span className="text-[10px] text-muted-foreground">Pods</span>
                      </div>
                      <p className="text-lg font-bold text-foreground">
                        {ns.podCount}
                        {unhealthyPods > 0 && (
                          <span className="text-sm text-error ml-1">({unhealthyPods} unhealthy)</span>
                        )}
                      </p>
                    </div>
                    {/* Running */}
                    <div>
                      <div className="flex items-center gap-1.5 mb-1">
                        <CheckCircle className="h-3 w-3 text-muted-foreground" />
                        <span className="text-[10px] text-muted-foreground">Running</span>
                      </div>
                      <p className="text-lg font-bold text-success">{ns.runningPods}</p>
                    </div>
                    {/* Age */}
                    <div>
                      <div className="flex items-center gap-1.5 mb-1">
                        <Clock className="h-3 w-3 text-muted-foreground" />
                        <span className="text-[10px] text-muted-foreground">Age</span>
                      </div>
                      <p className="text-lg font-bold text-foreground">{ns.namespace.age}</p>
                    </div>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}

function formatRelativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}
