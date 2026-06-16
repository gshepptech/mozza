import { useState, useEffect, useCallback } from "react";
import { Link, useNavigate } from "react-router-dom";
import { listDeployments, deleteDeployment, rollbackDeployment } from "@/api/client";
import type { Deployment } from "@/api/types";
import { useDashboard } from "./DashboardPage";
import {
  Rocket, CheckCircle, XCircle, Clock,
  RotateCcw, User, ChevronDown, ChevronUp,
  RefreshCw, Loader, Play, Search, Plus, ArrowUpRight, Trash2, ExternalLink,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { relativeTime, statusColor, statusBgColor, envColor } from "@/lib/format";

// ─── Status Config ──────────────────────────────────────────

const statusConfig: Record<string, { icon: typeof CheckCircle; label: string; pizza: string }> = {
  success: { icon: CheckCircle, label: "Success", pizza: "Served" },
  succeeded: { icon: CheckCircle, label: "Succeeded", pizza: "Served" },
  failed: { icon: XCircle, label: "Failed", pizza: "Returned" },
  running: { icon: Loader, label: "Running", pizza: "Baking" },
  "rolled-back": { icon: RotateCcw, label: "Rolled Back", pizza: "Rolled Back" },
  rolled_back: { icon: RotateCcw, label: "Rolled Back", pizza: "Rolled Back" },
  partial: { icon: Clock, label: "Partial", pizza: "Half-Baked" },
  pending: { icon: Clock, label: "Pending", pizza: "In Queue" },
};

const envFilters = ["all", "production", "staging", "development"] as const;
const statusFilters = ["all", "success", "failed", "running", "rolled-back"] as const;

// ─── Stat Card ──────────────────────────────────────────────

function StatCard({ label, value, sub, icon: Icon, accent, children }: {
  label: string;
  value: string | number;
  sub?: string;
  icon: typeof Rocket;
  accent?: string;
  children?: React.ReactNode;
}) {
  return (
    <Card className="oven-card feature-card bg-card border-border/50">
      <CardContent className="p-6">
        <div className="flex items-start justify-between mb-4">
          <div className={cn("p-2.5 rounded-xl", accent || "bg-brand/10")}>
            <Icon className={cn("h-5 w-5", accent ? "text-current" : "text-brand")} />
          </div>
        </div>
        <p className="text-4xl font-bold text-foreground tracking-tight leading-none">
          {value}
        </p>
        <p className="text-sm text-muted-foreground mt-2">{label}</p>
        {sub && (
          <p className="text-xs text-muted-foreground/60 mt-1.5">{sub}</p>
        )}
        {children}
      </CardContent>
    </Card>
  );
}

// ─── Timeline Item ──────────────────────────────────────────

function TimelineItem({ deployment, isLast, expanded, onToggle, onRefresh, onNavigate }: {
  deployment: Deployment;
  isLast: boolean;
  expanded: boolean;
  onToggle: () => void;
  onRefresh: () => void;
  onNavigate: (path: string) => void;
}) {
  const config = statusConfig[deployment.status] || statusConfig.pending!;
  const StatusIcon = config.icon;

  const dotColor = {
    success: "bg-success ring-success/30",
    succeeded: "bg-success ring-success/30",
    failed: "bg-error ring-error/30",
    running: "bg-info ring-info/30 animate-pulse",
    "rolled-back": "bg-warning ring-warning/30",
    rolled_back: "bg-warning ring-warning/30",
    partial: "bg-warning ring-warning/30",
    pending: "bg-muted-foreground ring-muted-foreground/30",
  }[deployment.status] || "bg-muted ring-muted";

  return (
    <div className="relative flex gap-5">
      {/* Timeline connector */}
      <div className="flex flex-col items-center">
        <div className={cn("h-3.5 w-3.5 rounded-full ring-4 shrink-0 mt-6 z-10", dotColor)} />
        {!isLast && (
          <div className="w-px flex-1 bg-border/60" />
        )}
      </div>

      {/* Deployment card */}
      <div className="flex-1 pb-6">
        <Card
          className={cn(
            "oven-card feature-card bg-card border-border/50 cursor-pointer",
            expanded && "border-brand/30 shadow-[0_0_30px_rgba(255,107,53,0.06)]",
          )}
          onClick={onToggle}
        >
          <CardContent className="p-6">
            {/* Header row */}
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-3 flex-wrap">
                  {deployment.order_number && (
                    <span className="text-lg font-bold font-mono text-brand">
                      Order #{deployment.order_number}
                    </span>
                  )}
                  <span className="text-base font-semibold text-foreground">{deployment.target}</span>
                  <Badge variant="outline" className={cn("text-xs py-0.5 px-2", envColor(deployment.environment))}>
                    {deployment.environment}
                  </Badge>
                  <Badge
                    variant="outline"
                    className={cn(
                      "text-xs gap-1.5 py-0.5 px-2",
                      statusBgColor(deployment.status),
                      statusColor(deployment.status),
                    )}
                  >
                    <StatusIcon className={cn("h-3.5 w-3.5", deployment.status === "running" && "animate-spin")} />
                    {config.pizza}
                  </Badge>
                </div>

                {/* Meta row */}
                <div className="flex items-center gap-4 mt-3 text-xs text-muted-foreground flex-wrap">
                  <span className="flex items-center gap-1.5">
                    <User className="h-3.5 w-3.5" />
                    {deployment.started_by}
                  </span>
                  <span className="flex items-center gap-1.5">
                    <Clock className="h-3.5 w-3.5" />
                    {relativeTime(deployment.started_at)}
                  </span>
                </div>
              </div>

              {/* Expand toggle */}
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground">
                {expanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
              </Button>
            </div>

            {/* Expanded details */}
            {expanded && (
              <div className="mt-6 pt-6 border-t border-border/50">
                <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-6">
                  <div>
                    <span className="text-xs text-muted-foreground block mb-1">Started</span>
                    <span className="text-sm text-foreground">{new Date(deployment.started_at).toLocaleString()}</span>
                  </div>
                  {deployment.finished_at && (
                    <div>
                      <span className="text-xs text-muted-foreground block mb-1">Finished</span>
                      <span className="text-sm text-foreground">{new Date(deployment.finished_at).toLocaleString()}</span>
                    </div>
                  )}
                  <div>
                    <span className="text-xs text-muted-foreground block mb-1">Target</span>
                    <span className="text-sm text-foreground">{deployment.target}</span>
                  </div>
                  <div>
                    <span className="text-xs text-muted-foreground block mb-1">Recipe</span>
                    <span className="text-sm font-mono text-foreground">{deployment.recipe_id.slice(0, 8)}</span>
                  </div>
                </div>

                {deployment.access_url && (
                  <div className="flex items-center gap-3 mb-6 px-4 py-3 rounded-lg bg-green-400/5 border border-green-400/15">
                    <ExternalLink className="h-4 w-4 text-green-400 shrink-0" />
                    <span className="text-sm text-muted-foreground">Live at</span>
                    <a
                      href={deployment.access_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-green-400 underline underline-offset-2 hover:text-green-300 transition-colors truncate"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {deployment.access_url}
                    </a>
                  </div>
                )}

                {deployment.log && (
                  <div className="rounded-lg bg-muted/30 border border-border/40 p-4 mb-6">
                    <pre className="text-xs font-mono text-muted-foreground whitespace-pre-wrap max-h-40 overflow-y-auto">
                      {deployment.log}
                    </pre>
                  </div>
                )}

                {/* Actions */}
                <div className="flex items-center gap-3">
                  {(deployment.status === "succeeded" || deployment.status === "failed" || deployment.status === "partial") && deployment.has_previous_state && (deployment.version ?? 0) > 1 && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs h-8 px-3 border-warning/30 text-warning hover:bg-warning/10"
                      onClick={async (e) => {
                        e.stopPropagation();
                        if (!confirm("Roll back this deployment to its previous state? This will re-apply the previous namespace snapshot.")) return;
                        try {
                          await rollbackDeployment(deployment.id);
                          onRefresh();
                        } catch (err) {
                          alert(err instanceof Error ? err.message : "Rollback failed");
                        }
                      }}
                    >
                      <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                      Undo this deploy
                    </Button>
                  )}
                  {(deployment.status === "succeeded" || deployment.status === "failed" || deployment.status === "partial") && !deployment.has_previous_state && (
                    <Button variant="outline" size="sm" className="text-xs h-8 px-3 border-border/50 opacity-50 cursor-not-allowed" title="No previous version to roll back to">
                      <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                      Undo this deploy
                    </Button>
                  )}
                  {deployment.status === "failed" && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs h-8 px-3 border-border/50 hover:border-brand/30"
                      onClick={async (e) => {
                        e.stopPropagation();
                        onNavigate("/app/deploy");
                      }}
                    >
                      <Play className="mr-1.5 h-3.5 w-3.5" />
                      Retry
                    </Button>
                  )}
                  {deployment.status === "running" && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs h-8 px-3 text-error border-error/30 hover:bg-error/10"
                      onClick={async (e) => {
                        e.stopPropagation();
                        if (!confirm("Cancel this deployment? This will remove it from history.")) return;
                        try {
                          await deleteDeployment(deployment.id);
                          onRefresh();
                        } catch (err) {
                          alert(err instanceof Error ? err.message : "Cancel failed");
                        }
                      }}
                    >
                      <XCircle className="mr-1.5 h-3.5 w-3.5" />
                      Cancel
                    </Button>
                  )}
                  {deployment.status === "success" && deployment.environment !== "production" && (
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs h-8 px-3 text-brand border-brand/30 hover:bg-brand/10"
                      onClick={(e) => {
                        e.stopPropagation();
                        onNavigate("/app/deploy");
                      }}
                    >
                      <ArrowUpRight className="mr-1.5 h-3.5 w-3.5" />
                      Promote
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-xs h-8 px-3 text-error border-error/30 hover:bg-error/10"
                    title="Removes from history — does not stop your running app"
                    onClick={async (e) => {
                      e.stopPropagation();
                      if (!confirm("Delete this deployment?")) return;
                      try {
                        await deleteDeployment(deployment.id);
                        onRefresh();
                      } catch (err) {
                        alert(err instanceof Error ? err.message : "Delete failed");
                      }
                    }}
                  >
                    <Trash2 className="mr-1.5 h-3.5 w-3.5" />
                    Delete
                  </Button>
                  {deployment.log && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-xs h-8 px-3 ml-auto text-muted-foreground hover:text-foreground"
                      onClick={(e) => { e.stopPropagation(); onToggle(); }}
                    >
                      View Logs
                    </Button>
                  )}
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

// ─── Main Page ──────────────────────────────────────────────

export default function DeploymentsPage() {
  const navigate = useNavigate();
  const { activeTeam } = useDashboard();
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [envFilter, setEnvFilter] = useState<typeof envFilters[number]>("all");
  const [statusFilter, setStatusFilter] = useState<typeof statusFilters[number]>("all");
  const [search, setSearch] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [showStatusLegend, setShowStatusLegend] = useState(() => !localStorage.getItem("mozza-status-legend-dismissed"));

  const fetchDeployments = useCallback(async () => {
    if (!activeTeam) {
      setDeployments([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const res = await listDeployments(activeTeam.id);
      setDeployments(res.deployments || []);
    } catch {
      setError("Failed to load deployments");
    } finally {
      setLoading(false);
    }
  }, [activeTeam]);

  useEffect(() => {
    fetchDeployments();
  }, [fetchDeployments]);

  // No team selected
  if (!activeTeam) {
    return (
      <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
        <div className="flex items-start justify-between mb-10">
          <div>
            <h1 className="text-2xl font-bold text-foreground tracking-tight">Deployments</h1>
            <p className="text-sm text-muted-foreground mt-1.5">Deployment history and rollback management</p>
          </div>
        </div>
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardContent className="flex flex-col items-center py-16">
            <div className="p-4 rounded-xl bg-brand/10 mb-4">
              <Plus className="h-8 w-8 text-brand" />
            </div>
            <p className="text-base font-semibold text-foreground">Create a team to start deploying</p>
            <p className="text-sm text-muted-foreground mt-1.5">
              You need a team before you can create deployments
            </p>
            <Link to="/app/teams/new">
              <Button className="mt-6 shadow-[0_0_12px_rgba(255,107,53,0.15)]">
                <Plus className="mr-2 h-4 w-4" />
                Create Team
              </Button>
            </Link>
          </CardContent>
        </Card>
      </div>
    );
  }

  const filtered = deployments.filter(d => {
    if (envFilter !== "all" && d.environment !== envFilter) return false;
    if (statusFilter !== "all" && d.status !== statusFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      return d.target.toLowerCase().includes(q)
        || d.status.toLowerCase().includes(q)
        || d.environment.toLowerCase().includes(q)
        || d.started_by.toLowerCase().includes(q);
    }
    return true;
  });

  const total = deployments.length;
  const successful = deployments.filter(d => d.status === "success").length;
  const failed = deployments.filter(d => d.status === "failed").length;
  const running = deployments.filter(d => d.status === "running").length;
  const rate = total > 0 ? Math.round((successful / total) * 100) : 0;

  const envCounts = {
    all: deployments.length,
    production: deployments.filter(d => d.environment === "production").length,
    staging: deployments.filter(d => d.environment === "staging").length,
    development: deployments.filter(d => d.environment === "development").length,
  };

  return (
    <div className="max-w-7xl mx-auto px-6 py-8 page-enter">

      {/* ── Header ─────────────────────────────────────────── */}
      <div className="flex items-start justify-between mb-10">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">
            Deployments
          </h1>
          <p className="text-sm text-muted-foreground mt-1.5">
            Deployment history and rollback management
          </p>
        </div>
        <div className="flex gap-3">
          <Button variant="outline" size="sm" onClick={fetchDeployments} disabled={loading}>
            {loading
              ? <Loader className="mr-2 h-3.5 w-3.5 animate-spin" />
              : <RefreshCw className="mr-2 h-3.5 w-3.5" />
            }
            Refresh
          </Button>
          <Link to="/app/deploy">
            <Button className="shadow-[0_0_12px_rgba(255,107,53,0.15)] h-10 px-5">
              <Rocket className="mr-2 h-4 w-4" />
              New Deployment
            </Button>
          </Link>
        </div>
      </div>

      {showStatusLegend && (
        <div className="rounded-xl border border-border/50 bg-card p-4 mb-6">
          <div className="flex items-center justify-between mb-3">
            <p className="text-sm font-medium text-foreground">Understanding your deployments</p>
            <button
              type="button"
              onClick={() => { setShowStatusLegend(false); localStorage.setItem("mozza-status-legend-dismissed", "1"); }}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Got it
            </button>
          </div>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-green-500" /> <span className="text-muted-foreground">Succeeded — Your app is live</span></div>
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-blue-500" /> <span className="text-muted-foreground">Running — Deploy in progress</span></div>
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-yellow-500" /> <span className="text-muted-foreground">Pending — Waiting to start</span></div>
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-orange-500" /> <span className="text-muted-foreground">Partial — Some parts running</span></div>
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-red-500" /> <span className="text-muted-foreground">Failed — Something went wrong</span></div>
            <div className="flex items-center gap-2"><span className="h-2 w-2 rounded-full bg-gray-500" /> <span className="text-muted-foreground">Rolled Back — Restored previous version</span></div>
          </div>
        </div>
      )}

      {error && (
        <div className="mb-6 rounded-lg border border-error/30 bg-error/5 p-4 text-sm text-error">
          {error}
        </div>
      )}

      {/* ── Stats Row ──────────────────────────────────────── */}
      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-6 mb-10">
          {[1, 2, 3, 4].map(i => <Skeleton key={i} className="h-36 w-full rounded-xl" />)}
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-6 mb-10">
          <StatCard
            icon={Rocket}
            value={total}
            label="Total Deployments"
            sub={`${running > 0 ? `${running} in progress` : "none in progress"}`}
            accent="bg-brand/10"
          />
          <StatCard
            icon={CheckCircle}
            value={`${rate}%`}
            label="Success Rate"
            sub={`${successful} of ${total} deployments`}
            accent="bg-success/10"
          >
            <div className="h-2 rounded-full bg-muted/60 overflow-hidden mt-3">
              <div
                className={cn(
                  "h-full rounded-full transition-all duration-500",
                  rate >= 80 ? "bg-success" : rate >= 60 ? "bg-warning" : "bg-error",
                )}
                style={{ width: `${rate}%` }}
              />
            </div>
          </StatCard>
          <StatCard
            icon={XCircle}
            value={failed}
            label="Failed"
            sub={running > 0 ? `${running} currently running` : "none in progress"}
            accent="bg-error/10"
          />
          <StatCard
            icon={Clock}
            value={running}
            label="In Progress"
            sub={`${total} total deployments`}
            accent="bg-info/10"
          />
        </div>
      )}

      {/* ── Divider ────────────────────────────────────────── */}
      <div className="pizza-cutter mb-10" />

      {/* ── Filters ────────────────────────────────────────── */}
      <Card className="oven-card feature-card bg-card border-border/50 mb-10">
        <CardContent className="p-6">
          <div className="flex flex-col lg:flex-row gap-4">
            {/* Environment tabs */}
            <div className="flex gap-2 flex-1">
              {envFilters.map(f => (
                <Button
                  key={f}
                  variant={envFilter === f ? "default" : "ghost"}
                  size="sm"
                  className={cn(
                    "text-xs h-8 px-3.5",
                    envFilter === f && f !== "all" && envColor(f),
                  )}
                  onClick={() => setEnvFilter(f)}
                >
                  {f === "all" ? "All Envs" : f.charAt(0).toUpperCase() + f.slice(1)}
                  <span className="ml-2 text-xs opacity-60">{envCounts[f]}</span>
                </Button>
              ))}
            </div>

            {/* Status filter */}
            <div className="flex gap-2">
              {statusFilters.map(f => {
                const cfg = f === "all" ? null : statusConfig[f];
                return (
                  <Button
                    key={f}
                    variant={statusFilter === f ? "default" : "ghost"}
                    size="sm"
                    className="text-xs h-8 px-3"
                    onClick={() => setStatusFilter(f)}
                  >
                    {cfg && <cfg.icon className="h-3.5 w-3.5 mr-1.5" />}
                    {f === "all" ? "All" : cfg?.label}
                  </Button>
                );
              })}
            </div>

            {/* Search */}
            <div className="relative w-full lg:w-64">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Filter deployments..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="h-8 text-xs pl-9 bg-elevated border-border/50"
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ── Timeline ───────────────────────────────────────── */}
      {loading ? (
        <div className="space-y-4 ml-2">
          {[1, 2, 3].map(i => <Skeleton key={i} className="h-28 w-full rounded-xl" />)}
        </div>
      ) : deployments.length === 0 ? (
        <div className="text-center py-16 space-y-3">
          <p className="text-sm text-muted-foreground">No deployments yet</p>
          <p className="text-xs text-muted-foreground/60">Head to the Deploy page to get started.</p>
        </div>
      ) : filtered.length === 0 ? (
        <Card className="oven-card feature-card bg-card border-border/50">
          <CardContent className="flex flex-col items-center py-16">
            <div className="p-4 rounded-xl bg-brand/10 mb-4">
              <Rocket className="h-8 w-8 text-brand" />
            </div>
            <p className="text-base font-semibold text-foreground">No deployments match your filters</p>
            <p className="text-sm text-muted-foreground mt-1.5">Try adjusting the environment or status filter</p>
            <Button
              variant="outline"
              size="sm"
              className="mt-6 text-xs h-8 px-4 border-border/50 hover:border-brand/30"
              onClick={() => { setEnvFilter("all"); setStatusFilter("all"); setSearch(""); }}
            >
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              Clear Filters
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="ml-2">
          {filtered.map((d, i) => (
            <TimelineItem
              key={d.id}
              deployment={d}
              isLast={i === filtered.length - 1}
              expanded={expandedId === d.id}
              onToggle={() => setExpandedId(expandedId === d.id ? null : d.id)}
              onRefresh={fetchDeployments}
              onNavigate={navigate}
            />
          ))}
        </div>
      )}

      {/* ── Divider ────────────────────────────────────────── */}
      {filtered.length > 0 && <div className="pizza-cutter mt-6 mb-6" />}

      {/* ── Footer Legend ──────────────────────────────────── */}
      {filtered.length > 0 && (
        <div className="flex items-center justify-between px-2">
          <div className="flex items-center gap-5 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-success" />
              Success
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-error" />
              Failed
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-info animate-pulse" />
              Running
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2.5 w-2.5 rounded-full bg-warning" />
              Rolled Back
            </span>
          </div>
          <span className="text-xs text-muted-foreground">
            Showing {filtered.length} of {deployments.length} deployments
          </span>
        </div>
      )}
    </div>
  );
}
