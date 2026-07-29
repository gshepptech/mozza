package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/cli"
)

// simpleRecipe is a minimal recipe every target can express fully.
const simpleRecipe = `App: testapp

Api:
  from image nginx:alpine
  open to the public on port 8080
  run 1 copy
`

// daemonRecipe uses a Kubernetes-only feature so the local target has something
// to warn about.
const daemonRecipe = `App: testapp

Collector:
  from image fluentd:latest
  run on every node
`

// writeRecipe writes content to a temp dir and returns the recipe path.
func writeRecipe(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.mozza")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// runCLI executes the root command with args and returns its combined output.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := cli.New()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return buf.String(), err
}

func TestCompileCommand_WritesTargetArtifacts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		wantFile  string
		wantInOut string
		args      []string
	}{
		{
			// No --target: must fall through to the kubernetes default.
			name:      "default target is kubernetes",
			args:      nil,
			wantFile:  filepath.Join("k8s", "api-deployment.yaml"),
			wantInOut: "Kubernetes manifests",
		},
		{
			name:      "explicit kubernetes target",
			args:      []string{"--target", "kubernetes"},
			wantFile:  filepath.Join("k8s", "api-deployment.yaml"),
			wantInOut: "Kubernetes manifests",
		},
		{
			name:      "local target",
			args:      []string{"--target", "local"},
			wantFile:  "docker-compose.yml",
			wantInOut: "docker-compose.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recipePath := writeRecipe(t, simpleRecipe)
			outDir := t.TempDir()

			args := append([]string{"compile", "--recipe", recipePath, "-o", outDir}, tt.args...)

			out, err := runCLI(t, args...)
			require.NoError(t, err, "output: %s", out)

			assert.FileExists(t, filepath.Join(outDir, tt.wantFile))
			assert.Contains(t, out, tt.wantInOut)
		})
	}
}

func TestCompileCommand_UnknownTargetListsAvailable(t *testing.T) {
	t.Parallel()

	recipePath := writeRecipe(t, simpleRecipe)

	out, err := runCLI(t, "compile", "--recipe", recipePath, "-o", t.TempDir(), "--target", "bogus")
	require.Error(t, err)

	assert.Contains(t, err.Error(), `unknown target "bogus"`)
	assert.Contains(t, err.Error(), "kubernetes", "error should list the kubernetes target")
	assert.Contains(t, err.Error(), "local", "error should list the local target")
	assert.NotContains(t, out, "wrote ", "an unknown target must not write anything")
}

// TestCompileCommand_OutputDirHonored is not parallel: it verifies that nothing
// lands in the working directory, which requires owning the working directory.
func TestCompileCommand_OutputDirHonored(t *testing.T) {
	recipePath := writeRecipe(t, simpleRecipe)
	workDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "nested", "out")

	original, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(workDir))
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	out, err := runCLI(t, "compile", "--recipe", recipePath, "--target", "local", "-o", outDir)
	require.NoError(t, err, "output: %s", out)

	assert.FileExists(t, filepath.Join(outDir, "docker-compose.yml"))

	entries, err := os.ReadDir(workDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "compile must write only into -o")
}

// TestCompileCommand_IsSideEffectFree guards AD-5: compiling contacts nothing
// and records nothing, so the working directory stays clean of state files.
func TestCompileCommand_IsSideEffectFree(t *testing.T) {
	recipePath := writeRecipe(t, simpleRecipe)
	workDir := t.TempDir()

	original, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(workDir))
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	out, err := runCLI(t, "compile", "--recipe", recipePath, "--target", "local", "-o", t.TempDir())
	require.NoError(t, err, "output: %s", out)

	for _, unwanted := range []string{"mozza.db", ".mozza", ".mozza-state.json"} {
		assert.NoFileExists(t, filepath.Join(workDir, unwanted))
	}
}

func TestCompileCommand_PrintsWarnings(t *testing.T) {
	t.Parallel()

	recipePath := writeRecipe(t, daemonRecipe)

	out, err := runCLI(t, "compile", "--recipe", recipePath, "--target", "local", "-o", t.TempDir())
	require.NoError(t, err, "output: %s", out)

	assert.Contains(t, out, "warning(s):")
	assert.Contains(t, out, "DaemonSet mode requires Docker Swarm")
}

func TestCompileCommand_MissingRecipeFails(t *testing.T) {
	t.Parallel()

	out, err := runCLI(t, "compile", "--recipe", filepath.Join(t.TempDir(), "absent.mozza"), "-o", t.TempDir())
	require.Error(t, err)
	assert.NotContains(t, out, "wrote ")
}
