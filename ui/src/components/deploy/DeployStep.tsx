import { cn } from "@/lib/utils";

// ─── Types ───────────────────────────────────────────────────

export type StepStatus = "pending" | "active" | "success" | "failed";

export interface DeployStepInfo {
  label: string;
  status: StepStatus;
}

interface DeployStepProps {
  step: DeployStepInfo;
  isLast: boolean;
}

// ─── Spinner keyframes (injected once) ──────────────────────

let injected = false;
function injectKeyframes(): void {
  if (injected || typeof document === "undefined") return;
  injected = true;
  const style = document.createElement("style");
  style.textContent = `
    @keyframes dt-spin { to { transform: rotate(360deg); } }
    @keyframes dt-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.5; } }
  `;
  document.head.appendChild(style);
}

// ─── Component ──────────────────────────────────────────────

export default function DeployStep({ step, isLast }: DeployStepProps) {
  injectKeyframes();

  const dotSize = 12;
  const lineHeight = 20;

  return (
    <div style={{ display: "flex", alignItems: "flex-start", gap: 12, position: "relative" }}>
      {/* Connector line */}
      {!isLast && (
        <div
          style={{
            position: "absolute",
            left: dotSize / 2 - 0.5,
            top: dotSize + 4,
            width: 1,
            height: lineHeight,
            backgroundColor: step.status === "success" ? "#00ff41" : "#333",
            transition: "background-color 0.3s ease",
          }}
        />
      )}

      {/* Status dot */}
      <div style={{ position: "relative", flexShrink: 0, paddingTop: 2 }}>
        {step.status === "active" ? (
          <div
            style={{
              width: dotSize,
              height: dotSize,
              borderRadius: "50%",
              border: "2px solid #ff6b35",
              borderTopColor: "transparent",
              animation: "dt-spin 0.8s linear infinite",
            }}
          />
        ) : (
          <div
            style={{
              width: dotSize,
              height: dotSize,
              borderRadius: "50%",
              backgroundColor:
                step.status === "success"
                  ? "#00ff41"
                  : step.status === "failed"
                    ? "#ef4444"
                    : "#555",
              transition: "background-color 0.3s ease",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            {step.status === "success" && (
              <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
                <path d="M1.5 4L3.2 5.7L6.5 2.3" stroke="#0a0a0a" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            )}
            {step.status === "failed" && (
              <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
                <path d="M2 2L6 6M6 2L2 6" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" />
              </svg>
            )}
          </div>
        )}
      </div>

      {/* Label */}
      <span
        className={cn(
          "text-xs font-mono leading-none transition-colors duration-200",
          step.status === "success" && "text-green-400",
          step.status === "active" && "text-[#ff6b35] font-medium",
          step.status === "failed" && "text-red-400",
          step.status === "pending" && "text-muted-foreground",
        )}
        style={{ paddingTop: 0 }}
      >
        {step.label}
      </span>
    </div>
  );
}
