import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import { LayoutGrid, GitBranch, FileCode, CheckCircle } from "lucide-react";

export type DeploySource = "template" | "github" | "custom";

interface SourceSelectorProps {
  selected: DeploySource | null;
  onSelect: (source: DeploySource) => void;
}

const SOURCES: Array<{
  id: DeploySource;
  icon: typeof LayoutGrid;
  title: string;
  subtitle: string;
  description: string;
}> = [
  {
    id: "template",
    icon: LayoutGrid,
    title: "Template Catalog",
    subtitle: "Pick from ready-to-deploy apps",
    description:
      "WordPress, Ghost, Gitea, and more — pre-configured and tested",
  },
  {
    id: "github",
    icon: GitBranch,
    title: "Import from GitHub",
    subtitle: "Paste a repo URL",
    description:
      "We'll scan for Docker Compose, Helm charts, or Dockerfiles",
  },
  {
    id: "custom",
    icon: FileCode,
    title: "Custom Recipe",
    subtitle: "Write or select a recipe",
    description:
      "Full control — create from scratch or use an existing recipe",
  },
];

export default function SourceSelector({ selected, onSelect }: SourceSelectorProps) {
  return (
    <div className="grid grid-cols-1 gap-4">
      {SOURCES.map((src) => {
        const active = selected === src.id;
        const Icon = src.icon;
        return (
          <Card
            key={src.id}
            className={cn(
              "oven-card feature-card bg-card cursor-pointer transition-all duration-200",
              active
                ? "border-brand/50 shadow-[0_0_25px_rgba(255,107,53,0.12)] bg-brand-muted"
                : "border-border/50 hover:border-brand/25 hover:shadow-[0_0_15px_rgba(255,107,53,0.06)]",
            )}
            onClick={() => onSelect(src.id)}
          >
            <CardContent className="p-6 flex items-start gap-5">
              <div
                className={cn(
                  "flex h-12 w-12 shrink-0 items-center justify-center rounded-xl transition-colors",
                  active
                    ? "bg-brand text-primary-foreground"
                    : "bg-muted text-muted-foreground",
                )}
              >
                <Icon className="h-6 w-6" />
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-bold text-foreground">{src.title}</p>
                <p className="text-xs font-medium text-brand/80 mt-0.5">
                  {src.subtitle}
                </p>
                <p className="text-xs text-muted-foreground mt-1.5 leading-relaxed">
                  {src.description}
                </p>
              </div>
              {active && (
                <CheckCircle className="h-5 w-5 shrink-0 text-brand mt-1" />
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
