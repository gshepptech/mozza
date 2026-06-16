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

func TestNew_ReturnsValidCommand(t *testing.T) {
	t.Parallel()

	cmd := cli.New()

	assert.Equal(t, "mozza", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, len(cmd.Commands()) > 0, "root command should have subcommands")
}

func TestNew_HasAllSubcommands(t *testing.T) {
	t.Parallel()

	cmd := cli.New()

	expected := []string{
		"version",
		"init",
		"up",
		"down",
		"deploy",
		"status",
		"doctor",
		"logs",
		"rollback",
		"promote",
		"serve",
		"validate",
	}

	names := make(map[string]bool, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}

	for _, want := range expected {
		assert.True(t, names[want], "missing subcommand: %s", want)
	}
}

func TestVersionCommand_Output(t *testing.T) {
	t.Parallel()

	cmd := cli.New()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"version"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "mozza v")
	assert.Contains(t, output, "commit:")
	assert.Contains(t, output, "built:")
}

// TestInitCommand_CreatesFile is not parallel because it uses os.Chdir.
func TestInitCommand_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	// Change to temp dir for the init command.
	original, err := os.Getwd()
	require.NoError(t, err)

	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(original)
	})

	cmd := cli.New()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"init", "myapp"})

	err = cmd.Execute()
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "app.mozza"))
	require.NoError(t, err)

	text := string(content)
	assert.Contains(t, text, "App: myapp", "recipe should contain app name")
	assert.Contains(t, text, "Api:", "recipe should contain api section")
	assert.Contains(t, text, "from image myapp:latest", "recipe should reference app image")
}

func TestInitCommand_RejectsInvalidName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appName string
	}{
		{"uppercase", "MyApp"},
		{"spaces", "my app"},
		{"special chars", "my_app!"},
		{"starts with hyphen", "-myapp"},
		{"ends with hyphen", "myapp-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := cli.New()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"init", tt.appName})

			err := cmd.Execute()
			assert.Error(t, err, "should reject invalid app name %q", tt.appName)
		})
	}
}
