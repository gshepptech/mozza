package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/importer"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import a Docker Compose file as a Mozza recipe",
		Long: `Import reads a Docker Compose (v3.x) YAML file and converts it into
a Mozza recipe. By default it writes the recipe to app.mozza in the
current directory. Use --dry-run to preview the output without writing.`,
		Args: cobra.ExactArgs(1),
		RunE: runImport,
	}

	cmd.Flags().Bool("dry-run", false, "print the generated recipe to stdout without writing")
	cmd.Flags().StringP("output", "o", "app.mozza", "output file path for the generated recipe")

	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	composePath := args[0]
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	outPath, _ := cmd.Flags().GetString("output")

	content, err := os.ReadFile(composePath)
	if err != nil {
		return fmt.Errorf("reading compose file: %w", err)
	}

	slog.Info("importing compose file", "path", composePath)

	r, warnings, err := importer.ComposeToRecipeAST(content)
	if err != nil {
		return fmt.Errorf("converting compose file: %w", err)
	}

	// Also generate the text form for output.
	recipeText, err := importer.ComposeToRecipe(string(content))
	if err != nil {
		return fmt.Errorf("generating recipe text: %w", err)
	}

	// Print warnings.
	for _, w := range warnings {
		cmd.Printf("  [%s] %s: %s\n", w.Severity, w.Feature, w.Message)
	}

	cmd.Printf("\nDetected %d service(s) in %s\n", len(r.Slices), filepath.Base(composePath))

	if dryRun {
		cmd.Println("\n--- Generated Recipe ---")
		cmd.Print(recipeText)
		cmd.Println("--- End Recipe ---")
		return nil
	}

	// Preview the recipe.
	cmd.Println("\n--- Generated Recipe ---")
	cmd.Print(recipeText)
	cmd.Println("--- End Recipe ---")

	absOut, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolving output path: %w", err)
	}

	if err := os.WriteFile(absOut, []byte(recipeText), 0o644); err != nil {
		return fmt.Errorf("writing recipe: %w", err)
	}

	cmd.Printf("\nRecipe written to %s\n", absOut)
	slog.Info("recipe written", "path", absOut, "slices", len(r.Slices))

	return nil
}
