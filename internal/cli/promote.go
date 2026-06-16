package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/state"
)

// newPromoteCmd creates the "mozza promote" command.
func newPromoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote a deployment to the next environment",
		Long:  "Promote the current deployment from one environment stage to the next.",
		RunE:  runPromote,
	}

	cmd.Flags().String("from", "staging", "source environment (dev, staging, production)")
	cmd.Flags().String("to", "production", "target environment (dev, staging, production)")

	return cmd
}

// runPromote promotes the latest deployment from the source to the target environment.
func runPromote(cmd *cobra.Command, _ []string) error {
	fromStr, _ := cmd.Flags().GetString("from")
	toStr, _ := cmd.Flags().GetString("to")

	from, err := state.ValidateEnvironment(fromStr)
	if err != nil {
		return fmt.Errorf("runPromote: %w", err)
	}

	to, err := state.ValidateEnvironment(toStr)
	if err != nil {
		return fmt.Errorf("runPromote: %w", err)
	}

	store := state.NewStore(".")

	record, err := store.Promote(from, to)
	if err != nil {
		return fmt.Errorf("runPromote: %w", err)
	}

	cmd.Printf("Promoted %q version %q from %s to %s (record %s).\n",
		record.AppName, record.Version, from, to, record.ID)

	return nil
}
