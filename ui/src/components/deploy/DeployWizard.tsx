import { useState, useCallback } from "react";
import * as api from "../../api/client";
import type { Deployment } from "../../api/types";
import type { DeployPreviewResponse } from "../../api/client";
import DeployProgress from "./DeployProgress";
import ReviewOrder from "./ReviewOrder";
import StatusBadge from "./StatusBadge";
import { OrderWizard } from "./OrderWizard";
import OnboardingOverlay from "./OnboardingOverlay";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Rocket, ArrowRight, Settings } from "lucide-react";
import { useNavigate } from "react-router-dom";

interface DeployWizardProps {
  teamId: string;
  onDeployComplete?: (deployment: Deployment) => void;
}

export default function DeployWizard({ teamId, onDeployComplete }: DeployWizardProps) {
  const navigate = useNavigate();
  const [showOnboarding, setShowOnboarding] = useState(() => !localStorage.getItem("mozza-onboarded"));
  const [, setDeploying] = useState(false);
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [recipeName, setRecipeName] = useState("");
  const [recipeSource, setRecipeSource] = useState("");
  const [preview, setPreview] = useState<DeployPreviewResponse | null>(null);
  const [deployStatus, setDeployStatus] = useState<"idle" | "reviewing" | "cooking" | "served" | "burned">("idle");
  const [, setDeployError] = useState("");

  const handleWizardComplete = useCallback(
    async (name: string, source: string) => {
      setRecipeName(name);
      setRecipeSource(source);
      try {
        const p = await api.previewDeploy(source);
        setPreview(p);
        setDeployStatus("reviewing");
      } catch {
        // If preview fails, skip review and deploy directly
        executeDeploy(source);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [teamId],
  );

  const executeDeploy = useCallback(
    async (source: string) => {
      setDeploying(true);
      setDeployStatus("cooking");
      try {
        const d = await api.deployImport({
          team_id: teamId,
          target: "local",
          namespace: "development",
          source,
        });
        setDeployment(d);
      } catch (err: unknown) {
        const raw = err instanceof Error ? err.message : "Unknown error";
        const msg = raw.includes("recipe parse failed") ? "Recipe has syntax errors — check your recipe and try again."
          : raw.includes("plan build failed") ? "Could not build a deployment plan — check service dependencies."
          : raw.includes("plan validation failed") ? "Plan validation failed — check service configuration."
          : raw.includes("not a team member") ? "You don't have access to this team."
          : raw.includes("deploy failed") ? "Deployment failed — check the logs for details."
          : `Something went wrong: ${raw}`;
        setDeployError(msg);
        setDeployStatus("burned");
      } finally {
        setDeploying(false);
      }
    },
    [teamId],
  );

  const handleConfirmDeploy = useCallback(() => {
    executeDeploy(recipeSource);
  }, [executeDeploy, recipeSource]);

  const handleBackToWizard = useCallback(() => {
    setPreview(null);
    setDeployStatus("idle");
  }, []);

  const handleDeployDone = useCallback((finalStatus: string) => {
    if (finalStatus === "succeeded" || finalStatus === "success") {
      setDeployStatus("served");
    } else {
      setDeployStatus("burned");
    }
  }, []);

  const handleEditOrder = useCallback(() => {
    // Return to wizard with state preserved (L-6 fix)
    setDeployment(null);
    setDeployStatus("idle");
  }, []);

  const resetWizard = useCallback(() => {
    setDeployment(null);
    setDeployStatus("idle");
    setRecipeSource("");
    setPreview(null);
  }, []);

  const handleAdvancedMode = useCallback(async () => {
    try {
      const recipe = await api.createRecipe(teamId, "Untitled Recipe", "", "{}");
      navigate(`/app/recipes/${recipe.id}`);
    } catch {
      navigate("/app/recipes");
    }
  }, [navigate, teamId]);

  // Review step — show preview before deploying
  if (deployStatus === "reviewing" && preview) {
    return (
      <div className="mx-auto max-w-2xl page-enter">
        <ReviewOrder
          preview={preview}
          recipeName={recipeName}
          recipeSource={recipeSource}
          onConfirm={handleConfirmDeploy}
          onBack={handleBackToWizard}
        />
      </div>
    );
  }

  // Post-deploy progress view
  if (deployment && deployStatus !== "idle") {
    return (
      <div className="mx-auto max-w-2xl page-enter">
        <div className="space-y-8">
          <div className="flex items-center justify-center gap-3">
            <StatusBadge status={deployment.status} />
            <Badge variant="outline" className="text-xs">{deployment.target}</Badge>
            <Badge variant="outline" className="text-xs">{deployment.environment}</Badge>
            {deployment.order_number && (
              <Badge variant="outline" className="text-xs font-mono">#{deployment.order_number}</Badge>
            )}
          </div>

          <DeployProgress
            deploymentId={deployment.id}
            onComplete={handleDeployDone}
            usePizzaMetaphor
            orderNumber={deployment.order_number}
          />

          {deployStatus === "served" && deployment?.access_url && (
            <div className="text-center space-y-1 mb-4">
              <p className="text-sm text-muted-foreground">Your app is live at:</p>
              <a
                href={deployment.access_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-brand hover:underline text-sm font-medium"
              >
                {deployment.access_url}
              </a>
            </div>
          )}

          {deployStatus !== "cooking" && (
            <div className="flex items-center justify-center gap-3">
              {deployStatus === "burned" && (
                <>
                  <Button
                    className="bg-brand hover:bg-brand-hover text-primary-foreground"
                    onClick={() => {
                      setDeployment(null);
                      setDeployError("");
                      executeDeploy(recipeSource);
                    }}
                  >
                    <Rocket className="mr-2 h-4 w-4" />
                    Try Again
                  </Button>
                  <Button
                    variant="outline"
                    onClick={handleEditOrder}
                  >
                    <Settings className="mr-2 h-4 w-4" />
                    Edit Order
                  </Button>
                </>
              )}
              <Button
                variant="outline"
                onClick={() => onDeployComplete?.(deployment)}
              >
                View Deployments
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
              {deployStatus === "served" && (
                <Button
                  className="bg-brand hover:bg-brand-hover text-primary-foreground"
                  onClick={resetWizard}
                >
                  <Rocket className="mr-2 h-4 w-4" />
                  Place Another Order
                </Button>
              )}
            </div>
          )}
        </div>
      </div>
    );
  }

  // OrderWizard is the single entry point — no SourceSelector gate
  return (
    <div className="mx-auto max-w-3xl page-enter">
      {showOnboarding && (
        <OnboardingOverlay onDismiss={() => setShowOnboarding(false)} />
      )}
      <OrderWizard
        onComplete={handleWizardComplete}
        onAdvancedMode={handleAdvancedMode}
        teamId={teamId}
      />
    </div>
  );
}
