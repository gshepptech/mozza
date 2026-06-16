package recipe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readTestdata reads a .mozza file from the testdata directory.
func readTestdata(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)

	return string(data)
}

func TestParser_Parse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantName   string
		wantSlices int
		wantErr    bool
		errSubstr  string
	}{
		{
			name:       "empty input",
			input:      "",
			wantName:   "",
			wantSlices: 0,
			wantErr:    false,
		},
		{
			name:       "app only",
			input:      "App: my-app",
			wantName:   "my-app",
			wantSlices: 0,
			wantErr:    false,
		},
		{
			name:       "unknown top-level token",
			input:      "App: test\n  deploy something",
			wantName:   "test",
			wantSlices: 0,
			wantErr:    true,
			errSubstr:  "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewParser(tt.input)
			recipe, err := parser.Parse()

			require.NotNil(t, recipe)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantName, recipe.Name)
			assert.Len(t, recipe.Slices, tt.wantSlices)
		})
	}
}

func TestParser_CommentsOnly(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "comments-only.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Empty(t, recipe.Name)
	assert.Empty(t, recipe.Slices)
}

func TestParser_SimpleAPI(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "simple-api.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "simple-api", recipe.Name)
	require.Len(t, recipe.Slices, 2)

	api := recipe.Slices[0]
	assert.Equal(t, "api", api.Name)
	assert.Equal(t, "myorg/api:1.0.0", api.Image)
	assert.Equal(t, 8080, api.Port)
	assert.True(t, api.Public)
	assert.Equal(t, "/healthz", api.Health)
	assert.Equal(t, 2, api.Replicas)
	assert.Equal(t, []string{"db"}, api.Needs)

	db := recipe.Slices[1]
	assert.Equal(t, "db", db.Name)
	assert.Equal(t, "postgres", db.Engine)
	assert.Equal(t, "16", db.Version)
	assert.Equal(t, "10Gi", db.Storage)
}

func TestParser_FullStack(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "full-stack.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "full-stack", recipe.Name)
	require.Len(t, recipe.Slices, 5)

	// Verify slice names in order (lowercased by parser).
	names := make([]string, len(recipe.Slices))
	for i, s := range recipe.Slices {
		names[i] = s.Name
	}

	assert.Equal(t, []string{"frontend", "backend", "worker", "db", "cache"}, names)

	// Verify frontend slice details.
	fe := recipe.Slices[0]
	assert.Equal(t, "myorg/frontend:2.4.1", fe.Image)
	assert.Equal(t, 3000, fe.Port)
	assert.True(t, fe.Public)
	assert.Equal(t, "/ready", fe.Health)
	assert.Equal(t, 3, fe.Replicas)
	assert.Equal(t, []string{"backend"}, fe.Needs)

	// Verify backend has multiple needs via "and".
	be := recipe.Slices[1]
	assert.False(t, be.Public)
	assert.Equal(t, 4, be.Replicas)
	assert.Equal(t, []string{"db", "cache"}, be.Needs)

	// Verify worker (no port, no public, no health).
	wk := recipe.Slices[2]
	assert.Zero(t, wk.Port)
	assert.False(t, wk.Public)
	assert.Empty(t, wk.Health)
	assert.Equal(t, 2, wk.Replicas)
	assert.Equal(t, []string{"db", "cache"}, wk.Needs)

	// Verify db uses engine shorthand.
	db := recipe.Slices[3]
	assert.Equal(t, "postgres", db.Engine)
	assert.Equal(t, "16", db.Version)
	assert.Equal(t, "50Gi", db.Storage)

	// Verify cache uses engine shorthand with storage.
	cache := recipe.Slices[4]
	assert.Equal(t, "redis", cache.Engine)
	assert.Equal(t, "7", cache.Version)
	assert.Equal(t, "1Gi", cache.Storage)
}

func TestParser_MissingBrace(t *testing.T) {
	t.Parallel()

	// In the new format, there are no braces — this testdata now has a
	// valid section with directives. It should parse successfully.
	input := readTestdata(t, "missing-brace.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "incomplete", recipe.Name)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "api", recipe.Slices[0].Name)
}

func TestParser_AllDirectives(t *testing.T) {
	t.Parallel()

	input := `App: directives-test

Full:
  from image myorg/svc:1.0
  open to the public on port 9090
  health check /health
  run 5 copies
  needs db and cache and queue`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Equal(t, "full", s.Name)
	assert.Equal(t, "myorg/svc:1.0", s.Image)
	assert.Equal(t, 9090, s.Port)
	assert.True(t, s.Public)
	assert.Equal(t, "/health", s.Health)
	assert.Equal(t, 5, s.Replicas)
	assert.Equal(t, []string{"db", "cache", "queue"}, s.Needs)
}

func TestParser_DatabaseShorthand(t *testing.T) {
	t.Parallel()

	input := `App: test

Db:
  postgres 16, 20Gi, daily backups`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	db := recipe.Slices[0]
	assert.Equal(t, "db", db.Name)
	assert.Equal(t, "postgres", db.Engine)
	assert.Equal(t, "16", db.Version)
	assert.Equal(t, "20Gi", db.Storage)
	assert.Equal(t, "daily", db.Backups)
}

func TestParser_CacheShorthand(t *testing.T) {
	t.Parallel()

	input := `App: test

Cache:
  redis 7, 1Gi`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	cache := recipe.Slices[0]
	assert.Equal(t, "cache", cache.Name)
	assert.Equal(t, "redis", cache.Engine)
	assert.Equal(t, "7", cache.Version)
	assert.Equal(t, "1Gi", cache.Storage)
}

func TestParser_UnknownDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  mystery directive here
  from image myorg/api:1.0
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown directive")
	require.NotNil(t, recipe)

	// The parser should recover and still parse subsequent directives.
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "myorg/api:1.0", recipe.Slices[0].Image)
	assert.Equal(t, 8080, recipe.Slices[0].Port)
}

func TestParser_MultipleSlices(t *testing.T) {
	t.Parallel()

	input := `App: multi

A:
  from image a:latest
  on port 3000

B:
  from image b:latest

C:
  postgres 16, 5Gi`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	require.Len(t, recipe.Slices, 3)

	assert.Equal(t, "a", recipe.Slices[0].Name)
	assert.Equal(t, 3000, recipe.Slices[0].Port)

	assert.Equal(t, "b", recipe.Slices[1].Name)

	assert.Equal(t, "c", recipe.Slices[2].Name)
	assert.Equal(t, "postgres", recipe.Slices[2].Engine)
	assert.Equal(t, "5Gi", recipe.Slices[2].Storage)
}

func TestParser_SliceLineTracking(t *testing.T) {
	t.Parallel()

	input := "App: test\n\nApi:\n  from image x:latest"
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	// "Api:" appears on line 3.
	assert.Equal(t, 3, recipe.Slices[0].Line)
}

func TestParser_NeedsCommaSeparated(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image x:latest
  needs db, cache, queue`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, []string{"db", "cache", "queue"}, recipe.Slices[0].Needs)
}

func TestParser_ErrorRecoveryMultiple(t *testing.T) {
	t.Parallel()

	// First section has a bad directive, second section is valid.
	input := `App: recover

Bad:
  unknown stuff here
  on port 3000

Good:
  from image good:latest`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.Error(t, err)
	require.NotNil(t, recipe)
	require.Len(t, recipe.Slices, 2)

	assert.Equal(t, "bad", recipe.Slices[0].Name)
	assert.Equal(t, "good", recipe.Slices[1].Name)
}

func TestParser_InvalidSyntaxTestdata(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "invalid-syntax.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.Error(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "broken", recipe.Name)
}

func TestParser_EmptyTestdata(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "empty.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Empty(t, recipe.Name)
	assert.Empty(t, recipe.Slices)
}

func TestParser_EnvLimitsTestdata(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "env-limits.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "env-limits", recipe.Name)
	assert.Equal(t, "staging", recipe.Namespace)
	require.Len(t, recipe.Slices, 2)

	api := recipe.Slices[0]
	assert.Equal(t, "api", api.Name)
	assert.Equal(t, "myorg/api:2.0", api.Image)
	assert.Equal(t, 8080, api.Port)
	assert.True(t, api.Public)
	assert.Equal(t, map[string]string{
		"DATABASE_URL": "postgres://db:5432/app",
		"LOG_LEVEL":    "info",
	}, api.Env)
	assert.Equal(t, "500m", api.CPULimit)
	assert.Equal(t, "256Mi", api.MemoryLimit)
	assert.Equal(t, "always", api.RestartPolicy)
	assert.Equal(t, "api.example.com", api.Domain)
	assert.Equal(t, []string{"db"}, api.Needs)

	db := recipe.Slices[1]
	assert.Equal(t, "db", db.Name)
	assert.Equal(t, "postgres", db.Engine)
}

func TestParser_SetDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  set DATABASE_URL to "postgres://localhost/db"
  set LOG_LEVEL to "debug"
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, map[string]string{
		"DATABASE_URL": "postgres://localhost/db",
		"LOG_LEVEL":    "debug",
	}, recipe.Slices[0].Env)
}

func TestParser_LimitDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  limit cpu to "500m"
  limit memory to "256Mi"
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "500m", recipe.Slices[0].CPULimit)
	assert.Equal(t, "256Mi", recipe.Slices[0].MemoryLimit)
}

func TestParser_RestartDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantPolicy string
	}{
		{
			name:       "restart always",
			input:      "App: test\n\nApi:\n  from image x:latest\n  restart always",
			wantPolicy: "always",
		},
		{
			name:       "restart unless-stopped",
			input:      "App: test\n\nApi:\n  from image x:latest\n  restart unless-stopped",
			wantPolicy: "unless-stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewParser(tt.input)
			recipe, err := parser.Parse()

			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantPolicy, recipe.Slices[0].RestartPolicy)
		})
	}
}

func TestParser_DomainDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  domain "api.example.com"
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "api.example.com", recipe.Slices[0].Domain)
}

func TestParser_SecretDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  secret DB_PASS from db-creds
  secret API_KEY from api-secrets key token
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Secrets, 2)

	assert.Equal(t, "DB_PASS", s.Secrets[0].EnvVar)
	assert.Equal(t, "db-creds", s.Secrets[0].SecretName)
	assert.Equal(t, "DB_PASS", s.Secrets[0].Key)

	assert.Equal(t, "API_KEY", s.Secrets[1].EnvVar)
	assert.Equal(t, "api-secrets", s.Secrets[1].SecretName)
	assert.Equal(t, "token", s.Secrets[1].Key)
}

func TestParser_PullSecretDirective(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  pull secret registry-creds
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "registry-creds", recipe.Slices[0].PullSecret)
}

func TestParser_NamespaceDirective(t *testing.T) {
	t.Parallel()

	input := `App: test
Namespace: production

Api:
  from image myorg/api:1.0
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "production", recipe.Namespace)
}

func TestParser_FullStackWithPhase2(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "full-stack.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "production", recipe.Namespace)

	// Check frontend has domain and restart.
	fe := recipe.Slices[0]
	assert.Equal(t, "shop.example.com", fe.Domain)
	assert.Equal(t, "always", fe.RestartPolicy)

	// Check backend has set and limit.
	be := recipe.Slices[1]
	assert.NotEmpty(t, be.Env)
	assert.Equal(t, "500m", be.CPULimit)
	assert.Equal(t, "512Mi", be.MemoryLimit)
	assert.Equal(t, "always", be.RestartPolicy)
}

// --- Item 3: Core workload directive tests ---

func TestParser_MultiPort(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  on port 8080 as http
  on port 9090 as grpc using grpc`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Zero(t, s.Port, "single Port should not be set when 'as' is used")
	require.Len(t, s.Ports, 2)
	assert.Equal(t, PortSpec{Name: "http", Port: 8080}, s.Ports[0])
	assert.Equal(t, PortSpec{Name: "grpc", Port: 9090, Protocol: "grpc"}, s.Ports[1])
}

func TestParser_MultiPortWithoutAs(t *testing.T) {
	t.Parallel()

	input := `App: test

Api:
  from image myorg/api:1.0
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, 8080, recipe.Slices[0].Port)
	assert.Empty(t, recipe.Slices[0].Ports)
}

func TestParser_ScheduleDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantCron string
	}{
		{
			name:     "daily at hour",
			input:    "App: t\n\nS:\n  from image x\n  run every day at 2am",
			wantCron: "0 2 * * *",
		},
		{
			name:     "every hour",
			input:    "App: t\n\nS:\n  from image x\n  run every hour",
			wantCron: "0 * * * *",
		},
		{
			name:     "every N minutes",
			input:    "App: t\n\nS:\n  from image x\n  run every 5 minutes",
			wantCron: "*/5 * * * *",
		},
		{
			name:     "weekday",
			input:    "App: t\n\nS:\n  from image x\n  run every monday at 9am",
			wantCron: "0 9 * * 1",
		},
		{
			name:     "raw cron",
			input:    "App: t\n\nS:\n  from image x\n  schedule \"15 3 * * 1-5\"",
			wantCron: "15 3 * * 1-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantCron, recipe.Slices[0].Schedule)
		})
	}
}

func TestParser_RunOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantRunOnce     bool
		wantParallelism int
		wantRetries     int
	}{
		{
			name:        "run once",
			input:       "App: t\n\nJ:\n  from image x\n  run once",
			wantRunOnce: true,
		},
		{
			name:        "run to completion",
			input:       "App: t\n\nJ:\n  from image x\n  run to completion",
			wantRunOnce: true,
		},
		{
			name:            "run once with parallel",
			input:           "App: t\n\nJ:\n  from image x\n  run once with 4 parallel",
			wantRunOnce:     true,
			wantParallelism: 4,
		},
		{
			name:        "run once with retries",
			input:       "App: t\n\nJ:\n  from image x\n  run once, retry up to 3 times",
			wantRunOnce: true,
			wantRetries: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			assert.Equal(t, tt.wantRunOnce, s.RunOnce)
			assert.Equal(t, tt.wantParallelism, s.Parallelism)
			assert.Equal(t, tt.wantRetries, s.Retries)
		})
	}
}

func TestParser_DaemonMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantDaemon     bool
		wantNodeLabels int
	}{
		{
			name:       "run on every node",
			input:      "App: t\n\nD:\n  from image x\n  run on every node",
			wantDaemon: true,
		},
		{
			name:           "run on every node labeled",
			input:          "App: t\n\nD:\n  from image x\n  run on every node labeled \"tier=edge\"",
			wantDaemon:     true,
			wantNodeLabels: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			assert.Equal(t, tt.wantDaemon, s.DaemonMode)
			if tt.wantNodeLabels > 0 {
				require.NotNil(t, s.Scheduling)
				assert.Len(t, s.Scheduling.NodeRequirements, tt.wantNodeLabels)
				assert.Equal(t, "tier", s.Scheduling.NodeRequirements[0].Key)
				assert.Equal(t, "edge", s.Scheduling.NodeRequirements[0].Value)
			}
		})
	}
}

func TestParser_Stateful(t *testing.T) {
	t.Parallel()

	input := `App: t

Store:
  from image myorg/store:1.0
  each copy needs its own storage of 10Gi
  start copies in order
  allow copies to find each other`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Equal(t, "10Gi", s.StatefulStorage)
	assert.True(t, s.OrderedStartup)
	assert.True(t, s.PeerDiscovery)
}

// --- Item 4: Container and config directive tests ---

func TestParser_InitBlock(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/app:1.0
  before starting:
    run image myorg/migrate:1.0 with "migrate up"
    run image myorg/init:1.0
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.InitSteps, 2)
	assert.Equal(t, "myorg/migrate:1.0", s.InitSteps[0].Image)
	assert.Equal(t, "migrate up", s.InitSteps[0].Command)
	assert.Equal(t, "myorg/init:1.0", s.InitSteps[1].Image)
	assert.Empty(t, s.InitSteps[1].Command)
	assert.Equal(t, 8080, s.Port)
}

func TestParser_Sidecar(t *testing.T) {
	t.Parallel()

	input := `App: t

Web:
  from image myorg/web:1.0
  with sidecar envoy from myorg/envoy:1.26 on port 9901`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Sidecars, 1)
	assert.Equal(t, "envoy", s.Sidecars[0].Name)
	assert.Equal(t, "myorg/envoy:1.26", s.Sidecars[0].Image)
	require.Len(t, s.Sidecars[0].Ports, 1)
	assert.Equal(t, 9901, s.Sidecars[0].Ports[0].Port)
}

func TestParser_Mounts(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  mount file "config/app.yaml" at /etc/app/config.yaml
  mount secret "db-creds" at /etc/secrets/db
  mount config "configs/" at /etc/app/configs`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Mounts, 3)

	assert.Equal(t, MountSpec{Type: "file", Source: "config/app.yaml", Target: "/etc/app/config.yaml"}, s.Mounts[0])
	assert.Equal(t, MountSpec{Type: "secret", Source: "db-creds", Target: "/etc/secrets/db"}, s.Mounts[1])
	assert.Equal(t, MountSpec{Type: "config-dir", Source: "configs/", Target: "/etc/app/configs"}, s.Mounts[2])
}

func TestParser_Permissions(t *testing.T) {
	t.Parallel()

	input := `App: t

Ctrl:
  from image myorg/ctrl:1.0
  needs permission to read pods
  needs permission to read and write deployments
  needs cluster-wide permission to manage secrets
  use account "controller-sa"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Permissions, 3)

	// "read pods"
	assert.Equal(t, []string{"get", "list", "watch"}, s.Permissions[0].Verbs)
	assert.Equal(t, []string{"pods"}, s.Permissions[0].Resources)
	assert.False(t, s.Permissions[0].ClusterWide)

	// "read and write deployments"
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch"}, s.Permissions[1].Verbs)
	assert.Equal(t, []string{"deployments"}, s.Permissions[1].Resources)
	assert.False(t, s.Permissions[1].ClusterWide)

	// "manage secrets" cluster-wide
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch", "delete"}, s.Permissions[2].Verbs)
	assert.Equal(t, []string{"secrets"}, s.Permissions[2].Resources)
	assert.True(t, s.Permissions[2].ClusterWide)

	assert.Equal(t, "controller-sa", s.ServiceAccount)
}

func TestParser_ProbeTypes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  readiness check /ready
  liveness check /alive
  startup check /started
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 3)
	assert.Equal(t, ProbeSpec{Type: "readiness", HTTPPath: "/ready"}, s.Probes[0])
	assert.Equal(t, ProbeSpec{Type: "liveness", HTTPPath: "/alive"}, s.Probes[1])
	assert.Equal(t, ProbeSpec{Type: "startup", HTTPPath: "/started"}, s.Probes[2])
}

func TestParser_ExecAndTCPProbes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  readiness check by running "cat /tmp/healthy"
  liveness check on tcp port 3306`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 2)
	assert.Equal(t, "readiness", s.Probes[0].Type)
	assert.Equal(t, "cat /tmp/healthy", s.Probes[0].Command)
	assert.Equal(t, "liveness", s.Probes[1].Type)
	assert.Equal(t, 3306, s.Probes[1].TCPPort)
}

func TestParser_HealthCheckBackwardCompat(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  health check /healthz
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Equal(t, "/healthz", s.Health)
	require.Len(t, s.Probes, 2)
	assert.Equal(t, "readiness", s.Probes[0].Type)
	assert.Equal(t, "/healthz", s.Probes[0].HTTPPath)
	assert.Equal(t, "liveness", s.Probes[1].Type)
	assert.Equal(t, "/healthz", s.Probes[1].HTTPPath)
}

func TestParser_HealthCheckWithTiming(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  health check /healthz every 10s, timeout 5s, wait 30s before starting
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 2)
	for _, probe := range s.Probes {
		assert.Equal(t, "/healthz", probe.HTTPPath)
		assert.Equal(t, 10, probe.Interval)
		assert.Equal(t, 5, probe.Timeout)
		assert.Equal(t, 30, probe.Delay)
	}
}

// --- Item 5: Operational directive tests ---

func TestParser_Lifecycle(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  before stopping, wait 30s
  before stopping, run "kill -SIGTERM 1"
  after starting, run "/hooks/warmup.sh"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Lifecycle)
	assert.Equal(t, 30, s.Lifecycle.PreStopWait)
	assert.Equal(t, "kill -SIGTERM 1", s.Lifecycle.PreStopCommand)
	assert.Equal(t, "/hooks/warmup.sh", s.Lifecycle.PostStartCommand)
}

func TestParser_SchedulingPreferences(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  prefer nodes labeled "zone=us-west-2a"
  require nodes labeled "disk=ssd"
  spread copies across zones
  never run two copies on the same node`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Scheduling)
	require.Len(t, s.Scheduling.NodePreferences, 1)
	assert.Equal(t, LabelConstraint{Key: "zone", Value: "us-west-2a"}, s.Scheduling.NodePreferences[0])
	require.Len(t, s.Scheduling.NodeRequirements, 1)
	assert.Equal(t, LabelConstraint{Key: "disk", Value: "ssd"}, s.Scheduling.NodeRequirements[0])
	assert.Equal(t, "zones", s.Scheduling.SpreadTopology)
	assert.True(t, s.Scheduling.AntiAffinity)
}

func TestParser_NetworkPolicy(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  only accept traffic from frontend and backend`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.NetworkPolicy)
	assert.Equal(t, []string{"frontend", "backend"}, s.NetworkPolicy.AllowFrom)
}

func TestParser_NetworkPolicyBlock(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  block all traffic except from namespace "production"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.NetworkPolicy)
	assert.Equal(t, []string{"production"}, s.NetworkPolicy.AllowNamespace)
}

func TestParser_AutoScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantMin    int
		wantMax    int
		wantCPU    int
		wantMemory int
	}{
		{
			name:    "cpu based",
			input:   "App: t\n\nS:\n  from image x\n  scale between 2 and 10 copies based on cpu 80%",
			wantMin: 2, wantMax: 10, wantCPU: 80,
		},
		{
			name:    "memory based",
			input:   "App: t\n\nS:\n  from image x\n  scale between 1 and 5 copies based on memory 70%",
			wantMin: 1, wantMax: 5, wantMemory: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			require.NotNil(t, s.AutoScale)
			assert.Equal(t, tt.wantMin, s.AutoScale.MinReplicas)
			assert.Equal(t, tt.wantMax, s.AutoScale.MaxReplicas)
			assert.Equal(t, tt.wantCPU, s.AutoScale.CPUTarget)
			assert.Equal(t, tt.wantMemory, s.AutoScale.MemoryTarget)
		})
	}
}

func TestParser_DisruptionBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantMin        int
		wantMaxUnavail int
	}{
		{
			name:    "min available",
			input:   "App: t\n\nS:\n  from image x\n  keep at least 2 copies running during updates",
			wantMin: 2,
		},
		{
			name:           "max unavailable",
			input:          "App: t\n\nS:\n  from image x\n  allow at most 1 copies down during updates",
			wantMaxUnavail: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			require.NotNil(t, s.DisruptionBudget)
			assert.Equal(t, tt.wantMin, s.DisruptionBudget.MinAvailable)
			assert.Equal(t, tt.wantMaxUnavail, s.DisruptionBudget.MaxUnavailable)
		})
	}
}

func TestParser_SecurityContext(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  run as user 1000
  run as group 1000
  drop all capabilities
  add capability NET_BIND_SERVICE
  read-only filesystem`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Security)
	assert.Equal(t, 1000, s.Security.RunAsUser)
	assert.Equal(t, 1000, s.Security.RunAsGroup)
	assert.Equal(t, []string{"ALL"}, s.Security.DropCapabilities)
	assert.Equal(t, []string{"NET_BIND_SERVICE"}, s.Security.AddCapabilities)
	assert.True(t, s.Security.ReadOnlyRoot)
}

func TestParser_UpdateStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantSpec *UpdateStrategySpec
	}{
		{
			name:     "one at a time",
			input:    "App: t\n\nS:\n  from image x\n  update one at a time",
			wantSpec: &UpdateStrategySpec{MaxSurge: "1", MaxUnavailable: "0"},
		},
		{
			name:     "percentage",
			input:    "App: t\n\nS:\n  from image x\n  update 25% at a time",
			wantSpec: &UpdateStrategySpec{MaxSurge: "25%", MaxUnavailable: "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantSpec, recipe.Slices[0].UpdateStrategy)
		})
	}
}

func TestParser_GracefulShutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantSeconds int
	}{
		{
			name:        "seconds",
			input:       "App: t\n\nS:\n  from image x\n  graceful shutdown 60s",
			wantSeconds: 60,
		},
		{
			name:        "minutes",
			input:       "App: t\n\nS:\n  from image x\n  graceful shutdown 2m",
			wantSeconds: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantSeconds, recipe.Slices[0].GracefulShutdown)
		})
	}
}

func TestParser_KindOverride(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  kind StatefulSet`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "StatefulSet", recipe.Slices[0].Kind)
}

func TestParser_DNSName(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  reachable as "my-service.local"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "my-service.local", recipe.Slices[0].DNSName)
}

func TestParser_FullTaxonomyTestdata(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "full-taxonomy.mozza")
	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.NotNil(t, recipe)
	assert.Equal(t, "taxonomy-test", recipe.Name)
	assert.Equal(t, "production", recipe.Namespace)

	// Verify we parsed a large number of slices without errors.
	assert.True(t, len(recipe.Slices) > 25, "expected >25 slices, got %d", len(recipe.Slices))

	// Spot check a few key slices.
	sliceByName := make(map[string]Slice, len(recipe.Slices))
	for _, s := range recipe.Slices {
		sliceByName[s.Name] = s
	}

	// ApiGateway: multi-port
	gw := sliceByName["apigateway"]
	require.Len(t, gw.Ports, 2)
	assert.Equal(t, "http", gw.Ports[0].Name)
	assert.Equal(t, "grpc", gw.Ports[1].Name)

	// LogCollector: daemon
	lc := sliceByName["logcollector"]
	assert.True(t, lc.DaemonMode)

	// SessionStore: stateful
	ss := sliceByName["sessionstore"]
	assert.Equal(t, "10Gi", ss.StatefulStorage)
	assert.True(t, ss.OrderedStartup)
	assert.True(t, ss.PeerDiscovery)

	// Controller: permissions
	ctrl := sliceByName["controller"]
	require.Len(t, ctrl.Permissions, 3)
	assert.Equal(t, "controller-sa", ctrl.ServiceAccount)

	// SecureContainer: security
	sec := sliceByName["securecontainer"]
	require.NotNil(t, sec.Security)
	assert.Equal(t, 1000, sec.Security.RunAsUser)
	assert.True(t, sec.Security.ReadOnlyRoot)

	// DnsService
	dns := sliceByName["dnsservice"]
	assert.Equal(t, "my-service.local", dns.DNSName)
}

func TestParser_PlainEnglishToCron(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantCron string
		wantErr  bool
	}{
		{"day at 2am", "0 2 * * *", false},
		{"day at 2pm", "0 14 * * *", false},
		{"hour", "0 * * * *", false},
		{"5 minutes", "*/5 * * * *", false},
		{"monday at 9am", "0 9 * * 1", false},
		{"friday at 5pm", "0 17 * * 5", false},
		{"garbage text", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			cron, err := plainEnglishToCron(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCron, cron)
			}
		})
	}
}

func TestParser_MapVerb(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"get", "list", "watch"}, mapVerb("read"))
	assert.Equal(t, []string{"create", "update", "patch"}, mapVerb("write"))
	assert.Equal(t, []string{"delete"}, mapVerb("delete"))
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch", "delete"}, mapVerb("manage"))
	assert.Nil(t, mapVerb("unknown"))
}

func TestParser_SpreadNodes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  spread copies across nodes`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	require.NotNil(t, recipe.Slices[0].Scheduling)
	assert.Equal(t, "nodes", recipe.Slices[0].Scheduling.SpreadTopology)
}
