package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/gitdeploy"
	"github.com/gshepptech/mozza/internal/store"
)

func newConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <repo_url>",
		Short: "Connect a GitHub repository for push-to-deploy",
		Long: `Connect registers a GitHub repository for automated deployments.
When you push to the connected repository, Mozza automatically detects
the framework, builds a Docker image, and deploys it.

You must provide a GitHub Personal Access Token (PAT) via the --token flag
or the MOZZA_GITHUB_TOKEN environment variable. The token needs repo and
admin:repo_hook permissions.

After connecting, configure a webhook in your GitHub repository settings
pointing to your Mozza server's webhook URL with the returned secret.`,
		Args: cobra.ExactArgs(1),
		RunE: runConnect,
	}

	cmd.Flags().String("token", "", "GitHub personal access token (or set MOZZA_GITHUB_TOKEN)")
	cmd.Flags().String("db", "mozza.db", "database path")

	return cmd
}

func runConnect(cmd *cobra.Command, args []string) error {
	repoURL := args[0]
	token, _ := cmd.Flags().GetString("token")
	dbPath, _ := cmd.Flags().GetString("db")

	if token == "" {
		token = os.Getenv("MOZZA_GITHUB_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("GitHub token required: use --token or set MOZZA_GITHUB_TOKEN")
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	svc := gitdeploy.NewService(s)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := svc.Connect(ctx, gitdeploy.ConnectRequest{
		RepoURL: repoURL,
		Token:   token,
	})
	if err != nil {
		return fmt.Errorf("connecting repository: %w", err)
	}

	slog.Info("repository connected", "repo_url", result.Repo.RepoURL)

	cmd.Printf("Repository connected: %s\n", result.Repo.RepoURL)
	cmd.Printf("Webhook secret: %s\n", result.WebhookSecret)
	cmd.Printf("\nConfigure a GitHub webhook:\n")
	cmd.Printf("  URL: https://<your-mozza-server>/api/v1/webhooks/github\n")
	cmd.Printf("  Content type: application/json\n")
	cmd.Printf("  Secret: %s\n", result.WebhookSecret)
	cmd.Printf("  Events: Just the push event\n")

	return nil
}

func newDisconnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect all GitHub repositories",
		Long: `Disconnect removes all connected GitHub repositories from Mozza.
This stops automated deployments from being triggered by pushes.`,
		RunE: runDisconnect,
	}

	cmd.Flags().String("db", "mozza.db", "database path")

	return cmd
}

func runDisconnect(cmd *cobra.Command, _ []string) error {
	dbPath, _ := cmd.Flags().GetString("db")

	s, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repos, err := s.ListConnectedRepos(ctx)
	if err != nil {
		return fmt.Errorf("listing repositories: %w", err)
	}

	if len(repos) == 0 {
		cmd.Println("No connected repositories.")
		return nil
	}

	svc := gitdeploy.NewService(s)

	for _, repo := range repos {
		if err := svc.Disconnect(ctx, repo.ID); err != nil {
			cmd.PrintErrln(fmt.Sprintf("Failed to disconnect %s: %v", repo.RepoURL, err))
			continue
		}
		cmd.Printf("Disconnected: %s\n", repo.RepoURL)
	}

	return nil
}
