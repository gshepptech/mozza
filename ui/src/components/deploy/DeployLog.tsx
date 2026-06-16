import { useState, useEffect, useRef } from "react";
import { streamDeploymentLogs } from "../../api/client";

interface DeployLogProps {
  deploymentId: string;
  onDone?: (status: string) => void;
}

export default function DeployLog({ deploymentId, onDone }: DeployLogProps) {
  const [lines, setLines] = useState<string[]>([]);
  const [done, setDone] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const close = streamDeploymentLogs(
      deploymentId,
      (line) => {
        setLines((prev) => [...prev, line]);
        if (ref.current) ref.current.scrollTop = ref.current.scrollHeight;
      },
      (status) => {
        setDone(true);
        onDone?.(status);
      },
    );
    return close;
  }, [deploymentId]);

  return (
    <div
      ref={ref}
      className="max-h-[400px] overflow-y-auto rounded-xl border border-border bg-surface p-4 font-mono text-xs leading-[1.8] text-foreground/80"
    >
      {lines.map((line, i) => (
        <div key={i}>{line}</div>
      ))}
      {!done && lines.length > 0 && (
        <span className="inline-block h-4 w-2 animate-pulse bg-brand align-middle" />
      )}
      {lines.length === 0 && <div className="text-muted-foreground">Waiting for logs...</div>}
    </div>
  );
}
