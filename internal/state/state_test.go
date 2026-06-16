package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/state"
)

func TestNewStore(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)
	require.NotNil(t, store)

	// Verify state file does not exist until a record is written.
	_, err := os.Stat(filepath.Join(dir, ".mozza-state.json"))
	assert.True(t, os.IsNotExist(err), "state file should not exist before first record")
}

func TestStore_Record(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	r := state.DeployRecord{
		AppName:     "myapp",
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "1.0.0",
		Status:      "deployed",
	}

	err := store.Record(r)
	require.NoError(t, err)

	// Verify state file was created.
	_, err = os.Stat(filepath.Join(dir, ".mozza-state.json"))
	require.NoError(t, err, "state file should exist after recording")

	// Verify we can retrieve the record.
	latest, err := store.Latest()
	require.NoError(t, err)
	assert.Equal(t, "myapp", latest.AppName)
	assert.Equal(t, "local", latest.Target)
	assert.Equal(t, state.EnvDev, latest.Environment)
	assert.Equal(t, "1.0.0", latest.Version)
	assert.Equal(t, "deployed", latest.Status)
	assert.NotEmpty(t, latest.ID, "ID should be auto-generated")
	assert.False(t, latest.Timestamp.IsZero(), "timestamp should be auto-set")
}

func TestStore_Latest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	// Record two deployments.
	require.NoError(t, store.Record(state.DeployRecord{
		AppName:     "myapp",
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "1.0.0",
		Status:      "deployed",
	}))
	require.NoError(t, store.Record(state.DeployRecord{
		AppName:     "myapp",
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "2.0.0",
		Status:      "deployed",
	}))

	latest, err := store.Latest()
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", latest.Version, "latest should return the most recent record")
}

func TestStore_Latest_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	_, err := store.Latest()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deploy records found")
}

func TestStore_History(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	// Record three deployments.
	for _, v := range []string{"1.0.0", "2.0.0", "3.0.0"} {
		require.NoError(t, store.Record(state.DeployRecord{
			AppName:     "myapp",
			Target:      "local",
			Environment: state.EnvDev,
			Version:     v,
			Status:      "deployed",
		}))
	}

	tests := []struct {
		name         string
		limit        int
		wantCount    int
		wantFirstVer string
	}{
		{
			name:         "all records with zero limit",
			limit:        0,
			wantCount:    3,
			wantFirstVer: "3.0.0",
		},
		{
			name:         "limited to 2",
			limit:        2,
			wantCount:    2,
			wantFirstVer: "3.0.0",
		},
		{
			name:         "limit exceeds total returns all",
			limit:        10,
			wantCount:    3,
			wantFirstVer: "3.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			history, err := store.History(tt.limit)
			require.NoError(t, err)
			assert.Len(t, history, tt.wantCount)

			// History should be in reverse chronological order.
			assert.Equal(t, tt.wantFirstVer, history[0].Version)
		})
	}
}

func TestStore_Rollback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	// Record two deployments.
	require.NoError(t, store.Record(state.DeployRecord{
		ID:          "1",
		AppName:     "myapp",
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "1.0.0",
		Status:      "deployed",
	}))
	require.NoError(t, store.Record(state.DeployRecord{
		ID:          "2",
		AppName:     "myapp",
		Target:      "local",
		Environment: state.EnvDev,
		Version:     "2.0.0",
		Status:      "deployed",
	}))

	rollback, err := store.Rollback()
	require.NoError(t, err)
	require.NotNil(t, rollback)

	assert.Equal(t, "1.0.0", rollback.Version, "rollback should target the second-most-recent deployed version")
	assert.Equal(t, "rolled-back", rollback.Status)
	assert.Equal(t, "myapp", rollback.AppName)
	assert.Equal(t, state.EnvDev, rollback.Environment)
	assert.NotEmpty(t, rollback.ID)

	// Verify the rollback record was persisted.
	history, err := store.History(0)
	require.NoError(t, err)
	assert.Len(t, history, 3, "should have 2 deployed + 1 rolled-back record")
	assert.Equal(t, "rolled-back", history[0].Status)
}

func TestStore_Rollback_NoRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	_, err := store.Rollback()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no previous deployment to roll back to")
}

func TestStore_Promote(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := state.NewStore(dir)

	// Record a deployment in dev.
	require.NoError(t, store.Record(state.DeployRecord{
		ID:          "1",
		AppName:     "myapp",
		Target:      "kubernetes",
		Environment: state.EnvDev,
		Version:     "1.0.0",
		Status:      "deployed",
	}))

	promoted, err := store.Promote(state.EnvDev, state.EnvStaging)
	require.NoError(t, err)
	require.NotNil(t, promoted)

	assert.Equal(t, "1.0.0", promoted.Version, "promoted record should carry the source version")
	assert.Equal(t, "promoted", promoted.Status)
	assert.Equal(t, state.EnvStaging, promoted.Environment, "promoted record should target the destination environment")
	assert.Equal(t, "myapp", promoted.AppName)
	assert.Equal(t, "kubernetes", promoted.Target)
	assert.NotEmpty(t, promoted.ID)

	// Verify the promoted record was persisted.
	history, err := store.History(0)
	require.NoError(t, err)
	assert.Len(t, history, 2, "should have 1 deployed + 1 promoted record")
	assert.Equal(t, "promoted", history[0].Status)
	assert.Equal(t, state.EnvStaging, history[0].Environment)
}

func TestValidateEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    state.Environment
		wantErr bool
	}{
		{name: "dev is valid", input: "dev", want: state.EnvDev},
		{name: "staging is valid", input: "staging", want: state.EnvStaging},
		{name: "production is valid", input: "production", want: state.EnvProduction},
		{name: "empty string is invalid", input: "", wantErr: true},
		{name: "unknown string is invalid", input: "qa", wantErr: true},
		{name: "uppercase is invalid", input: "DEV", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := state.ValidateEnvironment(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown environment")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
