import { Button } from "@/components/ui/button";
import { ArrowLeft, Rocket } from "lucide-react";
import { getNextButtonText, isReviewStep } from "@/lib/interview-engine";

interface StepNavigationProps {
  currentStep: number;
  canGoBack: boolean;
  canGoNext: boolean;
  onBack: () => void;
  onNext: () => void;
}

export function StepNavigation({
  currentStep,
  canGoBack,
  canGoNext,
  onBack,
  onNext,
}: StepNavigationProps) {
  const nextText = getNextButtonText(currentStep);
  const isReview = isReviewStep(currentStep);

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        padding: "16px 0 0",
        borderTop: "1px solid rgba(255,255,255,0.06)",
        marginTop: 24,
      }}
    >
      <div>
        {canGoBack && (
          <Button variant="ghost" onClick={onBack}>
            <ArrowLeft style={{ width: 16, height: 16, marginRight: 6 }} />
            Back
          </Button>
        )}
      </div>
      <Button
        onClick={onNext}
        disabled={!canGoNext}
        style={
          isReview
            ? {
                background: "#ff6b35",
                color: "#fff",
                fontWeight: 600,
                fontSize: 15,
                padding: "10px 28px",
                border: "none",
              }
            : undefined
        }
      >
        {isReview && (
          <Rocket style={{ width: 16, height: 16, marginRight: 6 }} />
        )}
        {nextText}
      </Button>
    </div>
  );
}
