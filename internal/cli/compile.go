package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/targets"
)

// newCompileCmd creates the "mozza compile" command.
func newCompileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compile",
		Short: "Compile the recipe to deployment artifacts",
		Long: "Compile the recipe into artifacts for a deployment target and write them to disk.\n\n" +
			"Compiling is offline and side-effect-free: no cluster is contacted, nothing is\n" +
			"applied, and no deployment state is recorded. Run `mozza targets` to see the\n" +
			"available targets.",
		Args: cobra.NoArgs,
		RunE: runCompile,
	}

	// No backquotes in flag usage: pflag reads a backquoted span as the flag's
	// value placeholder, which would print as `--target mozza targets`.
	cmd.Flags().String("target", targets.Default, `compile target (run "mozza targets" to list)`)
	cmd.Flags().StringP("output", "o", ".", "directory to write artifacts into")

	return cmd
}

// runCompile resolves the requested target and writes its artifacts.
func runCompile(cmd *cobra.Command, _ []string) error {
	target, _ := cmd.Flags().GetString("target")
	outDir, _ := cmd.Flags().GetString("output")

	// Resolve the target before doing any work, so a typo fails with the list
	// of valid targets rather than a bare "unknown target" from the registry.
	reg := targets.DefaultRegistry()
	if _, err := reg.Get(target); err != nil {
		return fmt.Errorf("runCompile: unknown target %q (available: %s)",
			target, strings.Join(targetNames(reg), ", "))
	}

	_, err := compileAndWrite(cmd, target, outDir)

	return err
}

// targetNames returns the registered target names in the order `mozza targets`
// lists them.
func targetNames(reg *compile.Registry) []string {
	list := targets.List(reg)

	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.Name)
	}

	return names
}
