package cli

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/store"
)

const composeDownTimeout = 5 * time.Minute

func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove the application",
		Long:  "Tear down all resources for the application. Use --target=kubernetes for K8s teardown.",
		RunE:  runDown,
	}

	cmd.Flags().String("target", "local", "deployment target (kubernetes, local)")
	cmd.Flags().Bool("delete-pvcs", false, "also delete persistent volume claims (irreversible)")

	return cmd
}

func runDown(cmd *cobra.Command, _ []string) error {
	target, _ := cmd.Flags().GetString("target")

	switch target {
	case "kubernetes":
		return runK8sDown(cmd)
	default:
		return runLocalDown(cmd)
	}
}

func runK8sDown(cmd *cobra.Command) error {
	recipePath := recipeFlagValue(cmd)
	deletePVCs, _ := cmd.Flags().GetBool("delete-pvcs")

	p, err := loadPlan(recipePath)
	if err != nil {
		return fmt.Errorf("runK8sDown: %w", err)
	}

	// Confirm PVC deletion if requested.
	if deletePVCs {
		cmd.Print("Delete persistent storage? This is irreversible. [y/N] ")
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer != "y" && answer != "yes" {
				cmd.Println("Aborted. PVCs will be preserved.")
				deletePVCs = false
			}
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("runK8sDown: %w", err)
	}

	s, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("runK8sDown: %w", err)
	}
	defer s.Close()

	deployer := k8sdeploy.New(s)

	cmd.Printf("Tearing down %q from Kubernetes...\n", p.Name)

	if err := deployer.Down(cmd.Context(), p.Name, deploy.DownOptions{
		DeletePVCs: deletePVCs,
	}); err != nil {
		return err
	}

	cmd.Printf("Teardown of %q complete.\n", p.Name)
	return nil
}

func runLocalDown(cmd *cobra.Command) error {
	runner, err := local.NewRunner(".")
	if err != nil {
		return fmt.Errorf("runLocalDown: %w", err)
	}

	cmd.Println("Stopping services...")

	ctx, cancel := context.WithTimeout(context.Background(), composeDownTimeout)
	defer cancel()

	if err := runner.Down(ctx, cmd.OutOrStdout()); err != nil {
		return fmt.Errorf("runLocalDown: %w", err)
	}

	cmd.Println("Services stopped successfully.")
	return nil
}
