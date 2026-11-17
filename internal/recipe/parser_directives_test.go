package recipe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
