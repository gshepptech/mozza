import {
  Activity, AlertTriangle, ArrowUpRight, Box, CheckCircle, Plus,
  RefreshCw, Rocket, Settings,
} from "lucide-react";
import type { ClusterEvent } from "@/api/types";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { statusBgColor, timeAgo } from "@/lib/format";

// ── Stat Card ──────────────────────────────────────────────────

export function StatCard({ label, value, sub, icon: Icon, trend, accent }: {
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

export function ResourceBar({ label, used, total, unit }: {
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

export function EventRow({ event, odd }: { event: ClusterEvent; odd: boolean }) {
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
