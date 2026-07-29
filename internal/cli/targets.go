package cli

import (
	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/targets"
)

// targetHeader is the name-column header, and the minimum width of that column.
const targetHeader = "TARGET"

// newTargetsCmd creates the "mozza targets" command.
func newTargetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "targets",
		Short: "List available compile targets",
		Long:  "List every deployment target accepted by `mozza compile --target`.",
		Args:  cobra.NoArgs,
		RunE:  runTargets,
	}
}

// runTargets prints the registered targets, marking the default.
func runTargets(cmd *cobra.Command, _ []string) error {
	list := targets.List(targets.DefaultRegistry())

	width := len(targetHeader)
	for _, t := range list {
		if len(t.Name) > width {
			width = len(t.Name)
		}
	}

	cmd.Printf("  %-*s  %s\n", width, targetHeader, "DESCRIPTION")

	for _, t := range list {
		marker := " "
		if t.Default {
			marker = "*"
		}

		cmd.Printf("%s %-*s  %s\n", marker, width, t.Name, t.Description)
	}

	cmd.Printf("\n* default target, used when --target is omitted\n")

	return nil
}
