import { useState, useEffect, useRef, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { streamDeploymentLogs, getDeployment } from "../../api/client";
import { CheckCircle, XCircle, Loader, Clock, PartyPopper, RotateCcw, ExternalLink } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { analyzeDeployError } from "@/lib/deploy-error-intelligence";

// ─── Types ───────────────────────────────────────────────────

interface DeployProgressProps {
  deploymentId: string;
  onComplete?: (status: string) => void;
  usePizzaMetaphor?: boolean;
  orderNumber?: number | string;
}

interface StepInfo {
  label: string;
  status: "pending" | "active" | "done" | "error";
}

const STEP_LABELS = [
  "Parsing recipe",
  "Building plan",
  "Validating plan",
  "Applying to your server",
  "Waiting for your app to start",
];

const PIZZA_STEP_LABELS = [
  "Taking your order\u2026",
  "Prepping ingredients\u2026",
  "In the oven\u2026",
  "Quality check\u2026",
  "Ready for pickup!",
];

// ─── Step Indicator ──────────────────────────────────────────

function StepIndicator({ step, index }: { step: StepInfo; index: number }) {
  return (
    <div className="flex items-center gap-3">
      {/* Circle */}
      <div
        className={cn(
          "relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-2 transition-all duration-300",
          step.status === "done" && "border-green-400 bg-green-400/15",
          step.status === "active" && "border-brand bg-brand/10 animate-pulse",
          step.status === "error" && "border-red-400 bg-red-400/15",
          step.status === "pending" && "border-border/50 bg-transparent",
        )}
      >
        {step.status === "done" ? (
          <CheckCircle className="h-4 w-4 text-green-400" />
        ) : step.status === "active" ? (
          <Loader className="h-4 w-4 text-brand animate-spin" />
        ) : step.status === "error" ? (
          <XCircle className="h-4 w-4 text-red-400" />
        ) : (
          <span className="text-xs font-mono text-muted-foreground">{index + 1}</span>
        )}
      </div>

      {/* Label */}
      <span
        className={cn(
          "text-sm transition-colors duration-200",
          step.status === "done" && "text-green-400",
          step.status === "active" && "text-foreground font-medium",
          step.status === "error" && "text-red-400",
          step.status === "pending" && "text-muted-foreground",
        )}
      >
        {step.label}
      </span>
    </div>
  );
}

// ─── Main Component ──────────────────────────────────────────

export default function DeployProgress({ deploymentId, onComplete, usePizzaMetaphor, orderNumber }: DeployProgressProps) {
  const navigate = useNavigate();
  const labels = usePizzaMetaphor ? PIZZA_STEP_LABELS : STEP_LABELS;
  const [showLogs, setShowLogs] = useState(false);
  const [steps, setSteps] = useState<StepInfo[]>(
    labels.map((label) => ({ label, status: "pending" }))
  );
  const [logLines, setLogLines] = useState<string[]>([]);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [finalStatus, setFinalStatus] = useState<string | null>(null);
  const [accessUrl, setAccessUrl] = useState<string | null>(null);
  const [sseError, setSseError] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Elapsed timer
  useEffect(() => {
    if (finalStatus) return;
    const interval = setInterval(() => setElapsedSeconds((s) => s + 1), 1000);
    return () => clearInterval(interval);
  }, [finalStatus]);

  // Parse step progress from a log line
  const parseStep = useCallback((line: string) => {
    const match = line.match(/Step (\d)\/5:/);
    if (match && match[1]) {
      const stepNum = parseInt(match[1], 10);
      setSteps((prev) =>
        prev.map((s, i) => {
          if (i < stepNum - 1) return { ...s, status: "done" };
          if (i === stepNum - 1) return { ...s, status: "active" };
          return s;
        })
      );
    }

    // Detect access URL from log
    const urlMatch = line.match(/App accessible at: (.+)/);
    if (urlMatch && urlMatch[1]) {
      setAccessUrl(urlMatch[1].trim());
    }

    // Check for success marker
    if (line.includes("\u2713 Deploy complete") || line.includes("\u2713 Deploy applied")) {
      setSteps((prev) => prev.map((s) => ({ ...s, status: "done" })));
    }

    // Check for failure marker
    if (line.includes("\u2717 Deploy failed")) {
      setSteps((prev) =>
        prev.map((s) =>
          s.status === "active" ? { ...s, status: "error" } : s
        )
      );
    }
  }, []);

  // SSE connection
  useEffect(() => {
    const close = streamDeploymentLogs(
      deploymentId,
      (line: string) => {
        setLogLines((prev) => [...prev, line]);
        parseStep(line);
        if (logRef.current) {
          logRef.current.scrollTop = logRef.current.scrollHeight;
        }
      },
      (status: string) => {
        setFinalStatus(status);
        onComplete?.(status);
      },
    );

    // Handle SSE errors by falling back to polling
    const es = (close as unknown as { _es?: EventSource })?._es;
    if (!es) {
      // If we can't access the EventSource, set up polling as backup after a delay
      const timeout = setTimeout(() => {
        if (!finalStatus) {
          setSseError(true);
        }
      }, 10000);
      return () => {
        close();
        clearTimeout(timeout);
      };
    }

    return close;
  }, [deploymentId, parseStep, onComplete, finalStatus]);

  // Polling fallback when SSE fails
  useEffect(() => {
    if (!sseError || finalStatus) return;

    pollRef.current = setInterval(async () => {
      try {
        const dep = await getDeployment(deploymentId);
        if (dep.log) {
          const lines = dep.log.split("\n").filter(Boolean);
          setLogLines(lines);
          lines.forEach(parseStep);
        }
        if (dep.access_url) {
          setAccessUrl(dep.access_url);
        }
        if (dep.finished_at) {
          setFinalStatus(dep.status);
          onComplete?.(dep.status);
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch {
        // Ignore poll errors
      }
    }, 3000);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [sseError, finalStatus, deploymentId, parseStep, onComplete]);

  const isSuccess = finalStatus === "succeeded" || finalStatus === "success";
  const isFailed = finalStatus === "failed" || finalStatus === "burned";
  const isPartial = finalStatus === "partial";
  const isDone = !!finalStatus;

  const formatTime = (s: number) =>
    `${Math.floor(s / 60)}:${(s % 60).toString().padStart(2, "0")}`;

  return (
    <div className="space-y-6">
      {/* Stepper */}
      <Card className="oven-card bg-card border-border/50">
        <CardContent className="p-6">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-3">
              <h3 className="text-sm font-semibold text-foreground">
                {usePizzaMetaphor ? "Your Order" : "Deploy Progress"}
              </h3>
              {orderNumber && (
                <span className="text-xs font-mono text-muted-foreground bg-muted px-2 py-0.5 rounded">
                  #{orderNumber}
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <Clock className="h-3.5 w-3.5" />
              <span>{formatTime(elapsedSeconds)}</span>
            </div>
          </div>

          <div className="space-y-4">
            {steps.map((step, i) => (
              <div key={i} className="relative">
                {/* Connector line */}
                {i < steps.length - 1 && (
                  <div
                    className={cn(
                      "absolute left-4 top-8 h-4 w-px -translate-x-px",
                      steps[i + 1]?.status !== "pending" ? "bg-green-400/40" : "bg-border/30",
                    )}
                  />
                )}
                <StepIndicator step={step} index={i} />
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Success card */}
      {isSuccess && (
        <Card className="oven-card bg-card border-green-400/20">
          <CardContent className="flex flex-col items-center py-8">
            <div className="relative mb-4">
              <div className="absolute inset-0 rounded-full bg-green-400/20 blur-2xl scale-150 animate-pulse" />
              <div className="relative rounded-2xl bg-green-400/10 border border-green-400/20 p-5">
                <PartyPopper className="h-8 w-8 text-green-400" />
              </div>
            </div>
            <h3 className="text-lg font-bold text-foreground">
              {usePizzaMetaphor ? "Ready for pickup!" : "Deploy complete!"}
            </h3>
            <p className="text-sm text-muted-foreground mt-1">
              {usePizzaMetaphor
                ? `Your app is served — cooked in ${formatTime(elapsedSeconds)}`
                : `All services are healthy in ${formatTime(elapsedSeconds)}`}
            </p>
            {accessUrl ? (
              <div className="mt-4 flex flex-col items-center gap-3">
                <p className="text-sm text-green-400">
                  Your app is live at{" "}
                  <a
                    href={accessUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline underline-offset-2 hover:text-green-300 transition-colors"
                  >
                    {accessUrl}
                  </a>
                </p>
                <a href={accessUrl} target="_blank" rel="noopener noreferrer">
                  <Button
                    size="sm"
                    className="h-8 px-4 text-xs bg-green-500/15 border border-green-400/30 text-green-400 hover:bg-green-500/25"
                  >
                    <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                    Open App
                  </Button>
                </a>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground/60 mt-3">
                Internal service — accessible within the cluster
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {/* Partial card */}
      {isPartial && (
        <Card className="oven-card bg-card border-yellow-400/20">
          <CardContent className="flex flex-col items-center py-8">
            <div className="relative mb-4">
              <div className="absolute inset-0 rounded-full bg-yellow-400/20 blur-2xl scale-150" />
              <div className="relative rounded-2xl bg-yellow-400/10 border border-yellow-400/20 p-5">
                <Clock className="h-8 w-8 text-yellow-400" />
              </div>
            </div>
            <h3 className="text-lg font-bold text-foreground">Deploy applied</h3>
            <p className="text-sm text-muted-foreground mt-1">
              Manifests applied but pods are not yet healthy
            </p>
          </CardContent>
        </Card>
      )}

      {/* Error card */}
      {isFailed && (() => {
        const errorInfo = analyzeDeployError(logLines);
        return (
          <Card className="oven-card bg-card border-red-400/20">
            <CardContent className="flex flex-col items-center py-8">
              <div className="relative mb-4">
                <div className="absolute inset-0 rounded-full bg-red-400/20 blur-2xl scale-150" />
                <div className="relative rounded-2xl bg-red-400/10 border border-red-400/20 p-5">
                  <XCircle className="h-8 w-8 text-red-400" />
                </div>
              </div>
              <h3 className="text-lg font-bold text-foreground">
                {errorInfo?.title ?? (usePizzaMetaphor ? "Something burned\u2026" : "Deploy failed")}
              </h3>
              <p className="text-sm text-muted-foreground mt-1 text-center max-w-md">
                {errorInfo?.message ?? (usePizzaMetaphor ? "Check the kitchen logs for what went wrong" : "Check the logs below for details")}
              </p>
              {errorInfo?.steps && errorInfo.steps.length > 0 && (
                <div className="mt-3 text-left max-w-md">
                  <p className="text-xs font-medium text-muted-foreground mb-1">What to do:</p>
                  <ul className="list-disc list-inside space-y-0.5">
                    {errorInfo.steps.map((step, i) => (
                      <li key={i} className="text-xs text-muted-foreground/80">{step}</li>
                    ))}
                  </ul>
                </div>
              )}
              {errorInfo?.technicalDetail && (
                <details className="mt-3 max-w-md">
                  <summary className="text-[10px] text-muted-foreground/40 cursor-pointer hover:text-muted-foreground/60">Technical details</summary>
                  <p className="text-[10px] text-muted-foreground/30 font-mono mt-1">{errorInfo.technicalDetail}</p>
                </details>
              )}
              <div className="flex gap-2 mt-4">

                <Button
                  variant="outline"
                  size="sm"
                  className="text-xs h-8 px-4 border-red-400/30 text-red-400 hover:bg-red-400/10"
                  onClick={() => navigate("/app/deploy")}
                >
                  <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                  Retry
                </Button>
              </div>
            </CardContent>
          </Card>
        );
      })()}

      {/* Raw log output — collapsible */}
      <div>
        <button
          type="button"
          onClick={() => setShowLogs(!showLogs)}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1 mb-2"
        >
          <span style={{ transform: showLogs ? "rotate(90deg)" : "none", transition: "transform 0.15s", display: "inline-block" }}>▸</span>
          {usePizzaMetaphor ? "View kitchen logs" : "View logs"}
        </button>
        {showLogs && (
          <Card className="oven-card bg-card border-border/50">
            <CardContent className="p-0">
              <div
                ref={logRef}
                className="max-h-[300px] overflow-y-auto rounded-xl border border-border bg-surface p-4 font-mono text-xs leading-[1.8] text-foreground/80"
              >
                {logLines.map((line, idx) => (
                  <div key={idx}>{line}</div>
                ))}
                {!isDone && logLines.length > 0 && (
                  <span className="inline-block h-4 w-2 animate-pulse bg-brand align-middle" />
                )}
                {logLines.length === 0 && (
                  <div className="text-muted-foreground">Waiting for logs...</div>
                )}
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
