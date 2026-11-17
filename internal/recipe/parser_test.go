package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// --- Core workload directive tests ---

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

func TestParser_PortNumberRange(t *testing.T) {
	t.Parallel()

	directives := []struct {
		name     string
		template string
	}{
		{"public port", "open to the public on port %s"},
		{"named port", "on port %s as http"},
		{"sidecar port", "with sidecar envoy from myorg/envoy:1.26 on port %s"},
		{"tcp probe port", "readiness check on tcp port %s"},
	}

	ports := []struct {
		name    string
		literal string
		wantErr bool
	}{
		{"zero", "0", true},
		{"lowest usable", "1", false},
		{"highest usable", "65535", false},
		{"one above the range", "65536", true},
		{"too wide for an int", "99999999999999999999", true},
		// The lexer classifies any Unicode digit as a number, so this reaches the
		// port check rather than failing as a syntax error a step earlier.
		{"arabic-indic digit", "٥", true},
	}

	for _, directive := range directives {
		for _, port := range ports {
			t.Run(directive.name+"/"+port.name, func(t *testing.T) {
				t.Parallel()

				input := fmt.Sprintf("App: t\n\nApi:\n  from image myorg/api:1.0\n  "+directive.template, port.literal)

				_, err := NewParser(input).Parse()

				if !port.wantErr {
					require.NoError(t, err)

					return
				}

				require.Error(t, err)
				assert.Contains(t, err.Error(), "is not a usable port — a port must be a number between 1 and 65535")
				assert.NotContains(t, err.Error(), "strconv", "the message must be Mozza's own, not Atoi's")
				// Parse joins its errors with newlines and no single parse error
				// contains one, so the line count is what proves the rejected port
				// left no tokens behind for the dispatcher to complain about twice.
				assert.Len(t, strings.Split(err.Error(), "\n"), 1)
			})
		}
	}
}

func TestParser_NegativePortFailsTheGrammar(t *testing.T) {
	t.Parallel()

	// The lexer's isInteger never accepts a leading "-", so a negative port is not
	// a number token and never reaches the port check: the grammar rejects it
	// first, which is why the range rule only has to cover 0 and above.
	input := `App: t

Api:
  from image myorg/api:1.0
  on port -1 as http`

	_, err := NewParser(input).Parse()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected port number after 'on port'")
}

func TestParser_RejectedPortStillParsesTheRest(t *testing.T) {
	t.Parallel()

	input := `App: t

Api:
  from image myorg/api:1.0
  on port 70000 as http
  run 3 copies
  needs db

Db:
  postgres 16, 5Gi`

	recipe, err := NewParser(input).Parse()

	require.Error(t, err)
	assert.Len(t, strings.Split(err.Error(), "\n"), 1)

	// Recovery skips the rest of the offending line only, so everything after it
	// still lands in the AST.
	require.Len(t, recipe.Slices, 2)
	assert.Empty(t, recipe.Slices[0].Ports)
	assert.Equal(t, 3, recipe.Slices[0].Replicas)
	assert.Equal(t, []string{"db"}, recipe.Slices[0].Needs)
	assert.Equal(t, "db", recipe.Slices[1].Name)
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

// --- Container and config directive tests ---

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

// --- Operational directive tests ---

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

// A directive begins a line. The lexer separates tokens by whitespace, so
// without that rule a value written into one entry starts the next one: an
// imported `restart: "always run 5 copies"` would set `replicas: 5`. Every
// construct that reads one entry per line has the same exposure, so each
// is refused here, and the injected field is asserted to have stayed zero
// rather than only that an error came back.
func TestParser_EntryMustBeginItsOwnLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
		check   func(t *testing.T, r *Recipe)
	}{
		{
			name: "second directive on a section line",
			input: `App: t

Api:
  from image nginx
  restart always run 5 copies`,
			wantErr: `line 5, col 18: unexpected "run" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, r *Recipe) {
				t.Helper()
				require.Len(t, r.Slices, 1)
				assert.Zero(t, r.Slices[0].Replicas)
				assert.Equal(t, "always", r.Slices[0].RestartPolicy)
			},
		},
		{
			name: "second alias on an Images line",
			input: `App: t

Images:
  a: n1 b: n2`,
			wantErr: `line 4, col 9: unexpected "b:" after a complete alias — each alias goes on its own line`,
			check: func(t *testing.T, r *Recipe) {
				t.Helper()
				assert.Equal(t, map[string]string{"a": "n1"}, r.Aliases)
			},
		},
		{
			name: "second entry on a CRDs line",
			input: `App: t

CRDs:
  from http://a from http://b`,
			wantErr: `line 4, col 17: unexpected "from" after a complete entry — each entry goes on its own line`,
			check: func(t *testing.T, r *Recipe) {
				t.Helper()
				assert.Equal(t, []string{"http://a"}, r.CRDs)
			},
		},
		{
			name: "second init step on an init block line",
			input: `App: t

Svc:
  from image myorg/app:1.0
  before starting:
    run image a:1 with "x" run image b:1 with "y"`,
			wantErr: `line 6, col 28: unexpected "run" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, r *Recipe) {
				t.Helper()
				require.Len(t, r.Slices, 1)
				require.Len(t, r.Slices[0].InitSteps, 1)
				assert.Equal(t, "a:1", r.Slices[0].InitSteps[0].Image)
			},
		},
		{
			// An init block is the one construct that legitimately spans lines,
			// so a guard comparing against the previous directive's start line
			// would wave this through and set RestartPolicy.
			name: "directive after the last step of an init block",
			input: `App: t

Svc:
  from image myorg/app:1.0
  before starting:
    run image myorg/migrate:1.0 with "migrate up" restart always
  on port 8080`,
			wantErr: `line 6, col 51: unexpected "restart" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, r *Recipe) {
				t.Helper()
				require.Len(t, r.Slices, 1)
				assert.Empty(t, r.Slices[0].RestartPolicy)
				assert.Len(t, r.Slices[0].InitSteps, 1)
				// Recovery resumes at the next line.
				assert.Equal(t, 8080, r.Slices[0].Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewParser(tt.input)
			recipe, err := parser.Parse()

			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
			require.NotNil(t, recipe)
			tt.check(t, recipe)
		})
	}
}

// Five directives finish with words they may ignore ("copies running during
// updates", "at a time"). Each stops at the first word its own phrasing does not
// name, so trailing tokens reach the line rule and are refused rather than
// silently discarded.
func TestParser_DirectivesWithTrailingWordsRefuseASecondDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		wantErr string
		check   func(t *testing.T, s Slice)
	}{
		{
			name:    "update",
			line:    `update one at a time domain evil.example.com`,
			wantErr: `line 5, col 24: unexpected "domain" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Empty(t, s.Domain)
				assert.Equal(t, "1", s.UpdateStrategy.MaxSurge)
			},
		},
		{
			name:    "keep",
			line:    `keep at least 2 restart always`,
			wantErr: `line 5, col 19: unexpected "restart" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Empty(t, s.RestartPolicy)
				assert.Equal(t, 2, s.DisruptionBudget.MinAvailable)
			},
		},
		{
			name:    "never",
			line:    `never run two copies on the same node run 9 copies`,
			wantErr: `line 5, col 41: unexpected "run" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Zero(t, s.Replicas)
				assert.True(t, s.Scheduling.AntiAffinity)
			},
		},
		{
			name:    "start copies in order",
			line:    `start copies in order set FOO to bar`,
			wantErr: `line 5, col 25: unexpected "set" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Empty(t, s.Env)
				assert.True(t, s.OrderedStartup)
			},
		},
		{
			name:    "allow at most",
			line:    `allow at most 1 copies down during updates domain evil.example.com`,
			wantErr: `line 5, col 46: unexpected "domain" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Empty(t, s.Domain)
				assert.Equal(t, 1, s.DisruptionBudget.MaxUnavailable)
			},
		},
		{
			name:    "allow copies to find each other",
			line:    `allow copies to find each other run 9 copies`,
			wantErr: `line 5, col 35: unexpected "run" after a complete directive — each directive goes on its own line`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Zero(t, s.Replicas)
				assert.True(t, s.PeerDiscovery)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewParser("App: t\n\nApi:\n  from image nginx\n  " + tt.line + "\n")
			recipe, err := parser.Parse()

			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
			require.Len(t, recipe.Slices, 1)
			tt.check(t, recipe.Slices[0])
		})
	}
}

// A leftover word that begins no directive is a different mistake from two
// directives on one line, and telling its author to use two lines names a cause
// that is not the cause: every line below stays wrong however it is broken up.
func TestParser_LeftoverWordThatStartsNoDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		wantErr string
	}{
		{
			name:    "unsupported clause",
			line:    `graceful shutdown after 45s`,
			wantErr: `line 5, col 27: unexpected "45s" — the directive on this line does not accept it, and no directive starts with it`,
		},
		{
			name:    "singular where the directive reads plural",
			line:    `drop capability NET_RAW`,
			wantErr: `line 5, col 8: unexpected "capability" — the directive on this line does not accept it, and no directive starts with it`,
		},
		{
			name:    "two clauses joined by 'and'",
			line:    `limit cpu to 500m and memory to 256Mi`,
			wantErr: `line 5, col 21: unexpected "and" — the directive on this line does not accept it, and no directive starts with it`,
		},
		{
			name:    "misremembered restart policy",
			line:    `restart unless stopped`,
			wantErr: `line 5, col 18: unexpected "stopped" — the directive on this line does not accept it, and no directive starts with it`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewParser("App: t\n\nApi:\n  from image nginx\n  " + tt.line + "\n").Parse()

			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}

	// An Images block reads the same distinction with its own noun: a stray word
	// is not a second alias, which would be written "name:".
	_, err := NewParser("App: t\n\nImages:\n  a: n1 junk\n").Parse()

	require.Error(t, err)
	assert.Equal(t, `line 4, col 9: unexpected "junk" — the alias on this line does not accept it, and no directive starts with it`, err.Error())
}

// identDirectiveWords mirrors a switch in parseIdentDirective, and a word listed
// here but not handled there would have reportOffLine call a typo a misplaced
// directive. Every word must parse as a directive of its own.
func TestParser_IdentDirectiveWordsAreAllHandled(t *testing.T) {
	t.Parallel()

	for word := range identDirectiveWords {
		_, err := NewParser("App: t\n\nApi:\n  from image nginx\n  " + word + "\n").Parse()
		assert.NoError(t, err, "%q is in identDirectiveWords but parseIdentDirective does not handle it", word)
	}
}

// The line rule refuses a second entry, not a long one. Every directive below
// spans several tokens, and the init block spans several lines.
func TestParser_MultiTokenEntriesStillParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, s Slice)
	}{
		{
			name:  "network policy with two sources",
			input: `  only accept traffic from "a" and "b"`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				require.NotNil(t, s.NetworkPolicy)
				assert.Equal(t, []string{"a", "b"}, s.NetworkPolicy.AllowFrom)
			},
		},
		{
			name:  "resource limit",
			input: `  limit cpu to 500m`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Equal(t, "500m", s.CPULimit)
			},
		},
		{
			name:  "per-copy storage",
			input: `  each copy needs its own storage of 10Gi`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Equal(t, "10Gi", s.StatefulStorage)
			},
		},
		{
			name:  "health check with three timing clauses",
			input: `  health check /healthz every 30s, timeout 9s, wait 25s before starting`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				require.Len(t, s.Probes, 2)
				assert.Equal(t, 30, s.Probes[0].Interval)
				assert.Equal(t, 9, s.Probes[0].Timeout)
				assert.Equal(t, 25, s.Probes[0].Delay)
			},
		},
		{
			name: "init block over two lines",
			input: `  before starting:
    run image myorg/migrate:1.0 with "migrate up"
    run image myorg/init:1.0`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				require.Len(t, s.InitSteps, 2)
				assert.Equal(t, "myorg/migrate:1.0", s.InitSteps[0].Image)
				assert.Equal(t, "myorg/init:1.0", s.InitSteps[1].Image)
			},
		},
		{
			name: "comment line between directives",
			input: `  # why this port
  on port 8080`,
			check: func(t *testing.T, s Slice) {
				t.Helper()
				assert.Equal(t, 8080, s.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parser := NewParser("App: t\n\nApi:\n  from image nginx\n" + tt.input + "\n")
			recipe, err := parser.Parse()

			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, "nginx", recipe.Slices[0].Image)
			tt.check(t, recipe.Slices[0])
		})
	}
}

// Aliases and CRDs entries each on their own line stay a clean parse; the
// guard has to leave the shape every recipe in the tree already uses alone.
func TestParser_ImagesAndCRDsOnePerLine(t *testing.T) {
	t.Parallel()

	input := `App: t

Images:
  web: nginx:1.25
  api: myorg/api:1.0

CRDs:
  from http://a
  from http://b

Api:
  from web`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"web": "nginx:1.25", "api": "myorg/api:1.0"}, recipe.Aliases)
	assert.Equal(t, []string{"http://a", "http://b"}, recipe.CRDs)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "web", recipe.Slices[0].Image)
}
