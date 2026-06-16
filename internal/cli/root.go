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
	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
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

	root.PersistentFlags().String("log-level", "info", "logging level (debug, info, warn, error)")
	root.PersistentFlags().String("recipe", "app.mozza", "path to the recipe file")

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newUpCmd(),
		newDownCmd(),
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

// newRegistry creates a compiler registry with all built-in compilers.
func newRegistry() *compile.Registry {
	reg := compile.NewRegistry()
	reg.Register("local", local.New())
	reg.Register("kubernetes", k8s.New())

	return reg
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

		cmd.Printf("  wrote %s\n", target)
	}

	return nil
}

// recipeFlagValue returns the recipe path from the --recipe flag.
func recipeFlagValue(cmd *cobra.Command) string {
	val, _ := cmd.Flags().GetString("recipe")
	return val
}

// compileAndWrite runs the full pipeline: load plan, compile with the named
// target, and write output files. It returns the compiled plan for callers
// that need to record deployment state.
func compileAndWrite(cmd *cobra.Command, target string) (*plan.AppPlan, error) {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return nil, err
	}

	reg := newRegistry()

	compiler, err := reg.Get(target)
	if err != nil {
		return nil, fmt.Errorf("compileAndWrite: %w", err)
	}

	result, err := compiler.Compile(context.Background(), p)
	if err != nil {
		return nil, fmt.Errorf("compileAndWrite: %w", err)
	}

	if err := writeCompileResult(cmd, result, "."); err != nil {
		return nil, err
	}

	cmd.Println(result.Summary)

	return p, nil
}
