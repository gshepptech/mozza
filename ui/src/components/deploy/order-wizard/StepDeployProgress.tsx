import { useState, useEffect, useRef, useCallback } from "react";
import { Button } from "@/components/ui/button";
import {
  CheckCircle, XCircle, Loader, Clock, PartyPopper,
  RotateCcw, ExternalLink, PenLine,
} from "lucide-react";
import {
  createRecipe, triggerDeploy, streamDeploymentLogs, getDeployment,
} from "@/api/client";
import type { WizardTarget } from "@/lib/interview-types";
import { analyzeDeployError, PIZZA_STAGE_LABELS } from "@/lib/deploy-error-intelligence";

// ─── Props ──────────────────────────────────────────────────

interface StepDeployProgressProps {
  recipeSource: string;
  recipeName: string;
  target: WizardTarget;
  teamId: string;
  clusterId: string;
  saveRecipe: boolean;
  onRetry: () => void;
  onEditOrder: () => void;
  onNewOrder: () => void;
}

// ─── Types ──────────────────────────────────────────────────

interface StageInfo {
  label: string;
  status: "pending" | "active" | "done" | "error";
}

// ─── Component ──────────────────────────────────────────────

export default function StepDeployProgress({
  recipeSource,
  recipeName,
  target,
  teamId,
  clusterId,
  saveRecipe,
  onRetry,
  onEditOrder,
  onNewOrder,
}: StepDeployProgressProps) {
  const [stages, setStages] = useState<StageInfo[]>(
    PIZZA_STAGE_LABELS.map((label) => ({ label, status: "pending" })),
  );
  const [logLines, setLogLines] = useState<string[]>([]);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [finalStatus, setFinalStatus] = useState<string | null>(null);
  const [accessUrl, setAccessUrl] = useState<string | null>(null);
  const [orderNumber, setOrderNumber] = useState<string | null>(null);
  const [deployError, setDeployError] = useState<string | null>(null);
  const [deploying, setDeploying] = useState(false);
  const logRef = useRef<HTMLDivElement>(null);
  const deployedRef = useRef(false);

  // Elapsed timer
  useEffect(() => {
    if (finalStatus) return;
    if (!deploying) return;
    const interval = setInterval(() => setElapsedSeconds((s) => s + 1), 1000);
    return () => clearInterval(interval);
  }, [finalStatus, deploying]);

  // Parse step from log line
  const parseStep = useCallback((line: string) => {
    const match = line.match(/Step (\d)\/5:/);
    if (match?.[1]) {
      const stepNum = parseInt(match[1], 10);
      // Map 5 deploy steps to our 5 pizza stages (1:1)
      const stageIdx = Math.min(stepNum - 1, PIZZA_STAGE_LABELS.length - 1);
      setStages((prev) =>
        prev.map((s, i) => {
          if (i < stageIdx) return { ...s, status: "done" };
          if (i === stageIdx) return { ...s, status: "active" };
          return s;
        }),
      );
    }

    const urlMatch = line.match(/App accessible at: (.+)/);
    if (urlMatch?.[1]) setAccessUrl(urlMatch[1].trim());

    if (line.includes("\u2713 Deploy complete") || line.includes("\u2713 Deploy applied")) {
      setStages((prev) => prev.map((s) => ({ ...s, status: "done" })));
    }
    if (line.includes("\u2717 Deploy failed")) {
      setStages((prev) =>
        prev.map((s) => (s.status === "active" ? { ...s, status: "error" } : s)),
      );
    }
  }, []);

  // Deploy flow
  useEffect(() => {
    if (deployedRef.current) return;
    deployedRef.current = true;

    const run = async () => {
      setDeploying(true);
      setStages((prev) => prev.map((s, i) => (i === 0 ? { ...s, status: "active" } : s)));

      try {
        // Save recipe if requested
        let recipeId: string | undefined;
        if (saveRecipe) {
          const recipe = await createRecipe(teamId, recipeName, recipeSource, "");
          recipeId = recipe.id;
        } else {
          // Create a temporary recipe
          const recipe = await createRecipe(teamId, recipeName, recipeSource, "");
          recipeId = recipe.id;
        }

        // Trigger deploy
        const targetStr = target === "kitchen" ? "kubernetes" : "local";
        const deployment = await triggerDeploy(
          recipeId,
          teamId,
          targetStr,
          "production",
          target === "kitchen" ? clusterId : undefined,
        );

        if (deployment.order_number) {
          const dateStr = new Date().toISOString().slice(0, 10).replace(/-/g, "");
          setOrderNumber(`#d-${dateStr}-${String(deployment.order_number).padStart(3, "0")}`);
        }

        // Stream logs
        const close = streamDeploymentLogs(
          deployment.id,
          (line: string) => {
            setLogLines((prev) => [...prev, line]);
            parseStep(line);
            if (logRef.current) {
              logRef.current.scrollTop = logRef.current.scrollHeight;
            }
          },
          (status: string) => {
            setFinalStatus(status);
          },
        );

        // Fallback: poll after 15s if no final status
        const pollTimeout = setTimeout(async () => {
          if (!finalStatus) {
            try {
              const dep = await getDeployment(deployment.id);
              if (dep.finished_at) {
                setFinalStatus(dep.status);
                if (dep.access_url) setAccessUrl(dep.access_url);
              }
            } catch { /* ignore */ }
          }
        }, 15000);

        return () => {
          close();
          clearTimeout(pollTimeout);
        };
      } catch (err) {
        setDeployError(err instanceof Error ? err.message : "Deploy failed to start");
        setFinalStatus("failed");
        setStages((prev) => prev.map((s, i) => (i === 0 ? { ...s, status: "error" } : s)));
      }
    };

    run();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const isSuccess = finalStatus === "succeeded" || finalStatus === "success";
  const isFailed = finalStatus === "failed" || finalStatus === "burned";
  const isDone = !!finalStatus;
  const errorInfo = isFailed ? analyzeDeployError(logLines) : null;

  const formatTime = (s: number) =>
    `${Math.floor(s / 60)}:${(s % 60).toString().padStart(2, "0")}`;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Order number */}
      {orderNumber && (
        <div
          style={{
            fontSize: 13,
            color: "rgba(255,255,255,0.4)",
            fontFamily: "'JetBrains Mono', monospace",
          }}
        >
          Order {orderNumber}
        </div>
      )}

      {/* Pizza stages stepper */}
      <div
        style={{
          padding: 20,
          background: "rgba(255,255,255,0.02)",
          borderRadius: 10,
          border: "1px solid rgba(255,255,255,0.06)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
          <span style={{ fontSize: 14, fontWeight: 600, color: "#fff" }}>
            {isDone
              ? isSuccess ? "Order served!" : "Order burned!"
              : "Preparing your order\u2026"}
          </span>
          <span style={{ fontSize: 12, color: "rgba(255,255,255,0.35)", display: "flex", alignItems: "center", gap: 4 }}>
            <Clock style={{ width: 13, height: 13 }} />
            {formatTime(elapsedSeconds)}
          </span>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {stages.map((stage, i) => (
            <div key={i} style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                  border: `2px solid ${
                    stage.status === "done" ? "rgba(74,222,128,0.6)"
                    : stage.status === "active" ? "#ff6b35"
                    : stage.status === "error" ? "rgba(248,113,113,0.6)"
                    : "rgba(255,255,255,0.1)"
                  }`,
                  background:
                    stage.status === "done" ? "rgba(74,222,128,0.1)"
                    : stage.status === "active" ? "rgba(255,107,53,0.1)"
                    : stage.status === "error" ? "rgba(248,113,113,0.1)"
                    : "transparent",
                }}
              >
                {stage.status === "done" ? (
                  <CheckCircle style={{ width: 14, height: 14, color: "#4ade80" }} />
                ) : stage.status === "active" ? (
                  <Loader style={{ width: 14, height: 14, color: "#ff6b35", animation: "spin 1s linear infinite" }} />
                ) : stage.status === "error" ? (
                  <XCircle style={{ width: 14, height: 14, color: "#f87171" }} />
                ) : (
                  <span style={{ fontSize: 11, color: "rgba(255,255,255,0.3)" }}>{i + 1}</span>
                )}
              </div>
              <span
                style={{
                  fontSize: 13,
                  color:
                    stage.status === "done" ? "#4ade80"
                    : stage.status === "active" ? "#fff"
                    : stage.status === "error" ? "#f87171"
                    : "rgba(255,255,255,0.3)",
                  fontWeight: stage.status === "active" ? 500 : 400,
                }}
              >
                {stage.label}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Success card */}
      {isSuccess && (
        <div
          style={{
            padding: 28,
            background: "rgba(74,222,128,0.05)",
            border: "1px solid rgba(74,222,128,0.15)",
            borderRadius: 10,
            textAlign: "center",
          }}
        >
          <PartyPopper style={{ width: 32, height: 32, color: "#4ade80", margin: "0 auto 12px" }} />
          <div style={{ fontSize: 18, fontWeight: 700, color: "#fff", marginBottom: 4 }}>
            Your order has been served!
          </div>
          <div style={{ fontSize: 13, color: "rgba(255,255,255,0.5)", marginBottom: 16 }}>
            All services are healthy in {formatTime(elapsedSeconds)}
          </div>
          {accessUrl && (
            <a href={accessUrl} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "#4ade80", fontSize: 13, textDecoration: "underline", marginBottom: 16 }}>
              <ExternalLink style={{ width: 14, height: 14 }} />
              {accessUrl}
            </a>
          )}
          <div style={{ display: "flex", justifyContent: "center", gap: 8, marginTop: 8 }}>
            <Button onClick={onNewOrder} style={{ background: "#ff6b35", color: "#fff", border: "none" }}>
              Place Another Order
            </Button>
          </div>
        </div>
      )}

      {/* Failure card */}
      {isFailed && (
        <div
          style={{
            padding: 24,
            background: "rgba(248,113,113,0.05)",
            border: "1px solid rgba(248,113,113,0.15)",
            borderRadius: 10,
          }}
        >
          <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
            <XCircle style={{ width: 20, height: 20, color: "#f87171", flexShrink: 0, marginTop: 2 }} />
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 15, fontWeight: 600, color: "#f87171", marginBottom: 4 }}>
                {errorInfo?.title || "Something went wrong"}
              </div>
              <div style={{ fontSize: 13, color: "rgba(255,255,255,0.55)", lineHeight: 1.5 }}>
                {errorInfo?.message || deployError || "Something went wrong during deployment. Check the logs for details."}
              </div>
              {errorInfo?.steps && errorInfo.steps.length > 0 && (
                <ul
                  style={{
                    margin: "12px 0 0 0",
                    paddingLeft: 18,
                    listStyle: "disc",
                    display: "flex",
                    flexDirection: "column",
                    gap: 6,
                  }}
                >
                  {errorInfo.steps.map((step, i) => (
                    <li key={i} style={{ fontSize: 13, color: "rgba(255,255,255,0.65)", lineHeight: 1.5 }}>
                      {step}
                    </li>
                  ))}
                </ul>
              )}
              {errorInfo?.technicalDetail && (
                <details
                  style={{
                    marginTop: 14,
                    fontSize: 12,
                    color: "rgba(255,255,255,0.35)",
                  }}
                >
                  <summary
                    style={{
                      cursor: "pointer",
                      userSelect: "none",
                      color: "rgba(255,255,255,0.4)",
                      fontSize: 12,
                    }}
                  >
                    Technical details
                  </summary>
                  <div
                    style={{
                      marginTop: 6,
                      padding: "8px 10px",
                      background: "rgba(0,0,0,0.25)",
                      borderRadius: 6,
                      fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
                      fontSize: 11,
                      lineHeight: 1.6,
                      color: "rgba(255,255,255,0.45)",
                    }}
                  >
                    {errorInfo.technicalDetail}
                  </div>
                </details>
              )}
            </div>
          </div>
          <div style={{ display: "flex", gap: 8, marginTop: 16, paddingLeft: 32 }}>
            <Button onClick={onRetry} variant="outline" style={{ fontSize: 13 }}>
              <RotateCcw style={{ width: 13, height: 13, marginRight: 5 }} />
              Try Again
            </Button>
            <Button onClick={onEditOrder} variant="ghost" style={{ fontSize: 13 }}>
              <PenLine style={{ width: 13, height: 13, marginRight: 5 }} />
              Edit Order
            </Button>
          </div>
        </div>
      )}

      {/* Log output */}
      <div
        ref={logRef}
        style={{
          maxHeight: 240,
          overflowY: "auto",
          padding: 14,
          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
          fontSize: 11,
          lineHeight: 1.8,
          background: "rgba(0,0,0,0.3)",
          borderRadius: 8,
          border: "1px solid rgba(255,255,255,0.06)",
          color: "rgba(255,255,255,0.6)",
        }}
      >
        {logLines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
        {!isDone && logLines.length > 0 && (
          <span
            style={{
              display: "inline-block",
              width: 7,
              height: 14,
              background: "#ff6b35",
              animation: "pulse 1s ease-in-out infinite",
              verticalAlign: "middle",
            }}
          />
        )}
        {logLines.length === 0 && !isDone && (
          <div style={{ color: "rgba(255,255,255,0.3)" }}>Waiting for logs...</div>
        )}
      </div>
    </div>
  );
}
