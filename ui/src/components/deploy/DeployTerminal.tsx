import { useState, useEffect, useRef, useCallback } from "react";
import { cn } from "@/lib/utils";
import { analyzeDeployError } from "@/lib/deploy-error-intelligence";
import DeployStep, { type DeployStepInfo, type StepStatus } from "./DeployStep";

// ─── Types ───────────────────────────────────────────────────

interface DeployTerminalProps {
  buildId: string;
  /** Base URL for API/WS connections. Defaults to current host. */
  baseUrl?: string;
  onComplete?: (status: "success" | "failed") => void;
  /** URL the deployed app is accessible at, shown on success. */
  appUrl?: string;
}

interface WsMessage {
  type: "step" | "log" | "error" | "complete";
  content: string;
  step?: number;
  timestamp?: number;
}

interface LogLine {
  text: string;
  isError: boolean;
  timestamp: number;
}

const STEP_LABELS = [
  "Clone repository",
  "Detect framework",
  "Build image",
  "Create containers",
  "Health check",
  "Route traffic",
];

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 8000;
const RECONNECT_MAX_ATTEMPTS = 10;

// ─── Keyframes (injected once) ──────────────────────────────

let injected = false;
function injectKeyframes(): void {
  if (injected || typeof document === "undefined") return;
  injected = true;
  const style = document.createElement("style");
  style.textContent = `
    @keyframes dt-blink { 0%,100% { opacity: 1; } 50% { opacity: 0; } }
    @keyframes dt-fade-in { from { opacity: 0; transform: translateY(4px); } to { opacity: 1; transform: translateY(0); } }
  `;
  document.head.appendChild(style);
}

// ─── Helpers ────────────────────────────────────────────────

function buildWsUrl(buildId: string, baseUrl?: string): string {
  const loc = typeof window !== "undefined" ? window.location : null;
  if (baseUrl) {
    const u = baseUrl.replace(/^http/, "ws").replace(/\/$/, "");
    return `${u}/api/v1/builds/${buildId}/logs`;
  }
  if (!loc) return `ws://localhost:8080/api/v1/builds/${buildId}/logs`;
  const proto = loc.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${loc.host}/api/v1/builds/${buildId}/logs`;
}

function formatElapsed(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

// ─── Component ──────────────────────────────────────────────

export default function DeployTerminal({
  buildId,
  baseUrl,
  onComplete,
  appUrl: appUrlProp,
}: DeployTerminalProps) {
  injectKeyframes();

  const [lines, setLines] = useState<LogLine[]>([]);
  const [steps, setSteps] = useState<DeployStepInfo[]>(
    STEP_LABELS.map((label) => ({ label, status: "pending" as StepStatus })),
  );
  const [status, setStatus] = useState<"running" | "success" | "failed" | "connecting">("connecting");
  const [scrollLocked, setScrollLocked] = useState(false);
  const [fullScreen, setFullScreen] = useState(false);
  const [reconnecting, setReconnecting] = useState(false);
  const [elapsed, setElapsed] = useState(0);
  const [appUrl, setAppUrl] = useState(appUrlProp ?? "");

  const logRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const completedRef = useRef(false);

  // Auto-scroll when not locked
  const scrollToBottom = useCallback(() => {
    if (!scrollLocked && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [scrollLocked]);

  // Detect user scroll to toggle scroll lock
  const handleScroll = useCallback(() => {
    if (!logRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = logRef.current;
    const atBottom = scrollHeight - scrollTop - clientHeight < 40;
    setScrollLocked(!atBottom);
  }, []);

  // Elapsed timer
  useEffect(() => {
    if (status === "success" || status === "failed") return;
    const id = setInterval(() => setElapsed((s) => s + 1), 1000);
    return () => clearInterval(id);
  }, [status]);

  // Update steps from a step message
  const updateStep = useCallback((stepIndex: number, stepStatus: StepStatus) => {
    setSteps((prev) =>
      prev.map((s, i) => {
        if (i < stepIndex) return { ...s, status: "success" };
        if (i === stepIndex) return { ...s, status: stepStatus };
        return s;
      }),
    );
  }, []);

  // Handle incoming WS message
  const handleMessage = useCallback(
    (msg: WsMessage) => {
      const ts = msg.timestamp ?? Date.now();

      switch (msg.type) {
        case "log":
          setLines((prev) => [...prev, { text: msg.content, isError: false, timestamp: ts }]);
          break;

        case "step":
          if (msg.step !== undefined && msg.step >= 0 && msg.step < STEP_LABELS.length) {
            updateStep(msg.step, "active");
          }
          if (msg.content) {
            setLines((prev) => [...prev, { text: msg.content, isError: false, timestamp: ts }]);
          }
          break;

        case "error":
          setLines((prev) => [...prev, { text: msg.content, isError: true, timestamp: ts }]);
          if (msg.step !== undefined && msg.step >= 0 && msg.step < STEP_LABELS.length) {
            updateStep(msg.step, "failed");
          }
          break;

        case "complete": {
          const isSuccess = msg.content === "success";
          completedRef.current = true;
          setSteps((prev) =>
            isSuccess
              ? prev.map((s) => ({ ...s, status: "success" as StepStatus }))
              : prev.map((s) =>
                  s.status === "active" ? { ...s, status: "failed" as StepStatus } : s,
                ),
          );
          setStatus(isSuccess ? "success" : "failed");
          onComplete?.(isSuccess ? "success" : "failed");

          // Extract app URL from content if present
          const urlMatch = msg.content.match(/https?:\/\/\S+/);
          if (urlMatch) setAppUrl(urlMatch[0]);
          break;
        }
      }
    },
    [onComplete, updateStep],
  );

  // Fetch build status on mount (page refresh resilience)
  useEffect(() => {
    let cancelled = false;

    async function fetchBuildStatus(): Promise<void> {
      try {
        const res = await fetch(`/api/v1/builds/${buildId}`, { credentials: "same-origin" });
        if (!res.ok || cancelled) return;
        const data = await res.json();

        if (data.status === "success" || data.status === "failed") {
          completedRef.current = true;
          setStatus(data.status);
          if (data.logs) {
            const logLines = data.logs.split("\n").filter(Boolean);
            setLines(logLines.map((text: string) => ({
              text,
              isError: text.toLowerCase().includes("error") || text.toLowerCase().includes("fatal"),
              timestamp: Date.now(),
            })));
          }
          setSteps((prev) =>
            data.status === "success"
              ? prev.map((s) => ({ ...s, status: "success" as StepStatus }))
              : prev.map((s, i) => ({
                  ...s,
                  status: i < prev.length - 1 ? "success" as StepStatus : "failed" as StepStatus,
                })),
          );
          if (data.image_tag) setAppUrl(data.image_tag);
          onComplete?.(data.status);
        }
      } catch {
        // Build status fetch failed; WS will handle it
      }
    }

    fetchBuildStatus();
    return () => { cancelled = true; };
  }, [buildId, onComplete]);

  // WebSocket connection with reconnection
  useEffect(() => {
    if (completedRef.current) return;

    function connect(): void {
      const url = buildWsUrl(buildId, baseUrl);
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        setStatus("running");
        setReconnecting(false);
        reconnectAttemptRef.current = 0;
      };

      ws.onmessage = (event) => {
        try {
          const msg: WsMessage = JSON.parse(event.data);
          handleMessage(msg);
        } catch {
          // Treat non-JSON as plain log line
          setLines((prev) => [
            ...prev,
            { text: event.data, isError: false, timestamp: Date.now() },
          ]);
        }
      };

      ws.onclose = () => {
        if (completedRef.current) return;
        scheduleReconnect();
      };

      ws.onerror = () => {
        // onclose will fire after this
      };
    }

    function scheduleReconnect(): void {
      if (completedRef.current) return;
      if (reconnectAttemptRef.current >= RECONNECT_MAX_ATTEMPTS) {
        setStatus("failed");
        return;
      }
      setReconnecting(true);
      const delay = Math.min(
        RECONNECT_BASE_MS * Math.pow(2, reconnectAttemptRef.current),
        RECONNECT_MAX_MS,
      );
      reconnectAttemptRef.current += 1;
      reconnectTimerRef.current = setTimeout(connect, delay);
    }

    connect();

    return () => {
      completedRef.current = true; // prevent reconnect on unmount
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };
  }, [buildId, baseUrl, handleMessage]);

  // Scroll to bottom on new lines
  useEffect(() => {
    scrollToBottom();
  }, [lines, scrollToBottom]);

  const isDone = status === "success" || status === "failed";
  const errorLines = lines.filter((l) => l.isError).map((l) => l.text);
  const errorInfo = status === "failed" ? analyzeDeployError(errorLines) : null;

  // ─── Styles ──────────────────────────────────────────────

  const containerStyle: React.CSSProperties = fullScreen
    ? {
        position: "fixed",
        inset: 0,
        zIndex: 50,
        display: "flex",
        flexDirection: "column",
        backgroundColor: "#0a0a0a",
      }
    : {
        display: "flex",
        flexDirection: "column",
        gap: 16,
      };

  const terminalWrapStyle: React.CSSProperties = {
    backgroundColor: "#1a1a1a",
    borderRadius: fullScreen ? 0 : 12,
    border: fullScreen ? "none" : "1px solid #262626",
    overflow: "hidden",
    display: "flex",
    flexDirection: "column",
    flex: fullScreen ? 1 : undefined,
  };

  const headerStyle: React.CSSProperties = {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    padding: "10px 16px",
    borderBottom: "1px solid #262626",
    backgroundColor: "#141414",
  };

  const logAreaStyle: React.CSSProperties = {
    fontFamily: "'SF Mono', 'Fira Code', 'Cascadia Code', monospace",
    fontSize: 12,
    lineHeight: 1.7,
    color: "#00ff41",
    padding: 16,
    overflowY: "auto",
    maxHeight: fullScreen ? undefined : 360,
    flex: fullScreen ? 1 : undefined,
    whiteSpace: "pre-wrap",
    wordBreak: "break-word",
  };

  return (
    <div style={containerStyle}>
      {/* Step progress sidebar (above terminal when not fullscreen) */}
      {!fullScreen && (
        <div
          style={{
            backgroundColor: "#141414",
            borderRadius: 12,
            border: "1px solid #262626",
            padding: 16,
          }}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
            <span
              style={{
                fontSize: 11,
                fontWeight: 600,
                color: "#a1a1aa",
                textTransform: "uppercase",
                letterSpacing: "0.05em",
              }}
            >
              Build Progress
            </span>
            <span style={{ fontSize: 11, fontFamily: "monospace", color: "#71717a" }}>
              {formatElapsed(elapsed)}
            </span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {steps.map((step, i) => (
              <DeployStep key={i} step={step} isLast={i === steps.length - 1} />
            ))}
          </div>
        </div>
      )}

      {/* Terminal */}
      <div style={terminalWrapStyle}>
        {/* Title bar */}
        <div style={headerStyle}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            {/* Traffic light dots */}
            <div style={{ display: "flex", gap: 6 }}>
              <div style={{ width: 10, height: 10, borderRadius: "50%", backgroundColor: "#ff5f57" }} />
              <div style={{ width: 10, height: 10, borderRadius: "50%", backgroundColor: "#febc2e" }} />
              <div style={{ width: 10, height: 10, borderRadius: "50%", backgroundColor: "#28c840" }} />
            </div>
            <span style={{ fontSize: 11, color: "#71717a", fontFamily: "monospace", marginLeft: 8 }}>
              build/{buildId}
            </span>
            {reconnecting && (
              <span
                style={{
                  fontSize: 10,
                  color: "#ff6b35",
                  marginLeft: 8,
                  animation: "dt-blink 1.2s ease-in-out infinite",
                }}
              >
                Reconnecting...
              </span>
            )}
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            {/* Scroll lock toggle */}
            <button
              type="button"
              onClick={() => {
                setScrollLocked((v) => !v);
                if (scrollLocked && logRef.current) {
                  logRef.current.scrollTop = logRef.current.scrollHeight;
                }
              }}
              style={{
                background: "none",
                border: "1px solid #333",
                borderRadius: 4,
                padding: "2px 8px",
                fontSize: 10,
                color: scrollLocked ? "#ff6b35" : "#71717a",
                cursor: "pointer",
                fontFamily: "monospace",
              }}
              title={scrollLocked ? "Auto-scroll paused (click to resume)" : "Auto-scroll active"}
            >
              {scrollLocked ? "scroll paused" : "auto-scroll"}
            </button>

            {/* Full-screen toggle */}
            <button
              type="button"
              onClick={() => setFullScreen((v) => !v)}
              style={{
                background: "none",
                border: "1px solid #333",
                borderRadius: 4,
                padding: "2px 8px",
                fontSize: 10,
                color: "#71717a",
                cursor: "pointer",
                fontFamily: "monospace",
              }}
            >
              {fullScreen ? "exit fullscreen" : "fullscreen"}
            </button>
          </div>
        </div>

        {/* Full-screen step bar */}
        {fullScreen && (
          <div
            style={{
              display: "flex",
              gap: 16,
              padding: "8px 16px",
              borderBottom: "1px solid #262626",
              backgroundColor: "#111",
              overflowX: "auto",
            }}
          >
            {steps.map((step, i) => (
              <div
                key={i}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  whiteSpace: "nowrap",
                }}
              >
                <div
                  style={{
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    backgroundColor:
                      step.status === "success"
                        ? "#00ff41"
                        : step.status === "active"
                          ? "#ff6b35"
                          : step.status === "failed"
                            ? "#ef4444"
                            : "#555",
                  }}
                />
                <span
                  style={{
                    fontSize: 11,
                    fontFamily: "monospace",
                    color:
                      step.status === "success"
                        ? "#00ff41"
                        : step.status === "active"
                          ? "#ff6b35"
                          : step.status === "failed"
                            ? "#ef4444"
                            : "#555",
                  }}
                >
                  {step.label}
                </span>
              </div>
            ))}
          </div>
        )}

        {/* Log output */}
        <div ref={logRef} style={logAreaStyle} onScroll={handleScroll}>
          {lines.length === 0 && status === "connecting" && (
            <span style={{ color: "#71717a" }}>Connecting to build stream...</span>
          )}
          {lines.length === 0 && status === "running" && (
            <span style={{ color: "#71717a" }}>Waiting for output...</span>
          )}
          {lines.map((line, i) => (
            <div
              key={i}
              style={{
                color: line.isError ? "#ef4444" : "#00ff41",
                animation: "dt-fade-in 0.15s ease-out",
              }}
            >
              {line.text}
            </div>
          ))}
          {!isDone && lines.length > 0 && (
            <span
              style={{
                display: "inline-block",
                width: 7,
                height: 14,
                backgroundColor: "#00ff41",
                animation: "dt-blink 1s step-end infinite",
                verticalAlign: "middle",
              }}
            />
          )}
        </div>
      </div>

      {/* Error callout */}
      {status === "failed" && (
        <div
          style={{
            backgroundColor: "rgba(239, 68, 68, 0.08)",
            border: "1px solid rgba(239, 68, 68, 0.25)",
            borderRadius: 12,
            padding: 20,
          }}
        >
          <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
            <div
              style={{
                width: 32,
                height: 32,
                borderRadius: 8,
                backgroundColor: "rgba(239, 68, 68, 0.15)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
              }}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <circle cx="8" cy="8" r="7" stroke="#ef4444" strokeWidth="1.5" />
                <path d="M5.5 5.5L10.5 10.5M10.5 5.5L5.5 10.5" stroke="#ef4444" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            </div>
            <div style={{ flex: 1 }}>
              <h4 style={{ fontSize: 14, fontWeight: 600, color: "#fca5a5", margin: 0 }}>
                {errorInfo?.title ?? "Build failed"}
              </h4>
              <p style={{ fontSize: 13, color: "#a1a1aa", margin: "6px 0 0" }}>
                {errorInfo?.message ?? "Something went wrong during the build. Check the logs above for details."}
              </p>
              {errorInfo?.steps && errorInfo.steps.length > 0 && (
                <div style={{ marginTop: 12 }}>
                  <p style={{ fontSize: 11, fontWeight: 600, color: "#71717a", marginBottom: 6 }}>
                    What to try:
                  </p>
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {errorInfo.steps.map((s, i) => (
                      <li key={i} style={{ fontSize: 12, color: "#a1a1aa", marginBottom: 3 }}>
                        {s}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              {errorInfo?.technicalDetail && (
                <details style={{ marginTop: 10 }}>
                  <summary
                    style={{ fontSize: 10, color: "#52525b", cursor: "pointer" }}
                  >
                    Technical details
                  </summary>
                  <p style={{ fontSize: 10, color: "#3f3f46", fontFamily: "monospace", marginTop: 4 }}>
                    {errorInfo.technicalDetail}
                  </p>
                </details>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Success callout */}
      {status === "success" && (
        <div
          style={{
            backgroundColor: "rgba(0, 255, 65, 0.06)",
            border: "1px solid rgba(0, 255, 65, 0.2)",
            borderRadius: 12,
            padding: 24,
            textAlign: "center",
          }}
        >
          <div
            style={{
              width: 48,
              height: 48,
              borderRadius: 16,
              backgroundColor: "rgba(0, 255, 65, 0.1)",
              border: "1px solid rgba(0, 255, 65, 0.2)",
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              marginBottom: 12,
            }}
          >
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
              <path d="M5 13l4 4L19 7" stroke="#00ff41" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </div>
          <h4 style={{ fontSize: 16, fontWeight: 700, color: "#e4e4e7", margin: 0 }}>
            Deploy complete
          </h4>
          <p style={{ fontSize: 13, color: "#71717a", marginTop: 4 }}>
            Built and deployed in {formatElapsed(elapsed)}
          </p>
          {appUrl && (
            <div style={{ marginTop: 16 }}>
              <p style={{ fontSize: 12, color: "#00ff41", marginBottom: 8 }}>
                Your app is live at{" "}
                <a
                  href={appUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{ color: "#00ff41", textDecoration: "underline" }}
                >
                  {appUrl}
                </a>
              </p>
              <a href={appUrl} target="_blank" rel="noopener noreferrer">
                <button
                  type="button"
                  className={cn(
                    "inline-flex items-center gap-2 rounded-lg px-5 py-2 text-sm font-semibold",
                    "transition-colors duration-150",
                  )}
                  style={{
                    backgroundColor: "rgba(0, 255, 65, 0.12)",
                    border: "1px solid rgba(0, 255, 65, 0.3)",
                    color: "#00ff41",
                    cursor: "pointer",
                  }}
                >
                  <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                    <path d="M5.25 8.75L12.25 1.75M12.25 1.75H8.75M12.25 1.75V5.25" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                    <path d="M10.5 7.875V11.375C10.5 11.6734 10.3815 11.9595 10.1705 12.1705C9.95951 12.3815 9.67337 12.5 9.375 12.5H2.625C2.32663 12.5 2.04048 12.3815 1.8295 12.1705C1.61853 11.9595 1.5 11.6734 1.5 11.375V4.625C1.5 4.32663 1.61853 4.04048 1.8295 3.8295C2.04048 3.61853 2.32663 3.5 2.625 3.5H6.125" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                  Open App
                </button>
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
