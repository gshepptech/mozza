// Shared formatting utilities for the Mozza UI.

/** Pass through K8s age strings (already formatted by the backend). */
export function timeAgo(ageStr: string): string {
  return ageStr || "";
}

/** Format an ISO timestamp as relative time. */
export function relativeTime(iso: string): string {
  if (!iso) return "";
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/** Map health status to a Tailwind text color class. */
export function statusColor(status: string): string {
  switch (status) {
    case "healthy":
    case "success":
    case "succeeded":
    case "Running":
      return "text-success";
    case "degraded":
    case "warning":
    case "Pending":
      return "text-warning";
    case "failing":
    case "failed":
    case "CrashLoopBackOff":
    case "ImagePullBackOff":
      return "text-error";
    default:
      return "text-muted-foreground";
  }
}

/** Map health status to a Tailwind background color class. */
export function statusBgColor(status: string): string {
  switch (status) {
    case "healthy":
    case "success":
    case "succeeded":
      return "bg-success/10 text-success border-success/20";
    case "degraded":
    case "warning":
      return "bg-warning/10 text-warning border-warning/20";
    case "failing":
    case "failed":
      return "bg-error/10 text-error border-error/20";
    default:
      return "bg-muted text-muted-foreground border-border";
  }
}

/** Map environment/namespace name to a display color. */
export function envColor(env: string): string {
  const lower = env.toLowerCase();
  if (lower.includes("prod")) return "text-error";
  if (lower.includes("stag")) return "text-warning";
  if (lower.includes("dev")) return "text-info";
  return "text-muted-foreground";
}
