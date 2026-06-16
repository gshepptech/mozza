package cli

import (
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the recipe file",
		Long:  "Parse and validate the recipe file without deploying.",
		RunE:  runValidate,
	}
}

func runValidate(cmd *cobra.Command, _ []string) error {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return err
	}

	cmd.Printf("Recipe %q is valid: %d slice(s) defined.\n", p.Name, len(p.Slices))
	return nil
}
