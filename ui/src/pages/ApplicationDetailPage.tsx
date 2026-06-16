import { useState, useEffect, useCallback } from "react";
import { useParams, Link } from "react-router-dom";
import {
  getClusterDeployments,
  getClusterPods,
  getClusterServices,
  getClusterPodLogs,
  restartDeployment,
} from "@/api/client";
import type { ClusterDeployment, ClusterPod, ClusterService } from "@/api/types";
import {
  ArrowLeft, RefreshCw, MoreHorizontal,
  CheckCircle, AlertTriangle, XCircle, Loader2, Server,
  Box, Network, Copy, Terminal,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { statusColor, statusBgColor } from "@/lib/format";

type AppStatus = "healthy" | "degraded" | "failing";

function deriveStatus(readyPods: number, totalPods: number): AppStatus {
  if (totalPods === 0) return "failing";
  if (readyPods === totalPods) return "healthy";
  if (readyPods === 0) return "failing";
  return "degraded";
}

const podStatusIcons: Record<string, typeof CheckCircle> = {
  Running: CheckCircle,
  Pending: Loader2,
  CrashLoopBackOff: XCircle,
  ImagePullBackOff: XCircle,
  Completed: CheckCircle,
  Terminating: AlertTriangle,
  Error: XCircle,
  Failed: XCircle,
};

function parseReadyFraction(ready: string): { ready: number; total: number } {
  const parts = ready.split("/");
  if (parts.length === 2) {
    return { ready: parseInt(parts[0] ?? "0", 10), total: parseInt(parts[1] ?? "0", 10) };
  }
  return { ready: 0, total: 0 };
}

export default function ApplicationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [tab, setTab] = useState("overview");

  const [deployment, setDeployment] = useState<ClusterDeployment | null>(null);
  const [appPods, setAppPods] = useState<ClusterPod[]>([]);
  const [services, setServices] = useState<ClusterService[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Logs state
  const [selectedPod, setSelectedPod] = useState<string | null>(null);
  const [logs, setLogs] = useState<string>("");
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);

  // Parse namespace--name from the id
  const idParts = id?.split("--") || [];
  const namespace = idParts[0] || "";
  const deploymentName = idParts.slice(1).join("--") || "";

  const loadData = useCallback(async () => {
    if (!namespace || !deploymentName) {
      setError("Invalid application ID");
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const [depRes, podRes, svcRes] = await Promise.all([
        getClusterDeployments(namespace),
        getClusterPods(namespace),
        getClusterServices(namespace),
      ]);

      const dep = depRes.deployments.find((d) => d.name === deploymentName);
      if (!dep) {
        setError(`Deployment "${deploymentName}" not found in namespace "${namespace}"`);
        setLoading(false);
        return;
      }

      setDeployment(dep);

      // Filter pods by the app label matching the deployment name
      const filteredPods = podRes.pods.filter((p) => p.app === deploymentName);
      setAppPods(filteredPods);

      // Filter services by namespace (show all services in same namespace)
      setServices(svcRes.services.filter((s) => s.namespace === namespace));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load application");
    } finally {
      setLoading(false);
    }
  }, [namespace, deploymentName]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Auto-select the first pod for logs when pods load
  useEffect(() => {
    if (appPods.length > 0 && !selectedPod) {
      setSelectedPod(appPods[0]!.name);
    }
  }, [appPods, selectedPod]);

  const loadLogs = useCallback(async (podName: string) => {
    setLogsLoading(true);
    setLogsError(null);
    try {
      const text = await getClusterPodLogs(namespace, podName);
      setLogs(text);
    } catch (err) {
      setLogsError(err instanceof Error ? err.message : "Failed to load logs");
      setLogs("");
    } finally {
      setLogsLoading(false);
    }
  }, [namespace]);

  // Load logs when selected pod changes or when the logs tab is active
  useEffect(() => {
    if (tab === "logs" && selectedPod) {
      loadLogs(selectedPod);
    }
  }, [tab, selectedPod, loadLogs]);

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <Loader2 className="h-8 w-8 text-brand animate-spin mb-4" />
        <p className="text-sm text-muted-foreground">Loading application...</p>
      </div>
    );
  }

  if (error || !deployment) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <Box className="h-12 w-12 text-muted-foreground mb-4" />
        <h2 className="text-lg font-semibold text-foreground mb-2">
          {error || "Application Not Found"}
        </h2>
        <Link to="/app/applications">
          <Button variant="outline" size="sm">
            <ArrowLeft className="mr-2 h-3.5 w-3.5" /> Back to Applications
          </Button>
        </Link>
      </div>
    );
  }

  const { ready: readyCount, total: totalCount } = parseReadyFraction(deployment.ready);
  const appStatus = deriveStatus(
    appPods.length > 0
      ? appPods.filter((p) => {
          const pr = parseReadyFraction(p.ready);
          return pr.ready === pr.total && pr.ready > 0;
        }).length
      : readyCount,
    appPods.length > 0 ? appPods.length : totalCount,
  );

  const logLines = logs.split("\n").filter((l) => l.trim() !== "");

  return (
    <div>
      {/* Breadcrumb + header */}
      <div className="mb-6">
        <Link
          to="/app/applications"
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground mb-3"
        >
          <ArrowLeft className="h-3 w-3" /> Applications
        </Link>
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className={cn("p-2 rounded-lg border", statusBgColor(appStatus))}>
              <Box className={cn("h-5 w-5", statusColor(appStatus))} />
            </div>
            <div>
              <h1 className="text-xl font-semibold text-foreground">{deployment.name}</h1>
              <div className="flex items-center gap-2 mt-1">
                <Badge variant="outline" className="text-[10px]">
                  {deployment.namespace}
                </Badge>
                <code className="text-xs font-mono text-muted-foreground">{deployment.namespace}</code>
                {deployment.image && (
                  <>
                    <span className="text-xs text-muted-foreground">&middot;</span>
                    <code className="text-xs font-mono text-muted-foreground truncate max-w-xs">
                      {deployment.image}
                    </code>
                  </>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={async () => {
                try {
                  await restartDeployment(namespace, deploymentName);
                } catch {
                  // Refetch to show updated state regardless
                }
                setTimeout(() => loadData(), 1500);
              }}
            >
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" /> Restart
            </Button>
            <Button variant="ghost" size="icon" className="h-8 w-8">
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={tab} onValueChange={setTab}>
        <TabsList className="bg-muted/50 mb-6">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="pods">Pods</TabsTrigger>
          <TabsTrigger value="services">Services</TabsTrigger>
          <TabsTrigger value="logs">Logs</TabsTrigger>
          <TabsTrigger value="config">Config</TabsTrigger>
        </TabsList>

        {/* Overview Tab */}
        <TabsContent value="overview" className="space-y-4">
          {/* Status cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <Card className="bg-card">
              <CardContent className="pt-4 pb-3 px-4">
                <p className="text-[10px] text-muted-foreground mb-1">Status</p>
                <Badge
                  variant="outline"
                  className={cn("text-xs", statusColor(appStatus), statusBgColor(appStatus))}
                >
                  {appStatus}
                </Badge>
              </CardContent>
            </Card>
            <Card className="bg-card">
              <CardContent className="pt-4 pb-3 px-4">
                <p className="text-[10px] text-muted-foreground mb-1">Replicas</p>
                <p className="text-lg font-bold text-foreground">
                  {deployment.ready}
                </p>
              </CardContent>
            </Card>
            <Card className="bg-card">
              <CardContent className="pt-4 pb-3 px-4">
                <p className="text-[10px] text-muted-foreground mb-1">Up to Date</p>
                <p className="text-lg font-bold text-foreground">{deployment.up_to_date}</p>
              </CardContent>
            </Card>
            <Card className="bg-card">
              <CardContent className="pt-4 pb-3 px-4">
                <p className="text-[10px] text-muted-foreground mb-1">Available</p>
                <p className="text-lg font-bold text-foreground">{deployment.available}</p>
              </CardContent>
            </Card>
          </div>

          {/* Pod health grid */}
          <Card className="bg-card">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Pod Health</CardTitle>
            </CardHeader>
            <CardContent>
              {appPods.length === 0 ? (
                <p className="text-sm text-muted-foreground">No pods found for this deployment.</p>
              ) : (
                <div className="flex gap-2 flex-wrap">
                  {appPods.map((pod) => {
                    const PodIcon = podStatusIcons[pod.status] || Server;
                    return (
                      <div
                        key={pod.name}
                        className={cn(
                          "flex items-center gap-2 px-3 py-2 rounded-lg border text-sm",
                          statusBgColor(pod.status),
                        )}
                      >
                        <PodIcon
                          className={cn(
                            "h-3.5 w-3.5",
                            statusColor(pod.status),
                            pod.status === "Pending" && "animate-spin",
                          )}
                        />
                        <span className="font-mono text-xs">
                          {pod.name.split("-").slice(-1)[0]}
                        </span>
                        <span className={cn("text-xs", statusColor(pod.status))}>
                          {pod.status}
                        </span>
                        {pod.restarts > 0 && (
                          <Badge
                            variant="outline"
                            className="text-[10px] py-0 text-warning border-warning/30"
                          >
                            {pod.restarts} restarts
                          </Badge>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Deployment info */}
          <Card className="bg-card">
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Deployment Info</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-[10px] text-muted-foreground mb-0.5">Namespace</p>
                  <p className="text-sm font-mono text-foreground">{deployment.namespace}</p>
                </div>
                <div>
                  <p className="text-[10px] text-muted-foreground mb-0.5">Age</p>
                  <p className="text-sm text-foreground">{deployment.age}</p>
                </div>
                {deployment.image && (
                  <div className="col-span-2">
                    <p className="text-[10px] text-muted-foreground mb-0.5">Image</p>
                    <p className="text-sm font-mono text-foreground break-all">{deployment.image}</p>
                  </div>
                )}
                {deployment.labels && Object.keys(deployment.labels).length > 0 && (
                  <div className="col-span-2">
                    <p className="text-[10px] text-muted-foreground mb-1">Labels</p>
                    <div className="flex flex-wrap gap-1.5">
                      {Object.entries(deployment.labels).map(([k, v]) => (
                        <code
                          key={k}
                          className="text-[10px] font-mono bg-muted px-2 py-0.5 rounded text-foreground"
                        >
                          {k}={v}
                        </code>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Pods Tab */}
        <TabsContent value="pods">
          {appPods.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12">
              <Server className="h-8 w-8 text-muted-foreground mb-3" />
              <p className="text-sm text-muted-foreground">No pods found for this deployment.</p>
            </div>
          ) : (
            <div className="rounded-lg border border-border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="text-xs">Name</TableHead>
                    <TableHead className="text-xs">Status</TableHead>
                    <TableHead className="text-xs">Ready</TableHead>
                    <TableHead className="text-xs">Restarts</TableHead>
                    <TableHead className="text-xs">Node</TableHead>
                    <TableHead className="text-xs">Age</TableHead>
                    <TableHead className="text-xs">IP</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {appPods.map((pod) => {
                    const PodIcon = podStatusIcons[pod.status] || Server;
                    return (
                      <TableRow key={pod.name}>
                        <TableCell className="font-mono text-xs">{pod.name}</TableCell>
                        <TableCell>
                          <Badge
                            variant="outline"
                            className={cn(
                              "text-[10px] gap-1",
                              statusColor(pod.status),
                              statusBgColor(pod.status),
                            )}
                          >
                            <PodIcon
                              className={cn(
                                "h-3 w-3",
                                pod.status === "Pending" && "animate-spin",
                              )}
                            />
                            {pod.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs font-mono">{pod.ready}</TableCell>
                        <TableCell
                          className={cn(
                            "text-xs",
                            pod.restarts > 5
                              ? "text-error font-semibold"
                              : pod.restarts > 0
                                ? "text-warning"
                                : "text-foreground",
                          )}
                        >
                          {pod.restarts}
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">{pod.node}</TableCell>
                        <TableCell className="text-xs text-muted-foreground">{pod.age}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {pod.ip || "\u2014"}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        {/* Services Tab */}
        <TabsContent value="services">
          {services.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12">
              <Network className="h-8 w-8 text-muted-foreground mb-3" />
              <p className="text-sm text-muted-foreground">
                No services found in namespace "{namespace}".
              </p>
            </div>
          ) : (
            <div className="rounded-lg border border-border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="text-xs">Name</TableHead>
                    <TableHead className="text-xs">Type</TableHead>
                    <TableHead className="text-xs">Cluster IP</TableHead>
                    <TableHead className="text-xs">Ports</TableHead>
                    <TableHead className="text-xs">Age</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {services.map((svc) => (
                    <TableRow key={svc.name}>
                      <TableCell className="font-mono text-xs font-semibold">{svc.name}</TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[10px]">
                          {svc.type}
                        </Badge>
                      </TableCell>
                      <TableCell className="font-mono text-xs">{svc.cluster_ip}</TableCell>
                      <TableCell className="font-mono text-xs">{svc.ports}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">{svc.age}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        {/* Logs Tab */}
        <TabsContent value="logs">
          <Card className="bg-code-bg border-border">
            <CardHeader className="pb-2 flex flex-row items-center justify-between">
              <CardTitle className="text-sm text-code-text flex items-center gap-2">
                <Terminal className="h-4 w-4" /> Pod Logs
              </CardTitle>
              <div className="flex items-center gap-2">
                {appPods.length > 1 && (
                  <select
                    value={selectedPod || ""}
                    onChange={(e) => setSelectedPod(e.target.value)}
                    className="text-xs bg-muted border border-border rounded px-2 py-1 text-foreground"
                  >
                    {appPods.map((p) => (
                      <option key={p.name} value={p.name}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                )}
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-xs text-code-muted h-7"
                  onClick={() => {
                    navigator.clipboard.writeText(logs);
                  }}
                >
                  <Copy className="mr-1.5 h-3 w-3" /> Copy
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-xs text-code-muted h-7"
                  onClick={() => {
                    if (selectedPod) loadLogs(selectedPod);
                  }}
                >
                  <RefreshCw className="mr-1.5 h-3 w-3" /> Refresh
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {appPods.length === 0 ? (
                <p className="text-sm text-muted-foreground">No pods available for log viewing.</p>
              ) : logsLoading ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="h-5 w-5 text-brand animate-spin mr-2" />
                  <span className="text-sm text-muted-foreground">Loading logs...</span>
                </div>
              ) : logsError ? (
                <div className="text-sm text-error py-4">{logsError}</div>
              ) : logLines.length === 0 ? (
                <p className="text-sm text-muted-foreground py-4">No log output available.</p>
              ) : (
                <div className="space-y-0.5 font-mono text-xs max-h-96 overflow-y-auto">
                  {logLines.map((line, i) => (
                    <div key={i} className="flex gap-2 py-0.5">
                      <span className="text-code-muted shrink-0 w-10 text-right select-none">
                        {i + 1}
                      </span>
                      <span className="text-code-text whitespace-pre-wrap break-all">{line}</span>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {/* Config Tab */}
        <TabsContent value="config">
          <div className="space-y-4">
            {/* Deployment spec info */}
            <Card className="bg-card">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">Container Configuration</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-[10px] text-muted-foreground mb-0.5">Image</p>
                    <code className="text-xs font-mono text-foreground break-all">
                      {deployment.image || "N/A"}
                    </code>
                  </div>
                  <div>
                    <p className="text-[10px] text-muted-foreground mb-0.5">Ready Replicas</p>
                    <p className="text-sm font-mono text-foreground">{deployment.ready}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-muted-foreground mb-0.5">Up to Date</p>
                    <p className="text-sm font-mono text-foreground">{deployment.up_to_date}</p>
                  </div>
                  <div>
                    <p className="text-[10px] text-muted-foreground mb-0.5">Available</p>
                    <p className="text-sm font-mono text-foreground">{deployment.available}</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Labels */}
            {deployment.labels && Object.keys(deployment.labels).length > 0 && (
              <Card className="bg-card">
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">Labels</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-wrap gap-1.5">
                    {Object.entries(deployment.labels).map(([k, v]) => (
                      <code
                        key={k}
                        className="text-[10px] font-mono bg-muted px-2 py-0.5 rounded text-foreground"
                      >
                        {k}={v}
                      </code>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Pod specs summary */}
            {appPods.length > 0 && (
              <Card className="bg-card">
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">Pod Details</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="rounded-lg border border-border overflow-hidden">
                    <Table>
                      <TableHeader>
                        <TableRow className="hover:bg-transparent">
                          <TableHead className="text-xs">Pod</TableHead>
                          <TableHead className="text-xs">Status</TableHead>
                          <TableHead className="text-xs">Ready</TableHead>
                          <TableHead className="text-xs">Restarts</TableHead>
                          <TableHead className="text-xs">Node</TableHead>
                          <TableHead className="text-xs">IP</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {appPods.map((pod) => (
                          <TableRow key={pod.name}>
                            <TableCell className="font-mono text-xs">{pod.name}</TableCell>
                            <TableCell className={cn("text-xs", statusColor(pod.status))}>
                              {pod.status}
                            </TableCell>
                            <TableCell className="text-xs font-mono">{pod.ready}</TableCell>
                            <TableCell className="text-xs">{pod.restarts}</TableCell>
                            <TableCell className="text-xs text-muted-foreground">{pod.node}</TableCell>
                            <TableCell className="font-mono text-xs text-muted-foreground">
                              {pod.ip || "\u2014"}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
