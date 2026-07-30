package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

func TestComposeToRecipe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  string
		contains []string
	}{
		{
			name:    "invalid yaml",
			input:   "::not yaml",
			wantErr: "parsing compose YAML",
		},
		{
			name:    "no services",
			input:   "version: '3'\n",
			wantErr: "no services found",
		},
		{
			name: "single service with image",
			input: `
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`,
			contains: []string{
				"App: web",
				"Web:",
				"from image nginx:latest",
				"open to the public on port 80",
			},
		},
		{
			name: "postgres engine detection",
			input: `
services:
  db:
    image: postgres:16-alpine
`,
			contains: []string{
				"Db:",
				"postgres 16, 10Gi",
			},
		},
		{
			name: "redis engine detection",
			input: `
services:
  cache:
    image: redis:7
`,
			contains: []string{
				"Cache:",
				"redis 7",
			},
		},
		{
			name: "depends_on as list",
			input: `
services:
  api:
    image: myapi:latest
    depends_on:
      - db
      - cache
  cache:
    image: redis:7
  db:
    image: postgres:16
`,
			contains: []string{
				"needs db and cache",
			},
		},
		{
			name: "depends_on as map",
			input: `
services:
  api:
    image: myapi:latest
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres:16
`,
			contains: []string{
				"needs db",
			},
		},
		{
			name: "environment as map",
			input: `
services:
  web:
    image: myapp:latest
    environment:
      DB_HOST: localhost
      DB_PORT: "5432"
`,
			contains: []string{
				`set DB_HOST to "localhost"`,
				`set DB_PORT to "5432"`,
			},
		},
		{
			name: "environment as list",
			input: `
services:
  web:
    image: myapp:latest
    environment:
      - DB_HOST=localhost
      - DB_PORT=5432
`,
			contains: []string{
				`set DB_HOST to "localhost"`,
				`set DB_PORT to "5432"`,
			},
		},
		{
			name: "healthcheck with curl",
			input: `
services:
  web:
    image: myapp:latest
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
`,
			contains: []string{
				"health check /health",
			},
		},
		{
			name: "replicas",
			input: `
services:
  web:
    image: myapp:latest
    deploy:
      replicas: 3
`,
			contains: []string{
				"run 3 copies",
			},
		},
		{
			name: "restart policy",
			input: `
services:
  web:
    image: myapp:latest
    restart: unless-stopped
`,
			contains: []string{
				"restart unless-stopped",
			},
		},
		{
			name: "build context warning",
			input: `
services:
  web:
    build: .
`,
			contains: []string{
				"WARNING: build context detected",
			},
		},
		{
			name: "named volumes produce storage",
			input: `
services:
  db:
    image: mydb:latest
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{
				"storage 10Gi",
				"# mount pgdata at /var/lib/postgresql/data",
			},
		},
		{
			name: "secrets reference",
			input: `
services:
  web:
    image: myapp:latest
    secrets:
      - source: db-password
        target: /run/secrets/db-password
secrets:
  db-password:
    external: true
`,
			contains: []string{
				"secret DB_PASSWORD from db-password",
			},
		},
		{
			name: "multi-service app name",
			input: `
services:
  api:
    image: myapi:latest
  worker:
    image: myworker:latest
`,
			contains: []string{
				"App: api-stack",
			},
		},
		{
			name: "reserved word in service name",
			input: `
services:
  app:
    image: myapp:latest
`,
			contains: []string{
				"App-svc:",
			},
		},
		{
			name: "port without host mapping",
			input: `
services:
  web:
    image: myapp:latest
    ports:
      - "3000"
`,
			contains: []string{
				"on port 3000",
			},
		},
		{
			name: "no image no build",
			input: `
services:
  web: {}
`,
			contains: []string{
				"WARNING: no image specified",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ComposeToRecipe(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.contains {
				assert.Contains(t, result, want, "missing: %s\n\nfull output:\n%s", want, result)
			}
		})
	}
}

func TestComposeToRecipeAST(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantErr      string
		wantSlices   int
		wantWarnings int
		check        func(t *testing.T, r *recipe.Recipe, w []Warning)
	}{
		{
			name:    "invalid yaml",
			input:   "::invalid",
			wantErr: "parsing compose YAML",
		},
		{
			name: "basic service produces slice",
			input: `
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    environment:
      FOO: bar
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				assert.Equal(t, "Web", s.Name)
				assert.Equal(t, "nginx:latest", s.Image)
				assert.Equal(t, 80, s.Port)
				assert.True(t, s.Public)
				assert.Equal(t, "bar", s.Env["FOO"])
			},
		},
		{
			name: "build context produces warning",
			input: `
services:
  api:
    build: ./api
`,
			wantSlices:   1,
			wantWarnings: 1,
		},
		{
			name: "secrets mapped to SecretRef",
			input: `
services:
  web:
    image: myapp:latest
    secrets:
      - source: db-pass
        target: /run/secrets/db-pass
secrets:
  db-pass:
    external: true
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				require.Len(t, s.Secrets, 1)
				assert.Equal(t, "DB_PASS", s.Secrets[0].EnvVar)
				assert.Equal(t, "db-pass", s.Secrets[0].SecretName)
			},
		},
		{
			name: "internal network produces warning",
			input: `
services:
  web:
    image: myapp:latest
networks:
  internal:
    internal: true
`,
			wantSlices:   1,
			wantWarnings: 1,
		},
		{
			name: "named volumes produce storage and mount",
			input: `
services:
  db:
    image: mydb:latest
    volumes:
      - data:/var/lib/data
volumes:
  data:
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				assert.Equal(t, "10Gi", s.Storage)
				require.Len(t, s.Mounts, 1)
				assert.Equal(t, "/var/lib/data", s.Mounts[0].Target)
				assert.Equal(t, "data", s.Mounts[0].Source)
			},
		},
		{
			name: "service networks produce NetworkPolicy",
			input: `
services:
  web:
    image: myapp:latest
    networks:
      - frontend
      - backend
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				require.NotNil(t, s.NetworkPolicy)
				assert.ElementsMatch(t, []string{"frontend", "backend"}, s.NetworkPolicy.AllowFrom)
			},
		},
		{
			name: "replicas and restart in AST",
			input: `
services:
  web:
    image: myapp:latest
    deploy:
      replicas: 5
    restart: on-failure
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				assert.Equal(t, 5, s.Replicas)
				assert.Equal(t, "on-failure", s.RestartPolicy)
			},
		},
		{
			name: "engine detected in AST",
			input: `
services:
  db:
    image: postgres:15-bookworm
`,
			wantSlices: 1,
			check: func(t *testing.T, r *recipe.Recipe, _ []Warning) {
				s := r.Slices[0]
				assert.Equal(t, "postgres", s.Engine)
				assert.Equal(t, "15", s.Version)
				assert.Equal(t, "10Gi", s.Storage)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, warnings, err := ComposeToRecipeAST([]byte(tt.input))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.wantSlices > 0 {
				assert.Len(t, r.Slices, tt.wantSlices)
			}
			if tt.wantWarnings > 0 {
				assert.GreaterOrEqual(t, len(warnings), tt.wantWarnings)
			}
			if tt.check != nil {
				tt.check(t, r, warnings)
			}
		})
	}
}

func TestParseEngineImage_Extended(t *testing.T) {
	tests := []struct {
		image       string
		wantEngine  string
		wantVersion string
	}{
		{"postgres:16", "postgres", "16"},
		{"postgres:16-alpine", "postgres", "16"},
		{"docker.io/library/postgres:15", "postgres", "15"},
		{"redis:7", "redis", "7"},
		{"redis:latest", "redis", "7"},
		{"mysql:8.0", "mysql", "8.0"},
		{"mariadb:11", "mysql", "11"},
		{"nginx:latest", "", ""},
		{"myapp:v1", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			engine, version := parseEngineImage(tt.image)
			assert.Equal(t, tt.wantEngine, engine)
			assert.Equal(t, tt.wantVersion, version)
		})
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"web", "Web"},
		{"api", "Api"},
		{"app", "App-svc"},
		{"namespace", "Namespace-svc"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, capitalize(tt.input))
		})
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input      string
		wantPort   int
		wantPublic bool
	}{
		{"8080:80", 80, true},
		{"3000", 3000, false},
		{"8080:80/tcp", 80, true},
		{"9090:9090/udp", 9090, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			port, public := parsePort(tt.input)
			assert.Equal(t, tt.wantPort, port)
			assert.Equal(t, tt.wantPublic, public)
		})
	}
}

func TestWarningStruct(t *testing.T) {
	w := Warning{
		Feature:  "build",
		Message:  "needs pre-built image",
		Severity: "warn",
	}
	assert.Equal(t, "build", w.Feature)
	assert.Equal(t, "warn", w.Severity)
}

func TestComposeToRecipe_FullStack(t *testing.T) {
	input := `
services:
  web:
    image: myapp:latest
    ports:
      - "8080:3000"
    depends_on:
      - api
      - db
    environment:
      API_URL: http://api:8080
    restart: always
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:3000/health || exit 1"]
    deploy:
      replicas: 2
  api:
    image: myapi:v2
    ports:
      - "8081:8080"
    depends_on:
      - db
    environment:
      - DATABASE_URL=postgres://db:5432/app
    secrets:
      - source: api-key
        target: /run/secrets/api-key
  db:
    image: postgres:16-alpine
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: admin
volumes:
  pgdata:
secrets:
  api-key:
    external: true
`
	result, err := ComposeToRecipe(input)
	require.NoError(t, err)

	// Verify all services present.
	assert.Contains(t, result, "App: api-stack")
	assert.Contains(t, result, "Api:")
	assert.Contains(t, result, "Db:")
	assert.Contains(t, result, "Web:")

	// Web service specifics.
	assert.Contains(t, result, "open to the public on port 3000")
	assert.Contains(t, result, "needs api and db")
	assert.Contains(t, result, "run 2 copies")
	assert.Contains(t, result, "restart always")
	assert.Contains(t, result, "health check /health")

	// API service specifics.
	assert.Contains(t, result, "secret API_KEY from api-key")

	// DB service specifics.
	assert.Contains(t, result, "postgres 16, 10Gi")

	// No blank lines at end of services.
	lines := strings.Split(strings.TrimSpace(result), "\n")
	assert.Greater(t, len(lines), 10, "expected multi-line output")
}

// TestImportedHealthCheckAlwaysHasAPort pins the port dimension of the import
// invariant: an imported health check always has a port to check, so mozza import
// never writes an unaimed one for mozza validate to reject. Both authoring paths
// are checked because `mozza import` uses both — ComposeToRecipe writes the file,
// and ComposeToRecipeAST supplies the warnings printed alongside it.
//
// It is deliberately not a claim that every compose file imports to a valid
// recipe. Two inputs still do not, both identically so before this change and for
// reasons that have nothing to do with ports: a build-context-only service imports
// to a comment with no kind to infer, and a health check URL carrying a query
// string imports a path the health path format rule rejects.
func TestImportedHealthCheckAlwaysHasAPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
		check    func(t *testing.T, s recipe.Slice, w []Warning)
	}{
		{
			name: "healthcheck url supplies the port when compose publishes none",
			input: `
services:
  web:
    image: myorg/web:1.0
    expose:
      - "3000"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
`,
			contains: []string{"on port 3000", "health check /health"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, 3000, s.Port, "the port in the check URL should be carried over")
				assert.Equal(t, "/health", s.Health)
				assert.Empty(t, w, "nothing was dropped, so there is nothing to warn about")
			},
		},
		{
			name: "a published port wins over the port in the check url",
			input: `
services:
  web:
    image: myorg/web:1.0
    ports:
      - "8080:3000"
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:9999/health || exit 1"]
`,
			contains: []string{"open to the public on port 3000", "health check /health"},
			absent:   []string{"on port 9999"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3000, s.Port)
			},
		},
		{
			name: "a check with no port anywhere is dropped and explained",
			input: `
services:
  web:
    image: myorg/web:1.0
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
`,
			contains: []string{`# WARNING: health check /health dropped`, `add "on port <n>", then restore "health check /health"`},
			// Anchored to a directive line, so the comment's own wording — which
			// quotes the check it dropped — cannot satisfy it.
			absent: []string{"\n  health check /health"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Health, "an unaimed check must not reach the recipe")
				assert.Zero(t, s.Port)
				require.Len(t, w, 1)
				assert.Equal(t, "healthcheck", w[0].Feature)
				assert.Equal(t, "warn", w[0].Severity)
				assert.Contains(t, w[0].Message, "no published port and none in the check URL")
			},
		},
		{
			name: "a non-http check is left alone",
			input: `
services:
  db:
    image: postgres:16
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "postgres"]
`,
			absent: []string{"health check"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Health)
				assert.Empty(t, w)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The text path: what mozza import writes to disk.
			text, err := ComposeToRecipe(tt.input)
			require.NoError(t, err)

			for _, want := range tt.contains {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range tt.absent {
				assert.NotContains(t, text, unwanted)
			}

			parsed, err := recipe.NewParser(text).Parse()
			require.NoError(t, err, "the written recipe must parse:\n%s", text)

			ap, err := plan.Build(parsed)
			require.NoError(t, err, "the written recipe must build:\n%s", text)
			require.NoError(t, plan.Validate(ap), "the written recipe must validate:\n%s", text)

			// The AST path: what the server reports and what the warnings come from.
			r, warnings, err := ComposeToRecipeAST([]byte(tt.input))
			require.NoError(t, err)
			require.Len(t, r.Slices, 1)

			astPlan, err := plan.Build(r)
			require.NoError(t, err)
			require.NoError(t, plan.Validate(astPlan), "the imported AST must validate too")

			tt.check(t, r.Slices[0], warnings)
		})
	}
}
