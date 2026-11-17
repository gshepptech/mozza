import type { Dispatch, ReactNode, SetStateAction } from "react";
import { Input } from "@/components/ui/input";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import { InlineHelp } from "@/components/ui/InlineHelp";
import { cn } from "@/lib/utils";
import { RESOURCE_SIZE_MAP } from "@/lib/interview-engine";
import type { ResourceSize, WizardState } from "@/lib/interview-types";

interface OrderCustomizePanelProps {
  wizardState: WizardState;
  setWizardState: Dispatch<SetStateAction<WizardState>>;
  expanded: boolean;
  onToggleExpanded: () => void;
}

// Advanced settings for the first slice: port, replicas, domain, schedule,
// resource size and health check.
export function OrderCustomizePanel({
  wizardState,
  setWizardState,
  expanded: customizeExpanded,
  onToggleExpanded,
}: OrderCustomizePanelProps) {
  const selectedCombo = wizardState.selectedCombo ?? null;
  return (
    <div>
      <button
        type="button"
        onClick={onToggleExpanded}
        className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
      >
        <span style={{ transform: customizeExpanded ? "rotate(90deg)" : "none", transition: "transform 0.15s", display: "inline-block" }}>▸</span>
        Customize more...
      </button>
      {customizeExpanded && (
        <div className="mt-4 space-y-4 rounded-xl border border-border/30 bg-card/50 p-4">
          {/* Port */}
          <div className="space-y-1.5">
            <label className="text-xs text-muted-foreground">Port <HelpTooltip term="port" /></label>
            <Input
              type="number"
              value={wizardState.services[0]?.port || ""}
              onChange={(e) => {
                const port = parseInt(e.target.value) || 0;
                setWizardState((prev) => {
                  const services = [...prev.services];
                  const svc = services[0];
                  if (!svc) return prev;
                  services[0] = { ...svc, port };
                  return { ...prev, services };
                });
              }}
              placeholder="8080"
              className="h-9 text-sm bg-elevated w-32"
            />
            <InlineHelp label="What port should I use?">
              Check your app's startup logs for "Listening on port XXXX". Common defaults: 3000 (Node.js), 8080 (Go/Java), 5000 (Python), 4000 (Phoenix).
            </InlineHelp>
          </div>

          {/* Replicas */}
          <div className="space-y-1.5">
            <label className="text-xs text-muted-foreground">How many copies? <HelpTooltip term="replica" /></label>
            <Input
              type="number"
              value={wizardState.services[0]?.replicas || 1}
              onChange={(e) => {
                const replicas = Math.max(1, Math.min(100, parseInt(e.target.value) || 1));
                setWizardState((prev) => {
                  const services = [...prev.services];
                  const svc = services[0];
                  if (!svc) return prev;
                  services[0] = { ...svc, replicas };
                  return { ...prev, services };
                });
              }}
              min={1}
              max={100}
              className="h-9 text-sm bg-elevated w-32"
            />
            <p className="text-[10px] text-muted-foreground/40 mt-1">
              {(wizardState.services[0]?.replicas ?? 1) === 1 ? "Single copy, fine for development" :
               (wizardState.services[0]?.replicas ?? 1) <= 3 ? "Recommended for production: stays up if one copy fails" :
               "High availability for heavy traffic"}
            </p>
          </div>

          {/* Custom domain */}
          <div className="space-y-1.5">
            <label className="text-xs text-muted-foreground">Custom address</label>
            <Input
              value={wizardState.services[0]?.domain || ""}
              onChange={(e) => {
                setWizardState((prev) => {
                  const services = [...prev.services];
                  const svc = services[0];
                  if (!svc) return prev;
                  services[0] = { ...svc, domain: e.target.value };
                  return { ...prev, services };
                });
              }}
              placeholder="app.example.com"
              className="h-9 text-sm bg-elevated"
            />
            <InlineHelp>To use your own address, enter it here. You'll need to update your DNS settings to point to your server. Mozza will show you how after deploying.</InlineHelp>
          </div>

          {/* Schedule (workers only) */}
          {(selectedCombo === "background-worker" || selectedCombo === "custom") && (
            <div className="space-y-1.5">
              <label className="text-xs text-muted-foreground">Schedule</label>
              <div className="flex gap-2">
                {[
                  { label: "Every hour", cron: "0 * * * *" },
                  { label: "Every day", cron: "0 0 * * *" },
                  { label: "Every week", cron: "0 0 * * 1" },
                ].map((preset) => (
                  <button
                    key={preset.cron}
                    type="button"
                    onClick={() => {
                      setWizardState((prev) => {
                        const services = [...prev.services];
                        const svc = services[0];
                        if (!svc) return prev;
                        services[0] = { ...svc, schedule: preset.cron };
                        return { ...prev, services };
                      });
                    }}
                    className={cn(
                      "rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all",
                      wizardState.services[0]?.schedule === preset.cron
                        ? "border-brand/50 bg-brand-muted text-foreground"
                        : "border-border/50 text-muted-foreground hover:border-brand/25",
                    )}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
              <p className="text-[10px] text-muted-foreground/40 mt-1.5">Your task will run at the scheduled time, do its work, then stop until the next run.</p>
            </div>
          )}

          {/* Resource size (T-shirt) */}
          <div className="space-y-1.5">
            <label className="text-xs text-muted-foreground">Resources</label>
            <div className="flex gap-2">
              {(["S", "M", "L", "XL"] as ResourceSize[]).map((size) => (
                <button
                  key={size}
                  type="button"
                  onClick={() => {
                    const resources = RESOURCE_SIZE_MAP[size];
                    setWizardState((prev) => {
                      const services = [...prev.services];
                      const svc = services[0];
                      if (!svc) return prev;
                      services[0] = { ...svc, resources };
                      return { ...prev, services, resourceSize: size };
                    });
                  }}
                  className={cn(
                    "rounded-lg border px-4 py-2 text-xs font-bold transition-all",
                    wizardState.resourceSize === size
                      ? "border-brand/50 bg-brand-muted text-foreground"
                      : "border-border/50 text-muted-foreground hover:border-brand/25",
                  )}
                >
                  {size}
                </button>
              ))}
            </div>
            <p className="text-[10px] text-muted-foreground/50">
              {wizardState.resourceSize === "S" ? "100m CPU, 64Mi memory" :
               wizardState.resourceSize === "M" ? "250m CPU, 128Mi memory" :
               wizardState.resourceSize === "L" ? "500m CPU, 256Mi memory" :
               "1000m CPU, 512Mi memory"}
            </p>
          </div>

          {/* Health check toggle */}
          <ExtraToggle
            label="Health check"
            enabled={wizardState.services[0]?.healthCheck.enabled ?? true}
            suffix={<HelpTooltip term="health-check" />}
            onToggle={(on) => {
              setWizardState((prev) => {
                const services = [...prev.services];
                const svc = services[0];
                if (!svc) return prev;
                services[0] = { ...svc, healthCheck: { ...svc.healthCheck, enabled: on } };
                return { ...prev, services };
              });
            }}
          />
        </div>
      )}
    </div>
  );
}

function ExtraToggle({
  label,
  enabled,
  onToggle,
  suffix,
}: {
  label: string;
  enabled: boolean;
  onToggle: (on: boolean) => void;
  suffix?: ReactNode;
}) {
  return (
    <button
      type="button"
      className="flex items-center justify-between w-full py-3 px-4 rounded-xl border border-border/50 bg-card hover:border-brand/25 transition-colors"
      onClick={() => onToggle(!enabled)}
    >
      <span className="text-sm text-foreground">{label}{suffix && <> {suffix}</>}</span>
      <div className={cn("relative w-11 h-6 rounded-full transition-colors", enabled ? "bg-brand" : "bg-muted")}>
        <div className={cn("absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform shadow-sm", enabled ? "translate-x-[22px]" : "translate-x-0.5")} />
      </div>
    </button>
  );
}
