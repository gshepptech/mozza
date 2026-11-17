// Package compile provides the compiler interface and registry for Mozza
// deployment targets.
package compile

import (
	"context"
	"fmt"

	"github.com/gshepptech/mozza/internal/plan"
)

// Compiler transforms an AppPlan into deployment artifacts for a specific target.
type Compiler interface {
	// Compile generates deployment artifacts from the given application plan.
	Compile(ctx context.Context, p *plan.AppPlan) (*Result, error)
	// Name returns the human-readable name of the compiler target.
	Name() string
}

// Result holds the output of a compilation.
type Result struct {
	// Files contains the generated deployment artifacts.
	Files []OutputFile
	// Summary is a human-readable description of what was generated.
	Summary string
	// Warnings lists non-fatal issues encountered during compilation,
	// such as features that are not supported by the target platform.
	Warnings []string
}

// OutputFile represents a single generated file.
type OutputFile struct {
	// Path is the relative file path for the output.
	Path string
	// Content holds the file content as bytes.
	Content []byte
}

// FormatWarnings renders a warning block as one string per output line: a count
// line followed by one indented entry per warning. It returns nil when there is
// nothing to report, so silence keeps meaning nothing was lost.
//
// Every surface that reports capability gaps renders them through here: the CLI
// on stderr, the dashboard and operator deployment logs. The wording is a
// user-facing contract that must not drift between them, so it lives in one
// place rather than being reimplemented per surface. A caller that has already
// rendered a result's warnings must not render them again.
func FormatWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}

	lines := make([]string, 0, len(warnings)+1)
	lines = append(lines, fmt.Sprintf("%d warning(s):", len(warnings)))
	for _, w := range warnings {
		lines = append(lines, "  ! "+w)
	}
	return lines
}
