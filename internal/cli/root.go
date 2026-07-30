// Package cli provides the Cobra command tree for the Mozza CLI.
// The entry point is New, which returns the fully assembled root command
// with all subcommands registered.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/config"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/targets"
)

// New creates the root Cobra command with all subcommands registered.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:   "mozza",
		Short: "Deploy apps like you order pizza",
		Long:  "Mozza turns a simple recipe file into production-ready deployments.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return setupConfig(cmd)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Cobra's Print* helpers resolve through OutOrStderr, which walks the
	// command tree for a writer set by SetOut and only falls back to os.Stderr
	// when it finds none. Nothing set one, so every Print* in the tree landed on
	// stderr and stdout was always empty — `mozza targets | grep local` matched
	// nothing and `mozza compile > out.txt` wrote an empty file. Setting the
	// writer once on the root reaches every subcommand through that same walk.
	//
	// Progress and diagnostics deliberately stay on stderr via PrintErr*: for a
	// command whose payload is files on disk, the chatter is not the output. See
	// writeCompileResult and printCompileWarnings.
	root.SetOut(os.Stdout)

	root.PersistentFlags().String("log-level", "info", "logging level (debug, info, warn, error)")
	root.PersistentFlags().String("recipe", "app.mozza", "path to the recipe file")

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newUpCmd(),
		newDownCmd(),
		newCompileCmd(),
		newTargetsCmd(),
		newDeployCmd(),
		newStatusCmd(),
		newDoctorCmd(),
		newLogsCmd(),
		newValidateCmd(),
		newRollbackCmd(),
		newPromoteCmd(),
		newServeCmd(),
		newOperatorCmd(),
		newImportCmd(),
		newRecipeCmd(),
		newConnectCmd(),
		newDisconnectCmd(),
		newProxyCmd(),
		newPreviewsCmd(),
	)

	return root
}

// setupConfig loads the application configuration and sets the slog default
// logger level based on the --log-level flag.
func setupConfig(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("setupConfig: %w", err)
	}

	level, _ := cmd.Flags().GetString("log-level")
	if level == "" {
		level = cfg.LogLevel
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		return fmt.Errorf("setupConfig: invalid log level %q: %w", level, err)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	return nil
}

// loadPlan reads the recipe file, parses it, builds and validates the plan.
func loadPlan(recipePath string) (*plan.AppPlan, error) {
	content, err := os.ReadFile(recipePath)
	if err != nil {
		return nil, fmt.Errorf("loadPlan: %w", err)
	}

	r, err := recipe.NewParser(string(content)).Parse()
	if err != nil {
		return nil, fmt.Errorf("loadPlan: %w", err)
	}

	p, err := plan.Build(r)
	if err != nil {
		return nil, fmt.Errorf("loadPlan: %w", err)
	}

	if err := plan.Validate(p); err != nil {
		return nil, fmt.Errorf("loadPlan: validation failed: %w", err)
	}

	return p, nil
}

// writeCompileResult writes all output files from a compile result to disk
// relative to the given output directory.
func writeCompileResult(cmd *cobra.Command, result *compile.Result, outDir string) error {
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("writeCompileResult: %w", err)
	}

	for _, f := range result.Files {
		target := filepath.Join(outDir, f.Path)

		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("writeCompileResult: %w", err)
		}

		if !strings.HasPrefix(absTarget, absOut+string(filepath.Separator)) {
			return fmt.Errorf("writeCompileResult: path traversal detected: %s", f.Path)
		}

		dir := filepath.Dir(target)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("writeCompileResult: %w", err)
		}

		if err := os.WriteFile(target, f.Content, 0o644); err != nil {
			return fmt.Errorf("writeCompileResult: %w", err)
		}

		// Progress, not payload: the artifacts are the files themselves, so this
		// goes to stderr and leaves stdout clean for redirection.
		cmd.PrintErrf("  wrote %s\n", target)
	}

	return nil
}

// recipeFlagValue returns the recipe path from the --recipe flag.
func recipeFlagValue(cmd *cobra.Command) string {
	val, _ := cmd.Flags().GetString("recipe")
	return val
}

// compileAndWrite runs the full pipeline: load plan, compile with the named
// target, and write output files into outDir. It returns the compiled plan for
// callers that need to record deployment state.
func compileAndWrite(cmd *cobra.Command, target, outDir string) (*plan.AppPlan, error) {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return nil, err
	}

	compiler, err := targets.DefaultRegistry().Get(target)
	if err != nil {
		return nil, fmt.Errorf("compileAndWrite: %w", err)
	}

	result, err := compiler.Compile(context.Background(), p)
	if err != nil {
		return nil, fmt.Errorf("compileAndWrite: %w", err)
	}

	if err := writeCompileResult(cmd, result, outDir); err != nil {
		return nil, err
	}

	cmd.PrintErrln(result.Summary)
	printCompileWarnings(cmd, result)

	return p, nil
}

// printCompileWarnings reports a target's capability gaps. Warnings exist to
// tell the user what the target could not express, so every caller of
// compileAndWrite surfaces them rather than each deciding for itself.
//
// This is the CLI's single renderer for compile diagnostics, on stderr so a
// redirected stdout still leaves gaps visible to a human. The block itself is
// built by compile.FormatWarnings, shared with the dashboard and operator deploy
// logs so the same recipe reads the same way on every surface; only the leading
// blank line is the terminal's own, for spacing.
func printCompileWarnings(cmd *cobra.Command, result *compile.Result) {
	lines := compile.FormatWarnings(result.Warnings)
	if len(lines) == 0 {
		return
	}

	cmd.PrintErrln()
	for _, line := range lines {
		cmd.PrintErrln(line)
	}
}
