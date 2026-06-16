import { useState, useEffect } from "react";
import * as api from "../../api/client";
import type { Template } from "../../api/types";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { Loader, CheckCircle, AlertTriangle } from "lucide-react";

const CATEGORIES = [
  "All",
  "CMS",
  "DevTools",
  "Databases",
  "Monitoring",
  "Analytics",
  "Automation",
  "Storage",
  "Learning",
] as const;

interface TemplateCatalogProps {
  selected: Template | null;
  onSelect: (template: Template) => void;
}

export default function TemplateCatalog({ selected, onSelect }: TemplateCatalogProps) {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeCategory, setActiveCategory] = useState<string>("All");

  useEffect(() => {
    setLoading(true);
    setError("");
    const cat = activeCategory === "All" ? undefined : activeCategory;
    api
      .listTemplates(cat)
      .then(({ templates: t }) => setTemplates(t))
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load templates"))
      .finally(() => setLoading(false));
  }, [activeCategory]);

  const filtered =
    activeCategory === "All"
      ? templates
      : templates.filter((t) => t.category === activeCategory);

  return (
    <div className="space-y-5">
      <p className="text-xs text-muted-foreground/60 mb-4">
        Not sure what to pick? <strong>Uptime Kuma</strong> is great for beginners — it monitors whether your websites are up.
      </p>

      {/* Category tabs */}
      <div className="flex flex-wrap gap-1.5">
        {CATEGORIES.map((cat) => (
          <button
            key={cat}
            type="button"
            className={cn(
              "rounded-lg px-3 py-1.5 text-xs font-semibold transition-all",
              activeCategory === cat
                ? "bg-brand text-primary-foreground shadow-[0_0_12px_rgba(255,107,53,0.2)]"
                : "bg-muted text-muted-foreground hover:bg-muted/80 hover:text-foreground",
            )}
            onClick={() => setActiveCategory(cat)}
          >
            {cat}
          </button>
        ))}
      </div>

      {/* Loading state */}
      {loading && (
        <div className="flex flex-col items-center py-14 text-center">
          <Loader className="h-6 w-6 text-brand animate-spin mb-3" />
          <p className="text-sm text-muted-foreground">Loading templates...</p>
        </div>
      )}

      {/* Error state */}
      {!loading && error && (
        <div className="flex flex-col items-center py-14 text-center">
          <AlertTriangle className="h-6 w-6 text-red-400 mb-3" />
          <p className="text-sm text-red-400">{error}</p>
          <button
            type="button"
            className="mt-3 text-xs font-medium text-brand hover:underline"
            onClick={() => {
              setError("");
              setLoading(true);
              const cat = activeCategory === "All" ? undefined : activeCategory;
              api
                .listTemplates(cat)
                .then(({ templates: t }) => setTemplates(t))
                .catch((e) => setError(e instanceof Error ? e.message : "Failed to load templates"))
                .finally(() => setLoading(false));
            }}
          >
            Retry
          </button>
        </div>
      )}

      {/* Empty state */}
      {!loading && !error && filtered.length === 0 && (
        <div className="flex flex-col items-center py-14 text-center">
          <p className="text-sm text-muted-foreground">
            No templates found{activeCategory !== "All" ? ` in ${activeCategory}` : ""}.
          </p>
        </div>
      )}

      {/* Template grid */}
      {!loading && !error && filtered.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
          {filtered.map((tpl) => {
            const active = selected?.id === tpl.id;
            return (
              <Card
                key={tpl.id}
                className={cn(
                  "oven-card feature-card bg-card cursor-pointer transition-all duration-200",
                  active
                    ? "border-brand/50 shadow-[0_0_25px_rgba(255,107,53,0.12)] bg-brand-muted"
                    : "border-border/50 hover:border-brand/25",
                )}
                onClick={() => onSelect(tpl)}
              >
                <CardContent className="p-4 flex flex-col gap-2.5 relative">
                  <div className="flex items-start gap-3">
                    <span className="text-2xl leading-none shrink-0" role="img" aria-label={tpl.name}>
                      {tpl.icon}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p className="text-sm font-bold text-foreground truncate">{tpl.name}</p>
                        {tpl.official && (
                          <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-brand/30 text-brand/80">
                            Official
                          </Badge>
                        )}
                      </div>
                      <p className="text-[11px] text-muted-foreground mt-0.5 line-clamp-2 leading-relaxed">
                        {tpl.description}
                      </p>
                    </div>
                    {active && (
                      <CheckCircle className="h-4 w-4 shrink-0 text-brand" />
                    )}
                  </div>
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <Badge variant="outline" className="text-[9px] px-1.5 py-0">
                      {tpl.category}
                    </Badge>
                    {tpl.est_resources && (
                      <Badge variant="outline" className="text-[9px] px-1.5 py-0 border-yellow-400/30 text-yellow-400/80">
                        {tpl.est_resources}
                      </Badge>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
