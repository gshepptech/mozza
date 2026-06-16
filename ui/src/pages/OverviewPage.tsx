import { useState, useEffect, useCallback } from "react";
import { Link } from "react-router-dom";
import {
  getClusterNodes, getClusterMetrics, getClusterPods,
  getClusterEvents, getClusterDeployments, listDeployments,
  listClusters,
} from "@/api/client";
import type {
  ClusterInfo, ClusterMetrics, ClusterPod, ClusterEvent,
  ClusterDeployment, Deployment,
} from "@/api/types";
import { useDashboard } from "./DashboardPage";
import { useClusterStatus } from "@/context/ClusterContext";
import {
  Box, Server, Rocket, Layers, Cpu, Activity,
  CheckCircle, AlertTriangle, ArrowUpRight,
  Zap, Stethoscope, Plus, Bell, ChevronRight, GitCommit,
  RefreshCw, Shield, Settings, Loader2,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { timeAgo, relativeTime, statusColor, statusBgColor } from "@/lib/format";

// ── Stat Card ──────────────────────────────────────────────────

function StatCard({ label, value, sub, icon: Icon, trend, accent }: {
  label: string;
  value: string | number;
  sub?: string;
  icon: typeof Box;
  trend?: { value: string; positive: boolean };
  accent?: string;
}) {
  return (
    <Card className="oven-card feature-card bg-card border-border/60">
      <CardContent className="p-6">
        <div className="flex items-start justify-between mb-5">
          <div className={cn(
            "p-3 rounded-xl border",
            accent || "bg-brand/10",
            accent ? "border-current/10" : "border-brand/15"
          )}>
            <Icon className={cn("h-5 w-5", accent ? "text-current" : "text-brand")} />
          </div>
          {trend && (
            <Badge variant="outline" className={cn(
              "text-xs font-semibold px-2.5 py-0.5",
              trend.positive ? "text-success border-success/30 bg-success/5" : "text-error border-error/30 bg-error/5"
            )}>
              {trend.value}
            </Badge>
          )}
        </div>
        <p className="text-4xl font-bold text-foreground tracking-tight leading-none" style={{ textShadow: '0 0 40px rgba(255,107,53,0.05)' }}>
          {value}
        </p>
        <p className="text-sm text-muted-foreground mt-2.5 font-medium">{label}</p>
        {sub && (
          <p className="text-xs text-muted-foreground/50 mt-1.5">{sub}</p>
        )}
      </CardContent>
    </Card>
  );
}

// ── Resource Bar ───────────────────────────────────────────────

function ResourceBar({ label, used, total, unit }: {
  label: string;
  used: number;
  total: number;
  unit: string;
}) {
  const pct = total > 0 ? Math.round((used / total) * 100) : 0;
  const color = pct > 80 ? "bg-error" : pct > 60 ? "bg-warning" : "bg-success";
  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm font-medium text-foreground">{label}</span>
        <span className="text-sm text-muted-foreground">
          {used.toFixed(1)} / {total.toFixed(1)} {unit}
        </span>
      </div>
      <div className="h-3 rounded-full bg-muted/60 overflow-hidden">
        <div
          className={cn("h-full rounded-full transition-all duration-500", color)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="text-xs text-muted-foreground/70 mt-1.5">{pct}% utilized</p>
    </div>
  );
}

// ── Event Row ──────────────────────────────────────────────────

const eventIcons: Record<string, typeof Rocket> = {
  deploy: Rocket,
  scale: Activity,
  rollback: RefreshCw,
  alert: AlertTriangle,
  config: Settings,
  create: Plus,
  promote: ArrowUpRight,
  Normal: CheckCircle,
  Warning: AlertTriangle,
};

function EventRow({ event, odd }: { event: ClusterEvent; odd: boolean }) {
  const Icon = eventIcons[event.type] || eventIcons[event.reason] || Activity;
  const sevColors: Record<string, string> = {
    Normal: "text-success",
    Warning: "text-warning",
    success: "text-success",
    error: "text-error",
    warning: "text-warning",
    info: "text-muted-foreground",
  };
  const severity = event.type === "Warning" ? "Warning" : "Normal";
  return (
    <div className={cn(
      "flex items-start gap-4 py-4 px-4 -mx-2 rounded-lg transition-colors",
      odd ? "bg-muted/20" : ""
    )}>
      <div className={cn(
        "mt-0.5 p-1.5 rounded-lg shrink-0",
        statusBgColor(severity === "Normal" ? "healthy" : "warning")
      )}>
        <Icon className={cn("h-4 w-4", sevColors[severity] || "text-muted-foreground")} />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm text-foreground leading-relaxed">{event.message}</p>
        <div className="flex items-center gap-3 mt-1.5">
          <span className="text-xs text-muted-foreground font-medium font-mono">{event.object}</span>
          <span className="text-xs text-muted-foreground/50">{event.namespace}</span>
          <span className="text-xs text-muted-foreground/50">{timeAgo(event.age)}</span>
        </div>
      </div>
    </div>
  );
}

// ── Main Page ──────────────────────────────────────────────────

export default function OverviewPage() {
  const { activeTeam } = useDashboard();
  const { status: clusterStatus } = useClusterStatus();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasCluster, setHasCluster] = useState<boolean | null>(null);
  const [clusterInfo, setClusterInfo] = useState<ClusterInfo | null>(null);
  const [metrics, setMetrics] = useState<ClusterMetrics | null>(null);
  const [pods, setPods] = useState<ClusterPod[]>([]);
  const [events, setEvents] = useState<ClusterEvent[]>([]);
  const [k8sDeployments, setK8sDeployments] = useState<ClusterDeployment[]>([]);
  const [recentDeploys, setRecentDeploys] = useState<Deployment[]>([]);
  const [showAllEvents, setShowAllEvents] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Check if any clusters are registered before trying to connect
      const { clusters } = await listClusters();
      const hasClusters = (clusters ?? []).length > 0;
      setHasCluster(hasClusters);

      if (!hasClusters) {
        // No cluster registered — skip cluster API calls entirely
        if (activeTeam) {
          try {
            const deploys = await listDeployments(activeTeam.id, 4);
            setRecentDeploys(deploys.deployments || []);
          } catch {
            setRecentDeploys([]);
          }
        }
        setLoading(false);
        return;
      }

      const [infoRes, metricsRes, podsRes, eventsRes, deploymentsRes] = await Promise.all([
        getClusterNodes(),
        getClusterMetrics(),
        getClusterPods(),
        getClusterEvents(),
        getClusterDeployments(),
      ]);
      setClusterInfo(infoRes);
      setMetrics(metricsRes);
      setPods(podsRes.pods);
      setEvents(eventsRes.events);
      setK8sDeployments(deploymentsRes.deployments);

      if (activeTeam) {
        try {
          const deploys = await listDeployments(activeTeam.id, 4);
          setRecentDeploys(deploys.deployments || []);
        } catch {
          setRecentDeploys([]);
        }
      } else {
        setRecentDeploys([]);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to connect to cluster");
    } finally {
      setLoading(false);
    }
  }, [activeTeam]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Derive stats from real data
  const connected = clusterInfo?.connected ?? false;
  const nodeCount = metrics?.nodes ?? clusterInfo?.nodes?.length ?? 0;
  const totalPods = metrics?.total_pods ?? pods.length;
  const runningPods = metrics?.running_pods ?? pods.filter(p => p.status === "Running").length;
  const pendingPods = metrics?.pending_pods ?? pods.filter(p => p.status === "Pending").length;
  const failedPods = metrics?.failed_pods ?? pods.filter(p => ["CrashLoopBackOff", "ImagePullBackOff", "Failed"].includes(p.status)).length;
  const cpuCores = metrics?.cpu_cores ?? 0;
  const cpuPercent = metrics?.cpu_percent ?? 0;
  const memoryGb = metrics?.memory_gb ?? 0;
  const memoryPercent = metrics?.memory_percent ?? 0;
  const uptime = metrics?.uptime ?? "";
  const cpuUsed = (cpuPercent / 100) * cpuCores;
  const memUsed = (memoryPercent / 100) * memoryGb;

  const displayEvents = showAllEvents ? events : events.slice(0, 6);
  const warningEvents = events.filter(e => e.type === "Warning");

  // No cluster registered — show welcome/onboarding state
  if (!loading && hasCluster === false) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Overview</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Welcome to Mozza</p>
          </div>
        </div>

        {/* Getting Started Card */}
        <Card className="oven-card feature-card bg-card border-brand/20 mb-10">
          <CardContent className="p-8 text-center">
            <Rocket className="h-12 w-12 text-brand mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-foreground mb-2">Get started with Mozza</h2>
            <p className="text-sm text-muted-foreground mb-6 max-w-md mx-auto">
              Connect a Kubernetes cluster to see live health metrics, deploy applications, and track your infrastructure.
            </p>
            <div className="flex items-center justify-center gap-4">
              <Link to="/app/clusters">
                <Button className="bg-brand text-white hover:bg-brand/90">
                  <Plus className="h-4 w-4 mr-2" /> Connect a Cluster
                </Button>
              </Link>
              <Link to="/app/deployments">
                <Button variant="outline" className="border-brand/30 text-brand hover:bg-brand/10">
                  <Rocket className="h-4 w-4 mr-2" /> Deploy Locally
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>

        {/* Quick Actions + Recent Deploys still visible without a cluster */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-10">
          <Card className="oven-card feature-card bg-card border-border/50">
            <CardHeader className="pb-4 px-6 pt-6">
              <CardTitle className="text-base font-semibold flex items-center gap-2.5">
                <Zap className="h-5 w-5 text-brand" />
                Quick Actions
              </CardTitle>
            </CardHeader>
            <CardContent className="px-6 pb-6">
              <div className="grid grid-cols-2 gap-4">
                <Link to="/app/clusters">
                  <Button variant="outline" className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30">
                    <Server className="h-5 w-5 mr-3 text-brand shrink-0" />
                    <div>
                      <p className="text-sm font-semibold">Clusters</p>
                      <p className="text-xs text-muted-foreground mt-0.5">Connect a cluster</p>
                    </div>
                  </Button>
                </Link>
                <Link to="/app/deployments">
                  <Button variant="outline" className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30">
                    <Rocket className="h-5 w-5 mr-3 text-brand shrink-0" />
                    <div>
                      <p className="text-sm font-semibold">Deploy</p>
                      <p className="text-xs text-muted-foreground mt-0.5">Ship an app</p>
                    </div>
                  </Button>
                </Link>
                <Link to="/app/marketplace">
                  <Button variant="outline" className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30">
                    <Layers className="h-5 w-5 mr-3 text-brand shrink-0" />
                    <div>
                      <p className="text-sm font-semibold">Templates</p>
                      <p className="text-xs text-muted-foreground mt-0.5">Browse marketplace</p>
                    </div>
                  </Button>
                </Link>
                <Link to="/app/doctor">
                  <Button variant="outline" className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30">
                    <Stethoscope className="h-5 w-5 mr-3 text-brand shrink-0" />
                    <div>
                      <p className="text-sm font-semibold">Doctor</p>
                      <p className="text-xs text-muted-foreground mt-0.5">Run diagnostics</p>
                    </div>
                  </Button>
                </Link>
              </div>
            </CardContent>
          </Card>

          {/* Recent Deployments — works without a cluster */}
          <Card className="oven-card feature-card bg-card border-border/50">
            <CardHeader className="px-6 pt-6 pb-4 flex flex-row items-center justify-between">
              <CardTitle className="text-base font-semibold flex items-center gap-2.5">
                <GitCommit className="h-5 w-5 text-brand" />
                Recent Deployments
              </CardTitle>
              <Link to="/app/deployments">
                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground hover:text-foreground h-8">
                  View all <ChevronRight className="h-3.5 w-3.5 ml-1" />
                </Button>
              </Link>
            </CardHeader>
            <CardContent className="px-6 pb-6">
              {recentDeploys.length === 0 ? (
                <div className="py-8 text-center">
                  <Rocket className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
                  <p className="text-sm text-muted-foreground">
                    {activeTeam ? "No deployments yet" : "Select a team to view deployments"}
                  </p>
                </div>
              ) : (
                <div className="space-y-0">
                  {recentDeploys.map((d, i) => (
                    <div
                      key={d.id}
                      className={cn(
                        "flex items-center gap-4 py-4 transition-colors",
                        i < recentDeploys.length - 1 && "border-b border-border/40"
                      )}
                    >
                      <div className={cn("h-2.5 w-2.5 rounded-full shrink-0", {
                        "bg-success": d.status === "completed",
                        "bg-error": d.status === "failed",
                        "bg-info animate-pulse": d.status === "running",
                        "bg-warning": d.status === "rolled-back",
                        "bg-muted-foreground": d.status === "pending",
                      })} />
                      <div className="flex-1 min-w-0">
                        <span className="text-sm font-medium text-foreground">{d.target || "deployment"}</span>
                        <div className="flex items-center gap-3 mt-1">
                          {d.order_number && (
                            <code className="text-xs font-mono text-muted-foreground">#{d.order_number}</code>
                          )}
                        </div>
                      </div>
                      <Badge variant="outline" className={cn("text-xs py-0.5", statusColor(d.status))}>
                        {d.status}
                      </Badge>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    );
  }

  // Instant cluster status check — no waiting for fetch to fail
  if (!clusterStatus.reachable && !loading && error) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Overview</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Cluster health and recent activity</p>
          </div>
          <Badge variant="outline" className="gap-2 text-sm text-brand border-brand/30 bg-brand/5 px-3 py-1">
            <Rocket className="h-3.5 w-3.5" />
            Ready to Deploy
          </Badge>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          <Link to="/app/deploy" className="block">
            <Card className="oven-card feature-card bg-card border-brand/20 hover:border-brand/40 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-brand/10 border border-brand/15">
                    <Rocket className="h-5 w-5 text-brand" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Deploy an App</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Deploy to Docker or Kubernetes. Pick a template, import a docker-compose file, or connect a GitHub repo.
                </p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/clusters" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Server className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Connect a Cluster</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Add a Kubernetes cluster to see live health metrics, running workloads, and scale to production.
                </p>
              </CardContent>
            </Card>
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <Link to="/app/marketplace" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Layers className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Marketplace</h3>
                </div>
                <p className="text-sm text-muted-foreground">Browse 15+ ready-to-deploy recipes</p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/recipes" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Stethoscope className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Recipes</h3>
                </div>
                <p className="text-sm text-muted-foreground">Manage your deployment recipes</p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/doctor" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Activity className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Doctor</h3>
                </div>
                <p className="text-sm text-muted-foreground">Diagnose and fix infrastructure issues</p>
              </CardContent>
            </Card>
          </Link>
        </div>
      </div>
    );
  }

  if (error) {
    // Same action-card layout as the no-cluster state — deploy is always available
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Overview</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Your deployment dashboard</p>
          </div>
          <Badge variant="outline" className="gap-2 text-sm text-brand border-brand/30 bg-brand/5 px-3 py-1">
            <Rocket className="h-3.5 w-3.5" />
            Ready to Deploy
          </Badge>
        </div>
        {error !== "no clusters configured" && (
          <Card className="oven-card feature-card bg-card border-warning/20 mb-6">
            <CardContent className="p-4 flex items-center gap-3">
              <AlertTriangle className="h-4 w-4 text-warning shrink-0" />
              <p className="text-sm text-muted-foreground">A cluster is configured but could not be reached. Check that it is running and accessible.</p>
              <Button onClick={fetchData} variant="outline" size="sm" className="ml-auto border-warning/30 text-warning hover:bg-warning/10 shrink-0">
                <RefreshCw className="h-3.5 w-3.5 mr-1.5" /> Retry
              </Button>
            </CardContent>
          </Card>
        )}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          <Link to="/app/deploy" className="block">
            <Card className="oven-card feature-card bg-card border-brand/20 hover:border-brand/40 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-brand/10 border border-brand/15">
                    <Rocket className="h-5 w-5 text-brand" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Deploy an App</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Deploy to Docker or Kubernetes. Pick a template, import a docker-compose file, or connect a GitHub repo.
                </p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/clusters" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Server className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Connect a Cluster</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Add a Kubernetes cluster for live health metrics, workload monitoring, and production deployments.
                </p>
              </CardContent>
            </Card>
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <Link to="/app/marketplace" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Layers className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Marketplace</h3>
                </div>
                <p className="text-sm text-muted-foreground">Browse 15+ ready-to-deploy recipes</p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/recipes" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Stethoscope className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Recipes</h3>
                </div>
                <p className="text-sm text-muted-foreground">Manage your deployment recipes</p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/doctor" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Activity className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="font-semibold text-foreground">Doctor</h3>
                </div>
                <p className="text-sm text-muted-foreground">Diagnose and fix infrastructure issues</p>
              </CardContent>
            </Card>
          </Link>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Overview</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Your deployment dashboard</p>
          </div>
          <Badge variant="outline" className="gap-2 text-sm text-muted-foreground border-border/30 px-3 py-1">
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            Checking cluster...
          </Badge>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-8">
          <Link to="/app/deploy" className="block">
            <Card className="oven-card feature-card bg-card border-brand/20 hover:border-brand/40 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-brand/10 border border-brand/15">
                    <Rocket className="h-5 w-5 text-brand" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Deploy an App</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Deploy to Docker or Kubernetes. Pick a template, import a docker-compose file, or connect a GitHub repo.
                </p>
              </CardContent>
            </Card>
          </Link>
          <Link to="/app/marketplace" className="block">
            <Card className="oven-card feature-card bg-card border-border/40 hover:border-brand/30 transition-colors cursor-pointer h-full">
              <CardContent className="p-6">
                <div className="flex items-center gap-3 mb-3">
                  <div className="p-2.5 rounded-xl bg-muted/10 border border-border/30">
                    <Layers className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <h3 className="text-lg font-semibold text-foreground">Marketplace</h3>
                </div>
                <p className="text-sm text-muted-foreground">
                  Browse 15+ ready-to-deploy recipes for common stacks.
                </p>
              </CardContent>
            </Card>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-6 py-8 page-enter">

      {/* ── Header ─────────────────────────────────────────── */}
      <div className="flex items-start justify-between mb-10">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">
            Overview
          </h1>
          <p className="text-sm text-muted-foreground mt-1.5">
            Cluster health and recent activity
          </p>
        </div>
        <div className="flex items-center gap-4">
          <Badge
            variant="outline"
            className={cn(
              "gap-2 text-sm px-3 py-1",
              connected
                ? "text-success border-success/30 bg-success/5"
                : "text-error border-error/30 bg-error/5"
            )}
          >
            <span className={cn(
              "h-2 w-2 rounded-full",
              connected ? "bg-success animate-pulse" : "bg-error"
            )} />
            {connected ? "Cluster Connected" : "Cluster Disconnected"}
          </Badge>
          <span className="text-sm text-muted-foreground">
            {nodeCount} node{nodeCount !== 1 ? "s" : ""}{uptime ? ` \u00b7 ${uptime} uptime` : ""}
          </span>
        </div>
      </div>

      {/* ── Warning Events Banner ────────────────────────── */}
      {warningEvents.length > 0 && (
        <div className="mb-10">
          <div
            className="rounded-xl border border-warning/20 bg-warning/5 overflow-hidden"
          >
            <div className="flex items-center gap-3 p-5">
              <Bell className="h-5 w-5 text-warning" />
              <span className="text-sm font-bold text-foreground">
                {warningEvents.length} Warning Event{warningEvents.length > 1 ? "s" : ""}
              </span>
            </div>
            <div className="px-5 pb-5 space-y-3">
              {warningEvents.slice(0, 3).map((ev, i) => (
                <div key={i} className="flex items-start gap-4 p-4 rounded-lg border bg-warning/10 border-warning/30">
                  <AlertTriangle className="h-5 w-5 mt-0.5 shrink-0 text-warning" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3">
                      <span className="text-sm font-semibold text-foreground">{ev.reason}</span>
                      <Badge variant="outline" className="text-xs text-warning">warning</Badge>
                    </div>
                    <p className="text-sm text-muted-foreground mt-1">{ev.message}</p>
                    <div className="flex items-center gap-3 mt-2">
                      <code className="text-xs font-mono text-muted-foreground">{ev.object}</code>
                      <span className="text-xs text-muted-foreground/60">{timeAgo(ev.age)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* ── Stats Row ──────────────────────────────────────── */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-6 mb-10">
        <StatCard
          icon={Server}
          value={nodeCount}
          label="Nodes"
          sub={clusterInfo?.nodes?.map(n => n.status).filter(s => s === "Ready").length + " ready"}
          accent="bg-brand/10"
        />
        <StatCard
          icon={Box}
          value={totalPods}
          label="Pods"
          sub={`${runningPods} running, ${pendingPods} pending`}
          accent="bg-info/10"
        />
        <StatCard
          icon={Rocket}
          value={k8sDeployments.length}
          label="K8s Deployments"
          sub={`across ${new Set(k8sDeployments.map(d => d.namespace)).size} namespace${new Set(k8sDeployments.map(d => d.namespace)).size !== 1 ? "s" : ""}`}
        />
        <StatCard
          icon={Layers}
          value={failedPods}
          label="Failed Pods"
          sub={failedPods === 0 ? "All pods healthy" : "Needs attention"}
          trend={failedPods > 0 ? { value: `${failedPods} failing`, positive: false } : undefined}
        />
      </div>

      {/* ── Divider ────────────────────────────────────────── */}
      <div className="pizza-cutter mb-10" />

      {/* ── Middle Row: Resources + Quick Actions ──────────── */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-10">

        {/* Resource Usage */}
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardHeader className="pb-4 px-6 pt-6">
            <CardTitle className="text-base font-semibold flex items-center gap-2.5">
              <Cpu className="h-5 w-5 text-brand" />
              Resource Usage
            </CardTitle>
          </CardHeader>
          <CardContent className="px-6 pb-6 space-y-6">
            <ResourceBar
              label="CPU"
              used={cpuUsed}
              total={cpuCores}
              unit="cores"
            />
            <ResourceBar
              label="Memory"
              used={memUsed}
              total={memoryGb}
              unit="GB"
            />
          </CardContent>
        </Card>

        {/* Quick Actions */}
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardHeader className="pb-4 px-6 pt-6">
            <CardTitle className="text-base font-semibold flex items-center gap-2.5">
              <Zap className="h-5 w-5 text-brand" />
              Quick Actions
            </CardTitle>
          </CardHeader>
          <CardContent className="px-6 pb-6">
            <div className="grid grid-cols-2 gap-4">
              <Link to="/app/applications">
                <Button
                  variant="outline"
                  className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30"
                >
                  <Box className="h-5 w-5 mr-3 text-brand shrink-0" />
                  <div>
                    <p className="text-sm font-semibold">Applications</p>
                    <p className="text-xs text-muted-foreground mt-0.5">View all apps</p>
                  </div>
                </Button>
              </Link>
              <Link to="/app/deployments">
                <Button
                  variant="outline"
                  className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30"
                >
                  <Rocket className="h-5 w-5 mr-3 text-brand shrink-0" />
                  <div>
                    <p className="text-sm font-semibold">Deploy</p>
                    <p className="text-xs text-muted-foreground mt-0.5">Ship changes</p>
                  </div>
                </Button>
              </Link>
              <Link to="/app/environments">
                <Button
                  variant="outline"
                  className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30"
                >
                  <Layers className="h-5 w-5 mr-3 text-brand shrink-0" />
                  <div>
                    <p className="text-sm font-semibold">Environments</p>
                    <p className="text-xs text-muted-foreground mt-0.5">Manage namespaces</p>
                  </div>
                </Button>
              </Link>
              <Link to="/app/doctor">
                <Button
                  variant="outline"
                  className="w-full justify-start h-auto py-4 px-4 text-left feature-card border-border/50 hover:border-brand/30"
                >
                  <Stethoscope className="h-5 w-5 mr-3 text-brand shrink-0" />
                  <div>
                    <p className="text-sm font-semibold">Doctor</p>
                    <p className="text-xs text-muted-foreground mt-0.5">Run diagnostics</p>
                  </div>
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* ── Divider ────────────────────────────────────────── */}
      <div className="pizza-cutter mb-10" />

      {/* ── Bottom Row: Deployments + Production Apps ──────── */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-10">

        {/* Recent Deployments (from Mozza's deployment history) */}
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardHeader className="px-6 pt-6 pb-4 flex flex-row items-center justify-between">
            <CardTitle className="text-base font-semibold flex items-center gap-2.5">
              <GitCommit className="h-5 w-5 text-brand" />
              Recent Deployments
            </CardTitle>
            <Link to="/app/deployments">
              <Button variant="ghost" size="sm" className="text-xs text-muted-foreground hover:text-foreground h-8">
                View all <ChevronRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent className="px-6 pb-6">
            {recentDeploys.length === 0 ? (
              <div className="py-8 text-center">
                <Rocket className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
                <p className="text-sm text-muted-foreground">
                  {activeTeam ? "No deployments yet" : "Select a team to view deployments"}
                </p>
              </div>
            ) : (
              <div className="space-y-0">
                {recentDeploys.map((d, i) => (
                  <div
                    key={d.id}
                    className={cn(
                      "flex items-center gap-4 py-4 transition-colors",
                      i < recentDeploys.length - 1 && "border-b border-border/40"
                    )}
                  >
                    <div className={cn("h-2.5 w-2.5 rounded-full shrink-0", {
                      "bg-success": d.status === "completed",
                      "bg-error": d.status === "failed",
                      "bg-info animate-pulse": d.status === "running",
                      "bg-warning": d.status === "rolled-back",
                      "bg-muted-foreground": d.status === "pending",
                    })} />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2.5">
                        <span className="text-sm font-medium text-foreground">
                          {d.target || "deployment"}
                        </span>
                        <Badge
                          variant="outline"
                          className="text-xs py-0.5 px-2"
                        >
                          {d.environment}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-3 mt-1">
                        {d.order_number && (
                          <code className="text-xs font-mono text-muted-foreground">
                            #{d.order_number}
                          </code>
                        )}
                      </div>
                    </div>
                    <div className="text-right shrink-0">
                      <Badge
                        variant="outline"
                        className={cn("text-xs py-0.5", statusColor(d.status))}
                      >
                        {d.status}
                      </Badge>
                      <p className="text-xs text-muted-foreground/50 mt-1">
                        {relativeTime(d.started_at)}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Production Apps (real K8s deployments) */}
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardHeader className="px-6 pt-6 pb-4 flex flex-row items-center justify-between">
            <CardTitle className="text-base font-semibold flex items-center gap-2.5">
              <Shield className="h-5 w-5 text-brand" />
              Cluster Deployments
            </CardTitle>
            <Link to="/app/applications">
              <Button variant="ghost" size="sm" className="text-xs text-muted-foreground hover:text-foreground h-8">
                View all <ChevronRight className="h-3.5 w-3.5 ml-1" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent className="px-6 pb-6">
            {k8sDeployments.length === 0 ? (
              <div className="py-8 text-center">
                <Box className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
                <p className="text-sm text-muted-foreground">No deployments found in cluster</p>
              </div>
            ) : (
              <div className="space-y-0">
                {k8sDeployments.slice(0, 6).map((dep, i) => {
                  const parts = dep.ready.split("/").map(Number);
                  const ready = parts[0] ?? 0;
                  const desired = parts[1] ?? 0;
                  const isHealthy = ready === desired && desired > 0;
                  return (
                    <div
                      key={`${dep.namespace}-${dep.name}`}
                      className={cn(
                        "flex items-center gap-4 py-4 -mx-3 px-3 rounded-lg transition-colors",
                        i < Math.min(k8sDeployments.length, 6) - 1 && "border-b border-border/40"
                      )}
                    >
                      <div className={cn("h-2.5 w-2.5 rounded-full shrink-0", {
                        "bg-success": isHealthy,
                        "bg-warning": !isHealthy && ready > 0,
                        "bg-error": ready === 0,
                      })} />
                      <div className="flex-1 min-w-0">
                        <span className="text-sm font-medium text-foreground">
                          {dep.name}
                        </span>
                        <div className="flex items-center gap-3 mt-1">
                          <Badge variant="outline" className="text-xs py-0 px-1.5 text-muted-foreground">
                            {dep.namespace}
                          </Badge>
                          <span className="text-xs text-muted-foreground/50">
                            {dep.ready} pods ready
                          </span>
                        </div>
                      </div>
                      <div className="text-right shrink-0">
                        <Badge
                          variant="outline"
                          className={cn("text-xs py-0.5", isHealthy ? "text-success" : "text-warning")}
                        >
                          {isHealthy ? "healthy" : "degraded"}
                        </Badge>
                        <p className="text-xs text-muted-foreground/50 mt-1">
                          {timeAgo(dep.age)}
                        </p>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* ── Divider ────────────────────────────────────────── */}
      <div className="pizza-cutter mb-10" />

      {/* ── Activity Feed (real K8s events) ───────────────── */}
      <Card className="oven-card feature-card bg-card border-border/50 mb-10">
        <CardHeader className="px-6 pt-6 pb-4 flex flex-row items-center justify-between">
          <CardTitle className="text-base font-semibold flex items-center gap-2.5">
            <Activity className="h-5 w-5 text-brand" />
            Activity
          </CardTitle>
          {events.length > 6 && (
            <Button
              variant="ghost"
              size="sm"
              className="text-xs text-muted-foreground hover:text-foreground h-8"
              onClick={() => setShowAllEvents(!showAllEvents)}
            >
              {showAllEvents ? "Show less" : `Show all (${events.length})`}
            </Button>
          )}
        </CardHeader>
        <CardContent className="px-6 pb-6">
          {events.length === 0 ? (
            <div className="py-8 text-center">
              <Activity className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
              <p className="text-sm text-muted-foreground">No recent events</p>
            </div>
          ) : (
            <div className="space-y-0">
              {displayEvents.map((e, i) => (
                <EventRow key={`${e.namespace}-${e.object}-${i}`} event={e} odd={i % 2 === 1} />
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
