package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/store"
)

func newPreviewsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "previews",
		Short: "Manage branch preview deployments",
		Long:  "List, inspect, and clean up branch preview deployments.",
	}

	cmd.AddCommand(
		newPreviewsListCmd(),
		newPreviewsCleanCmd(),
	)

	return cmd
}

func newPreviewsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show active preview deployments",
		RunE:  runPreviewsList,
	}

	cmd.Flags().String("db", "mozza.db", "database path")

	return cmd
}

func runPreviewsList(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")

	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deploys, err := s.ListPreviewDeploys(ctx)
	if err != nil {
		return fmt.Errorf("listing previews: %w", err)
	}

	if len(deploys) == 0 {
		cmd.Println("No active preview deployments.")
		return nil
	}

	cmd.Printf("%-6s %-8s %-24s %-40s %s\n", "ID", "APP", "BRANCH", "DOMAIN", "LAST ACTIVITY")
	for _, d := range deploys {
		lastActivity := "never"
		if d.LastActivity != nil {
			lastActivity = d.LastActivity.Format("2006-01-02 15:04")
		}
		cmd.Printf("%-6d %-8d %-24s %-40s %s\n",
			d.ID, d.AppID, d.Branch, d.Domain, lastActivity,
		)
	}

	return nil
}

func newPreviewsCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove stale preview deployments",
		Long:  "Remove preview deployments with no activity in the last 7 days.",
		RunE:  runPreviewsClean,
	}

	cmd.Flags().String("db", "mozza.db", "database path")

	return cmd
}

func runPreviewsClean(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")

	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	n, err := s.CleanStalePreviewDeploys(ctx, cutoff)
	if err != nil {
		return fmt.Errorf("cleaning previews: %w", err)
	}

	if n == 0 {
		cmd.Println("No stale previews to clean.")
	} else {
		cmd.Printf("Cleaned %d stale preview(s).\n", n)
	}

	return nil
}
