package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/doctor/rules"
)

// doctorTimeout is the maximum time allowed for the doctor engine to collect
// signals and evaluate rules.
const doctorTimeout = 30 * time.Second

// newDoctorCmd creates the "mozza doctor" command.
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health checks on the application environment",
		Long:  "Diagnose problems with the runtime environment by running all registered rules.\nUse --fix to automatically apply safe remediations (health checks, resource limits).",
		RunE:  runDoctor,
	}

	cmd.Flags().Bool("fix", false, "automatically apply safe fixes (health checks, resource limits)")

	return cmd
}

// runDoctor loads the recipe, builds a plan, and runs the doctor engine.
func runDoctor(cmd *cobra.Command, _ []string) error {
	recipePath := recipeFlagValue(cmd)

	p, err := loadPlan(recipePath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()

	collector := doctor.NewCollector(slog.Default())
	engine := doctor.New(collector, rules.Default()...)

	report, err := engine.Run(ctx, p)
	if err != nil {
		return fmt.Errorf("runDoctor: %w", err)
	}

	fix, _ := cmd.Flags().GetBool("fix")
	if fix {
		results := doctor.AutoFix(p, report.Findings)
		if len(results) > 0 {
			cmd.Println("Auto-fixes applied:")
			for _, r := range results {
				cmd.Printf("  - [%s] %s: %s\n", r.Rule, r.SliceName, r.Description)
			}
			cmd.Println()

			// Re-run the engine to show the updated report.
			report, err = engine.Run(ctx, p)
			if err != nil {
				return fmt.Errorf("runDoctor: re-check after fix: %w", err)
			}
		} else {
			cmd.Println("No auto-fixable issues found.")
			cmd.Println()
		}
	}

	cmd.Print(doctor.FormatText(report))

	return nil
}
