import { useState } from "react";
import { Input } from "@/components/ui/input";
import { ChevronDown, ChevronRight, Heart, Cpu, Scaling } from "lucide-react";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import { InlineHelp } from "@/components/ui/InlineHelp";
import type {
  Trait,
  WizardTarget,
  WorkloadType,
  HealthCheckDefaults,
  ResourceDefaults,
  ScalingDefaults,
} from "@/lib/interview-types";

interface StepAnythingElseProps {
  traits: Trait[];
  target: WizardTarget;
  workloadType: WorkloadType;
  port: number;
  healthCheck: HealthCheckDefaults;
  resources: ResourceDefaults;
  scaling: ScalingDefaults;
  onUpdate: (updates: {
    healthCheck?: HealthCheckDefaults;
    resources?: ResourceDefaults;
    scaling?: ScalingDefaults;
  }) => void;
}

function CollapsibleSection({
  icon: Icon,
  title,
  summary,
  open,
  onToggle,
  children,
}: {
  icon: typeof Heart;
  title: React.ReactNode;
  summary: string;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <div
      style={{
        background: "rgba(255,255,255,0.03)",
        border: "1px solid rgba(255,255,255,0.1)",
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      <button
        type="button"
        onClick={onToggle}
        style={{
          width: "100%",
          display: "flex",
          alignItems: "center",
          gap: 12,
          padding: "16px 20px",
          background: "transparent",
          border: "none",
          cursor: "pointer",
          textAlign: "left",
        }}
      >
        <Icon size={20} color="#ff6b35" />
        <div style={{ flex: 1 }}>
          <span style={{ color: "#fff", fontSize: 14, fontWeight: 500 }}>{title}</span>
          {!open && (
            <span style={{ color: "rgba(255,255,255,0.35)", fontSize: 12, marginLeft: 12 }}>
              {summary}
            </span>
          )}
        </div>
        {open ? (
          <ChevronDown size={18} color="rgba(255,255,255,0.4)" />
        ) : (
          <ChevronRight size={18} color="rgba(255,255,255,0.4)" />
        )}
      </button>
      {open && (
        <div style={{ padding: "0 20px 20px", display: "flex", flexDirection: "column", gap: 14 }}>
          {children}
        </div>
      )}
    </div>
  );
}

function FieldRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
      <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 13, minWidth: 100 }}>{label}</label>
      {children}
    </div>
  );
}

export default function StepAnythingElse({
  target,
  port,
  healthCheck,
  resources,
  scaling,
  onUpdate,
}: StepAnythingElseProps) {
  const [healthOpen, setHealthOpen] = useState(false);
  const [resourcesOpen, setResourcesOpen] = useState(false);
  const [scalingOpen, setScalingOpen] = useState(false);

  const isLocal = target === "local";

  const healthSummary = healthCheck.enabled
    ? `${healthCheck.type.toUpperCase()} ${healthCheck.type === "http" ? healthCheck.path : "port " + healthCheck.port} every ${healthCheck.intervalSeconds}s`
    : "Disabled";

  const resourcesSummary = `CPU ${resources.cpuRequest}–${resources.cpuLimit}, Mem ${resources.memoryRequest}–${resources.memoryLimit}`;

  const scalingSummary = scaling.enabled
    ? `${scaling.minReplicas}–${scaling.maxReplicas} replicas at ${scaling.cpuTarget}% CPU`
    : "Manual (single replica)";

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      <p style={{ color: "rgba(255,255,255,0.5)", fontSize: 14, margin: 0 }}>
        Fine-tune the defaults or leave them as-is. Most apps work great without changes.
      </p>

      {/* Health Check — hidden for local target */}
      {!isLocal && (
        <CollapsibleSection
          icon={Heart}
          title={<>How should we check the crust? <HelpTooltip term="health-check" /></>}
          summary={healthSummary}
          open={healthOpen}
          onToggle={() => setHealthOpen(!healthOpen)}
        >
          <div style={{ display: "flex", gap: 8 }}>
            {(["http", "tcp"] as const).map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => onUpdate({ healthCheck: { ...healthCheck, type: t } })}
                style={{
                  padding: "8px 16px",
                  background: healthCheck.type === t ? "rgba(255,107,53,0.15)" : "rgba(255,255,255,0.03)",
                  border: `1px solid ${healthCheck.type === t ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
                  borderRadius: 6,
                  color: healthCheck.type === t ? "#ff6b35" : "rgba(255,255,255,0.6)",
                  fontSize: 13,
                  cursor: "pointer",
                  transition: "all 0.15s ease",
                }}
              >
                {t === "http" ? "Web check (HTTP)" : "Connection check (TCP)"}
              </button>
            ))}
          </div>
          {healthCheck.type === "http" && (
            <div>
              <FieldRow label="Path">
                <Input
                  value={healthCheck.path}
                  onChange={(e) => onUpdate({ healthCheck: { ...healthCheck, path: e.target.value } })}
                  placeholder="/health"
                  style={{ maxWidth: 200, fontFamily: "monospace", fontSize: 13 }}
                />
              </FieldRow>
              <InlineHelp>Most frameworks have a /health or /healthz endpoint. If yours doesn't, "/" (the homepage) works as a basic check.</InlineHelp>
            </div>
          )}
          <FieldRow label="Port">
            <Input
              type="number"
              value={healthCheck.port}
              onChange={(e) =>
                onUpdate({ healthCheck: { ...healthCheck, port: parseInt(e.target.value) || port } })
              }
              style={{ maxWidth: 120 }}
            />
          </FieldRow>
          <FieldRow label="Interval (s)">
            <Input
              type="number"
              value={healthCheck.intervalSeconds}
              onChange={(e) =>
                onUpdate({
                  healthCheck: { ...healthCheck, intervalSeconds: parseInt(e.target.value) || 10 },
                })
              }
              style={{ maxWidth: 120 }}
            />
          </FieldRow>
        </CollapsibleSection>
      )}

      {/* Resources */}
      <CollapsibleSection
        icon={Cpu}
        title={<>Oven temperature <HelpTooltip term="cpu-memory" /></>}
        summary={resourcesSummary}
        open={resourcesOpen}
        onToggle={() => setResourcesOpen(!resourcesOpen)}
      >
        <p className="text-[10px] text-muted-foreground/40 mt-1">S = light traffic, M = most apps, L = heavy traffic, XL = data crunching</p>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, marginBottom: 6, display: "block" }}>
              CPU Request
            </label>
            <Input
              value={resources.cpuRequest}
              onChange={(e) => onUpdate({ resources: { ...resources, cpuRequest: e.target.value } })}
              placeholder="250m"
              style={{ fontFamily: "monospace", fontSize: 13 }}
            />
          </div>
          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, marginBottom: 6, display: "block" }}>
              CPU Limit
            </label>
            <Input
              value={resources.cpuLimit}
              onChange={(e) => onUpdate({ resources: { ...resources, cpuLimit: e.target.value } })}
              placeholder="500m"
              style={{ fontFamily: "monospace", fontSize: 13 }}
            />
          </div>
          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, marginBottom: 6, display: "block" }}>
              Memory Request
            </label>
            <Input
              value={resources.memoryRequest}
              onChange={(e) => onUpdate({ resources: { ...resources, memoryRequest: e.target.value } })}
              placeholder="128Mi"
              style={{ fontFamily: "monospace", fontSize: 13 }}
            />
          </div>
          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 12, marginBottom: 6, display: "block" }}>
              Memory Limit
            </label>
            <Input
              value={resources.memoryLimit}
              onChange={(e) => onUpdate({ resources: { ...resources, memoryLimit: e.target.value } })}
              placeholder="256Mi"
              style={{ fontFamily: "monospace", fontSize: 13 }}
            />
          </div>
        </div>
      </CollapsibleSection>

      {/* Scaling — hidden for local target */}
      {!isLocal && (
        <CollapsibleSection
          icon={Scaling}
          title={<>Automatic copies <HelpTooltip term="auto-scaling" /></>}
          summary={scalingSummary}
          open={scalingOpen}
          onToggle={() => setScalingOpen(!scalingOpen)}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <button
              type="button"
              onClick={() => onUpdate({ scaling: { ...scaling, enabled: !scaling.enabled } })}
              style={{
                width: 44,
                height: 24,
                borderRadius: 12,
                background: scaling.enabled ? "#ff6b35" : "rgba(255,255,255,0.15)",
                border: "none",
                cursor: "pointer",
                position: "relative",
                transition: "background 0.2s ease",
              }}
            >
              <span
                style={{
                  position: "absolute",
                  top: 3,
                  left: scaling.enabled ? 23 : 3,
                  width: 18,
                  height: 18,
                  borderRadius: "50%",
                  background: "#fff",
                  transition: "left 0.2s ease",
                }}
              />
            </button>
            <span style={{ color: "rgba(255,255,255,0.7)", fontSize: 14 }}>
              {scaling.enabled ? "Auto-scaling enabled" : "Auto-scaling disabled"}
            </span>
          </div>
          {scaling.enabled && (
            <>
              <FieldRow label="Min replicas">
                <Input
                  type="number"
                  value={scaling.minReplicas}
                  onChange={(e) =>
                    onUpdate({ scaling: { ...scaling, minReplicas: parseInt(e.target.value) || 1 } })
                  }
                  style={{ maxWidth: 100 }}
                />
              </FieldRow>
              <FieldRow label="Max replicas">
                <Input
                  type="number"
                  value={scaling.maxReplicas}
                  onChange={(e) =>
                    onUpdate({ scaling: { ...scaling, maxReplicas: parseInt(e.target.value) || 5 } })
                  }
                  style={{ maxWidth: 100 }}
                />
              </FieldRow>
              <FieldRow label="CPU target %">
                <Input
                  type="number"
                  value={scaling.cpuTarget}
                  onChange={(e) =>
                    onUpdate({ scaling: { ...scaling, cpuTarget: parseInt(e.target.value) || 80 } })
                  }
                  style={{ maxWidth: 100 }}
                />
              </FieldRow>
            </>
          )}
        </CollapsibleSection>
      )}
    </div>
  );
}
