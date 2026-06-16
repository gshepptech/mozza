import { useDraggable } from "@dnd-kit/core";
import { Package } from "lucide-react";
import type { CanvasBlock } from "../../api/types";
import { kindStyles, kindIcons, defaultKindStyle } from "./BlockPalette";
import { cn } from "@/lib/utils";

interface ServiceBlockProps {
  block: CanvasBlock;
  selected: boolean;
  onSelect: (id: string) => void;
}

export default function ServiceBlock({ block, selected, onSelect }: ServiceBlockProps) {
  const { attributes, listeners, setNodeRef, transform, isDragging } = useDraggable({
    id: block.id,
    data: { type: "block", block },
  });

  const styles = kindStyles[block.kind] ?? defaultKindStyle;
  const Icon = kindIcons[block.kind] ?? Package;

  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      onClick={(e) => { e.stopPropagation(); onSelect(block.id); }}
      className={cn(
        "absolute w-[180px] rounded-xl bg-elevated p-4",
        selected
          ? "border-2 border-brand shadow-[0_0_20px_rgba(255,107,53,.2)]"
          : cn("border", styles.border, "shadow-md"),
        isDragging ? "cursor-grabbing opacity-70" : "cursor-grab",
      )}
      style={{
        left: block.x,
        top: block.y,
        transform: transform ? `translate(${transform.x}px, ${transform.y}px)` : undefined,
        zIndex: isDragging ? 100 : selected ? 10 : 1,
      }}
    >
      <div className="mb-2 flex items-center gap-2">
        <Icon className={cn("h-[18px] w-[18px]", styles.text)} />
        <span className="text-sm font-bold text-foreground">{block.name}</span>
      </div>
      <div className="mb-1 truncate font-mono text-[11px] text-muted-foreground">{block.image}</div>
      <div className="flex gap-2 text-[11px] text-muted-foreground/70">
        {block.port > 0 && <span>:{block.port}</span>}
        {block.replicas > 0 && <span>{block.replicas}x</span>}
        {block.public && <span className="text-success">public</span>}
      </div>
      <div className={cn("absolute -right-1 -top-1 h-2.5 w-2.5 rounded-full border-2 border-elevated", styles.dot)} />
    </div>
  );
}
