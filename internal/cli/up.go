package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/state"
)

// composeUpTimeout is the maximum time allowed for docker compose up to complete.
const composeUpTimeout = 5 * time.Minute

// newUpCmd creates the "mozza up" command.
func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the application locally",
		Long:  "Parse the recipe, generate a docker-compose.yml, and run docker compose up.",
		RunE:  runUp,
	}
}

// runUp generates a docker-compose.yml from the recipe and starts services
// with docker compose up -d.
func runUp(cmd *cobra.Command, _ []string) error {
	p, err := compileAndWrite(cmd, "local")
	if err != nil {
		return err
	}

	if err := composeUp(cmd); err != nil {
		return err
	}

	store := state.NewStore(".")
	if err := store.Record(state.DeployRecord{
		AppName:     p.Name,
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "latest",
		Status:      "deployed",
	}); err != nil {
		return fmt.Errorf("runUp: %w", err)
	}

	return nil
}

// composeUp detects the docker compose binary and runs "up -d", streaming
// output to the command's stdout.
func composeUp(cmd *cobra.Command) error {
	runner, err := local.NewRunner(".")
	if err != nil {
		return fmt.Errorf("composeUp: %w", err)
	}

	cmd.Println("Starting services...")

	ctx, cancel := context.WithTimeout(context.Background(), composeUpTimeout)
	defer cancel()

	if err := runner.Up(ctx, cmd.OutOrStdout()); err != nil {
		return fmt.Errorf("composeUp: %w", err)
	}

	cmd.Println("Services started successfully.")
	return nil
}
