package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/store"
)

// newRollbackCmd creates the "mozza rollback" command.
func newRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to a previous deployment",
		Long:  "Revert the application to the previously successful deployment by re-applying its recipe.",
		RunE:  runRollback,
	}
}

// runRollback retrieves the previous successful deploy's recipe and re-deploys it.
func runRollback(cmd *cobra.Command, _ []string) error {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return fmt.Errorf("runRollback: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("runRollback: %w", err)
	}

	s, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("runRollback: %w", err)
	}
	defer s.Close()

	deployer := k8sdeploy.New(s)

	cmd.Printf("Rolling back %q to previous successful deployment...\n", p.Name)

	if err := deployer.Rollback(cmd.Context(), p.Name); err != nil {
		return err
	}

	cmd.Printf("Rollback of %q complete.\n", p.Name)
	return nil
}
