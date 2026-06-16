import { useNavigate } from "react-router-dom";
import { useDashboard } from "./DashboardPage";
import DeployWizard from "../components/deploy/DeployWizard";
import { EmptyState } from "@/components/custom/empty-state";
import { Rocket } from "lucide-react";

export default function DeployWizardPage() {
  const { activeTeam } = useDashboard();
  const navigate = useNavigate();

  if (!activeTeam) {
    return (
      <EmptyState
        icon={Rocket}
        title="No team selected"
        description="Create a team before deploying"
        action={{ label: "Create Team", onClick: () => navigate("/app/teams/new") }}
      />
    );
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-xl font-semibold text-foreground">Deploy</h1>
        <p className="text-sm text-muted-foreground mt-0.5">
          Place your order
        </p>
      </div>
      <DeployWizard
        teamId={activeTeam.id}
        onDeployComplete={() => navigate("/app/deployments")}
      />
    </div>
  );
}
