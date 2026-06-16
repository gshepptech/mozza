import { useState, useEffect, useCallback } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { MozzaLogoLarge } from "@/components/custom/MozzaLogo";

const STORAGE_KEY = "mozza-onboarded";

interface OnboardingOverlayProps {
  onDismiss: () => void;
}

export default function OnboardingOverlay({ onDismiss }: OnboardingOverlayProps) {
  const [visible, setVisible] = useState(() => !localStorage.getItem(STORAGE_KEY));

  const dismiss = useCallback(() => {
    localStorage.setItem(STORAGE_KEY, "1");
    setVisible(false);
    onDismiss();
  }, [onDismiss]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        dismiss();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [dismiss]);

  if (!visible) {
    return null;
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-label="Welcome to Mozza"
    >
      <Card className="w-full max-w-md mx-4">
        <CardContent className="pt-2 text-center space-y-6">
          <div aria-hidden="true">
            <MozzaLogoLarge className="text-brand mx-auto" size={64} />
          </div>

          <h2 className="text-2xl font-bold text-foreground">
            Welcome to Mozza!
          </h2>

          <p className="text-muted-foreground">
            Deploy your app in 3 simple steps:
          </p>

          <ol className="text-left space-y-3 text-sm">
            <li className="flex gap-3 items-start">
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-brand text-primary-foreground text-xs font-bold">
                1
              </span>
              <span className="text-foreground">
                <strong>Pick</strong>
                <span className="text-muted-foreground">
                  {" "}&mdash; Choose from your menu, templates, or GitHub
                </span>
              </span>
            </li>
            <li className="flex gap-3 items-start">
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-brand text-primary-foreground text-xs font-bold">
                2
              </span>
              <span className="text-foreground">
                <strong>Customize</strong>
                <span className="text-muted-foreground">
                  {" "}&mdash; Tell us what kind of app it is
                </span>
              </span>
            </li>
            <li className="flex gap-3 items-start">
              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-brand text-primary-foreground text-xs font-bold">
                3
              </span>
              <span className="text-foreground">
                <strong>Order</strong>
                <span className="text-muted-foreground">
                  {" "}&mdash; Review and deploy
                </span>
              </span>
            </li>
          </ol>

          <div className="flex flex-col gap-2 pt-2">
            <Button
              className="w-full bg-brand hover:bg-brand-hover text-primary-foreground"
              size="lg"
              onClick={dismiss}
            >
              Get started
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={dismiss}
            >
              I've done this before
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
