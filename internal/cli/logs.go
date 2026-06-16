package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/store"
)

const logsTimeout = 30 * time.Second

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [slice]",
		Short: "View application logs",
		Long:  "Stream logs from the running application. Use --target=kubernetes for K8s pod logs.",
		RunE:  runLogs,
	}

	cmd.Flags().Int("tail", 0, "number of lines to show from the end of the logs (0 = all)")
	cmd.Flags().String("target", "local", "deployment target (kubernetes, local)")
	cmd.Flags().Bool("follow", true, "follow log output")
	cmd.Flags().Duration("since", 0, "show logs since this duration ago (e.g., 5m, 1h)")

	return cmd
}

func runLogs(cmd *cobra.Command, args []string) error {
	target, _ := cmd.Flags().GetString("target")

	switch target {
	case "kubernetes":
		return runK8sLogs(cmd, args)
	default:
		return runLocalLogs(cmd)
	}
}

func runK8sLogs(cmd *cobra.Command, args []string) error {
	recipePath := recipeFlagValue(cmd)
	follow, _ := cmd.Flags().GetBool("follow")
	since, _ := cmd.Flags().GetDuration("since")

	p, err := loadPlan(recipePath)
	if err != nil {
		return fmt.Errorf("runK8sLogs: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("runK8sLogs: %w", err)
	}

	s, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("runK8sLogs: %w", err)
	}
	defer s.Close()

	deployer := k8sdeploy.New(s)

	opts := deploy.LogOptions{
		Follow: follow,
		Since:  since,
	}
	if len(args) > 0 {
		opts.SliceName = args[0]
	}

	reader, err := deployer.Logs(cmd.Context(), p.Name, opts)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(cmd.OutOrStdout(), reader)
	return err
}

func runLocalLogs(cmd *cobra.Command) error {
	tail, _ := cmd.Flags().GetInt("tail")

	runner, err := local.NewRunner(".")
	if err != nil {
		return fmt.Errorf("runLocalLogs: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), logsTimeout)
	defer cancel()

	if err := runner.Logs(ctx, cmd.OutOrStdout(), tail); err != nil {
		return fmt.Errorf("runLocalLogs: %w", err)
	}

	return nil
}
