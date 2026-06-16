import { useState, useCallback } from "react";
import { DndContext, DragEndEvent, DragStartEvent, DragOverlay, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import type { CanvasBlock, CanvasState } from "../../api/types";
import ServiceBlock from "./ServiceBlock";
import ConnectionLine from "./ConnectionLine";
import BlockPalette, { blockTypes, kindStyles, defaultKindStyle } from "./BlockPalette";
import PropertyPanel from "./PropertyPanel";
import { cn } from "@/lib/utils";

interface CanvasProps {
  state: CanvasState;
  onChange: (state: CanvasState) => void;
}

let nextBlockId = 1;

function generateId(): string {
  return `block-${Date.now()}-${nextBlockId++}`;
}

export default function Canvas({ state, onChange }: CanvasProps) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [dragKind, setDragKind] = useState<string | null>(null);
  const selectedBlock = state.blocks.find(b => b.id === selectedId) || null;

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } })
  );

  const handleDragStart = useCallback((event: DragStartEvent) => {
    const data = event.active.data.current;
    if (data?.type === "palette") {
      setDragKind(data.kind as string);
    } else {
      setDragKind(null);
    }
  }, []);

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, delta } = event;
    const data = active.data.current;
    setDragKind(null);

    if (data?.type === "palette") {
      const bt = blockTypes.find(b => b.kind === data.kind);
      if (!bt) return;

      const noReplicaKinds = ["task", "scheduled", "daemon"];
      const newBlock: CanvasBlock = {
        id: generateId(),
        kind: bt.kind as CanvasBlock["kind"],
        name: bt.kind,
        image: bt.defaultImage,
        port: bt.defaultPort,
        public: bt.kind === "web" || bt.kind === "gateway",
        replicas: noReplicaKinds.includes(bt.kind) ? 0 : 1,
        health_path: bt.kind === "web" || bt.kind === "api" ? "/healthz" : "",
        x: Math.max(0, 200 + delta.x),
        y: Math.max(0, 100 + delta.y),
        ...(bt.kind === "task" && { parallelism: 1 }),
        ...(bt.kind === "scheduled" && { schedule: "" }),
        ...(bt.kind === "stateful" && { stateful_storage: "" }),
      };

      onChange({
        ...state,
        blocks: [...state.blocks, newBlock],
      });
      setSelectedId(newBlock.id);
    } else if (data?.type === "block") {
      onChange({
        ...state,
        blocks: state.blocks.map(b =>
          b.id === active.id
            ? { ...b, x: b.x + delta.x, y: b.y + delta.y }
            : b
        ),
      });
    }
  }, [state, onChange]);

  const handleUpdate = useCallback((id: string, updates: Partial<CanvasBlock>) => {
    onChange({
      ...state,
      blocks: state.blocks.map(b => b.id === id ? { ...b, ...updates } : b),
    });
  }, [state, onChange]);

  const handleRemove = useCallback((id: string) => {
    onChange({
      blocks: state.blocks.filter(b => b.id !== id),
      connections: state.connections.filter(c => c.from !== id && c.to !== id),
    });
    if (selectedId === id) setSelectedId(null);
  }, [state, selectedId, onChange]);

  const handleConnect = useCallback((fromId: string, toId: string) => {
    const exists = state.connections.some(c => c.from === fromId && c.to === toId);
    if (exists) return;
    onChange({
      ...state,
      connections: [...state.connections, { from: fromId, to: toId }],
    });
  }, [state, onChange]);

  const dragOverlayContent = dragKind ? (() => {
    const bt = blockTypes.find(b => b.kind === dragKind);
    const styles = kindStyles[dragKind] ?? defaultKindStyle;
    return (
      <div className={cn(
        "flex items-center gap-2.5 rounded-[10px] border-2 px-4 py-3 opacity-90 shadow-lg pointer-events-none",
        styles.bg, styles.border,
      )}>
        {bt && <bt.icon className={cn("h-5 w-5", styles.text)} />}
        <span className={cn("text-[13px] font-semibold", styles.text)}>{bt?.label}</span>
      </div>
    );
  })() : null;

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
      <div className="flex flex-1">
        <BlockPalette />

        {/* Canvas area */}
        <div
          className="relative flex-1 overflow-auto bg-background min-h-[400px]"
          style={{
            backgroundImage: "radial-gradient(circle, #1a1a1a 1px, transparent 1px)",
            backgroundSize: "24px 24px",
          }}
          onClick={() => setSelectedId(null)}
        >
          <svg className="pointer-events-none absolute inset-0 h-full w-full">
            {state.connections.map((conn, i) => (
              <ConnectionLine key={i} connection={conn} blocks={state.blocks} />
            ))}
          </svg>

          {state.blocks.map(block => (
            <ServiceBlock
              key={block.id}
              block={block}
              selected={block.id === selectedId}
              onSelect={setSelectedId}
            />
          ))}

          {state.blocks.length === 0 && (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
              <p className="text-base font-semibold text-muted-foreground/30">
                Drag blocks from the palette to get started
              </p>
            </div>
          )}
        </div>

        <PropertyPanel
          block={selectedBlock}
          allBlocks={state.blocks}
          onUpdate={handleUpdate}
          onRemove={handleRemove}
          onConnect={handleConnect}
        />
      </div>

      <DragOverlay dropAnimation={null}>
        {dragOverlayContent}
      </DragOverlay>
    </DndContext>
  );
}
