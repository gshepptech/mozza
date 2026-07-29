package cli_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/cli"
)

// cliStreams holds the separated output of one command run.
type cliStreams struct {
	stdout string
	stderr string
}

// runCLIStreams executes the root command with stdout and stderr bound to
// SEPARATE buffers. The shared-buffer helper in compile_test.go is deliberately
// stream-agnostic, which is what makes it useless for asserting the split — so
// stream tests get their own harness.
func runCLIStreams(t *testing.T, args ...string) (cliStreams, error) {
	t.Helper()

	cmd := cli.New()

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return cliStreams{stdout: outBuf.String(), stderr: errBuf.String()}, err
}

// TestTargetsCommand_WritesToStdout guards the pipeability of `mozza targets`.
// Its table is the command's payload, so it must survive `| grep`. Before the
// root SetOut, every Print* resolved to stderr and this output was invisible to
// a pipe.
func TestTargetsCommand_WritesToStdout(t *testing.T) {
	t.Parallel()

	got, err := runCLIStreams(t, "targets")
	require.NoError(t, err)

	assert.Contains(t, got.stdout, "kubernetes", "the targets table is payload and belongs on stdout")
	assert.Contains(t, got.stdout, "local")
	assert.Empty(t, got.stderr, "targets emits no diagnostics on a successful run")
}

// TestCompileCommand_PayloadIsFilesNotStdout pins the deliberate inversion for
// compile: its artifacts are the files on disk, so progress and the summary go
// to stderr and stdout stays clean. This is what makes
// `mozza compile > out.txt` produce an empty file rather than captured chatter.
func TestCompileCommand_PayloadIsFilesNotStdout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recipe := writeRecipe(t, simpleRecipe)

	got, err := runCLIStreams(t,
		"compile", "--target", "local", "--recipe", recipe, "-o", dir)
	require.NoError(t, err)

	assert.Empty(t, got.stdout,
		"compile writes files, not stdout — redirecting stdout must not capture progress")
	assert.Contains(t, got.stderr, "wrote", "progress belongs on stderr where a human still sees it")
	assert.FileExists(t, filepath.Join(dir, "docker-compose.yml"),
		"the real payload is the artifact on disk")
}

// TestDeployExport_DeprecationOnStderr keeps the deprecation notice out of any
// redirected payload while still putting it in front of a human.
//
// Not parallel, and chdir'd: --export hardcodes "." as its output directory (it
// predates compile's -o flag), so without an isolated working directory this
// test writes k8s/*.yaml into the package source tree.
func TestDeployExport_DeprecationOnStderr(t *testing.T) {
	t.Chdir(t.TempDir())

	recipe := writeRecipe(t, simpleRecipe)

	got, err := runCLIStreams(t, "deploy", "--export", "--recipe", recipe)
	require.NoError(t, err)

	assert.Contains(t, got.stderr, "deprecated", "the notice is a diagnostic, not payload")
	assert.NotContains(t, got.stdout, "deprecated")
}
