package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/store"
)

// newDeployCmd creates the "mozza deploy" command.
func newDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy the application to Kubernetes",
		Long:  "Parse the recipe, compile to K8s manifests, and apply via server-side apply.",
		RunE:  runDeploy,
	}

	cmd.Flags().String("context", "", "kubeconfig context override")
	cmd.Flags().Duration("timeout", 5*time.Minute, "readiness wait timeout")
	cmd.Flags().Bool("export", false, "write manifests to disk without applying")

	return cmd
}

// runDeploy loads the recipe, pre-validates, and deploys to Kubernetes.
func runDeploy(cmd *cobra.Command, _ []string) error {
	recipePath := recipeFlagValue(cmd)
	k8sContext, _ := cmd.Flags().GetString("context")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	export, _ := cmd.Flags().GetBool("export")

	// Load and validate the plan.
	p, err := loadPlan(recipePath)
	if err != nil {
		return err
	}

	// Export mode: write manifests and exit.
	if export {
		return exportManifests(cmd, p)
	}

	// Read raw recipe content for audit trail.
	recipeContent, err := os.ReadFile(recipePath)
	if err != nil {
		return fmt.Errorf("runDeploy: read recipe: %w", err)
	}

	// Pre-validate images.
	cmd.Println("Validating images...")
	if err := k8sdeploy.ValidateImages(cmd.Context(), p); err != nil {
		return err
	}

	// Open the state store.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("runDeploy: %w", err)
	}

	s, err := store.Open(cfg.Database.Path)
	if err != nil {
		return fmt.Errorf("runDeploy: %w", err)
	}
	defer s.Close()

	// Deploy with progress reporting and confirmation prompts.
	deployer := k8sdeploy.New(s).
		WithProgress(func(phase string, current, total int, message string) {
			if total > 0 {
				cmd.Printf("[%d/%d] %s\n", current, total, message)
			} else {
				cmd.Println(message)
			}
		}).
		WithConfirm(func(prompt string) bool {
			cmd.Printf("%s [y/N] ", prompt)
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if scanner.Scan() {
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				return answer == "y" || answer == "yes"
			}
			return false
		})

	result, err := deployer.Deploy(cmd.Context(), p, deploy.DeployOptions{
		Timeout:       timeout,
		Context:       k8sContext,
		RecipeContent: string(recipeContent),
		TriggeredBy:   "cli",
	})
	if err != nil {
		if result != nil {
			cmd.Printf("Deploy %s (%s) in %s\n", result.Status, result.DeployID, result.Duration.Round(time.Second))
		}
		cmd.PrintErrln(k8sdeploy.HumanError(err, p.Name))
		return err
	}

	cmd.Printf("Deploy complete (%s) in %s\n", result.DeployID, result.Duration.Round(time.Second))
	for _, r := range result.Resources {
		cmd.Printf("  %s/%s: %s\n", r.Kind, r.Name, r.Status)
	}

	return nil
}

// exportManifests compiles and writes K8s manifests to disk without applying.
func exportManifests(cmd *cobra.Command, p *plan.AppPlan) error {
	_, err := compileAndWrite(cmd, "kubernetes")
	return err
}
