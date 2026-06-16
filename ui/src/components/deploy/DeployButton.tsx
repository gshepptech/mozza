import { useState } from "react";
import * as api from "../../api/client";
import type { Deployment } from "../../api/types";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import DeployLog from "./DeployLog";
import StatusBadge from "./StatusBadge";

interface DeployButtonProps {
  recipeId: string;
  teamId: string;
}

export default function DeployButton({ recipeId, teamId }: DeployButtonProps) {
  const [deploying, setDeploying] = useState(false);
  const [deployment, setDeployment] = useState<Deployment | null>(null);
  const [showLog, setShowLog] = useState(false);
  const [error, setError] = useState("");

  const handleDeploy = async () => {
    setError("");
    setDeploying(true);
    try {
      const d = await api.triggerDeploy(recipeId, teamId, "kubernetes", "production");
      setDeployment(d);
      setShowLog(true);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Deploy failed");
    } finally {
      setDeploying(false);
    }
  };

  return (
    <>
      <Button onClick={handleDeploy} disabled={deploying}>
        {deploying ? "Deploying..." : "Deploy"}
      </Button>

      {error && (
        <span className="ml-3 text-xs text-error">{error}</span>
      )}

      <Dialog open={showLog} onOpenChange={setShowLog}>
        <DialogContent className="max-w-xl">
          <DialogHeader>
            <DialogTitle>Deployment</DialogTitle>
          </DialogHeader>
          {deployment && (
            <div>
              <div className="mb-4 flex items-center gap-3">
                <StatusBadge status={deployment.status} />
                <span className="text-xs text-muted-foreground">
                  {deployment.environment} &middot; {deployment.target}
                </span>
              </div>
              <DeployLog
                deploymentId={deployment.id}
                onDone={(status) => setDeployment({ ...deployment, status })}
              />
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
