// Package local provides a compile.Compiler that generates Docker Compose
// files for running a Mozza application on a local development machine.
package local

import (
	"context"
	"fmt"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/plan"
)

// composeFilePath is the output file name for the generated Docker Compose file.
const composeFilePath = "docker-compose.yml"

// Compiler generates Docker Compose deployment artifacts from an AppPlan.
// It implements the compile.Compiler interface.
type Compiler struct{}

// New creates a local Compiler.
func New() *Compiler {
	return &Compiler{}
}

// Name returns the human-readable name of this compiler target.
func (c *Compiler) Name() string {
	return "local"
}

// Compile generates a docker-compose.yml file from the given application plan.
// It returns a compile.Result containing a single OutputFile.
func (c *Compiler) Compile(_ context.Context, p *plan.AppPlan) (*compile.Result, error) {
	if p == nil {
		return nil, fmt.Errorf("Compile: app plan must not be nil")
	}

	br, err := BuildComposeFileWithWarnings(p)
	if err != nil {
		return nil, fmt.Errorf("Compile: %w", err)
	}

	content, err := MarshalComposeFile(br.File)
	if err != nil {
		return nil, fmt.Errorf("Compile: %w", err)
	}

	summary := buildSummary(p, br.File)

	return &compile.Result{
		Files: []compile.OutputFile{
			{
				Path:    composeFilePath,
				Content: content,
			},
		},
		Summary:  summary,
		Warnings: br.Warnings,
	}, nil
}

// Ensure Compiler satisfies the compile.Compiler interface at compile time.
var _ compile.Compiler = (*Compiler)(nil)
