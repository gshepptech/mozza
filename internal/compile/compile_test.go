package compile

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

func TestNewRegistry(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	assert.NotNil(t, reg, "NewRegistry should return a non-nil Registry")
	assert.Empty(t, reg.Targets(), "new registry should have no targets")
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	mock := &mockCompiler{name: "docker-compose"}
	reg.Register("local", mock)

	got, err := reg.Get("local")
	require.NoError(t, err)
	assert.Equal(t, mock, got)
}

func TestRegistry_GetUnknownTarget(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()

	_, err := reg.Get("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown target")
}

func TestRegistry_Targets(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	reg.Register("k8s", &mockCompiler{name: "kubernetes"})
	reg.Register("compose", &mockCompiler{name: "docker-compose"})
	reg.Register("argo", &mockCompiler{name: "argo-cd"})

	targets := reg.Targets()
	assert.Equal(t, []string{"argo", "compose", "k8s"}, targets, "targets should be sorted alphabetically")
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	first := &mockCompiler{name: "first"}
	second := &mockCompiler{name: "second"}

	reg.Register("target", first)
	reg.Register("target", second)

	got, err := reg.Get("target")
	require.NoError(t, err)
	assert.Equal(t, "second", got.Name(), "second registration should overwrite first")
}

func TestResultAndOutputFileTypes(t *testing.T) {
	t.Parallel()

	result := &Result{
		Files: []OutputFile{
			{Path: "deployment.yaml", Content: []byte("kind: Deployment")},
			{Path: "service.yaml", Content: []byte("kind: Service")},
		},
		Summary:  "Generated 2 files",
		Warnings: []string{"PVC not supported"},
	}

	assert.Len(t, result.Files, 2)
	assert.Equal(t, "deployment.yaml", result.Files[0].Path)
	assert.Equal(t, []byte("kind: Deployment"), result.Files[0].Content)
	assert.Equal(t, "Generated 2 files", result.Summary)
	assert.Len(t, result.Warnings, 1)
}

// Compile-time interface check.
var _ Compiler = (*mockCompiler)(nil)

// mockCompiler implements the Compiler interface for testing the Registry.
type mockCompiler struct {
	name string
}

func (m *mockCompiler) Compile(_ context.Context, _ *plan.AppPlan) (*Result, error) {
	return nil, nil
}

func (m *mockCompiler) Name() string {
	return m.name
}
