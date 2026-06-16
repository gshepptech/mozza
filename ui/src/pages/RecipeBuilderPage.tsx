import { useState, useEffect, useCallback } from "react";
import { useParams, useNavigate } from "react-router-dom";
import * as api from "../api/client";
import type { CanvasState } from "../api/types";
import Canvas from "../components/builder/Canvas";
import { ArrowLeft, Code, Eye, Save, Loader, CheckCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { CodeBlock } from "@/components/custom/code-block";
import { toast } from "sonner";

function generateSource(state: CanvasState, recipeName?: string): string {
  if (state.blocks.length === 0) return "";

  const appName = recipeName || state.blocks.find(b => b.kind === "web")?.name || state.blocks[0]?.name || "app";
  let source = `App: ${appName}\n`;

  for (const block of state.blocks) {
    source += `\n${block.name}:\n`;
    source += `  from image ${block.image}\n`;

    switch (block.kind) {
      case "web":
      case "worker":
        if (block.public && block.port > 0) {
          source += `  open to the public on port ${block.port}\n`;
        } else if (block.port > 0) {
          source += `  listen on port ${block.port}\n`;
        }
        break;

      case "api":
        if (block.port > 0) {
          source += `  on port ${block.port}\n`;
        }
        break;

      case "gateway":
        if (block.port > 0) {
          source += `  open to the public on port ${block.port}\n`;
        }
        break;

      case "task":
        source += `  run once\n`;
        if (block.parallelism && block.parallelism > 1) {
          source += `  parallelism ${block.parallelism}\n`;
        }
        if (block.retries && block.retries > 0) {
          source += `  retries ${block.retries}\n`;
        }
        break;

      case "scheduled":
        if (block.schedule) {
          source += `  ${block.schedule}\n`;
        }
        break;

      case "database": {
        const engine = block.engine ?? "postgres";
        const ver = block.version ?? "";
        const stor = block.storage ?? "";
        const parts = [engine, ver, stor].filter(Boolean).join(" ");
        if (parts) source += `  ${parts}\n`;
        if (block.backups) source += `  enable backups\n`;
        break;
      }

      case "stateful":
        if (block.stateful_storage) {
          source += `  each copy needs its own storage of ${block.stateful_storage}\n`;
        }
        if (block.ordered_startup) {
          source += `  start copies in order\n`;
        }
        break;

      case "daemon":
        source += `  run on every node\n`;
        break;

      default:
        if (block.public && block.port > 0) {
          source += `  open to the public on port ${block.port}\n`;
        } else if (block.port > 0) {
          source += `  listen on port ${block.port}\n`;
        }
        break;
    }

    if (block.health_path) {
      source += `  health check ${block.health_path}\n`;
    }

    // Replicas for kinds that support them
    if (block.kind !== "task" && block.kind !== "scheduled" && block.kind !== "daemon") {
      if (block.replicas > 1) {
        source += `  run ${block.replicas} copies\n`;
      } else {
        source += `  run 1 copy\n`;
      }
    }

    const deps = state.connections.filter(c => c.from === block.id);
    for (const dep of deps) {
      const target = state.blocks.find(b => b.id === dep.to);
      if (target) {
        source += `  needs ${target.name}\n`;
      }
    }
  }

  return source;
}

export default function RecipeBuilderPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [canvas, setCanvas] = useState<CanvasState>({ blocks: [], connections: [] });
  const [saving, setSaving] = useState(false);
  const [showSource, setShowSource] = useState(false);
  const [name, setName] = useState("");

  useEffect(() => {
    if (!id) return;
    api.getRecipe(id).then(r => {
      setName(r.name);
      try {
        const parsed = JSON.parse(r.canvas);
        if (parsed.blocks) setCanvas(parsed);
      } catch {
        // Invalid canvas JSON, start fresh.
      }
    }).catch(() => navigate("/app"));
  }, [id]);

  const handleSave = useCallback(async () => {
    if (!id) return;
    setSaving(true);
    try {
      const source = generateSource(canvas, name);
      await api.updateRecipe(id, name, source, JSON.stringify(canvas));
      toast.success("Recipe saved");
    } catch {
      toast.error("Failed to save recipe");
    } finally {
      setSaving(false);
    }
  }, [id, name, canvas]);

  const handleValidate = useCallback(async () => {
    const src = generateSource(canvas, name);
    if (!src) { toast.error("Add blocks to validate"); return; }
    try {
      const result = await api.validateRecipe(src);
      if (result.valid) {
        toast.success("Recipe is valid");
      } else {
        toast.error(result.errors?.join(", ") || "Invalid recipe");
      }
    } catch {
      toast.error("Validation failed");
    }
  }, [canvas, name]);

  const source = generateSource(canvas, name);

  return (
    <div className="flex flex-col h-[calc(100vh-64px)]">
      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-3 bg-surface border-b border-border">
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => navigate("/app")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="flex-1 border-none bg-transparent text-lg font-semibold focus-visible:ring-0 px-0"
        />
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowSource(!showSource)}
        >
          {showSource ? <Eye className="mr-2 h-3.5 w-3.5" /> : <Code className="mr-2 h-3.5 w-3.5" />}
          {showSource ? "Canvas" : "Source"}
        </Button>
        <Button variant="outline" size="sm" onClick={handleValidate} disabled={canvas.blocks.length === 0}>
          <CheckCircle className="mr-2 h-3.5 w-3.5" />
          Validate
        </Button>
        <Button size="sm" onClick={handleSave} disabled={saving}>
          {saving ? <Loader className="mr-2 h-3.5 w-3.5 animate-spin" /> : <Save className="mr-2 h-3.5 w-3.5" />}
          {saving ? "Saving..." : "Save"}
        </Button>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Canvas */}
        <div className="flex-1 flex">
          <Canvas state={canvas} onChange={setCanvas} />
        </div>

        {/* Source preview */}
        {showSource && (
          <div className="w-80 border-l border-border overflow-y-auto p-4 bg-code-bg">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-3">
              Generated .mozza
            </h3>
            {source ? (
              <CodeBlock code={source} showLineNumbers={false} className="border-none" />
            ) : (
              <p className="text-sm text-muted-foreground">
                Drag blocks onto the canvas to generate recipe source
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
