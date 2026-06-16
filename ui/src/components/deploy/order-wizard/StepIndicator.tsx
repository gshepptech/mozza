import type { InterviewQuestion } from "@/lib/interview-types";

interface StepIndicatorProps {
  activeSteps: InterviewQuestion[];
  currentStep: number;
}

export function StepIndicator({ activeSteps, currentStep }: StepIndicatorProps) {
  const currentIdx = activeSteps.findIndex((s) => s.stepNumber === currentStep);
  // Don't show indicator on deploy step (step 9)
  const visibleSteps = activeSteps.filter((s) => s.stepNumber <= 8);

  return (
    <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "0 4px" }}>
      {visibleSteps.map((step, i) => {
        const stepIdx = activeSteps.findIndex((s) => s.stepNumber === step.stepNumber);
        const isCompleted = stepIdx < currentIdx;
        const isCurrent = step.stepNumber === currentStep;

        return (
          <div key={step.id} style={{ display: "flex", alignItems: "center", gap: 6 }}>
            <div
              style={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                background: isCompleted
                  ? "#ff6b35"
                  : isCurrent
                    ? "#ff6b35"
                    : "rgba(255,255,255,0.15)",
                boxShadow: isCurrent ? "0 0 8px rgba(255,107,53,0.5)" : "none",
                transition: "all 0.2s ease",
              }}
            />
            {i < visibleSteps.length - 1 && (
              <div
                style={{
                  width: 20,
                  height: 2,
                  background: isCompleted ? "#ff6b35" : "rgba(255,255,255,0.1)",
                  borderRadius: 1,
                  transition: "background 0.2s ease",
                }}
              />
            )}
          </div>
        );
      })}
      <span
        style={{
          marginLeft: 12,
          fontSize: 13,
          color: "rgba(255,255,255,0.45)",
          fontVariantNumeric: "tabular-nums",
        }}
      >
        Step {Math.min(currentIdx + 1, visibleSteps.length)} of {visibleSteps.length}
      </span>
    </div>
  );
}
