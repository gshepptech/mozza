import { useState, useEffect, useMemo, useCallback } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getClusterDeployments, getClusterPods, getClusterNamespaces, listClusters } from "@/api/client";
import type { ClusterDeployment, ClusterPod, ClusterNamespace } from "@/api/types";
import {
  Box, Search, Grid3X3, List, Layers, Plus,
  Server, CheckCircle, AlertTriangle, XCircle, Loader2,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { statusColor, statusBgColor } from "@/lib/format";

type AppStatus = "healthy" | "degraded" | "failing";

interface AppSummary {
  id: string;
  name: string;
  namespace: string;
  image: string;
  age: string;
  readyPods: number;
  totalPods: number;
  status: AppStatus;
  ready: string;
}

function deriveStatus(readyPods: number, totalPods: number): AppStatus {
  if (totalPods === 0) return "failing";
  if (readyPods === totalPods) return "healthy";
  if (readyPods === 0) return "failing";
  return "degraded";
}

function buildAppSummaries(
  deployments: ClusterDeployment[],
  pods: ClusterPod[],
): AppSummary[] {
  return deployments.map((dep) => {
    const depPods = pods.filter(
      (p) => p.namespace === dep.namespace && p.app === dep.name,
    );
    const readyPods = depPods.filter((p) => {
      const parts = p.ready.split("/");
      return parts.length === 2 && parts[0] === parts[1] && parts[0] !== "0";
    }).length;
    const totalPods = depPods.length;

    // Fall back to the deployment's own ready string if no pods matched
    const readyParts = dep.ready.split("/");
    const effectiveReady = totalPods > 0 ? readyPods : parseInt(readyParts[0] ?? "0", 10);
    const effectiveTotal = totalPods > 0 ? totalPods : parseInt(readyParts[1] ?? "0", 10);

    return {
      id: `${dep.namespace}--${dep.name}`,
      name: dep.name,
      namespace: dep.namespace,
      image: dep.image || "unknown",
      age: dep.age,
      readyPods: effectiveReady,
      totalPods: effectiveTotal,
      status: deriveStatus(effectiveReady, effectiveTotal),
      ready: dep.ready,
    };
  });
}

const statusIcons: Record<string, typeof CheckCircle> = {
  healthy: CheckCircle,
  degraded: AlertTriangle,
  failing: XCircle,
};

export default function ApplicationsPage() {
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [nsFilter, setNsFilter] = useState<string>("all");
  const [view, setView] = useState<"grid" | "list">("grid");

  const [deployments, setDeployments] = useState<ClusterDeployment[]>([]);
  const [pods, setPods] = useState<ClusterPod[]>([]);
  const [namespaces, setNamespaces] = useState<ClusterNamespace[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasCluster, setHasCluster] = useState<boolean | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const { clusters } = await listClusters();
      const hasClusters = (clusters ?? []).length > 0;
      setHasCluster(hasClusters);

      if (!hasClusters) {
        setLoading(false);
        return;
      }

      const [depRes, podRes, nsRes] = await Promise.all([
        getClusterDeployments(),
        getClusterPods(),
        getClusterNamespaces(),
      ]);
      setDeployments(depRes.deployments);
      setPods(podRes.pods);
      setNamespaces(nsRes.namespaces);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load cluster data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const apps = useMemo(
    () => buildAppSummaries(deployments, pods),
    [deployments, pods],
  );

  const filtered = useMemo(
    () =>
      apps
        .filter((a) => search === "" || a.name.toLowerCase().includes(search.toLowerCase()))
        .filter((a) => nsFilter === "all" || a.namespace === nsFilter),
    [apps, search, nsFilter],
  );

  const activeNamespaces = useMemo(
    () => namespaces.filter((ns) => ns.status === "Active"),
    [namespaces],
  );

  const nsCounts = useMemo(() => {
    const counts: Record<string, number> = { all: apps.length };
    for (const ns of activeNamespaces) {
      counts[ns.name] = apps.filter((a) => a.namespace === ns.name).length;
    }
    return counts;
  }, [apps, activeNamespaces]);

  // Only show namespace filters that have deployments, plus "all"
  const filterNamespaces = useMemo(
    () => activeNamespaces.filter((ns) => (nsCounts[ns.name] || 0) > 0),
    [activeNamespaces, nsCounts],
  );

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-20 page-enter">
        <Loader2 className="h-8 w-8 text-brand animate-spin mb-4" />
        <p className="text-sm text-muted-foreground">Loading applications...</p>
      </div>
    );
  }

  if (!loading && hasCluster === false) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Applications</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Manage your deployed applications</p>
          </div>
        </div>
        <Card className="oven-card feature-card bg-card border-brand/20">
          <CardContent className="p-8 text-center">
            <Server className="h-12 w-12 text-brand mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-foreground mb-2">No cluster connected</h2>
            <p className="text-sm text-muted-foreground mb-6 max-w-md mx-auto">
              Connect a Kubernetes cluster to see your running applications, monitor pod health, and manage deployments.
            </p>
            <div className="flex items-center justify-center gap-4">
              <Link to="/app/clusters">
                <Button className="bg-brand text-white hover:bg-brand/90">
                  <Plus className="h-4 w-4 mr-2" /> Connect a Cluster
                </Button>
              </Link>
              <Link to="/app/marketplace">
                <Button variant="outline" className="border-brand/30 text-brand hover:bg-brand/10">
                  <Layers className="h-4 w-4 mr-2" /> Browse Templates
                </Button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center py-20 page-enter">
        <XCircle className="h-10 w-10 text-error mb-4" />
        <h2 className="text-lg font-semibold text-foreground mb-2">No cluster connected</h2>
        <p className="text-sm text-muted-foreground mb-4 max-w-md text-center">
          Connect a Kubernetes cluster to see your applications. Check your cluster connection or add a new one.
        </p>
        <div className="flex gap-3">
          <Button variant="outline" size="sm" onClick={fetchData}>
            Retry
          </Button>
          <a href="/app/clusters">
            <Button size="sm">Manage Clusters</Button>
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
      {/* Header */}
      <div className="flex items-start justify-between mb-10">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">Applications</h1>
          <p className="text-sm text-muted-foreground mt-1.5">
            {filtered.length} application{filtered.length !== 1 ? "s" : ""} across{" "}
            {new Set(filtered.map((a) => a.namespace)).size} namespace
            {new Set(filtered.map((a) => a.namespace)).size !== 1 ? "s" : ""}
          </p>
        </div>
        <Button
          className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.15)]"
          onClick={() => navigate("/app/deploy")}
        >
          <Box className="mr-2 h-4 w-4" />
          New Application
        </Button>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 mb-8">
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search applications..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-10 h-10 text-sm bg-elevated"
          />
        </div>
        <div className="flex gap-1.5 flex-wrap">
          <Button
            variant={nsFilter === "all" ? "default" : "outline"}
            size="sm"
            className={cn(
              "text-xs h-9 px-3",
              nsFilter === "all" && "bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_8px_rgba(255,107,53,0.2)]",
            )}
            onClick={() => setNsFilter("all")}
          >
            All
            <Badge variant="secondary" className="ml-1.5 text-[10px] h-4 px-1">
              {nsCounts.all}
            </Badge>
          </Button>
          {filterNamespaces.map((ns) => (
            <Button
              key={ns.name}
              variant={nsFilter === ns.name ? "default" : "outline"}
              size="sm"
              className={cn(
                "text-xs h-9 px-3",
                nsFilter === ns.name && "bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_8px_rgba(255,107,53,0.2)]",
              )}
              onClick={() => setNsFilter(ns.name)}
            >
              {ns.name}
              <Badge variant="secondary" className="ml-1.5 text-[10px] h-4 px-1">
                {nsCounts[ns.name] || 0}
              </Badge>
            </Button>
          ))}
        </div>
        <div className="ml-auto flex gap-1">
          <Button
            variant={view === "grid" ? "default" : "ghost"}
            size="icon"
            className={cn("h-9 w-9", view === "grid" && "bg-brand hover:bg-brand-hover text-primary-foreground")}
            onClick={() => setView("grid")}
          >
            <Grid3X3 className="h-4 w-4" />
          </Button>
          <Button
            variant={view === "list" ? "default" : "ghost"}
            size="icon"
            className={cn("h-9 w-9", view === "list" && "bg-brand hover:bg-brand-hover text-primary-foreground")}
            onClick={() => setView("list")}
          >
            <List className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {/* Divider */}
      <div className="pizza-cutter mb-8" />

      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16">
          <Server className="h-10 w-10 text-muted-foreground mb-4" />
          <h3 className="text-base font-semibold text-foreground mb-1">No applications found</h3>
          <p className="text-sm text-muted-foreground">
            {search ? "Try a different search term." : "No deployments detected in the cluster."}
          </p>
        </div>
      ) : view === "grid" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filtered.map((app) => {
            const StatusIcon = statusIcons[app.status] || Server;
            return (
              <Link key={app.id} to={`/app/applications/${app.id}`}>
                <Card className="oven-card feature-card bg-card border-border/50 cursor-pointer h-full">
                  <CardContent className="p-6">
                    {/* Header */}
                    <div className="flex items-start justify-between mb-5">
                      <div className="flex items-center gap-3">
                        <div className={cn("p-2 rounded-xl border", statusBgColor(app.status))}>
                          <StatusIcon className={cn("h-4 w-4", statusColor(app.status))} />
                        </div>
                        <div>
                          <h3 className="text-sm font-bold text-foreground">{app.name}</h3>
                          <p className="text-[11px] text-muted-foreground font-mono">{app.namespace}</p>
                        </div>
                      </div>
                      <Badge variant="outline" className="text-[10px]">
                        {app.namespace}
                      </Badge>
                    </div>

                    {/* Pods */}
                    <div className="mb-4">
                      <div className="flex items-center justify-between mb-1.5">
                        <span className="text-xs text-muted-foreground font-medium">Pods</span>
                        <span className="text-xs font-mono text-foreground">
                          {app.readyPods}/{app.totalPods}
                        </span>
                      </div>
                      <div className="flex gap-1">
                        {app.totalPods > 0 ? (
                          Array.from({ length: app.totalPods }, (_, i) => (
                            <div
                              key={i}
                              className={cn(
                                "h-2 flex-1 rounded-full",
                                i < app.readyPods ? "bg-success" : "bg-error/60",
                              )}
                            />
                          ))
                        ) : (
                          <div className="h-2 flex-1 rounded-full bg-muted/40" />
                        )}
                      </div>
                    </div>

                    {/* Image */}
                    <div className="mb-4">
                      <span className="text-xs text-muted-foreground font-medium">Image</span>
                      <p className="text-[11px] font-mono text-foreground truncate mt-0.5">
                        {app.image}
                      </p>
                    </div>

                    {/* Footer */}
                    <div className="flex items-center justify-between pt-4 border-t border-border/40">
                      <Badge
                        variant="outline"
                        className={cn("text-[10px]", statusColor(app.status))}
                      >
                        {app.status}
                      </Badge>
                      <span className="text-[11px] text-muted-foreground">{app.age}</span>
                    </div>
                  </CardContent>
                </Card>
              </Link>
            );
          })}
        </div>
      ) : (
        /* List view */
        <Card className="oven-card bg-card border-border/50 overflow-hidden">
          <div className="grid grid-cols-[1fr_120px_80px_80px_1fr_80px] gap-4 px-6 py-3 bg-muted/30 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground border-b border-border/50">
            <span>Application</span>
            <span>Namespace</span>
            <span>Status</span>
            <span>Pods</span>
            <span>Image</span>
            <span>Age</span>
          </div>
          {filtered.map((app) => {
            const StatusIcon = statusIcons[app.status] || Server;
            return (
              <Link
                key={app.id}
                to={`/app/applications/${app.id}`}
                className="grid grid-cols-[1fr_120px_80px_80px_1fr_80px] gap-4 px-6 py-4 border-b border-border/30 last:border-0 hover:bg-muted/20 transition-colors items-center"
              >
                <div className="flex items-center gap-3">
                  <StatusIcon className={cn("h-4 w-4 shrink-0", statusColor(app.status))} />
                  <div>
                    <span className="text-sm font-medium text-foreground">{app.name}</span>
                    <p className="text-[10px] text-muted-foreground font-mono">{app.namespace}</p>
                  </div>
                </div>
                <Badge variant="outline" className="text-[10px] justify-center w-fit">
                  {app.namespace}
                </Badge>
                <Badge variant="outline" className={cn("text-[10px] justify-center w-fit", statusColor(app.status))}>
                  {app.status}
                </Badge>
                <span className="text-xs font-mono">{app.readyPods}/{app.totalPods}</span>
                <code className="text-[10px] font-mono text-muted-foreground truncate">{app.image}</code>
                <span className="text-xs text-muted-foreground">{app.age}</span>
              </Link>
            );
          })}
        </Card>
      )}
    </div>
  );
}
