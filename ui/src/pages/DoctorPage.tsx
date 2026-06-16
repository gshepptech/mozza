import { useState, useEffect, useCallback } from "react";
import * as api from "../api/client";
import { ApiError } from "../api/client";
import type { DoctorFinding } from "../api/types";
import { RefreshCw, AlertTriangle, CheckCircle, Info, XCircle, Loader } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const severityConfig = {
  error: { icon: XCircle, color: "text-error", bg: "bg-error-bg", border: "border-error/30", label: "Error" },
  warning: { icon: AlertTriangle, color: "text-warning", bg: "bg-warning-bg", border: "border-warning/30", label: "Warning" },
  info: { icon: Info, color: "text-info", bg: "bg-info-bg", border: "border-info/30", label: "Info" },
  ok: { icon: CheckCircle, color: "text-success", bg: "bg-success-bg", border: "border-success/30", label: "OK" },
};

export default function DoctorPage() {
  const [findings, setFindings] = useState<DoctorFinding[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notConfigured, setNotConfigured] = useState(false);

  const runDoctor = useCallback(async () => {
    setLoading(true);
    setError("");
    setNotConfigured(false);
    try {
      const report = await api.getDoctor();
      setFindings(report.findings || []);
    } catch (err) {
      if (err instanceof ApiError && err.status === 503) {
        setNotConfigured(true);
      } else {
        setError("Failed to run doctor diagnostics");
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { runDoctor(); }, [runDoctor]);

  const counts = {
    error: findings.filter(f => f.severity === "error").length,
    warning: findings.filter(f => f.severity === "warning").length,
    info: findings.filter(f => f.severity === "info").length,
    ok: findings.filter(f => f.severity === "ok").length,
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-xl font-semibold text-foreground">Doctor</h1>
          <p className="text-sm text-muted-foreground mt-1">Infrastructure health diagnostics</p>
        </div>
        <Button variant="outline" size="sm" onClick={runDoctor} disabled={loading}>
          {loading
            ? <Loader className="mr-2 h-3.5 w-3.5 animate-spin" />
            : <RefreshCw className="mr-2 h-3.5 w-3.5" />
          }
          Re-run
        </Button>
      </div>

      {/* Summary bar */}
      {!loading && !notConfigured && findings.length > 0 && (
        <div className="flex gap-3 mb-6">
          {(["error", "warning", "info", "ok"] as const).map(sev => {
            const cfg = severityConfig[sev];
            return (
              <Badge key={sev} variant="outline" className={cn("gap-1.5 px-3 py-1", cfg.color, cfg.border, cfg.bg)}>
                <cfg.icon className="h-3.5 w-3.5" />
                {counts[sev]} {cfg.label}
              </Badge>
            );
          })}
        </div>
      )}

      {error && (
        <div className="mb-6 rounded-lg border border-error/30 bg-error-bg p-4 text-sm text-error">
          {error}
        </div>
      )}

      {notConfigured && (
        <Card className="oven-card">
          <CardContent className="flex flex-col items-center py-12">
            <div className="relative mb-4">
              <div className="absolute inset-0 rounded-full bg-warning/10 blur-xl scale-150" />
              <AlertTriangle className="relative h-12 w-12 text-warning" />
            </div>
            <p className="text-lg font-semibold text-foreground">Doctor not configured</p>
            <p className="text-sm text-muted-foreground mt-1 text-center max-w-md">
              The doctor service is not available. Make sure Mozza is configured with
              a Kubernetes cluster connection and the doctor checks are enabled.
            </p>
            <Button variant="outline" size="sm" className="mt-6" onClick={runDoctor}>
              <RefreshCw className="mr-2 h-3.5 w-3.5" />
              Try Again
            </Button>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      ) : !notConfigured && findings.length === 0 && !error ? (
        <Card className="oven-card">
          <CardContent className="flex flex-col items-center py-12">
            <div className="relative mb-4">
              <div className="absolute inset-0 rounded-full bg-success/10 blur-xl scale-150" />
              <CheckCircle className="relative h-12 w-12 text-success" />
            </div>
            <p className="text-lg font-semibold text-foreground">All checks passed</p>
            <p className="text-sm text-muted-foreground mt-1">Your kitchen is in great shape</p>
          </CardContent>
        </Card>
      ) : !notConfigured && findings.length > 0 ? (
        <div className="space-y-3">
          {(["error", "warning", "info", "ok"] as const).map(severity => {
            const group = findings.filter(f => f.severity === severity);
            if (group.length === 0) return null;
            const cfg = severityConfig[severity];
            return (
              <div key={severity}>
                {group.map((finding, i) => (
                  <Card key={i} className={cn("mb-2 border", cfg.border)}>
                    <CardContent className="flex items-start gap-3 py-3">
                      <cfg.icon className={cn("h-5 w-5 mt-0.5 shrink-0", cfg.color)} />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-sm font-semibold text-foreground">{finding.rule}</span>
                          <Badge variant="outline" className={cn("text-xs", cfg.color, cfg.border)}>{cfg.label}</Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">{finding.message}</p>
                        {finding.fix && (
                          <p className="mt-1 text-sm text-foreground/80">
                            <span className="font-medium">Fix:</span> {finding.fix}
                          </p>
                        )}
                      </div>
                    </CardContent>
                  </Card>
                ))}
              </div>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
