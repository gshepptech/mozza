// Package targets is the single place where every built-in compile target is
// registered. It exists to break an import cycle: internal/compile defines the
// Compiler interface, and every backend (internal/k8s, internal/local, ...)
// imports internal/compile to return a *compile.Result. That means
// internal/compile cannot import the backends to assemble a default registry.
//
// This package sits above both — it imports internal/compile and every backend,
// and hands out an assembled registry. Callers that need to compile
// (internal/cli, internal/server) import this package rather than reaching for a
// concrete backend.
//
// Adding a target is two edits here: register it in DefaultRegistry and describe
// it in descriptions. Nothing else in the tree needs to change.
package targets

import (
	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/local"
)

// Default is the target used when the caller does not name one. Kubernetes is
// the reference semantics for the recipe language, so it is the default
// everywhere.
const Default = "kubernetes"

// descriptions maps a target name to a one-line human-readable summary of what
// compiling to it produces. Used by `mozza targets` and GET /api/v1/targets.
//
// Compiler.Name() returns the target key itself, not a description, so this
// metadata lives here rather than on the interface. Enriching the Compiler
// interface is M1's job (Capabilities()); M0 keeps the interface untouched.
var descriptions = map[string]string{ //nolint:gochecknoglobals // immutable lookup table
	"kubernetes": "Kubernetes manifests (Deployment, Service, Ingress, PVC, RBAC, ...)",
	"local":      "Docker Compose file for running on one machine",
}

// DefaultRegistry returns a registry with every built-in compiler registered.
// Each call returns a fresh registry; compilers are stateless, so callers may
// build one per command or hold one for the process lifetime.
func DefaultRegistry() *compile.Registry {
	reg := compile.NewRegistry()
	reg.Register("local", local.New())
	reg.Register(Default, k8s.New())

	return reg
}

// Target pairs a registered target name with its human-readable description.
type Target struct {
	// Name is the registry key, as passed to --target.
	Name string `json:"name"`
	// Description is a one-line summary of what this target produces.
	Description string `json:"description"`
	// Default reports whether this is the target used when none is named.
	Default bool `json:"default"`
}

// List returns every target in the given registry, sorted by name, paired with
// its description. A registered target with no description entry gets an empty
// one rather than being omitted — a missing description is a docs gap, not a
// reason to hide a working target.
func List(reg *compile.Registry) []Target {
	names := reg.Targets()
	out := make([]Target, 0, len(names))

	for _, name := range names {
		out = append(out, Target{
			Name:        name,
			Description: descriptions[name],
			Default:     name == Default,
		})
	}

	return out
}
