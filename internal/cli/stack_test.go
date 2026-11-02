package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every stack subcommand that takes a STACK must fall back to
// ./mozza-stack.yaml rather than indexing a missing argument.
func TestStackCommandsDefaultToTheRepoStackFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("MOZZA_HOME", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(dir, defaultStackFile), []byte(`
name: demo
cluster: {provider: kind}
secrets:
  - name: s
    namespace: demo
    data: {pw: {generate: password, show: true}}
components:
  - name: app
    namespace: demo
    helm: {chart: oci://example.test/app}
`), 0o644))

	for _, sub := range []string{"secrets", "plan"} {
		t.Run(sub, func(t *testing.T) {
			root := New()
			root.SetArgs([]string{"stack", sub})
			assert.NotPanics(t, func() { _ = root.Execute() })
		})
	}
	assert.Equal(t, defaultStackFile, stackRef(nil))
	assert.Equal(t, "inventory", stackRef([]string{"inventory"}))
}
