// Package compile provides the compiler interface and registry for Mozza
// deployment targets.
package compile

import (
	"context"

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
