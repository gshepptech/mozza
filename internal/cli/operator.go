package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeployer "github.com/gshepptech/mozza/internal/deploy/k8s"
	localdeployer "github.com/gshepptech/mozza/internal/deploy/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

// newOperatorCmd creates the "mozza operator" command.
func newOperatorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Run the deployment operator",
		Long:  "Watch for pending deployments and execute them automatically.",
		RunE:  runOperator,
	}

	cmd.Flags().String("db", "", "SQLite database path (default: mozza.db)")
	cmd.Flags().String("db-url", "", "Database URL (postgres://... or file path for SQLite)")
	cmd.Flags().Duration("poll-interval", 5*time.Second, "How often to check for pending deployments")

	return cmd
}

// runOperator polls the database for pending deployments and executes them.
func runOperator(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")
	dbURL, _ := cmd.Flags().GetString("db-url")
	pollInterval, _ := cmd.Flags().GetDuration("poll-interval")

	// Resolve database DSN using shared logic.
	dsn := resolveDSN(dbURL, dbPath)

	st, err := store.Open(dsn)
	if err != nil {
		return fmt.Errorf("runOperator: %w", err)
	}
	defer st.Close()

	recipePath := recipeFlagValue(cmd)
	projectDir := filepath.Dir(recipePath)

	// Create deployers (same as serve.go).
	localDeploy := localdeployer.New(st, projectDir)
	k8sDeploy := k8sdeployer.New(st)

	// Set up graceful shutdown on SIGINT/SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	slog.Info("operator started", "db", dsn, "poll_interval", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("operator shutting down")
			return nil
		case <-ticker.C:
			if err := pollPendingDeployments(ctx, st, k8sDeploy, localDeploy); err != nil {
				slog.Error("poll cycle failed", "err", err)
			}
		}
	}
}

// pollPendingDeployments checks for pending deployments and executes the oldest one.
func pollPendingDeployments(
	ctx context.Context,
	st *store.Store,
	k8sDeploy deploy.Deployer,
	localDeploy deploy.Deployer,
) error {
	pending, err := st.PendingDeployments()
	if err != nil {
		return fmt.Errorf("pollPendingDeployments: %w", err)
	}

	for _, dep := range pending {
		if ctx.Err() != nil {
			return nil
		}

		if err := st.ClaimDeployment(dep.ID); err != nil {
			slog.Warn("failed to claim deployment", "id", dep.ID, "err", err)
			continue
		}

		slog.Info("claimed deployment", "id", dep.ID, "target", dep.Target, "env", dep.Environment)

		rec, err := st.RecipeByID(dep.RecipeID)
		if err != nil {
			failDeployment(st, dep.ID, fmt.Sprintf("recipe lookup failed: %v", err))
			continue
		}

		deployer := k8sDeploy
		if dep.Target == "local" {
			deployer = localDeploy
		}

		executeDeployment(ctx, st, deployer, dep.ID, rec.Source, dep.Target, dep.Environment)
	}

	return nil
}

// executeDeployment runs the full deploy pipeline for a single deployment.
func executeDeployment(
	ctx context.Context,
	st *store.Store,
	deployer deploy.Deployer,
	deployID, source, target, env string,
) {
	logMsg := func(msg string) {
		if err := st.AppendDeploymentLog(deployID, msg+"\n"); err != nil {
			slog.Error("failed to append deploy log", "deploy_id", deployID, "err", err)
		}
	}
	fail := func(msg string) {
		logMsg("ERROR: " + msg)
		if err := st.UpdateDeploymentStatus(deployID, "failed", "", true); err != nil {
			slog.Error("failed to update deploy status", "deploy_id", deployID, "err", err)
		}
	}

	logMsg("Parsing recipe...")
	parser := recipe.NewParser(source)
	rec, err := parser.Parse()
	if err != nil {
		fail(fmt.Sprintf("recipe parse failed: %v", err))
		return
	}

	logMsg("Building plan...")
	p, err := plan.Build(rec)
	if err != nil {
		fail(fmt.Sprintf("plan build failed: %v", err))
		return
	}

	logMsg("Validating plan...")
	if err := plan.Validate(p); err != nil {
		fail(fmt.Sprintf("plan validation failed: %v", err))
		return
	}

	logMsg(fmt.Sprintf("Deploying to %s (%s)...", target, env))
	opts := deploy.DeployOptions{
		RecipeContent: source,
		TriggeredBy:   "operator",
	}

	result, err := deployer.Deploy(ctx, p, opts)
	if err != nil {
		fail(fmt.Sprintf("deploy failed: %v", err))
		return
	}

	status := deployStatusString(result.Status)
	logMsg(fmt.Sprintf("Deploy completed in %s — status: %s", result.Duration, status))
	if err := st.UpdateDeploymentStatus(deployID, status, "", true); err != nil {
		slog.Error("failed to update deploy status", "deploy_id", deployID, "err", err)
	}
	slog.Info("deployment completed", "id", deployID, "status", status)
}

// deployStatusString converts a deploy status to a string for storage.
func deployStatusString(s deploy.DeployStatus) string {
	switch s { //nolint:exhaustive // only failure states need special handling
	case deploy.StatusFailed:
		return "failed"
	case deploy.StatusRolledBack:
		return "rolled_back"
	default:
		return "succeeded"
	}
}

// failDeployment marks a deployment as failed with an error message.
func failDeployment(st *store.Store, deployID, msg string) {
	slog.Error("deployment failed", "id", deployID, "reason", msg)
	_ = st.AppendDeploymentLog(deployID, "ERROR: "+msg+"\n")
	if err := st.UpdateDeploymentStatus(deployID, "failed", "", true); err != nil {
		slog.Error("failed to update deploy status", "deploy_id", deployID, "err", err)
	}
}
