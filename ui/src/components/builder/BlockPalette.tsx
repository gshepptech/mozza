import { useDraggable } from "@dnd-kit/core";
import { Globe, Cog, Database, Zap, Server, Shield, Play, Clock, HardDrive, Cpu, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

const blockTypes = [
  { kind: "web", icon: Globe, label: "Web Service", defaultImage: "nginx:alpine", defaultPort: 8080 },
  { kind: "worker", icon: Cog, label: "Worker", defaultImage: "node:20-alpine", defaultPort: 0 },
  { kind: "database", icon: Database, label: "Database", defaultImage: "postgres:16-alpine", defaultPort: 5432 },
  { kind: "cache", icon: Zap, label: "Cache", defaultImage: "redis:7-alpine", defaultPort: 6379 },
  { kind: "api", icon: Server, label: "API / Backend", defaultImage: "node:20-alpine", defaultPort: 8080 },
  { kind: "gateway", icon: Shield, label: "Gateway", defaultImage: "envoyproxy/envoy:v1.28", defaultPort: 8080 },
  { kind: "task", icon: Play, label: "One-time Task", defaultImage: "node:20-alpine", defaultPort: 0 },
  { kind: "scheduled", icon: Clock, label: "Scheduled Job", defaultImage: "node:20-alpine", defaultPort: 0 },
  { kind: "stateful", icon: HardDrive, label: "Stateful", defaultImage: "bitnami/kafka:3.6", defaultPort: 9092 },
  { kind: "daemon", icon: Cpu, label: "Node Agent", defaultImage: "fluent/fluent-bit:latest", defaultPort: 0 },
] as const;

const kindStyles: Record<string, { bg: string; text: string; border: string; dot: string }> = {
  web:       { bg: "bg-kind-web/15", text: "text-kind-web", border: "border-kind-web/30", dot: "bg-kind-web" },
  worker:    { bg: "bg-kind-worker/15", text: "text-kind-worker", border: "border-kind-worker/30", dot: "bg-kind-worker" },
  database:  { bg: "bg-kind-database/15", text: "text-kind-database", border: "border-kind-database/30", dot: "bg-kind-database" },
  cache:     { bg: "bg-kind-cache/15", text: "text-kind-cache", border: "border-kind-cache/30", dot: "bg-kind-cache" },
  api:       { bg: "bg-kind-api/15", text: "text-kind-api", border: "border-kind-api/30", dot: "bg-kind-api" },
  gateway:   { bg: "bg-kind-gateway/15", text: "text-kind-gateway", border: "border-kind-gateway/30", dot: "bg-kind-gateway" },
  task:      { bg: "bg-kind-task/15", text: "text-kind-task", border: "border-kind-task/30", dot: "bg-kind-task" },
  scheduled: { bg: "bg-kind-scheduled/15", text: "text-kind-scheduled", border: "border-kind-scheduled/30", dot: "bg-kind-scheduled" },
  stateful:  { bg: "bg-kind-stateful/15", text: "text-kind-stateful", border: "border-kind-stateful/30", dot: "bg-kind-stateful" },
  daemon:    { bg: "bg-kind-daemon/15", text: "text-kind-daemon", border: "border-kind-daemon/30", dot: "bg-kind-daemon" },
};

const defaultKindStyle = { bg: "bg-muted/50", text: "text-muted-foreground", border: "border-border", dot: "bg-muted-foreground" };

const kindIcons: Record<string, LucideIcon> = {
  web: Globe,
  worker: Cog,
  database: Database,
  cache: Zap,
  api: Server,
  gateway: Shield,
  task: Play,
  scheduled: Clock,
  stateful: HardDrive,
  daemon: Cpu,
};

function DraggableBlock({ kind, icon: Icon, label }: { kind: string; icon: LucideIcon; label: string }) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: `palette-${kind}`,
    data: { type: "palette", kind },
  });

  const styles = kindStyles[kind] ?? defaultKindStyle;

  return (
    <div
      ref={setNodeRef}
      className={cn(
        "flex items-center gap-2.5 rounded-[10px] border px-4 py-3 cursor-grab",
        styles.bg, styles.border,
        isDragging && "opacity-50",
      )}
      style={{ transform: transform ? `translate(${transform.x}px, ${transform.y}px)` : undefined }}
      {...listeners}
      {...attributes}
    >
      <Icon className={cn("h-5 w-5", styles.text)} />
      <span className={cn("text-[13px] font-semibold", styles.text)}>{label}</span>
    </div>
  );
}

export default function BlockPalette() {
  return (
    <div className="flex w-[200px] flex-col gap-2 overflow-y-auto border-r border-border bg-surface p-4">
      <h3 className="mb-2 text-xs font-bold uppercase tracking-wider text-muted-foreground">Blocks</h3>
      {blockTypes.map(bt => (
        <DraggableBlock key={bt.kind} kind={bt.kind} icon={bt.icon} label={bt.label} />
      ))}
      <div className="mt-4 rounded-lg border border-border bg-elevated p-3">
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          Drag blocks onto the canvas to build your recipe. Click a block to configure its properties.
        </p>
      </div>
    </div>
  );
}

export { blockTypes, kindStyles, kindIcons, defaultKindStyle };
