import { useClusterStatus } from "@/context/ClusterContext";
import { Clock, RefreshCw } from "lucide-react";

interface ClusterBannerProps {
  /** Custom message when unreachable. Falls back to context error. */
  message?: string;
  /** Show a CTA button linking to clusters page. */
  showCTA?: boolean;
}

/**
 * Displays a subtle banner when the environment connection is stale.
 * For the unreachable state, pages should use useClusterStatus() directly
 * and show their own friendly setup UX instead of a red error.
 *
 * Renders nothing when connected and fresh.
 */
export function ClusterBanner({ message }: ClusterBannerProps) {
  const { status, loading, refresh } = useClusterStatus();

  if (loading) return null;
  if (status.reachable && !status.stale) return null;

  // For fully unreachable, render nothing — let the page show its own setup UX
  if (!status.reachable) return null;

  // Stale data warning
  const age = status.lastSeen
    ? `Last updated ${new Date(status.lastSeen).toLocaleTimeString()}`
    : "Data may be outdated";

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "10px 16px",
        background: "rgba(250,204,21,0.08)",
        border: "1px solid rgba(250,204,21,0.2)",
        borderRadius: 8,
        marginBottom: 16,
      }}
    >
      <Clock style={{ width: 16, height: 16, color: "#facc15", flexShrink: 0 }} />
      <span style={{ flex: 1, fontSize: 13, color: "#facc15" }}>
        {message || "Stale data"} — {age}
      </span>
      <button
        onClick={refresh}
        style={{
          background: "none",
          border: "none",
          color: "#facc15",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          gap: 4,
          fontSize: 12,
        }}
      >
        <RefreshCw style={{ width: 12, height: 12 }} />
        Retry
      </button>
    </div>
  );
}
