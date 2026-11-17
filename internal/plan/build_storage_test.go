package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

func TestBuild_DatabaseSpec(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "db-test",
		Slices: []recipe.Slice{
			{Name: "db", Engine: "postgres", Version: "16", Storage: "10Gi"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	db := plan.Slices[0]
	require.NotNil(t, db.Database)
	assert.Equal(t, "10Gi", db.Database.Storage)
	assert.Nil(t, db.Cache)
}

func TestBuild_DatabaseSpecWithBackups(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "db-backup-test",
		Slices: []recipe.Slice{
			{Name: "db", Engine: "postgres", Version: "16", Storage: "20Gi", Backups: "daily"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	db := plan.Slices[0]
	require.NotNil(t, db.Database)
	assert.Equal(t, "20Gi", db.Database.Storage)
	assert.Equal(t, "daily", db.Database.BackupPolicy)
}

func TestBuild_CacheSpec(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "cache-test",
		Slices: []recipe.Slice{
			{Name: "cache", Engine: "redis", Version: "7", Storage: "2Gi"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	cache := plan.Slices[0]
	require.NotNil(t, cache.Cache)
	assert.Equal(t, "2Gi", cache.Cache.Storage)
	assert.Nil(t, cache.Database)
}

func TestBuild_DatabaseWithoutStorage(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "no-storage",
		Slices: []recipe.Slice{
			{Name: "db", Engine: "postgres", Version: "16"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	db := plan.Slices[0]
	require.NotNil(t, db.Database)
	assert.Empty(t, db.Database.Storage)
}

func TestBuild_CacheWithoutStorage(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "no-storage",
		Slices: []recipe.Slice{
			{Name: "cache", Engine: "redis", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	cache := plan.Slices[0]
	assert.Nil(t, cache.Cache)
}

func TestBuild_WebStorageIgnored(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "web-storage",
		Slices: []recipe.Slice{
			{Name: "api", Kind: "web", Image: "api:latest", Storage: "5Gi"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	assert.Nil(t, api.Database)
	assert.Nil(t, api.Cache)
}

func TestBuild_MountPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		engine        string
		version       string
		wantMountPath string
		wantKind      SliceKind
	}{
		{
			name:          "postgres mount path",
			engine:        "postgres",
			version:       "16",
			wantMountPath: "/var/lib/postgresql/data",
			wantKind:      SliceKindDatabase,
		},
		{
			name:          "mysql mount path",
			engine:        "mysql",
			version:       "8",
			wantMountPath: "/var/lib/mysql",
			wantKind:      SliceKindDatabase,
		},
		{
			name:          "mongo mount path",
			engine:        "mongo",
			version:       "7",
			wantMountPath: "/data/db",
			wantKind:      SliceKindDatabase,
		},
		{
			name:          "redis mount path",
			engine:        "redis",
			version:       "7",
			wantMountPath: "/data",
			wantKind:      SliceKindCache,
		},
		{
			name:          "memcached mount path",
			engine:        "memcached",
			version:       "1.6",
			wantMountPath: "/data",
			wantKind:      SliceKindCache,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "mount-test",
				Slices: []recipe.Slice{
					{
						Name:    "svc",
						Engine:  tt.engine,
						Version: tt.version,
						Storage: "10Gi",
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			s := plan.Slices[0]
			assert.Equal(t, tt.wantKind, s.Kind)

			switch tt.wantKind {
			case SliceKindDatabase:
				require.NotNil(t, s.Database, "expected DatabaseSpec for engine %s", tt.engine)
				assert.Equal(t, tt.wantMountPath, s.Database.MountPath)
			case SliceKindCache:
				require.NotNil(t, s.Cache, "expected CacheSpec for engine %s", tt.engine)
				assert.Equal(t, tt.wantMountPath, s.Cache.MountPath)
			case SliceKindWeb, SliceKindWorker, SliceKindAPI, SliceKindTask,
				SliceKindScheduled, SliceKindStateful, SliceKindGateway,
				SliceKindDaemon:
				// No storage spec to check.
			}
		})
	}
}
