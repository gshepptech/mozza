package compile

import (
	"fmt"
	"maps"
	"slices"
)

// Registry maps target names to Compiler implementations.
type Registry struct {
	compilers map[string]Compiler
}

// NewRegistry creates an empty compiler registry.
func NewRegistry() *Registry {
	return &Registry{
		compilers: make(map[string]Compiler),
	}
}

// Register adds a compiler for the given target name.
func (r *Registry) Register(target string, c Compiler) {
	r.compilers[target] = c
}

// Get returns the compiler for the given target, or an error if not found.
func (r *Registry) Get(target string) (Compiler, error) {
	c, ok := r.compilers[target]
	if !ok {
		return nil, fmt.Errorf("Get: unknown target %q", target)
	}
	return c, nil
}

// Targets returns all registered target names sorted alphabetically.
func (r *Registry) Targets() []string {
	targets := slices.Collect(maps.Keys(r.compilers))
	slices.Sort(targets)
	return targets
}
