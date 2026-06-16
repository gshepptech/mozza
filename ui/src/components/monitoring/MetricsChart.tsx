import { useMemo } from "react";

// ── Types ────────────────────────────────────────────────────────

export interface DataPoint {
  time: string;
  value: number;
}

export interface DualDataPoint {
  time: string;
  rx: number;
  tx: number;
}

interface BaseChartProps {
  width?: number;
  height?: number;
}

interface LineChartProps extends BaseChartProps {
  type: "line";
  data: DataPoint[];
  color?: string;
  maxY?: number;
}

interface AreaChartProps extends BaseChartProps {
  type: "area";
  data: DataPoint[];
  thresholds?: { green: number; yellow: number };
}

interface DualLineChartProps extends BaseChartProps {
  type: "dual-line";
  data: DualDataPoint[];
  colors?: { rx: string; tx: string };
}

export type MetricsChartProps = LineChartProps | AreaChartProps | DualLineChartProps;

// ── Sparkline (mini chart for cards) ─────────────────────────────

export interface SparklineProps {
  data: number[];
  width?: number;
  height?: number;
  color?: string;
}

export function Sparkline({
  data,
  width = 120,
  height = 32,
  color = "#ff6b35",
}: SparklineProps) {
  const path = useMemo(() => {
    if (data.length < 2) return "";
    const max = Math.max(...data, 1);
    const stepX = width / (data.length - 1);
    const points = data.map((v, i) => {
      const x = i * stepX;
      const y = height - (v / max) * height * 0.9 - height * 0.05;
      return `${x},${y}`;
    });
    return `M${points.join(" L")}`;
  }, [data, width, height]);

  if (data.length < 2) return null;

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      style={{ display: "block" }}
    >
      <path
        d={path}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// ── SVG Path Helpers ─────────────────────────────────────────────

function buildPath(
  data: { x: number; y: number }[],
): string {
  if (data.length < 2) return "";
  return `M${data.map((p) => `${p.x},${p.y}`).join(" L")}`;
}

function buildAreaPath(
  data: { x: number; y: number }[],
  bottomY: number,
): string {
  if (data.length < 2) return "";
  const first = data[0]!;
  const last = data[data.length - 1]!;
  const linePath = data.map((p) => `${p.x},${p.y}`).join(" L");
  return `M${first.x},${bottomY} L${linePath} L${last.x},${bottomY} Z`;
}

// ── Time axis labels ─────────────────────────────────────────────

function timeLabels(data: { time: string }[], count: number): string[] {
  if (data.length === 0) return [];
  const step = Math.max(1, Math.floor(data.length / count));
  const labels: string[] = [];
  for (let i = 0; i < data.length; i += step) {
    const item = data[i];
    if (!item) continue;
    const d = new Date(item.time);
    labels.push(
      d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
    );
  }
  return labels;
}

// ── Y axis labels ────────────────────────────────────────────────

function yLabels(maxY: number, count: number): string[] {
  const labels: string[] = [];
  for (let i = 0; i <= count; i++) {
    const val = Math.round((maxY / count) * i);
    labels.push(`${val}%`);
  }
  return labels;
}

// ── Main Chart Component ─────────────────────────────────────────

const PADDING = { top: 8, right: 12, bottom: 24, left: 36 };

export function MetricsChart(props: MetricsChartProps) {
  const width = props.width ?? 400;
  const height = props.height ?? 160;
  const plotW = width - PADDING.left - PADDING.right;
  const plotH = height - PADDING.top - PADDING.bottom;

  if (props.type === "line") {
    return <LineChart {...props} w={width} h={height} pw={plotW} ph={plotH} />;
  }
  if (props.type === "area") {
    return <AreaChart {...props} w={width} h={height} pw={plotW} ph={plotH} />;
  }
  return <DualLineChart {...props} w={width} h={height} pw={plotW} ph={plotH} />;
}

// ── Line Chart (CPU) ─────────────────────────────────────────────

function LineChart({
  data,
  color = "#ff6b35",
  maxY = 100,
  w,
  h,
  pw,
  ph,
}: LineChartProps & { w: number; h: number; pw: number; ph: number }) {
  const points = useMemo(() => {
    if (data.length < 2) return [];
    return data.map((d, i) => ({
      x: PADDING.left + (i / (data.length - 1)) * pw,
      y: PADDING.top + ph - (d.value / maxY) * ph,
    }));
  }, [data, pw, ph, maxY]);

  const tLabels = timeLabels(data, 4);
  const yLbls = yLabels(maxY, 4);

  if (data.length < 2) {
    return <EmptyChart w={w} h={h} />;
  }

  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ display: "block" }}>
      <GridLines pw={pw} ph={ph} count={4} />
      {yLbls.map((label, i) => (
        <text
          key={i}
          x={PADDING.left - 4}
          y={PADDING.top + ph - (i / 4) * ph + 3}
          textAnchor="end"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {label}
        </text>
      ))}
      {tLabels.map((label, i) => (
        <text
          key={i}
          x={PADDING.left + (i / (tLabels.length - 1 || 1)) * pw}
          y={h - 4}
          textAnchor="middle"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {label}
        </text>
      ))}
      <path d={buildPath(points)} fill="none" stroke={color} strokeWidth={2} />
    </svg>
  );
}

// ── Area Chart (Memory) ──────────────────────────────────────────

function AreaChart({
  data,
  thresholds = { green: 60, yellow: 80 },
  w,
  h,
  pw,
  ph,
}: AreaChartProps & { w: number; h: number; pw: number; ph: number }) {
  const points = useMemo(() => {
    if (data.length < 2) return [];
    return data.map((d, i) => ({
      x: PADDING.left + (i / (data.length - 1)) * pw,
      y: PADDING.top + ph - (d.value / 100) * ph,
    }));
  }, [data, pw, ph]);

  const lastPoint = data[data.length - 1];
  const latestValue = lastPoint ? lastPoint.value : 0;
  const fillColor =
    latestValue > thresholds.yellow
      ? "rgba(239,68,68,0.15)"
      : latestValue > thresholds.green
        ? "rgba(234,179,8,0.15)"
        : "rgba(34,197,94,0.15)";
  const strokeColor =
    latestValue > thresholds.yellow
      ? "#ef4444"
      : latestValue > thresholds.green
        ? "#eab308"
        : "#22c55e";

  const tLabels = timeLabels(data, 4);
  const yLbls = yLabels(100, 4);
  const bottomY = PADDING.top + ph;

  if (data.length < 2) {
    return <EmptyChart w={w} h={h} />;
  }

  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ display: "block" }}>
      <GridLines pw={pw} ph={ph} count={4} />
      {/* Threshold bands */}
      <rect
        x={PADDING.left}
        y={PADDING.top}
        width={pw}
        height={(1 - thresholds.yellow / 100) * ph}
        fill="rgba(239,68,68,0.05)"
      />
      <rect
        x={PADDING.left}
        y={PADDING.top + (1 - thresholds.yellow / 100) * ph}
        width={pw}
        height={((thresholds.yellow - thresholds.green) / 100) * ph}
        fill="rgba(234,179,8,0.05)"
      />
      {yLbls.map((label, i) => (
        <text
          key={i}
          x={PADDING.left - 4}
          y={PADDING.top + ph - (i / 4) * ph + 3}
          textAnchor="end"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {label}
        </text>
      ))}
      {tLabels.map((label, i) => (
        <text
          key={i}
          x={PADDING.left + (i / (tLabels.length - 1 || 1)) * pw}
          y={h - 4}
          textAnchor="middle"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {label}
        </text>
      ))}
      <path d={buildAreaPath(points, bottomY)} fill={fillColor} />
      <path d={buildPath(points)} fill="none" stroke={strokeColor} strokeWidth={2} />
    </svg>
  );
}

// ── Dual Line Chart (Network) ────────────────────────────────────

function DualLineChart({
  data,
  colors = { rx: "#3b82f6", tx: "#ff6b35" },
  w,
  h,
  pw,
  ph,
}: DualLineChartProps & { w: number; h: number; pw: number; ph: number }) {
  const { rxPoints, txPoints, maxVal } = useMemo(() => {
    if (data.length < 2) return { rxPoints: [], txPoints: [], maxVal: 0 };
    const allVals = data.flatMap((d) => [d.rx, d.tx]);
    const mv = Math.max(...allVals, 1);
    const rx = data.map((d, i) => ({
      x: PADDING.left + (i / (data.length - 1)) * pw,
      y: PADDING.top + ph - (d.rx / mv) * ph,
    }));
    const tx = data.map((d, i) => ({
      x: PADDING.left + (i / (data.length - 1)) * pw,
      y: PADDING.top + ph - (d.tx / mv) * ph,
    }));
    return { rxPoints: rx, txPoints: tx, maxVal: mv };
  }, [data, pw, ph]);

  const tLabels = timeLabels(data, 4);

  if (data.length < 2) {
    return <EmptyChart w={w} h={h} />;
  }

  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ display: "block" }}>
      <GridLines pw={pw} ph={ph} count={4} />
      {[0, 1, 2, 3, 4].map((i) => (
        <text
          key={i}
          x={PADDING.left - 4}
          y={PADDING.top + ph - (i / 4) * ph + 3}
          textAnchor="end"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {formatBytes(Math.round((maxVal / 4) * i))}
        </text>
      ))}
      {tLabels.map((label, i) => (
        <text
          key={i}
          x={PADDING.left + (i / (tLabels.length - 1 || 1)) * pw}
          y={h - 4}
          textAnchor="middle"
          style={{ fontSize: 9, fill: "#666" }}
        >
          {label}
        </text>
      ))}
      <path d={buildPath(rxPoints)} fill="none" stroke={colors.rx} strokeWidth={2} />
      <path d={buildPath(txPoints)} fill="none" stroke={colors.tx} strokeWidth={2} />
      {/* Legend */}
      <circle cx={PADDING.left + 4} cy={PADDING.top + 4} r={3} fill={colors.rx} />
      <text x={PADDING.left + 12} y={PADDING.top + 8} style={{ fontSize: 9, fill: "#999" }}>
        RX
      </text>
      <circle cx={PADDING.left + 36} cy={PADDING.top + 4} r={3} fill={colors.tx} />
      <text x={PADDING.left + 44} y={PADDING.top + 8} style={{ fontSize: 9, fill: "#999" }}>
        TX
      </text>
    </svg>
  );
}

// ── Shared Components ────────────────────────────────────────────

function GridLines({
  pw,
  ph,
  count,
}: {
  pw: number;
  ph: number;
  count: number;
}) {
  return (
    <>
      {Array.from({ length: count + 1 }, (_, i) => (
        <line
          key={i}
          x1={PADDING.left}
          x2={PADDING.left + pw}
          y1={PADDING.top + (i / count) * ph}
          y2={PADDING.top + (i / count) * ph}
          stroke="#333"
          strokeWidth={0.5}
        />
      ))}
    </>
  );
}

function EmptyChart({ w, h }: { w: number; h: number }) {
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} style={{ display: "block" }}>
      <text
        x={w / 2}
        y={h / 2}
        textAnchor="middle"
        style={{ fontSize: 11, fill: "#666" }}
      >
        No data available
      </text>
    </svg>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`;
  if (bytes < 1048576) return `${(bytes / 1024).toFixed(0)}K`;
  return `${(bytes / 1048576).toFixed(1)}M`;
}

// ── Time Period Selector ─────────────────────────────────────────

export type TimePeriod = "1h" | "6h" | "24h" | "7d";

export interface TimePeriodSelectorProps {
  value: TimePeriod;
  onChange: (period: TimePeriod) => void;
}

const PERIODS: TimePeriod[] = ["1h", "6h", "24h", "7d"];

export function TimePeriodSelector({ value, onChange }: TimePeriodSelectorProps) {
  return (
    <div style={{ display: "flex", gap: 2, background: "#1a1a1a", borderRadius: 6, padding: 2 }}>
      {PERIODS.map((p) => (
        <button
          key={p}
          onClick={() => onChange(p)}
          style={{
            padding: "4px 10px",
            fontSize: 11,
            fontWeight: value === p ? 600 : 400,
            color: value === p ? "#fff" : "#888",
            background: value === p ? "#ff6b35" : "transparent",
            border: "none",
            borderRadius: 4,
            cursor: "pointer",
            transition: "all 0.15s",
          }}
        >
          {p}
        </button>
      ))}
    </div>
  );
}
