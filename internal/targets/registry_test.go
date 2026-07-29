package targets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/plan"
)

func TestDefaultRegistry_RegistersBuiltins(t *testing.T) {
	t.Parallel()

	reg := DefaultRegistry()

	// Targets() sorts, so this asserts both membership and ordering.
	assert.Equal(t, []string{"kubernetes", "local"}, reg.Targets())

	for _, name := range []string{"kubernetes", "local"} {
		c, err := reg.Get(name)
		require.NoError(t, err, "built-in target %q must resolve", name)
		assert.Equal(t, name, c.Name(),
			"Compiler.Name() must match its registry key — `mozza targets` and --target would disagree otherwise")
	}
}

func TestDefaultRegistry_IsIndependentPerCall(t *testing.T) {
	t.Parallel()

	// Callers may build a registry per command or hold one for the process
	// lifetime; mutating one must not leak into another.
	a, b := DefaultRegistry(), DefaultRegistry()
	a.Register("scratch", &stubCompiler{name: "scratch"})

	_, err := b.Get("scratch")
	require.Error(t, err, "registries returned by DefaultRegistry must not share state")
}

func TestDefault_IsRegistered(t *testing.T) {
	t.Parallel()

	// The default target must actually exist, or every bare `mozza compile`
	// fails with "unknown target".
	_, err := DefaultRegistry().Get(Default)
	require.NoError(t, err, "Default (%q) must be a registered target", Default)
}

func TestList(t *testing.T) {
	t.Parallel()

	got := List(DefaultRegistry())
	require.Len(t, got, 2)

	assert.Equal(t, "kubernetes", got[0].Name, "List must be sorted by name")
	assert.Equal(t, "local", got[1].Name)

	assert.True(t, got[0].Default, "kubernetes is the default target")
	assert.False(t, got[1].Default)

	for _, target := range got {
		assert.NotEmpty(t, target.Description,
			"every built-in target needs a description for `mozza targets` output")
	}
}

func TestList_KeepsTargetsWithoutDescriptions(t *testing.T) {
	t.Parallel()

	// A missing description is a docs gap, not a reason to hide a working
	// target — this is the behavior third-party/future backends depend on
	// before their description lands.
	reg := compile.NewRegistry()
	reg.Register("undocumented", &stubCompiler{name: "undocumented"})

	got := List(reg)
	require.Len(t, got, 1)
	assert.Equal(t, "undocumented", got[0].Name)
	assert.Empty(t, got[0].Description)
	assert.False(t, got[0].Default)
}

func TestList_EmptyRegistry(t *testing.T) {
	t.Parallel()

	assert.Empty(t, List(compile.NewRegistry()))
}

// stubCompiler is a minimal compile.Compiler for registry tests. It is never
// invoked — only registered and looked up.
type stubCompiler struct {
	name string
}

func (s *stubCompiler) Name() string { return s.name }

func (s *stubCompiler) Compile(_ context.Context, _ *plan.AppPlan) (*compile.Result, error) {
	return &compile.Result{Summary: "stub"}, nil
}
