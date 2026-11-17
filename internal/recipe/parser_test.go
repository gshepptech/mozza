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

	// The recipe grammar has no braces, so this testdata is an ordinary
	// section with directives and must parse successfully.
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
