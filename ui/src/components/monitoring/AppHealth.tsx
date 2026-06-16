import { useState } from "react";

export type HealthStatus = "healthy" | "degraded" | "down" | "unknown";

export interface AppHealthProps {
  status: HealthStatus;
  appName: string;
  details?: string;
}

const STATUS_CONFIG: Record<HealthStatus, { color: string; label: string; pulse: boolean }> = {
  healthy: { color: "#22c55e", label: "Healthy", pulse: false },
  degraded: { color: "#eab308", label: "Degraded", pulse: true },
  down: { color: "#ef4444", label: "Down", pulse: true },
  unknown: { color: "#666", label: "Unknown", pulse: false },
};

export function AppHealth({ status, appName, details }: AppHealthProps) {
  const [showTooltip, setShowTooltip] = useState(false);
  const config = STATUS_CONFIG[status];

  return (
    <div
      style={{ position: "relative", display: "inline-flex", alignItems: "center", gap: 6 }}
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
    >
      <span
        style={{
          width: 10,
          height: 10,
          borderRadius: "50%",
          backgroundColor: config.color,
          display: "inline-block",
          boxShadow: `0 0 6px ${config.color}40`,
          animation: config.pulse ? "healthPulse 2s ease-in-out infinite" : "none",
        }}
      />
      <span style={{ fontSize: 12, color: "#ccc" }}>{appName}</span>

      {showTooltip && (
        <div
          style={{
            position: "absolute",
            bottom: "calc(100% + 6px)",
            left: "50%",
            transform: "translateX(-50%)",
            background: "#1a1a1a",
            border: "1px solid #333",
            borderRadius: 8,
            padding: "8px 12px",
            whiteSpace: "nowrap",
            zIndex: 20,
            boxShadow: "0 4px 12px rgba(0,0,0,0.4)",
          }}
        >
          <div style={{ fontSize: 11, fontWeight: 600, color: config.color, marginBottom: 2 }}>
            {config.label}
          </div>
          {details && (
            <div style={{ fontSize: 10, color: "#888" }}>{details}</div>
          )}
          <div
            style={{
              position: "absolute",
              top: "100%",
              left: "50%",
              transform: "translateX(-50%)",
              width: 0,
              height: 0,
              borderLeft: "5px solid transparent",
              borderRight: "5px solid transparent",
              borderTop: "5px solid #333",
            }}
          />
        </div>
      )}

      <style>{`
        @keyframes healthPulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.5; }
        }
      `}</style>
    </div>
  );
}

/** Derive health status from pod metrics. */
export function deriveHealthStatus(
  runningPods: number,
  totalPods: number,
  cpuPercent: number,
  memPercent: number,
): HealthStatus {
  if (totalPods === 0) return "unknown";
  if (runningPods === 0) return "down";
  if (runningPods < totalPods || cpuPercent > 90 || memPercent > 90) return "degraded";
  return "healthy";
}
