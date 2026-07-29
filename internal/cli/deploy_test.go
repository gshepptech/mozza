package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDeployCommand_ExportStillWorks covers the deprecated --export flag: it
// keeps writing manifests to the working directory, and it says it is on the
// way out. Not parallel because --export writes relative to the working
// directory.
func TestDeployCommand_ExportStillWorks(t *testing.T) {
	recipePath := writeRecipe(t, simpleRecipe)
	workDir := t.TempDir()

	original, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(workDir))
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	out, err := runCLI(t, "deploy", "--recipe", recipePath, "--export")
	require.NoError(t, err, "output: %s", out)

	assert.FileExists(t, filepath.Join(workDir, "k8s", "api-deployment.yaml"))
	assert.Contains(t, out, "--export is deprecated")
	assert.Contains(t, out, "mozza compile --target kubernetes")
}
