import { useState } from "react";
import type { DeployPreviewResponse } from "@/api/client";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  ArrowLeft, Rocket, Globe, Server, Database as DatabaseIcon,
  Cog, ChevronDown, ChevronUp,
} from "lucide-react";
import { cn } from "@/lib/utils";

// ─── Props ──────────────────────────────────────────────────

interface ReviewOrderProps {
  preview: DeployPreviewResponse;
  recipeName: string;
  recipeSource: string;
  onConfirm: () => void;
  onBack: () => void;
}

// ─── Kind Icon ──────────────────────────────────────────────

const kindIcons: Record<string, typeof Globe> = {
  web: Globe,
  api: Globe,
  gateway: Globe,
  worker: Cog,
  database: DatabaseIcon,
  cache: Server,
  task: Cog,
  scheduled: Cog,
  stateful: Server,
  daemon: Server,
};

const kindColors: Record<string, string> = {
  web: "text-blue-400 bg-blue-400/10",
  api: "text-blue-400 bg-blue-400/10",
  gateway: "text-indigo-400 bg-indigo-400/10",
  worker: "text-emerald-400 bg-emerald-400/10",
  database: "text-purple-400 bg-purple-400/10",
  cache: "text-yellow-400 bg-yellow-400/10",
  task: "text-orange-400 bg-orange-400/10",
  scheduled: "text-cyan-400 bg-cyan-400/10",
  stateful: "text-rose-400 bg-rose-400/10",
  daemon: "text-gray-400 bg-gray-400/10",
};

// ─── Component ──────────────────────────────────────────────

export default function ReviewOrder({ preview, recipeName, recipeSource, onConfirm, onBack }: ReviewOrderProps) {
  const [showRecipe, setShowRecipe] = useState(false);
  const { summary, slices } = preview;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="text-center space-y-2">
        <h2 className="text-xl font-bold text-foreground">Review Your Order</h2>
        <p className="text-sm text-muted-foreground">
          Here's what we'll deploy for <span className="font-medium text-foreground">{preview.name || recipeName}</span>
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <SummaryCard label="Services" count={summary.services} accent="text-blue-400" />
        <SummaryCard label="Databases" count={summary.databases} accent="text-purple-400" />
        <SummaryCard label="Caches" count={summary.caches} accent="text-yellow-400" />
        <SummaryCard label="Workers" count={summary.workers} accent="text-emerald-400" />
      </div>

      {/* Slice list */}
      <Card className="oven-card bg-card border-border/50">
        <CardContent className="p-0">
          <div className="px-6 py-4 border-b border-border/50">
            <h3 className="text-sm font-semibold text-foreground">
              What you're getting ({slices.length} {slices.length === 1 ? "component" : "components"})
            </h3>
          </div>
          <div className="divide-y divide-border/30">
            {slices.map((sl) => {
              const Icon = kindIcons[sl.kind] || Server;
              const color = kindColors[sl.kind] || "text-gray-400 bg-gray-400/10";
              return (
                <div key={sl.name} className="flex items-center gap-4 px-6 py-3">
                  <div className={cn("p-2 rounded-lg", color)}>
                    <Icon className="h-4 w-4" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-medium text-foreground">{sl.name}</span>
                    <span className="text-xs text-muted-foreground ml-2 font-mono">{sl.image}</span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <Badge variant="outline" className="text-[10px] px-1.5 py-0">{sl.kind}</Badge>
                    {sl.port > 0 && (
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0">:{sl.port}</Badge>
                    )}
                    {sl.public && (
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-blue-400 border-blue-400/30">public</Badge>
                    )}
                    {sl.replicas > 1 && (
                      <Badge variant="outline" className="text-[10px] px-1.5 py-0">x{sl.replicas}</Badge>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>

      {/* Compiled manifests — collapsible */}
      {preview.manifests && (
        <div>
          <button
            type="button"
            onClick={() => setShowRecipe(!showRecipe)}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1 mb-2"
          >
            {showRecipe ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
            View compiled manifests
          </button>
          {showRecipe && (
            <Card className="oven-card bg-card border-border/50">
              <CardContent className="p-0">
                <pre className="text-xs font-mono text-foreground/80 p-4 max-h-[300px] overflow-auto whitespace-pre-wrap">
                  {preview.manifests}
                </pre>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Recipe source — collapsible */}
      {!preview.manifests && (
        <div>
          <button
            type="button"
            onClick={() => setShowRecipe(!showRecipe)}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1 mb-2"
          >
            {showRecipe ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
            View recipe source
          </button>
          {showRecipe && (
            <Card className="oven-card bg-card border-border/50">
              <CardContent className="p-0">
                <pre className="text-xs font-mono text-foreground/80 p-4 max-h-[300px] overflow-auto whitespace-pre-wrap">
                  {recipeSource}
                </pre>
              </CardContent>
            </Card>
          )}
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center justify-between pt-2">
        <Button variant="outline" onClick={onBack}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back
        </Button>
        <Button
          className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_12px_rgba(255,107,53,0.15)]"
          onClick={onConfirm}
        >
          <Rocket className="mr-2 h-4 w-4" />
          Deploy
        </Button>
      </div>
    </div>
  );
}

// ─── Summary Card ───────────────────────────────────────────

function SummaryCard({ label, count, accent }: { label: string; count: number; accent: string }) {
  return (
    <Card className="oven-card bg-card border-border/50">
      <CardContent className="p-4 text-center">
        <p className={cn("text-2xl font-bold", count > 0 ? accent : "text-muted-foreground/40")}>
          {count}
        </p>
        <p className="text-xs text-muted-foreground mt-1">{label}</p>
      </CardContent>
    </Card>
  );
}
