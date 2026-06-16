import { ArrowLeft, ArrowRight, Rocket, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface WizardStepProps {
  stepNumber: number;
  totalSteps: number;
  title: string;
  description: string;
  children: React.ReactNode;
  onNext: () => void;
  onBack: () => void;
  isValid: boolean;
  isFirst: boolean;
  isLast: boolean;
}

export default function WizardStep({
  stepNumber,
  totalSteps,
  title,
  description,
  children,
  onNext,
  onBack,
  isValid,
  isFirst,
  isLast,
}: WizardStepProps) {
  const progress = (stepNumber / totalSteps) * 100;

  return (
    <div className="flex flex-col gap-8">
      {/* Step indicators */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          {Array.from({ length: totalSteps }, (_, i) => {
            const step = i + 1;
            const completed = step < stepNumber;
            const active = step === stepNumber;
            return (
              <div key={step} className="flex items-center flex-1 last:flex-0">
                <div
                  className={cn(
                    "flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-sm font-bold transition-all",
                    completed && "bg-brand/20 text-brand",
                    active && "bg-brand text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.3)]",
                    !completed && !active && "bg-muted text-muted-foreground"
                  )}
                >
                  {completed ? <Check className="h-4 w-4" /> : step}
                </div>
                {step < totalSteps && (
                  <div className={cn(
                    "flex-1 h-0.5 mx-2",
                    step < stepNumber ? "bg-brand/40" : "bg-muted"
                  )} />
                )}
              </div>
            );
          })}
        </div>

        {/* Progress bar */}
        <div className="space-y-2">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span className="font-medium">
              Step {stepNumber} of {totalSteps}
            </span>
            <span>{Math.round(progress)}%</span>
          </div>
          <div className="h-2 rounded-full bg-muted/60 overflow-hidden">
            <div
              className="h-full rounded-full bg-brand transition-all duration-500 ease-out shadow-[0_0_8px_rgba(255,107,53,0.3)]"
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
      </div>

      {/* Step header */}
      <div className="flex items-start gap-5">
        <div
          className={cn(
            "flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl text-lg font-bold",
            "bg-brand text-primary-foreground shadow-[0_0_20px_rgba(255,107,53,0.25)]"
          )}
        >
          {stepNumber}
        </div>
        <div>
          <h2 className="text-xl font-bold text-foreground tracking-tight">{title}</h2>
          <p className="text-sm text-muted-foreground mt-1">{description}</p>
        </div>
      </div>

      {/* Step content */}
      <div className="min-h-[240px]">{children}</div>

      {/* Navigation */}
      <div className="flex items-center justify-between pt-6 border-t border-border/50">
        {!isFirst ? (
          <Button
            variant="ghost"
            size="lg"
            onClick={onBack}
            className="text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
        ) : (
          <div />
        )}
        <Button
          onClick={onNext}
          disabled={!isValid}
          size="lg"
          className={cn(
            "min-w-[140px]",
            isLast
              ? "shadow-[0_0_25px_rgba(255,107,53,0.25)] bg-brand hover:bg-brand-hover text-primary-foreground"
              : "bg-brand hover:bg-brand-hover text-primary-foreground"
          )}
        >
          {isLast ? (
            <>
              <Rocket className="mr-2 h-4 w-4" />
              Deploy
            </>
          ) : (
            <>
              Next
              <ArrowRight className="ml-2 h-4 w-4" />
            </>
          )}
        </Button>
      </div>
    </div>
  );
}
