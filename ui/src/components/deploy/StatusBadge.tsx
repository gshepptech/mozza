import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

const statusStyles: Record<string, string> = {
  running:   "border-warning/30 bg-warning-bg text-warning",
  succeeded: "border-success/30 bg-success-bg text-success",
  failed:    "border-error/30 bg-error-bg text-error",
  pending:   "border-border bg-muted text-muted-foreground",
};

interface StatusBadgeProps {
  status: string;
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  return (
    <Badge variant="outline" className={cn("text-xs font-semibold", statusStyles[status])}>
      {status}
    </Badge>
  );
}
