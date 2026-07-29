package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTargetsCommand_ListsRegisteredTargets(t *testing.T) {
	t.Parallel()

	out, err := runCLI(t, "targets")
	require.NoError(t, err, "output: %s", out)

	assert.Contains(t, out, "kubernetes")
	assert.Contains(t, out, "local")
	assert.Contains(t, out, "Docker Compose", "each target should show its description")
	assert.Contains(t, out, "Kubernetes manifests")
}

func TestTargetsCommand_MarksDefault(t *testing.T) {
	t.Parallel()

	out, err := runCLI(t, "targets")
	require.NoError(t, err, "output: %s", out)

	assert.Contains(t, out, "* kubernetes", "the default target should be marked")
	assert.Contains(t, out, "  local", "a non-default target should not be marked")
	assert.Contains(t, out, "* default target")
}
