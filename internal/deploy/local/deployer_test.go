package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/store"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	st, err := store.Open(dbPath)
	require.NoError(t, err)
	defer st.Close()

	d := New(st, tmpDir)

	assert.NotNil(t, d, "New should return a non-nil Deployer")
	assert.Equal(t, tmpDir, d.projectDir)
	assert.Equal(t, st, d.store)
}

func TestNew_PreservesProjectDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		projectDir string
	}{
		{name: "absolute path", projectDir: "/tmp/my-project"},
		{name: "relative path", projectDir: "my-project"},
		{name: "empty path", projectDir: ""},
		{name: "nested path", projectDir: "/home/user/projects/app/v2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			dbPath := tmpDir + "/test.db"

			st, err := store.Open(dbPath)
			require.NoError(t, err)
			defer st.Close()

			d := New(st, tt.projectDir)
			assert.Equal(t, tt.projectDir, d.projectDir)
		})
	}
}

func TestRollback_NoPreviousDeploy(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	st, err := store.Open(dbPath)
	require.NoError(t, err)
	defer st.Close()

	d := New(st, tmpDir)

	// Rollback with no previous deploy should return an error.
	err = d.Rollback(t.Context(), "nonexistent-app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no previous deployment")
}
