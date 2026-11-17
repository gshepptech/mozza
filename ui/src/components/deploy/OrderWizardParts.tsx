import { Check, CheckCircle } from "lucide-react";
import { cn } from "@/lib/utils";
import type { ImageAlias } from "@/api/types";

// ─── Fast Lane Step Indicator ───────────────────────────────

const STEP_LABELS = ["Pick", "Customize", "Order"];

export function FastLaneIndicator({ currentStep }: { currentStep: number }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      {STEP_LABELS.map((label, i) => {
        const stepNum = i + 1;
        const isCompleted = stepNum < currentStep;
        const isCurrent = stepNum === currentStep;
        return (
          <div key={label} style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  background: isCompleted || isCurrent ? "#ff6b35" : "rgba(255,255,255,0.08)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 12,
                  fontWeight: 700,
                  color: isCompleted || isCurrent ? "#fff" : "rgba(255,255,255,0.35)",
                  boxShadow: isCurrent ? "0 0 12px rgba(255,107,53,0.4)" : "none",
                  transition: "all 0.2s ease",
                }}
              >
                {isCompleted ? <Check size={14} /> : stepNum}
              </div>
              <span
                style={{
                  fontSize: 13,
                  fontWeight: isCurrent ? 600 : 400,
                  color: isCurrent ? "#fff" : "rgba(255,255,255,0.45)",
                }}
              >
                {label}
              </span>
            </div>
            {i < STEP_LABELS.length - 1 && (
              <div
                style={{
                  width: 32,
                  height: 2,
                  background: isCompleted ? "#ff6b35" : "rgba(255,255,255,0.1)",
                  borderRadius: 1,
                }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// ─── Alias Card ─────────────────────────────────────────────

function aliasColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  const hues = [0, 30, 60, 120, 180, 210, 270, 330];
  return `hsl(${hues[Math.abs(hash) % hues.length]}, 60%, 45%)`;
}

export function AliasCard({
  alias,
  selected,
  onClick,
}: {
  alias: ImageAlias;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "relative rounded-xl border p-4 text-left transition-all w-full",
        selected
          ? "border-brand/50 bg-brand-muted shadow-[0_0_20px_rgba(255,107,53,0.1)]"
          : "border-border/50 bg-card hover:border-brand/25",
      )}
    >
      {selected && (
        <div className="absolute top-3 right-3">
          <CheckCircle className="h-4 w-4 text-brand" />
        </div>
      )}
      <div className="flex items-center gap-3">
        <div
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-white text-sm font-bold"
          style={{ background: aliasColor(alias.name) }}
        >
          {alias.name.charAt(0).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-foreground truncate">{alias.name}</p>
          <p className="text-[11px] text-muted-foreground/60 truncate font-mono mt-0.5">
            {alias.image_ref}
          </p>
        </div>
      </div>
    </button>
  );
}

// ─── Tab Button ─────────────────────────────────────────────

export function TabButton({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-lg px-4 py-2 text-sm font-semibold transition-all",
        active
          ? "bg-brand text-primary-foreground shadow-[0_0_12px_rgba(255,107,53,0.2)]"
          : "bg-muted text-muted-foreground hover:bg-muted/80 hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}


export function TargetToggle({
  target,
  onChange,
}: {
  target: "local" | "kitchen";
  onChange: (t: "local" | "kitchen") => void;
}) {
  return (
    <div className="space-y-1.5">
      <label className="text-xs text-muted-foreground">Where to deploy</label>
      <div className="flex gap-2">
        {(["local", "kitchen"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => onChange(t)}
            className={cn(
              "flex-1 rounded-lg border px-3 py-2.5 text-sm font-semibold transition-all text-center",
              target === t
                ? "border-brand/50 bg-brand-muted text-foreground"
                : "border-border/50 text-muted-foreground hover:border-brand/25",
            )}
          >
            {t === "local" ? "My computer" : "Cloud server"}
          </button>
        ))}
      </div>
    </div>
  );
}
