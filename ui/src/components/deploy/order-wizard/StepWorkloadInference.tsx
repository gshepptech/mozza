import { useMemo } from "react";
import { Server, Layers, Radio, Clock, Check } from "lucide-react";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import type { Trait, WorkloadType } from "@/lib/interview-types";
import {
  getRecommendation,
  workloadLabel,
  workloadSubtitle,
  workloadAnalogy,
} from "@/lib/recommendation-engine";

interface StepWorkloadInferenceProps {
  traits: Trait[];
  replicas: number;
  workloadType: WorkloadType;
  workloadOverridden: boolean;
  schedule: string;
  onUpdate: (updates: {
    workloadType?: WorkloadType;
    workloadOverridden?: boolean;
    schedule?: string;
  }) => void;
}

const SCHEDULE_PRESETS = [
  { id: "hourly", label: "Every hour", cron: "0 * * * *" },
  { id: "daily", label: "Every day at midnight", cron: "0 0 * * *" },
  { id: "weekly", label: "Every Monday", cron: "0 0 * * 1" },
  { id: "custom", label: "Custom", cron: "" },
];

const BASE_CARDS: Array<{ type: WorkloadType; icon: typeof Server }> = [
  { type: "replicaset", icon: Layers },
  { type: "statefulset", icon: Server },
  { type: "daemonset", icon: Radio },
];

export default function StepWorkloadInference({
  traits,
  replicas,
  workloadType,
  workloadOverridden: _workloadOverridden,
  schedule,
  onUpdate,
}: StepWorkloadInferenceProps) {
  const recommendation = useMemo(() => getRecommendation(traits), [traits]);
  const showCronJob = traits.includes("worker");

  const cards: Array<{ type: WorkloadType; icon: typeof Server }> = showCronJob
    ? [...BASE_CARDS, { type: "cronjob", icon: Clock }]
    : BASE_CARDS;

  const selectWorkload = (wt: WorkloadType) => {
    const isRecommended = wt === recommendation.workloadType;
    onUpdate({
      workloadType: wt,
      workloadOverridden: !isRecommended,
      schedule: wt === "cronjob" && !schedule ? "0 * * * *" : schedule,
    });
  };

  const activePreset =
    SCHEDULE_PRESETS.find((p) => p.cron === schedule)?.id ?? "custom";

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Recommendation text */}
      <div
        style={{
          padding: 14,
          background: "rgba(255,107,53,0.06)",
          border: "1px solid rgba(255,107,53,0.2)",
          borderRadius: 10,
          fontSize: 14,
          lineHeight: 1.5,
          color: "rgba(255,255,255,0.8)",
        }}
      >
        <HelpTooltip term="workload-type" /> Based on your answers, this looks like a{" "}
        <strong style={{ color: "#ff6b35" }}>
          {workloadLabel(recommendation.workloadType)}
        </strong>{" "}
        workload. We recommend{" "}
        <strong style={{ color: "#ff6b35" }}>
          {workloadLabel(recommendation.workloadType)}
        </strong>{" "}
        with <strong style={{ color: "#ff6b35" }}>{replicas}</strong>{" "}
        {replicas === 1 ? "copy" : "copies"}.
        <br />
        <span style={{ color: "rgba(255,255,255,0.5)", fontSize: 13 }}>
          {recommendation.reasoning}
        </span>
      </div>

      {/* Workload cards */}
      <div style={{ display: "flex", gap: 12 }}>
        {cards.map(({ type, icon: Icon }) => {
          const selected = workloadType === type;
          const isRecommended = recommendation.workloadType === type;
          return (
            <button
              key={type}
              type="button"
              onClick={() => selectWorkload(type)}
              style={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: 8,
                padding: 16,
                background: "rgba(255,255,255,0.03)",
                border: `1px solid ${selected ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
                borderRadius: 10,
                cursor: "pointer",
                position: "relative",
                transition: "border-color 0.15s ease, box-shadow 0.15s ease",
                boxShadow: isRecommended
                  ? "0 0 12px rgba(255,107,53,0.4)"
                  : "none",
              }}
            >
              {/* Selected checkmark */}
              {selected && (
                <span
                  style={{
                    position: "absolute",
                    top: 8,
                    right: 8,
                    width: 20,
                    height: 20,
                    borderRadius: "50%",
                    background: "#ff6b35",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <Check size={12} color="#fff" />
                </span>
              )}

              {/* Recommended badge */}
              {isRecommended && (
                <span
                  style={{
                    position: "absolute",
                    top: -10,
                    left: "50%",
                    transform: "translateX(-50%)",
                    background: "#ff6b35",
                    color: "#fff",
                    fontSize: 10,
                    fontWeight: 600,
                    padding: "2px 8px",
                    borderRadius: 8,
                    whiteSpace: "nowrap",
                  }}
                >
                  Chef&apos;s recommendation
                </span>
              )}

              <Icon
                size={24}
                color={selected ? "#ff6b35" : "rgba(255,255,255,0.5)"}
              />
              <span
                style={{
                  color: selected ? "#fff" : "rgba(255,255,255,0.7)",
                  fontSize: 14,
                  fontWeight: 600,
                }}
              >
                {workloadLabel(type)}
              </span>
              <span
                style={{
                  color: "rgba(255,255,255,0.4)",
                  fontSize: 12,
                  textAlign: "center",
                }}
              >
                {workloadSubtitle(type)}
              </span>
              <span
                style={{
                  color: "rgba(255,255,255,0.3)",
                  fontSize: 11,
                  fontStyle: "italic",
                  textAlign: "center",
                }}
              >
                {workloadAnalogy(type)}
              </span>
            </button>
          );
        })}
      </div>

      {/* Schedule picker for CronJob */}
      {workloadType === "cronjob" && (
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: 8,
            padding: 14,
            background: "rgba(255,255,255,0.03)",
            border: "1px solid rgba(255,255,255,0.1)",
            borderRadius: 10,
          }}
        >
          <span style={{ color: "rgba(255,255,255,0.7)", fontSize: 13 }}>
            How often should it run?
          </span>
          <div style={{ display: "flex", gap: 8 }}>
            {SCHEDULE_PRESETS.map((preset) => (
              <button
                key={preset.id}
                type="button"
                onClick={() => {
                  if (preset.id === "custom") {
                    onUpdate({ schedule: "" });
                  } else {
                    onUpdate({ schedule: preset.cron });
                  }
                }}
                style={{
                  padding: "6px 12px",
                  fontSize: 13,
                  background:
                    activePreset === preset.id
                      ? "rgba(255,107,53,0.15)"
                      : "rgba(255,255,255,0.05)",
                  border: `1px solid ${
                    activePreset === preset.id
                      ? "#ff6b35"
                      : "rgba(255,255,255,0.1)"
                  }`,
                  borderRadius: 6,
                  color:
                    activePreset === preset.id
                      ? "#ff6b35"
                      : "rgba(255,255,255,0.6)",
                  cursor: "pointer",
                }}
              >
                {preset.label}
              </button>
            ))}
          </div>

          {activePreset === "custom" && (
            <input
              type="text"
              placeholder="e.g. */15 * * * * (every 15 min)"
              value={schedule}
              onChange={(e) => onUpdate({ schedule: e.target.value })}
              style={{
                padding: "8px 12px",
                fontSize: 13,
                background: "rgba(255,255,255,0.05)",
                border: "1px solid rgba(255,255,255,0.15)",
                borderRadius: 6,
                color: "#fff",
                outline: "none",
              }}
            />
          )}
        </div>
      )}
    </div>
  );
}
