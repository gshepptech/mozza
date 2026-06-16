export type AlertLevel = "warning" | "critical" | "none";

export interface AlertBadgeProps {
  level: AlertLevel;
  label?: string;
}

const LEVEL_STYLES: Record<AlertLevel, { bg: string; color: string; border: string }> = {
  warning: { bg: "rgba(234,179,8,0.12)", color: "#eab308", border: "rgba(234,179,8,0.3)" },
  critical: { bg: "rgba(239,68,68,0.12)", color: "#ef4444", border: "rgba(239,68,68,0.3)" },
  none: { bg: "transparent", color: "transparent", border: "transparent" },
};

export function AlertBadge({ level, label }: AlertBadgeProps) {
  if (level === "none") return null;

  const styles = LEVEL_STYLES[level];
  const text = label ?? (level === "critical" ? "CRIT" : "WARN");

  return (
    <span
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 4,
        padding: "2px 8px",
        fontSize: 10,
        fontWeight: 600,
        letterSpacing: 0.5,
        textTransform: "uppercase",
        color: styles.color,
        background: styles.bg,
        border: `1px solid ${styles.border}`,
        borderRadius: 4,
      }}
    >
      <span
        style={{
          width: 5,
          height: 5,
          borderRadius: "50%",
          backgroundColor: styles.color,
        }}
      />
      {text}
    </span>
  );
}

/** Determine alert level from resource percentage. */
export function resourceAlertLevel(percent: number): AlertLevel {
  if (percent > 90) return "critical";
  if (percent > 80) return "warning";
  return "none";
}
