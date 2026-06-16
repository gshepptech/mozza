import { useState, useEffect, useCallback } from "react";
import * as api from "../api/client";
import type { ContainerStatus } from "../api/types";
import { Activity, RefreshCw, Server } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/custom/empty-state";
import { StatusDot } from "@/components/custom/status-dot";

function containerHealth(state: string): "success" | "error" | "warning" | "neutral" {
  switch (state.toLowerCase()) {
    case "running": return "success";
    case "exited":
    case "dead": return "error";
    case "restarting": return "warning";
    default: return "neutral";
  }
}

export default function StatusPage() {
  const [containers, setContainers] = useState<ContainerStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const fetchStatus = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const report = await api.getStatus();
      setContainers(report.containers || []);
    } catch {
      setError("Failed to fetch status");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 30000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Status</h1>
          <p className="text-sm text-muted-foreground mt-1">Container health and service topology</p>
        </div>
        <Button variant="outline" size="sm" onClick={fetchStatus} disabled={loading}>
          <RefreshCw className={`mr-2 h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          Refresh
        </Button>
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-error/30 bg-error-bg p-4 text-sm text-error">
          {error}
        </div>
      )}

      {loading && containers.length === 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3].map(i => (
            <Card key={i}><CardContent className="pt-6"><Skeleton className="h-20 w-full" /></CardContent></Card>
          ))}
        </div>
      ) : containers.length === 0 ? (
        <EmptyState
          icon={Activity}
          title="No containers running"
          description="Deploy a recipe to see container status here"
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {containers.map((c) => (
            <Card key={c.name} className="bg-card">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-semibold">
                  <Server className="h-4 w-4 text-muted-foreground" />
                  {c.name}
                </CardTitle>
                <StatusDot status={containerHealth(c.state)} pulse={c.state === "running"} />
              </CardHeader>
              <CardContent className="space-y-2">
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-xs font-mono">{c.state}</Badge>
                  {c.health && (
                    <Badge variant="outline" className="text-xs">{c.health}</Badge>
                  )}
                </div>
                <p className="text-xs font-mono text-muted-foreground truncate">{c.image}</p>
                {c.ports && (
                  <div className="flex flex-wrap gap-1">
                    <Badge variant="secondary" className="text-xs font-mono">{c.ports}</Badge>
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
