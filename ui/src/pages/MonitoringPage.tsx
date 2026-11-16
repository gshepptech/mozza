import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import {
  getClusterMetrics, getClusterPods, getClusterEvents, getClusterNodes,
} from "@/api/client";
import type { ClusterMetrics, ClusterPod, ClusterEvent, ClusterInfo } from "@/api/types";
import {
  Activity, Cpu, HardDrive, Server, AlertTriangle, CheckCircle,
  Bell, RefreshCw, Loader2,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { statusColor, statusBgColor } from "@/lib/format";
import { AppHealth, deriveHealthStatus } from "@/components/monitoring/AppHealth";
import { AlertBadge, resourceAlertLevel } from "@/components/monitoring/AlertBadge";

// ── Types ────────────────────────────────────────────────────────

interface AppSummary {
  name: string;
  namespace: string;
  pods: ClusterPod[];
  runningCount: number;
  totalCount: number;
  restartCount: number;
}

// ── Group pods by app label ──────────────────────────────────────

function groupPodsByApp(pods: ClusterPod[]): AppSummary[] {
  const groups = new Map<string, ClusterPod[]>();
  for (const pod of pods) {
    const appName = pod.app || pod.name.replace(/-[a-z0-9]{5,10}(-[a-z0-9]{5})?$/, "");
    const key = `${pod.namespace}/${appName}`;
    const existing = groups.get(key);
    if (existing) {
      existing.push(pod);
    } else {
      groups.set(key, [pod]);
    }
  }

  const summaries: AppSummary[] = [];
  for (const [key, appPods] of groups) {
    const slashIdx = key.indexOf("/");
    const namespace = slashIdx >= 0 ? key.slice(0, slashIdx) : "";
    const name = slashIdx >= 0 ? key.slice(slashIdx + 1) : key;
    const running = appPods.filter((p) => p.status === "Running").length;
    const restarts = appPods.reduce((s, p) => s + p.restarts, 0);

    summaries.push({
      name,
      namespace,
      pods: appPods,
      runningCount: running,
      totalCount: appPods.length,
      restartCount: restarts,
    });
  }
  return summaries.sort((a, b) => b.totalCount - a.totalCount);
}

// ── Pod Health Grid ──────────────────────────────────────────────

function PodGrid({ pods }: { pods: ClusterPod[] }) {
  if (pods.length === 0) {
    return (
      <div className="py-6 text-center">
        <Server className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
        <p className="text-sm text-muted-foreground">No pods found</p>
      </div>
    );
  }

  return (
    <div className="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 lg:grid-cols-10 gap-1.5">
      {pods.map(pod => {
        const bg = pod.status === "Running" ? "bg-success" :
                   pod.status === "Pending" ? "bg-info" :
                   ["CrashLoopBackOff", "ImagePullBackOff", "Failed"].includes(pod.status) ? "bg-error" :
                   pod.status === "Succeeded" || pod.status === "Completed" ? "bg-muted-foreground" : "bg-muted";
        return (
          <div key={`${pod.namespace}-${pod.name}`} className="group relative">
            <div className={cn("h-6 rounded-sm transition-all cursor-pointer hover:scale-110", bg, pod.status === "Pending" && "animate-pulse")} />
            <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1 hidden group-hover:block z-10">
              <div className="bg-popover border border-border rounded-lg shadow-lg px-3 py-2 whitespace-nowrap">
                <p className="text-xs font-semibold font-mono">{pod.name}</p>
                <p className={cn("text-[10px]", statusColor(pod.status))}>{pod.status}</p>
                <p className="text-[10px] text-muted-foreground">{pod.namespace} | {pod.ready} ready</p>
                {pod.restarts > 0 && (
                  <p className="text-[10px] text-warning">{pod.restarts} restart{pod.restarts !== 1 ? "s" : ""}</p>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ── App Card (grid item) ─────────────────────────────────────────

function AppCard({ app }: { app: AppSummary }) {
  const health = deriveHealthStatus(app.runningCount, app.totalCount);

  return (
    <Card className="bg-card">
      <CardContent className="pt-4 pb-3 px-4">
        <div className="mb-3">
          <AppHealth
            status={health}
            appName={app.name}
            details={`${app.runningCount}/${app.totalCount} pods running`}
          />
        </div>
        <p className="text-[10px] text-muted-foreground">{app.namespace}</p>
        <p className="text-[10px] text-muted-foreground mt-0.5">
          {app.runningCount}/{app.totalCount} pods
          {app.restartCount > 0 && (
            <span className="text-warning ml-1">({app.restartCount} restarts)</span>
          )}
        </p>
      </CardContent>
    </Card>
  );
}

// ── Main Page ────────────────────────────────────────────────────

const AUTO_REFRESH_MS = 30000;

export default function MonitoringPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [metrics, setMetrics] = useState<ClusterMetrics | null>(null);
  const [clusterInfo, setClusterInfo] = useState<ClusterInfo | null>(null);
  const [pods, setPods] = useState<ClusterPod[]>([]);
  const [events, setEvents] = useState<ClusterEvent[]>([]);
  const [alertFilter, setAlertFilter] = useState<"all" | "Warning" | "Normal">("all");
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(async (isRefresh = false) => {
    if (isRefresh) {
      setRefreshing(true);
    } else {
      setLoading(true);
    }
    setError(null);
    try {
      const [metricsRes, clusterRes, podsRes, eventsRes] = await Promise.all([
        getClusterMetrics(),
        getClusterNodes(),
        getClusterPods(),
        getClusterEvents(),
      ]);
      setMetrics(metricsRes);
      setClusterInfo(clusterRes);
      setPods(podsRes.pods);
      setEvents(eventsRes.events);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to connect to cluster");
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Auto-refresh every 30s when page is visible
  useEffect(() => {
    function startTimer() {
      if (timerRef.current) clearInterval(timerRef.current);
      timerRef.current = setInterval(() => {
        if (document.visibilityState === "visible") {
          fetchData(true);
        }
      }, AUTO_REFRESH_MS);
    }

    function handleVisibility() {
      if (document.visibilityState === "visible") {
        fetchData(true);
        startTimer();
      } else if (timerRef.current) {
        clearInterval(timerRef.current);
        timerRef.current = null;
      }
    }

    startTimer();
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, [fetchData]);

  const filteredEvents = events.filter(e => alertFilter === "all" || e.type === alertFilter);
  const warningCount = events.filter(e => e.type === "Warning").length;

  const cpuPercent = metrics?.cpu_percent ?? 0;
  const cpuCores = metrics?.cpu_cores ?? 0;
  const cpuUsed = (cpuPercent / 100) * cpuCores;
  const memPercent = metrics?.memory_percent ?? 0;
  const memGb = metrics?.memory_gb ?? 0;
  const memUsed = (memPercent / 100) * memGb;
  const nodeCount = metrics?.nodes ?? clusterInfo?.nodes?.length ?? 0;
  const uptime = metrics?.uptime ?? "";
  const connected = clusterInfo?.connected ?? false;
  const readyNodes = clusterInfo?.nodes?.filter(n => n.status === "Ready").length ?? nodeCount;

  const appSummaries = useMemo(() => groupPodsByApp(pods), [pods]);

  if (error && !metrics) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Monitoring</h1>
            <p className="text-sm text-muted-foreground mt-0.5">Cluster health, resources, and alerts</p>
          </div>
        </div>
        <Card className="oven-card feature-card bg-card border-brand/20">
          <CardContent className="p-8 text-center">
            <Activity className="h-12 w-12 text-brand mx-auto mb-4" />
            <h2 className="text-lg font-semibold text-foreground mb-2">No monitoring data yet</h2>
            <p className="text-sm text-muted-foreground mb-6 max-w-md mx-auto">
              Monitoring reads CPU, memory, pods and events from a connected Kubernetes cluster. Connect one, or check that it is reachable, then refresh.
            </p>
            <div className="flex gap-3 justify-center">
              <a href="/app/deploy">
                <Button className="bg-brand hover:bg-brand/90 text-white">
                  <Activity className="h-4 w-4 mr-2" /> Deploy an App
                </Button>
              </a>
              <Button onClick={() => fetchData(false)} variant="outline" className="border-brand/30 text-brand hover:bg-brand/10">
                <RefreshCw className="h-4 w-4 mr-2" /> Refresh
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (loading) {
    return (
      <div>
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-xl font-semibold text-foreground">Monitoring</h1>
            <p className="text-sm text-muted-foreground mt-0.5">Cluster health, resources, and alerts</p>
          </div>
        </div>
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-brand" />
          <span className="ml-3 text-sm text-muted-foreground">Loading cluster data...</span>
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Monitoring</h1>
          <p className="text-sm text-muted-foreground mt-0.5">Cluster health, resources, and alerts</p>
        </div>
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => fetchData(true)}
            disabled={refreshing}
          >
            <RefreshCw className={cn("mr-1.5 h-3.5 w-3.5", refreshing && "animate-spin")} />
            {refreshing ? "Refreshing..." : "Refresh"}
          </Button>
        </div>
      </div>

      {/* Resource overview cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
        <Card className="oven-card bg-card">
          <CardContent className="pt-4 pb-3 px-4">
            <div className="flex items-center gap-2 mb-2">
              <Cpu className="h-4 w-4 text-brand" />
              <span className="text-xs font-medium text-foreground">CPU</span>
              <AlertBadge level={resourceAlertLevel(cpuPercent)} />
            </div>
            <p className="text-2xl font-bold text-foreground">{Math.round(cpuPercent)}%</p>
            <div className="h-1.5 rounded-full bg-muted overflow-hidden mt-2">
              <div
                className={cn(
                  "h-full rounded-full",
                  cpuPercent > 80 ? "bg-error" : cpuPercent > 60 ? "bg-warning" : "bg-brand"
                )}
                style={{ width: `${cpuPercent}%` }}
              />
            </div>
            <p className="text-[10px] text-muted-foreground mt-1">
              {cpuUsed.toFixed(1)} / {cpuCores} cores
            </p>
          </CardContent>
        </Card>
        <Card className="oven-card bg-card">
          <CardContent className="pt-4 pb-3 px-4">
            <div className="flex items-center gap-2 mb-2">
              <HardDrive className="h-4 w-4 text-brand" />
              <span className="text-xs font-medium text-foreground">Memory</span>
              <AlertBadge level={resourceAlertLevel(memPercent)} />
            </div>
            <p className="text-2xl font-bold text-foreground">{Math.round(memPercent)}%</p>
            <div className="h-1.5 rounded-full bg-muted overflow-hidden mt-2">
              <div
                className={cn(
                  "h-full rounded-full",
                  memPercent > 80 ? "bg-error" : memPercent > 60 ? "bg-warning" : "bg-brand"
                )}
                style={{ width: `${memPercent}%` }}
              />
            </div>
            <p className="text-[10px] text-muted-foreground mt-1">
              {memUsed.toFixed(1)} / {memGb.toFixed(1)} GB
            </p>
          </CardContent>
        </Card>
        <Card className="oven-card bg-card">
          <CardContent className="pt-4 pb-3 px-4">
            <div className="flex items-center gap-2 mb-2">
              <Server className="h-4 w-4 text-brand" />
              <span className="text-xs font-medium text-foreground">Nodes</span>
            </div>
            <p className="text-2xl font-bold text-foreground">{nodeCount}</p>
            <p className="text-[10px] text-muted-foreground mt-2">
              {readyNodes === nodeCount
                ? "All healthy"
                : `${readyNodes}/${nodeCount} ready`}
            </p>
            {!connected && (
              <p className="text-[10px] text-error mt-1">Cluster disconnected</p>
            )}
          </CardContent>
        </Card>
        <Card className="oven-card bg-card">
          <CardContent className="pt-4 pb-3 px-4">
            <div className="flex items-center gap-2 mb-2">
              <Activity className="h-4 w-4 text-brand" />
              <span className="text-xs font-medium text-foreground">Uptime</span>
            </div>
            <p className="text-2xl font-bold text-foreground">{uptime || "N/A"}</p>
            <p className="text-[10px] text-muted-foreground mt-2">
              {connected ? "Cluster connected" : "Unknown"}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Per-app grid */}
      {appSummaries.length > 0 ? (
        <Card className="bg-card mb-6">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <Activity className="h-4 w-4 text-brand" />
              Applications
              <Badge variant="outline" className="text-[10px] ml-1 text-muted-foreground">
                {appSummaries.length} app{appSummaries.length !== 1 ? "s" : ""}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {appSummaries.map((app) => (
                <AppCard key={`${app.namespace}/${app.name}`} app={app} />
              ))}
            </div>
          </CardContent>
        </Card>
      ) : (
        <Card className="bg-card mb-6">
          <CardContent className="py-8 text-center">
            <Server className="h-10 w-10 text-muted-foreground/30 mx-auto mb-3" />
            <p className="text-sm text-muted-foreground">No monitoring data yet</p>
            <p className="text-[10px] text-muted-foreground/60 mt-1">
              Deploy an application to see its pods here
            </p>
          </CardContent>
        </Card>
      )}

      {/* Pod health grid */}
      <Card className="bg-card mb-6">
        <CardHeader className="pb-3">
          <CardTitle className="text-sm flex items-center gap-2">
            <Server className="h-4 w-4 text-brand" />
            Pod Health Map
            <Badge variant="outline" className="text-[10px] ml-1 text-muted-foreground">
              {pods.length} pod{pods.length !== 1 ? "s" : ""}
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <PodGrid pods={pods} />
          <div className="flex items-center gap-4 mt-3 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-sm bg-success" /> Running</span>
            <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-sm bg-info" /> Pending</span>
            <span className="flex items-center gap-1"><span className="h-2.5 w-2.5 rounded-sm bg-error" /> Failed</span>
          </div>
        </CardContent>
      </Card>

      {/* Events / Alerts */}
      <Card className="bg-card">
        <CardHeader className="pb-3 flex flex-row items-center justify-between">
          <CardTitle className="text-sm flex items-center gap-2">
            <Bell className="h-4 w-4 text-brand" />
            Cluster Events
            {warningCount > 0 && (
              <Badge variant="outline" className="text-error border-error/30 text-[10px] ml-1">
                {warningCount} warning{warningCount !== 1 ? "s" : ""}
              </Badge>
            )}
          </CardTitle>
          <div className="flex gap-1">
            {(["all", "Warning", "Normal"] as const).map(f => (
              <Button
                key={f}
                variant={alertFilter === f ? "default" : "ghost"}
                size="sm"
                className="text-[10px] h-6 px-2"
                onClick={() => setAlertFilter(f)}
              >
                {f === "all" ? "all" : f.toLowerCase()}
              </Button>
            ))}
          </div>
        </CardHeader>
        <CardContent className="space-y-2">
          {filteredEvents.length === 0 ? (
            <div className="py-6 text-center">
              <CheckCircle className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
              <p className="text-sm text-muted-foreground">No events to display</p>
            </div>
          ) : (
            filteredEvents.slice(0, 20).map((event, idx) => {
              const icons: Record<string, typeof AlertTriangle> = {
                Warning: AlertTriangle,
                Normal: CheckCircle,
              };
              const colors: Record<string, string> = {
                Warning: "text-warning",
                Normal: "text-info",
              };
              const Icon = icons[event.type] || CheckCircle;
              const isWarning = event.type === "Warning";
              return (
                <div key={`${event.namespace}-${event.object}-${idx}`} className={cn(
                  "flex items-start gap-3 p-3 rounded-lg border",
                  statusBgColor(isWarning ? "warning" : "Normal"),
                )}>
                  <Icon className={cn("h-4 w-4 mt-0.5 shrink-0", colors[event.type] || "text-info")} />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-semibold text-foreground">{event.reason}</span>
                      <Badge variant="outline" className={cn("text-[10px]", colors[event.type] || "text-info")}>
                        {event.type.toLowerCase()}
                      </Badge>
                      {event.count > 1 && (
                        <Badge variant="outline" className="text-[10px] text-muted-foreground">
                          x{event.count}
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">{event.message}</p>
                    <div className="flex items-center gap-2 mt-1">
                      <code className="text-[10px] font-mono text-muted-foreground">{event.object}</code>
                      <span className="text-[10px] text-muted-foreground/50">{event.namespace}</span>
                      <span className="text-[10px] text-muted-foreground/50">{event.age}</span>
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </CardContent>
      </Card>
    </div>
  );
}
