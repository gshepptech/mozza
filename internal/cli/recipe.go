package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/marketplace"
	"github.com/gshepptech/mozza/internal/template"
)

// newRecipeCmd creates the "mozza recipe" command group.
func newRecipeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "Browse and install recipes from the marketplace",
		Long:  "Search, inspect, install, and deploy recipes from the Mozza marketplace.",
	}

	cmd.AddCommand(
		newRecipeSearchCmd(),
		newRecipeInfoCmd(),
		newRecipeInstallCmd(),
		newRecipeDeployCmd(),
		newRecipeUpdateCmd(),
	)

	return cmd
}

func loadMarketplace() (*marketplace.Service, error) {
	catalog, err := template.Load()
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	return marketplace.New(catalog, nil), nil
}

// newRecipeSearchCmd creates "mozza recipe search <query>".
func newRecipeSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search recipes by name, category, or tags",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRecipeSearch,
	}

	cmd.Flags().String("category", "", "filter by category")
	cmd.Flags().StringSlice("tags", nil, "filter by tags (comma-separated)")

	return cmd
}

func runRecipeSearch(cmd *cobra.Command, args []string) error {
	svc, err := loadMarketplace()
	if err != nil {
		return err
	}

	var query string
	if len(args) > 0 {
		query = args[0]
	}

	category, _ := cmd.Flags().GetString("category")
	tags, _ := cmd.Flags().GetStringSlice("tags")

	result, err := svc.Search(context.Background(), marketplace.ListParams{
		Query:    query,
		Category: category,
		Tags:     tags,
		PerPage:  100,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(result.Recipes) == 0 {
		cmd.Println("No recipes found.")
		return nil
	}

	cmd.Printf("%-20s %-15s %-40s\n", "NAME", "CATEGORY", "DESCRIPTION")
	cmd.Printf("%-20s %-15s %-40s\n", "----", "--------", "-----------")
	for _, r := range result.Recipes {
		name := r.Template.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		desc := r.Template.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		cmd.Printf("%-20s %-15s %-40s\n", name, r.Template.Category, desc)
	}
	cmd.Printf("\n%d recipe(s) found.\n", result.Total)

	return nil
}

// newRecipeInfoCmd creates "mozza recipe info <name>".
func newRecipeInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show detailed recipe information",
		Args:  cobra.ExactArgs(1),
		RunE:  runRecipeInfo,
	}
}

func runRecipeInfo(cmd *cobra.Command, args []string) error {
	svc, err := loadMarketplace()
	if err != nil {
		return err
	}

	tmpl, err := svc.Get(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("recipe info: %w", err)
	}

	cmd.Printf("Name:        %s\n", tmpl.Name)
	cmd.Printf("ID:          %s\n", tmpl.ID)
	cmd.Printf("Category:    %s\n", tmpl.Category)
	cmd.Printf("Description: %s\n", tmpl.Description)
	if tmpl.Repo != "" {
		cmd.Printf("Repository:  %s\n", tmpl.Repo)
	}
	if tmpl.EstResources != "" {
		cmd.Printf("Resources:   %s\n", tmpl.EstResources)
	}
	if len(tmpl.Tags) > 0 {
		cmd.Printf("Tags:        %s\n", strings.Join(tmpl.Tags, ", "))
	}
	if tmpl.Official {
		cmd.Println("Official:    yes")
	}
	if len(tmpl.Variables) > 0 {
		cmd.Println("\nVariables:")
		for _, v := range tmpl.Variables {
			req := ""
			if v.Required {
				req = " (required)"
			}
			def := ""
			if v.Default != "" {
				def = fmt.Sprintf(" [default: %s]", v.Default)
			}
			cmd.Printf("  %-20s %s%s%s\n", v.Key, v.Description, def, req)
		}
	}

	return nil
}

// newRecipeInstallCmd creates "mozza recipe install <name>".
func newRecipeInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <name>",
		Short: "Download recipe .mozza file to current directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runRecipeInstall,
	}

	cmd.Flags().StringP("output", "o", "", "output filename (default: <name>.mozza)")

	return cmd
}

func runRecipeInstall(cmd *cobra.Command, args []string) error {
	svc, err := loadMarketplace()
	if err != nil {
		return err
	}

	name := args[0]
	source, err := svc.Install(context.Background(), name)
	if err != nil {
		return fmt.Errorf("recipe install: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "" {
		output = name + ".mozza"
	}

	if err := os.WriteFile(output, []byte(source), 0o644); err != nil {
		return fmt.Errorf("recipe install: write file: %w", err)
	}

	cmd.Printf("Installed %s to %s\n", name, output)
	return nil
}

// newRecipeDeployCmd creates "mozza recipe deploy <name>".
func newRecipeDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Download and deploy a recipe directly",
		Args:  cobra.ExactArgs(1),
		RunE:  runRecipeDeploy,
	}

	cmd.Flags().StringToString("var", nil, "variable overrides (key=value)")

	return cmd
}

func runRecipeDeploy(cmd *cobra.Command, args []string) error {
	svc, err := loadMarketplace()
	if err != nil {
		return err
	}

	name := args[0]
	source, err := svc.Install(context.Background(), name)
	if err != nil {
		return fmt.Errorf("recipe deploy: %w", err)
	}

	// Apply variable overrides.
	vars, _ := cmd.Flags().GetStringToString("var")
	for k, v := range vars {
		source = strings.ReplaceAll(source, "{{"+k+"}}", v)
	}

	// Write as temporary recipe and deploy.
	tmpFile := name + ".mozza"
	if err := os.WriteFile(tmpFile, []byte(source), 0o644); err != nil {
		return fmt.Errorf("recipe deploy: write file: %w", err)
	}

	cmd.Printf("Downloaded %s, deploying...\n", name)

	// Set the recipe flag so deploy uses this file.
	if err := cmd.Root().PersistentFlags().Set("recipe", tmpFile); err != nil {
		return fmt.Errorf("recipe deploy: set recipe flag: %w", err)
	}

	return runDeploy(cmd, nil)
}

// newRecipeUpdateCmd creates "mozza recipe update".
func newRecipeUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Force refresh the recipe index from GitHub",
		Args:  cobra.NoArgs,
		RunE:  runRecipeUpdate,
	}
}

func runRecipeUpdate(cmd *cobra.Command, _ []string) error {
	svc, err := loadMarketplace()
	if err != nil {
		return err
	}

	cmd.Println("Refreshing marketplace index...")
	if err := svc.Refresh(context.Background()); err != nil {
		cmd.Printf("Warning: remote index refresh failed: %v\n", err)
		cmd.Println("Using embedded catalog only.")
		return nil
	}

	cmd.Println("Marketplace index updated successfully.")
	return nil
}
