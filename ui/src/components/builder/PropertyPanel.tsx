import type { CanvasBlock } from "../../api/types";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";

interface PropertyPanelProps {
  block: CanvasBlock | null;
  allBlocks: CanvasBlock[];
  onUpdate: (id: string, updates: Partial<CanvasBlock>) => void;
  onRemove: (id: string) => void;
  onConnect: (fromId: string, toId: string) => void;
}

export default function PropertyPanel({ block, allBlocks, onUpdate, onRemove, onConnect }: PropertyPanelProps) {
  if (!block) {
    return (
      <div className="w-[260px] border-l border-border bg-surface p-5">
        <p className="mt-10 text-center text-[13px] text-muted-foreground">
          Select a block to edit its properties
        </p>
      </div>
    );
  }

  const otherBlocks = allBlocks.filter(b => b.id !== block.id);

  return (
    <div className="w-[260px] overflow-y-auto border-l border-border bg-surface p-5">
      <h3 className="mb-4 text-sm font-bold text-foreground">Properties</h3>

      <div className="mb-4 space-y-1.5">
        <Label>Name</Label>
        <Input value={block.name} onChange={(e) => onUpdate(block.id, { name: e.target.value })} />
      </div>

      <div className="mb-4 space-y-1.5">
        <Label>Image</Label>
        <Input value={block.image} onChange={(e) => onUpdate(block.id, { image: e.target.value })} />
      </div>

      {/* Port — hide for daemon */}
      {block.kind !== "daemon" && (
        <div className="mb-4 space-y-1.5">
          <Label>Port</Label>
          <Input type="number" value={String(block.port)} onChange={(e) => onUpdate(block.id, { port: parseInt(e.target.value) || 0 })} />
        </div>
      )}

      {/* Replicas — hide for task, scheduled, daemon */}
      {block.kind !== "task" && block.kind !== "scheduled" && block.kind !== "daemon" && (
        <div className="mb-4 space-y-1.5">
          <Label>Replicas</Label>
          <Input type="number" value={String(block.replicas)} onChange={(e) => onUpdate(block.id, { replicas: parseInt(e.target.value) || 1 })} />
        </div>
      )}

      {/* Task-specific: parallelism + retries */}
      {block.kind === "task" && (
        <>
          <div className="mb-4 space-y-1.5">
            <Label>Parallelism</Label>
            <Input type="number" value={String(block.parallelism ?? 1)} onChange={(e) => onUpdate(block.id, { parallelism: parseInt(e.target.value) || 1 })} />
          </div>
          <div className="mb-4 space-y-1.5">
            <Label>Retries</Label>
            <Input type="number" value={String(block.retries ?? 0)} onChange={(e) => onUpdate(block.id, { retries: parseInt(e.target.value) || 0 })} />
          </div>
        </>
      )}

      {/* Scheduled-specific: schedule expression */}
      {block.kind === "scheduled" && (
        <div className="mb-4 space-y-1.5">
          <Label>Schedule</Label>
          <Input
            value={block.schedule ?? ""}
            placeholder="e.g., every day at 3am"
            onChange={(e) => onUpdate(block.id, { schedule: e.target.value })}
          />
        </div>
      )}

      {/* Database-specific: engine, version, storage, backups */}
      {block.kind === "database" && (
        <>
          <div className="mb-4 space-y-1.5">
            <Label>Engine</Label>
            <select
              value={block.engine ?? "postgres"}
              onChange={(e) => onUpdate(block.id, { engine: e.target.value })}
              className="w-full rounded-lg border border-border bg-elevated px-3 py-2 text-[13px] text-foreground"
            >
              <option value="postgres">PostgreSQL</option>
              <option value="mysql">MySQL</option>
              <option value="mongo">MongoDB</option>
            </select>
          </div>
          <div className="mb-4 space-y-1.5">
            <Label>Version</Label>
            <Input value={block.version ?? ""} placeholder="e.g., 16" onChange={(e) => onUpdate(block.id, { version: e.target.value })} />
          </div>
          <div className="mb-4 space-y-1.5">
            <Label>Storage</Label>
            <Input value={block.storage ?? ""} placeholder="e.g., 10Gi" onChange={(e) => onUpdate(block.id, { storage: e.target.value })} />
          </div>
          <div className="mb-4">
            <label className="flex cursor-pointer items-center gap-2 text-[13px] text-muted-foreground">
              <input
                type="checkbox"
                checked={block.backups ?? false}
                onChange={(e) => onUpdate(block.id, { backups: e.target.checked })}
                className="accent-brand"
              />
              Enable backups
            </label>
          </div>
        </>
      )}

      {/* Stateful-specific: storage + ordered startup */}
      {block.kind === "stateful" && (
        <>
          <div className="mb-4 space-y-1.5">
            <Label>Storage per copy</Label>
            <Input value={block.stateful_storage ?? ""} placeholder="e.g., 20Gi" onChange={(e) => onUpdate(block.id, { stateful_storage: e.target.value })} />
          </div>
          <div className="mb-4">
            <label className="flex cursor-pointer items-center gap-2 text-[13px] text-muted-foreground">
              <input
                type="checkbox"
                checked={block.ordered_startup ?? false}
                onChange={(e) => onUpdate(block.id, { ordered_startup: e.target.checked })}
                className="accent-brand"
              />
              Start copies in order
            </label>
          </div>
        </>
      )}

      <div className="mb-4 space-y-1.5">
        <Label>Health Path</Label>
        <Input value={block.health_path} onChange={(e) => onUpdate(block.id, { health_path: e.target.value })} />
      </div>

      <div className="mb-4">
        <label className="flex cursor-pointer items-center gap-2 text-[13px] text-muted-foreground">
          <input
            type="checkbox"
            checked={block.public}
            onChange={(e) => onUpdate(block.id, { public: e.target.checked })}
            className="accent-brand"
          />
          Public (exposed to internet)
        </label>
      </div>

      {otherBlocks.length > 0 && (
        <div className="mb-4 space-y-1.5">
          <Label>Connect to...</Label>
          <select
            onChange={(e) => { if (e.target.value) { onConnect(block.id, e.target.value); e.target.value = ""; } }}
            defaultValue=""
            className="w-full rounded-lg border border-border bg-elevated px-3 py-2 text-[13px] text-foreground"
          >
            <option value="">Select a block...</option>
            {otherBlocks.map(b => (
              <option key={b.id} value={b.id}>{b.name} ({b.kind})</option>
            ))}
          </select>
        </div>
      )}

      <Button variant="destructive" className="mt-2 w-full" onClick={() => onRemove(block.id)}>
        Remove Block
      </Button>
    </div>
  );
}
