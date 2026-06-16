package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

func TestBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		recipe          *recipe.Recipe
		wantName        string
		wantSlices      int
		wantIngredients int
		wantErr         bool
		errSubstr       string
	}{
		{
			name:            "empty recipe",
			recipe:          &recipe.Recipe{},
			wantName:        "",
			wantSlices:      0,
			wantIngredients: 0,
		},
		{
			name: "simple web and database with explicit kind",
			recipe: &recipe.Recipe{
				Name: "my-app",
				Slices: []recipe.Slice{
					{
						Name:     "api",
						Kind:     "web",
						Image:    "myorg/api:1.0",
						Port:     8080,
						Public:   true,
						Health:   "/healthz",
						Replicas: 2,
						Needs:    []string{"db"},
					},
					{
						Name:    "db",
						Kind:    "database",
						Image:   "postgres:16",
						Port:    5432,
						Storage: "10Gi",
					},
				},
			},
			wantName:        "my-app",
			wantSlices:      2,
			wantIngredients: 1,
		},
		{
			name: "kind inference from engine shorthand",
			recipe: &recipe.Recipe{
				Name: "inferred",
				Slices: []recipe.Slice{
					{Name: "api", Image: "myorg/api:1.0", Port: 8080, Public: true},
					{Name: "db", Engine: "postgres", Version: "16", Storage: "10Gi"},
					{Name: "cache", Engine: "redis", Version: "7"},
				},
			},
			wantName:        "inferred",
			wantSlices:      3,
			wantIngredients: 0,
		},
		{
			name: "unknown needs reference returns error",
			recipe: &recipe.Recipe{
				Name: "bad-needs",
				Slices: []recipe.Slice{
					{Name: "api", Image: "myorg/api:1.0", Port: 8080, Public: true, Needs: []string{"nonexistent"}},
				},
			},
			wantErr:   true,
			errSubstr: "needs unknown slice",
		},
		{
			name:    "nil recipe returns error",
			recipe:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := Build(tt.recipe)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "Build:")
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, plan)
			assert.Equal(t, tt.wantName, plan.Name)
			assert.Len(t, plan.Slices, tt.wantSlices)
			assert.Len(t, plan.Ingredients, tt.wantIngredients)
		})
	}
}

func TestBuild_SliceConversion(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "convert-test",
		Slices: []recipe.Slice{
			{
				Name:     "api",
				Image:    "myorg/api:2.0",
				Port:     9090,
				Public:   true,
				Health:   "/ready",
				Replicas: 4,
				Needs:    []string{"db"},
				Line:     5,
			},
			{
				Name:    "db",
				Engine:  "postgres",
				Version: "16",
				Storage: "50Gi",
			},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)
	require.Len(t, plan.Slices, 2)

	api := plan.Slices[0]
	assert.Equal(t, "api", api.Name)
	assert.Equal(t, SliceKindWeb, api.Kind)
	assert.Equal(t, "myorg/api:2.0", api.Image)
	assert.Equal(t, 9090, api.Port)
	assert.True(t, api.Public)
	assert.Equal(t, "/ready", api.HealthPath)
	assert.Equal(t, 4, api.Replicas)
	assert.Equal(t, []string{"db"}, api.Needs)
	assert.Nil(t, api.Database)
	assert.Nil(t, api.Cache)

	db := plan.Slices[1]
	assert.Equal(t, "db", db.Name)
	assert.Equal(t, SliceKindDatabase, db.Kind)
	assert.Equal(t, "postgres:16-alpine", db.Image)
	assert.Equal(t, 5432, db.Port)
	assert.False(t, db.Public)
	assert.Empty(t, db.Needs)
}

func TestBuild_EngineDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		engine    string
		version   string
		wantImage string
		wantPort  int
		wantKind  SliceKind
	}{
		{"postgres", "postgres", "16", "postgres:16-alpine", 5432, SliceKindDatabase},
		{"mysql", "mysql", "8", "mysql:8", 3306, SliceKindDatabase},
		{"mongo", "mongo", "7", "mongo:7", 27017, SliceKindDatabase},
		{"redis", "redis", "7", "redis:7-alpine", 6379, SliceKindCache},
		{"memcached", "memcached", "1.6", "memcached:1.6-alpine", 11211, SliceKindCache},
		{"postgres no version", "postgres", "", "postgres:latest-alpine", 5432, SliceKindDatabase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "test",
				Slices: []recipe.Slice{
					{Name: "svc", Engine: tt.engine, Version: tt.version},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			s := plan.Slices[0]
			assert.Equal(t, tt.wantImage, s.Image)
			assert.Equal(t, tt.wantPort, s.Port)
			assert.Equal(t, tt.wantKind, s.Kind)
		})
	}
}

func TestBuild_KindInference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		slice    recipe.Slice
		wantKind SliceKind
	}{
		{
			name:     "explicit kind",
			slice:    recipe.Slice{Name: "svc", Kind: "worker", Image: "img:latest"},
			wantKind: SliceKindWorker,
		},
		{
			name:     "database engine",
			slice:    recipe.Slice{Name: "db", Engine: "postgres", Version: "16"},
			wantKind: SliceKindDatabase,
		},
		{
			name:     "cache engine",
			slice:    recipe.Slice{Name: "cache", Engine: "redis", Version: "7"},
			wantKind: SliceKindCache,
		},
		{
			name:     "schedule means scheduled",
			slice:    recipe.Slice{Name: "backup", Image: "backup:latest", Schedule: "0 3 * * *"},
			wantKind: SliceKindScheduled,
		},
		{
			name:     "run once means task",
			slice:    recipe.Slice{Name: "migrate", Image: "migrate:latest", RunOnce: true},
			wantKind: SliceKindTask,
		},
		{
			name:     "daemon mode means daemon",
			slice:    recipe.Slice{Name: "log-collector", Image: "fluentd:latest", DaemonMode: true},
			wantKind: SliceKindDaemon,
		},
		{
			name:     "stateful storage means stateful",
			slice:    recipe.Slice{Name: "zk", Image: "zookeeper:latest", StatefulStorage: "10Gi"},
			wantKind: SliceKindStateful,
		},
		{
			name:     "ordered startup means stateful",
			slice:    recipe.Slice{Name: "etcd", Image: "etcd:latest", OrderedStartup: true},
			wantKind: SliceKindStateful,
		},
		{
			name:     "peer discovery means stateful",
			slice:    recipe.Slice{Name: "consul", Image: "consul:latest", PeerDiscovery: true},
			wantKind: SliceKindStateful,
		},
		{
			name:     "public plus port means web",
			slice:    recipe.Slice{Name: "fe", Image: "fe:latest", Port: 3000, Public: true},
			wantKind: SliceKindWeb,
		},
		{
			name:     "port plus name contains gateway means gateway",
			slice:    recipe.Slice{Name: "api-gateway", Image: "gw:latest", Port: 8080},
			wantKind: SliceKindGateway,
		},
		{
			name:     "port plus name contains proxy means gateway",
			slice:    recipe.Slice{Name: "reverse-proxy", Image: "nginx:latest", Port: 80},
			wantKind: SliceKindGateway,
		},
		{
			name:     "port plus name contains api means api",
			slice:    recipe.Slice{Name: "user-api", Image: "api:latest", Port: 8080},
			wantKind: SliceKindAPI,
		},
		{
			name:     "port only means api",
			slice:    recipe.Slice{Name: "backend", Image: "be:latest", Port: 8080},
			wantKind: SliceKindAPI,
		},
		{
			name:     "name contains worker",
			slice:    recipe.Slice{Name: "bg-worker", Image: "w:latest"},
			wantKind: SliceKindWorker,
		},
		{
			name:     "name contains job",
			slice:    recipe.Slice{Name: "email-job", Image: "j:latest"},
			wantKind: SliceKindWorker,
		},
		{
			name:     "name contains processor",
			slice:    recipe.Slice{Name: "data-processor", Image: "proc:latest"},
			wantKind: SliceKindWorker,
		},
		{
			name:     "image no port means worker",
			slice:    recipe.Slice{Name: "something", Image: "proc:latest"},
			wantKind: SliceKindWorker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name:   "test",
				Slices: []recipe.Slice{tt.slice},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)
			assert.Equal(t, tt.wantKind, plan.Slices[0].Kind)
		})
	}
}

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

func TestBuild_Ingredients(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "deps-test",
		Slices: []recipe.Slice{
			{Name: "frontend", Image: "fe:latest", Port: 3000, Public: true, Needs: []string{"backend"}},
			{Name: "backend", Image: "be:latest", Port: 8080, Needs: []string{"db", "cache"}},
			{Name: "worker", Image: "w:latest", Needs: []string{"db"}},
			{Name: "db", Engine: "postgres", Version: "16"},
			{Name: "cache", Engine: "redis", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	expected := []Ingredient{
		{From: "frontend", To: "backend"},
		{From: "backend", To: "db"},
		{From: "backend", To: "cache"},
		{From: "worker", To: "db"},
	}
	assert.Equal(t, expected, plan.Ingredients)
}

func TestBuild_NameNormalization(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "test",
		Slices: []recipe.Slice{
			{Name: "Storefront", Image: "sf:latest", Port: 3000, Public: true},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)
	assert.Equal(t, "storefront", plan.Slices[0].Name)
}

func TestBuild_MultipleErrors(t *testing.T) {
	t.Parallel()

	// Slices with no kind determination possible.
	r := &recipe.Recipe{
		Name: "multi-err",
		Slices: []recipe.Slice{
			{Name: "a"},
			{Name: "b"},
		},
	}

	_, err := Build(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `slice "a"`)
	assert.Contains(t, err.Error(), `slice "b"`)
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

func TestBuild_EnvPassthrough(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		wantEnv map[string]string
	}{
		{
			name:    "single env var",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/db"},
			wantEnv: map[string]string{"DATABASE_URL": "postgres://localhost/db"},
		},
		{
			name: "multiple env vars",
			env: map[string]string{
				"PORT":     "3000",
				"NODE_ENV": "production",
				"API_KEY":  "secret-123",
			},
			wantEnv: map[string]string{
				"PORT":     "3000",
				"NODE_ENV": "production",
				"API_KEY":  "secret-123",
			},
		},
		{
			name:    "nil env",
			env:     nil,
			wantEnv: nil,
		},
		{
			name:    "empty env",
			env:     map[string]string{},
			wantEnv: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "env-test",
				Slices: []recipe.Slice{
					{
						Name:  "api",
						Image: "api:latest",
						Port:  8080,
						Env:   tt.env,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantEnv, plan.Slices[0].Env)
		})
	}
}

func TestBuild_ResourceSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cpuLimit      string
		memoryLimit   string
		wantResources *ResourceSpec
	}{
		{
			name:        "both cpu and memory",
			cpuLimit:    "500m",
			memoryLimit: "256Mi",
			wantResources: &ResourceSpec{
				CPULimit:    "500m",
				MemoryLimit: "256Mi",
			},
		},
		{
			name:     "cpu only",
			cpuLimit: "1",
			wantResources: &ResourceSpec{
				CPULimit: "1",
			},
		},
		{
			name:        "memory only",
			memoryLimit: "1Gi",
			wantResources: &ResourceSpec{
				MemoryLimit: "1Gi",
			},
		},
		{
			name:          "no resource limits",
			wantResources: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "resource-test",
				Slices: []recipe.Slice{
					{
						Name:        "api",
						Image:       "api:latest",
						Port:        8080,
						CPULimit:    tt.cpuLimit,
						MemoryLimit: tt.memoryLimit,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantResources, plan.Slices[0].Resources)
		})
	}
}

func TestBuild_RestartPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		policy     string
		wantPolicy string
	}{
		{"always", "always", "always"},
		{"unless-stopped", "unless-stopped", "unless-stopped"},
		{"on-failure", "on-failure", "on-failure"},
		{"no", "no", "no"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "restart-test",
				Slices: []recipe.Slice{
					{
						Name:          "api",
						Image:         "api:latest",
						Port:          8080,
						RestartPolicy: tt.policy,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantPolicy, plan.Slices[0].RestartPolicy)
		})
	}
}

func TestBuild_Domain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		domain     string
		wantDomain string
	}{
		{"simple domain", "example.com", "example.com"},
		{"subdomain", "api.example.com", "api.example.com"},
		{"deep subdomain", "v1.api.example.com", "v1.api.example.com"},
		{"empty domain", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name: "domain-test",
				Slices: []recipe.Slice{
					{
						Name:   "api",
						Image:  "api:latest",
						Port:   8080,
						Domain: tt.domain,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)

			assert.Equal(t, tt.wantDomain, plan.Slices[0].Domain)
		})
	}
}

func TestBuild_Namespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		namespace     string
		wantNamespace string
	}{
		{"production", "production", "production"},
		{"staging", "staging", "staging"},
		{"custom", "my-namespace", "my-namespace"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &recipe.Recipe{
				Name:      "ns-test",
				Namespace: tt.namespace,
				Slices: []recipe.Slice{
					{
						Name:  "api",
						Image: "api:latest",
						Port:  8080,
					},
				},
			}

			plan, err := Build(r)
			require.NoError(t, err)

			assert.Equal(t, tt.wantNamespace, plan.Namespace)
		})
	}
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

func TestBuild_AutoWirePostgres(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"db"}},
			{Name: "db", Engine: "postgres", Version: "16"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "postgres://db:5432/my-app", api.Env["DATABASE_URL"])
}

func TestBuild_AutoWireRedis(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"cache"}},
			{Name: "cache", Engine: "redis", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "redis://cache:6379", api.Env["REDIS_URL"])
}

func TestBuild_AutoWireMongo(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mongo"}},
			{Name: "mongo", Engine: "mongo", Version: "7"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mongodb://mongo:27017/my-app", api.Env["MONGO_URL"])
}

func TestBuild_AutoWireMySQL(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mydb"}},
			{Name: "mydb", Engine: "mysql", Version: "8"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mysql://mydb:3306/my-app", api.Env["DATABASE_URL"])
}

func TestBuild_AutoWireMemcached(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "api", Image: "api:latest", Port: 8080, Needs: []string{"mc"}},
			{Name: "mc", Engine: "memcached", Version: "1.6"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	require.NotNil(t, api.Env)
	assert.Equal(t, "mc:11211", api.Env["MEMCACHED_URL"])
}

func TestBuild_AutoWireGenericService(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{Name: "frontend", Image: "fe:latest", Port: 3000, Public: true, Needs: []string{"auth-service"}},
			{Name: "auth-service", Image: "auth:latest", Port: 9090},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	fe := plan.Slices[0]
	require.NotNil(t, fe.Env)
	assert.Equal(t, "http://auth-service:9090", fe.Env["AUTH_SERVICE_URL"])
}

func TestBuild_AutoWireDoesNotOverrideExisting(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "my-app",
		Slices: []recipe.Slice{
			{
				Name:  "api",
				Image: "api:latest",
				Port:  8080,
				Needs: []string{"db"},
				Env:   map[string]string{"DATABASE_URL": "postgres://custom:5432/custom"},
			},
			{Name: "db", Engine: "postgres", Version: "16"},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)

	api := plan.Slices[0]
	assert.Equal(t, "postgres://custom:5432/custom", api.Env["DATABASE_URL"])
}

func TestBuild_NewFieldConversion(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "full-test",
		Slices: []recipe.Slice{
			{
				Name:  "api",
				Image: "api:latest",
				Port:  8080,
				Ports: []recipe.PortSpec{
					{Name: "http", Port: 8080, Protocol: "TCP"},
					{Name: "grpc", Port: 9090, Protocol: "TCP"},
				},
				Probes: []recipe.ProbeSpec{
					{Type: "readiness", HTTPPath: "/ready", Interval: 10, Timeout: 5},
					{Type: "liveness", HTTPPath: "/health", Interval: 30},
				},
				InitSteps: []recipe.InitStep{
					{Image: "busybox:latest", Command: "echo init"},
				},
				Sidecars: []recipe.Sidecar{
					{Name: "proxy", Image: "envoy:latest", Ports: []recipe.PortSpec{{Name: "admin", Port: 9901}}},
				},
				Mounts: []recipe.MountSpec{
					{Type: "configmap", Source: "app-config", Target: "/etc/config", ReadOnly: true},
				},
				Lifecycle: &recipe.LifecycleSpec{
					PreStopCommand: "kill -SIGTERM 1",
					PreStopWait:    15,
				},
				Permissions: []recipe.Permission{
					{Verbs: []string{"get", "list"}, Resources: []string{"pods"}},
				},
				ServiceAccount:   "api-sa",
				GracefulShutdown: 30,
				AutoScale: &recipe.AutoScaleSpec{
					MinReplicas: 2,
					MaxReplicas: 10,
					CPUTarget:   80,
				},
				Security: &recipe.SecuritySpec{
					RunAsUser:    1000,
					ReadOnlyRoot: true,
				},
				UpdateStrategy: &recipe.UpdateStrategySpec{
					MaxSurge:       "25%",
					MaxUnavailable: "0",
				},
				DNSName: "api.internal",
			},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)
	require.Len(t, plan.Slices, 1)

	s := plan.Slices[0]

	// Multi-port.
	require.Len(t, s.Ports, 2)
	assert.Equal(t, "http", s.Ports[0].Name)
	assert.Equal(t, 8080, s.Ports[0].Port)
	assert.Equal(t, "grpc", s.Ports[1].Name)

	// Probes.
	require.Len(t, s.Probes, 2)
	assert.Equal(t, "readiness", s.Probes[0].Type)
	assert.Equal(t, "/ready", s.Probes[0].HTTPPath)

	// HealthPath backward compat from first readiness probe.
	assert.Equal(t, "/ready", s.HealthPath)

	// Init steps.
	require.Len(t, s.InitSteps, 1)
	assert.Equal(t, "busybox:latest", s.InitSteps[0].Image)

	// Sidecars.
	require.Len(t, s.Sidecars, 1)
	assert.Equal(t, "proxy", s.Sidecars[0].Name)
	require.Len(t, s.Sidecars[0].Ports, 1)

	// Mounts.
	require.Len(t, s.Mounts, 1)
	assert.Equal(t, "/etc/config", s.Mounts[0].Target)
	assert.True(t, s.Mounts[0].ReadOnly)

	// Lifecycle.
	require.NotNil(t, s.Lifecycle)
	assert.Equal(t, "kill -SIGTERM 1", s.Lifecycle.PreStopCommand)

	// Permissions.
	require.Len(t, s.Permissions, 1)
	assert.Equal(t, []string{"get", "list"}, s.Permissions[0].Verbs)

	// Scalar fields.
	assert.Equal(t, "api-sa", s.ServiceAccount)
	assert.Equal(t, 30, s.GracefulShutdown)
	assert.Equal(t, "api.internal", s.DNSName)

	// AutoScale.
	require.NotNil(t, s.AutoScale)
	assert.Equal(t, 2, s.AutoScale.MinReplicas)
	assert.Equal(t, 10, s.AutoScale.MaxReplicas)

	// Security.
	require.NotNil(t, s.Security)
	assert.Equal(t, 1000, s.Security.RunAsUser)

	// UpdateStrategy.
	require.NotNil(t, s.UpdateStrategy)
	assert.Equal(t, "25%", s.UpdateStrategy.MaxSurge)
}

func TestBuild_BackwardCompatPortFromPorts(t *testing.T) {
	t.Parallel()

	r := &recipe.Recipe{
		Name: "compat-test",
		Slices: []recipe.Slice{
			{
				Name:  "svc",
				Image: "svc:latest",
				Ports: []recipe.PortSpec{
					{Name: "http", Port: 3000},
				},
			},
		},
	}

	plan, err := Build(r)
	require.NoError(t, err)
	require.Len(t, plan.Slices, 1)

	// Legacy Port field should be set from first Ports entry.
	assert.Equal(t, 3000, plan.Slices[0].Port)
}

func TestBuild_ExplicitKindAllTen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind     string
		wantKind SliceKind
		extra    func(s *recipe.Slice)
	}{
		{"web", SliceKindWeb, nil},
		{"api", SliceKindAPI, nil},
		{"worker", SliceKindWorker, nil},
		{"task", SliceKindTask, func(s *recipe.Slice) { s.RunOnce = true }},
		{"scheduled", SliceKindScheduled, func(s *recipe.Slice) { s.Schedule = "0 * * * *" }},
		{"database", SliceKindDatabase, nil},
		{"cache", SliceKindCache, nil},
		{"stateful", SliceKindStateful, func(s *recipe.Slice) { s.StatefulStorage = "5Gi" }},
		{"gateway", SliceKindGateway, nil},
		{"daemon", SliceKindDaemon, nil},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()

			s := recipe.Slice{Name: "svc", Kind: tt.kind, Image: "img:latest"}
			if tt.extra != nil {
				tt.extra(&s)
			}

			r := &recipe.Recipe{
				Name:   "test",
				Slices: []recipe.Slice{s},
			}

			plan, err := Build(r)
			require.NoError(t, err)
			require.Len(t, plan.Slices, 1)
			assert.Equal(t, tt.wantKind, plan.Slices[0].Kind)
		})
	}
}
