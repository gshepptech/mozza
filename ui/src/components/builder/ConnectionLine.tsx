import type { CanvasBlock, CanvasConnection } from "../../api/types";

interface ConnectionLineProps {
  connection: CanvasConnection;
  blocks: CanvasBlock[];
}

export default function ConnectionLine({ connection, blocks }: ConnectionLineProps) {
  const from = blocks.find(b => b.id === connection.from);
  const to = blocks.find(b => b.id === connection.to);
  if (!from || !to) return null;

  const x1 = from.x + 180;
  const y1 = from.y + 30;
  const x2 = to.x;
  const y2 = to.y + 30;

  const midX = (x1 + x2) / 2;

  return (
    <g>
      <path
        d={`M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`}
        fill="none"
        stroke="var(--color-brand-border)"
        strokeWidth={2}
        strokeDasharray="6,4"
      />
      <circle cx={x2} cy={y2} r={4} fill="var(--color-brand)" />
    </g>
  );
}
