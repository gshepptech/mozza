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
