package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/version"
)

// newVersionCmd creates the "mozza version" command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Mozza version",
		Long:  "Display the Mozza build version, git commit, and build date.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println(formatVersion())
			return nil
		},
	}
}

// formatVersion returns the formatted version string.
func formatVersion() string {
	return fmt.Sprintf("mozza v%s\ncommit: %s\nbuilt:  %s\ngo:     %s",
		version.Version, version.Commit, version.Date, version.GoVersion())
}
