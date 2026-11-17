import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import {
  Activity, ChevronRight, GitCommit, Layers, Loader2, Plus, Rocket,
  Server, Stethoscope, Zap,
} from "lucide-react";
import type { Deployment, Team } from "@/api/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { statusColor } from "@/lib/format";

// Shown when no cluster is registered: onboarding, quick actions and the
// team's recent deployments, which do not need a cluster.
export function NoClusterOverview({ activeTeam, recentDeploys }: {
  activeTeam: Team | null;
  recentDeploys: Deployment[];
}) {
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

        {/* Recent deployments work without a cluster. */}
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

// Shown when the cluster cannot be reached. Deploying still works, so the page
// leads with the deploy and connect actions.
export function ReadyToDeployOverview({ subtitle, clusterBlurb, notice }: {
  subtitle: string;
  clusterBlurb: string;
  notice?: ReactNode;
}) {
  return (
    <div className="max-w-7xl mx-auto px-6 py-8 page-enter">
      <div className="flex items-start justify-between mb-10">
        <div>
          <h1 className="text-2xl font-bold text-foreground tracking-tight">Overview</h1>
          <p className="text-sm text-muted-foreground mt-1.5">{subtitle}</p>
        </div>
        <Badge variant="outline" className="gap-2 text-sm text-brand border-brand/30 bg-brand/5 px-3 py-1">
          <Rocket className="h-3.5 w-3.5" />
          Ready to Deploy
        </Badge>
      </div>
      {notice}
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
                {clusterBlurb}
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

export function OverviewLoading() {
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
