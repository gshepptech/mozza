package cli

import (
	"context"
	"fmt"
	"log/slog"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

// statusTimeout is the maximum time allowed for status queries.
const statusTimeout = 15 * time.Second

// newStatusCmd creates the "mozza status" command.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [slice]",
		Short: "Show the application status",
		Long:  "Display the current deployment status. Use --target=kubernetes for K8s status.",
		RunE:  runStatus,
	}

	cmd.Flags().String("target", "local", "deployment target (kubernetes, local)")
	cmd.Flags().Bool("detail", false, "show detailed status")

	return cmd
}

// runStatus queries the deployment target for current status.
func runStatus(cmd *cobra.Command, args []string) error {
	target, _ := cmd.Flags().GetString("target")

	switch target {
	case "kubernetes":
		return runK8sStatus(cmd, args)
	default:
		return runLocalStatus(cmd)
	}
}

// runK8sStatus queries Kubernetes for application health.
func runK8sStatus(cmd *cobra.Command, args []string) error {
	recipePath := recipeFlagValue(cmd)
	detail, _ := cmd.Flags().GetBool("detail")

	p, err := loadPlan(recipePath)
	if err != nil {
		return fmt.Errorf("runK8sStatus: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("runK8sStatus: %w", err)
	}

	s, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("runK8sStatus: %w", err)
	}
	defer s.Close()

	deployer := k8sdeploy.New(s)

	ctx, cancel := context.WithTimeout(cmd.Context(), statusTimeout)
	defer cancel()

	status, err := deployer.Status(ctx, p.Name)
	if err != nil {
		return err
	}

	// Filter to specific slice if argument provided.
	if len(args) > 0 {
		sliceName := args[0]
		var filtered []deploy.SliceStatus
		for _, ss := range status.Slices {
			if ss.Name == sliceName {
				filtered = append(filtered, ss)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("slice %q not found", sliceName)
		}
		status.Slices = filtered
		detail = true
	}

	if detail {
		printDetailStatus(cmd, status)
	} else {
		printCompactStatus(cmd, status)
	}

	return nil
}

// printCompactStatus prints the compact status table.
func printCompactStatus(cmd *cobra.Command, status *deploy.AppStatus) {
	cmd.Printf("%s (namespace: %s, context: %s)\n", status.AppName, status.Namespace, status.Context)

	for _, ss := range status.Slices {
		icon := statusIcon(ss.Status)
		cmd.Printf("  %s  %s %s  %d/%d pods  restarts: %d\n",
			ss.Name, icon, ss.Status, ss.Ready, ss.Desired, ss.Restarts)
	}
}

// printDetailStatus prints detailed status for each slice.
func printDetailStatus(cmd *cobra.Command, status *deploy.AppStatus) {
	for _, ss := range status.Slices {
		cmd.Printf("%s (Deployment)\n", ss.Name)
		cmd.Printf("  Replicas: %d/%d ready\n", ss.Ready, ss.Desired)
		cmd.Printf("  Image: %s\n", ss.Image)
		cmd.Printf("  Status: %s\n", ss.Status)
		cmd.Printf("  Restarts: %d\n", ss.Restarts)
		cmd.Printf("  Age: %s\n", ss.Age.Round(time.Second))
		cmd.Println()
	}
}

// statusIcon returns a status indicator character.
func statusIcon(status string) string {
	switch status {
	case "running":
		return "ok"
	case "degraded":
		return "!!"
	case "down":
		return "XX"
	default:
		return ".."
	}
}

// runLocalStatus queries local Docker for container status.
func runLocalStatus(cmd *cobra.Command) error {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return fmt.Errorf("runLocalStatus: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()

	query := &local.StatusQuery{ProjectDir: "."}
	statuses, err := query.Run(ctx)
	if err != nil {
		return fmt.Errorf("runLocalStatus: %w", err)
	}

	printLocalStatusTable(cmd, p, statuses)
	return nil
}

// printLocalStatusTable writes a formatted table matching containers to plan slices.
func printLocalStatusTable(cmd *cobra.Command, p *plan.AppPlan, statuses []local.ContainerStatus) {
	statusMap := buildStatusMap(statuses)

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tIMAGE\tPORTS")

	for _, s := range p.Slices {
		cs, ok := statusMap[s.Name]
		if ok {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, cs.State, cs.Image, cs.Ports)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, "not running", s.Image, "-")
		}
	}

	if err := w.Flush(); err != nil {
		slog.Error("failed to flush status table", "error", err)
	}
}

// buildStatusMap creates a lookup from service name to container status.
func buildStatusMap(statuses []local.ContainerStatus) map[string]local.ContainerStatus {
	m := make(map[string]local.ContainerStatus, len(statuses))
	for _, s := range statuses {
		m[s.Service] = s
	}
	return m
}
