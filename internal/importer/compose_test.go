package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"

	"github.com/gshepptech/mozza/internal/k8s"
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
			name: "named volume on an engine-less service is announced as dropped",
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
				"# WARNING: volume pgdata at /var/lib/postgresql/data dropped — this slice has " +
					"no database or cache engine to carry it",
				`add "each copy needs its own storage of 10Gi" to get a 10Gi volume per copy, ` +
					"mounted at /data rather than /var/lib/postgresql/data",
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
			name: "named volume produces a warning, not storage and not a mount",
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
			// Exactly one named volume on one engine-less service, so one warning.
			wantWarnings: 1,
			check: func(t *testing.T, r *recipe.Recipe, w []Warning) {
				s := r.Slices[0]
				assert.Empty(t, s.Storage, "no engine shorthand can carry a size here")
				assert.Empty(t, s.Mounts,
					"a mount would claim a volume named data that mozza never creates")

				require.Len(t, w, 1)
				assert.Equal(t, "volumes", w[0].Feature)
				assert.Equal(t, "warn", w[0].Severity)
				assert.Contains(t, w[0].Message,
					`service "db" volume data at /var/lib/data dropped — this service has no `+
						"database or cache engine to carry it")
				assert.Contains(t, w[0].Message, `add "each copy needs its own storage of 10Gi"`)
			},
		},
		{
			// AllowFrom holds slice names and plan.Validate rejects anything else,
			// so compose network names must never land there. The importer sets no
			// policy at all, from any compose file, and reports the boundary as a
			// warning instead.
			name: "service networks set no policy and are reported instead",
			input: `
services:
  web:
    image: myapp:latest
    networks:
      - frontend
      - backend
  api:
    image: myapi:latest
    networks:
      - backend
`,
			wantSlices: 2,
			check: func(t *testing.T, r *recipe.Recipe, w []Warning) {
				for _, s := range r.Slices {
					assert.Nil(t, s.NetworkPolicy, "slice %q carries no boundary", s.Name)
				}

				assert.Equal(t, []string{
					declares("backend", "api", "web"),
					declares("frontend", "web"),
					remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
					remedy("web", []string{"backend", "frontend"}, `"api" and "web"`, "Web"),
				}, networksWarnings(w), "and both boundaries are named")
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

// TestSplitVolume covers the shapes compose's short volume syntax really takes.
// The importer decides from the mount path both whether a volume is carried and
// what to tell the user about it, so a mode field left on the end of the path is
// a false statement in a recipe, not a cosmetic slip.
func TestSplitVolume(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSource string
		wantTarget string
	}{
		{"named volume", "mysqldata:/var/lib/mysql", "mysqldata", "/var/lib/mysql"},
		{"read-write", "mysqldata:/var/lib/mysql:rw", "mysqldata", "/var/lib/mysql"},
		{"read-only", "mysqldata:/var/lib/mysql:ro", "mysqldata", "/var/lib/mysql"},
		{"selinux shared", "mysqldata:/var/lib/mysql:z", "mysqldata", "/var/lib/mysql"},
		{"selinux private", "mysqldata:/var/lib/mysql:Z", "mysqldata", "/var/lib/mysql"},
		{"macos cached", "mysqldata:/var/lib/mysql:cached", "mysqldata", "/var/lib/mysql"},
		{"macos delegated", "mysqldata:/var/lib/mysql:delegated", "mysqldata", "/var/lib/mysql"},
		{"several modes at once", "pg:/var/lib/pg:rw,z", "pg", "/var/lib/pg"},
		{"bind mount", "./src:/app", "./src", "/app"},
		{"bind mount read-only", "./src:/app:ro", "./src", "/app"},
		{"absolute host path", "/srv/data:/app/data:ro", "/srv/data", "/app/data"},
		// A drive letter is not a volume name, and "\data" is not a container path.
		{"windows source", `C:\data:/app/data`, `C:\data`, "/app/data"},
		{"windows source with mode", `C:\data:/app/data:ro`, `C:\data`, "/app/data"},
		{"name with no path", "appdata", "appdata", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, target := splitVolume(tt.input)
			assert.Equal(t, tt.wantSource, source)
			assert.Equal(t, tt.wantTarget, target)
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
// are checked because `mozza import` uses both: ComposeToRecipe writes the file,
// and ComposeToRecipeAST supplies the warnings printed alongside it.
//
// It is deliberately not a claim that every compose file imports to a valid
// recipe. Two inputs do not, for
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
			// Anchored to a directive line, so the comment's own wording (which
			// quotes the check it dropped) cannot satisfy it.
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

// TestImportedStorageIsAlwaysAFormTheParserAccepts pins the storage dimension of
// the same import invariant. A recipe states storage in exactly one place (the
// engine shorthand's size argument, "postgres 16, 10Gi"), so that is the only
// form the importer may write, and a compose volume that cannot be expressed
// there is announced rather than dropped in silence. `mozza import` uses both
// authoring paths, so both are checked, and they are checked against each other:
// the size the written recipe parses back to must equal the size the AST carries,
// and so must the mounts, which is the same claim about the other half of what a
// volume asks for. One import cannot hand back two different sets of volumes.
func TestImportedStorageIsAlwaysAFormTheParserAccepts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
		check    func(t *testing.T, s recipe.Slice, w []Warning)
	}{
		{
			name: "a database engine carries its volume on its own shorthand",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{"postgres 16, 10Gi", "# mount pgdata at /var/lib/postgresql/data"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage)
				assert.Empty(t, s.Mounts,
					"the shorthand's own claim template holds this volume; a second one "+
						"would name a claim mozza never creates and leave the real one unmounted")
				assert.Empty(t, w, "the shorthand carries the volume, so nothing was lost")
			},
		},
		{
			name: "a cache with a volume gets the size the volume implies",
			input: `
services:
  cache:
    image: redis:7
    volumes:
      - cachedata:/data
volumes:
  cachedata:
`,
			contains: []string{"redis 7, 10Gi", "# mount cachedata at /data"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage, "a cache with a volume is meant to persist")
				assert.Empty(t, w, "compose mounted it where redis mounts it, so nothing was lost")
			},
		},
		{
			name: "a cache with no volume stays ephemeral",
			input: `
services:
  cache:
    image: redis:7
`,
			contains: []string{"redis 7"},
			absent:   []string{"redis 7, "},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Storage, "compose asked for no volume, so neither should the recipe")
				assert.Empty(t, w)
			},
		},
		{
			name: "an engine-less service is told its volume was dropped",
			input: `
services:
  web:
    image: myorg/web:1.0
    volumes:
      - appdata:/var/lib/app
volumes:
  appdata:
`,
			contains: []string{
				// The reason clause is pinned with the rest: a reason phrased as a
				// rule about the grammar would contradict the remedy that follows
				// it, since "each copy needs its own storage" is storage a recipe
				// can state.
				"# WARNING: volume appdata at /var/lib/app dropped — this slice has no " +
					"database or cache engine to carry it",
				// Pinned verbatim because the remedy is advice a user will follow
				// literally: TestTheStatefulRemedyMountsWhereItSaysItDoes proves
				// this is where the volume lands, and this keeps the sentence from
				// drifting back to a promise the compiler does not keep.
				`add "each copy needs its own storage of 10Gi" to get a 10Gi volume per copy, ` +
					"mounted at /data rather than /var/lib/app",
			},
			// The recipe is not the place to name a Kubernetes object; stderr is.
			absent: []string{"# mount appdata", "StatefulSet"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Storage, "the compiler would discard it, and no directive can write it")
				require.Len(t, w, 1)
				assert.Equal(t, "volumes", w[0].Feature)
				assert.Equal(t, "warn", w[0].Severity)
				assert.Contains(t, w[0].Message,
					`service "web" volume appdata at /var/lib/app dropped — this service has no `+
						"database or cache engine to carry it")
				assert.Contains(t, w[0].Message,
					`add "each copy needs its own storage of 10Gi" to deploy the service as a `+
						"StatefulSet with a 10Gi volume per copy, mounted at /data rather than /var/lib/app")
			},
		},
		{
			name: "a volume with no mount path is named without one",
			input: `
services:
  web:
    image: myorg/web:1.0
    volumes:
      - appdata
volumes:
  appdata:
`,
			contains: []string{"# WARNING: volume appdata dropped", "mounted at /data\n"},
			// Nothing to displace, so the remedy names only where the volume lands.
			absent: []string{"volume appdata at", "rather than"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Mounts, "compose named no path to mount it at")
				require.Len(t, w, 1)
				assert.Contains(t, w[0].Message, `service "web" volume appdata dropped`)
				assert.Contains(t, w[0].Message, "mounted at /data")
			},
		},
		{
			name: "a bind mount is not a volume",
			input: `
services:
  web:
    image: myorg/web:1.0
    volumes:
      - ./src:/app
`,
			absent: []string{"# WARNING: volume", "# mount"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Storage)
				assert.Empty(t, s.Mounts)
				assert.Empty(t, w)
			},
		},
		{
			name: "an access mode is not part of the path a volume asks for",
			input: `
services:
  maria:
    image: mariadb:11
    volumes:
      - mysqldata:/var/lib/mysql:rw
volumes:
  mysqldata:
`,
			contains: []string{"mysql 11, 10Gi", "# mount mysqldata at /var/lib/mysql"},
			// Compose puts ":rw" where a reader expects the rest of a path, and it is
			// the commonest thing to find after one. Carried into the mount path it
			// makes a volume the engine does carry look like one it does not.
			absent: []string{"/var/lib/mysql:rw", "not carried"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage)
				assert.Empty(t, w, "compose mounted it where mysql mounts it, so nothing was lost")
			},
		},
		{
			name: "an SELinux label is no more part of the path than an access mode",
			input: `
services:
  web:
    image: myorg/web:1.0
    volumes:
      - appdata:/var/lib/app:z
volumes:
  appdata:
`,
			contains: []string{
				"# WARNING: volume appdata at /var/lib/app dropped",
				"mounted at /data rather than /var/lib/app",
			},
			absent: []string{"/var/lib/app:z"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1)
				assert.Contains(t, w[0].Message, `volume appdata at /var/lib/app dropped`)
				assert.NotContains(t, w[0].Message, ":z",
					"a label the container is read through is not somewhere a volume lives")
			},
		},
		{
			name: "an engine's second volume is as uncarried as any other",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - pg:/var/lib/postgresql/data
      - wal:/wal
volumes:
  pg:
  wal:
`,
			contains: []string{
				"postgres 16, 10Gi",
				"# mount pg at /var/lib/postgresql/data",
				"# WARNING: volume wal at /wal not carried — a postgres slice gets one volume, " +
					"mounted at /var/lib/postgresql/data",
			},
			// The engine shorthand already holds all the storage there is to hold,
			// so the remedy for an engine-less slice would be wrong advice here.
			absent: []string{"# mount wal", "each copy needs its own storage"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage)
				require.Len(t, w, 1, "one volume rides on the shorthand; the other rides on nothing")
				assert.Equal(t, "volumes", w[0].Feature)
				assert.Contains(t, w[0].Message,
					`service "db" volume wal at /wal not carried — a postgres slice gets one volume, `+
						"mounted at /var/lib/postgresql/data")
			},
		},
		{
			name: "an engine volume compose put somewhere else is not carried there",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/pgdata
volumes:
  pgdata:
`,
			contains: []string{
				"postgres 16, 10Gi",
				"# WARNING: volume pgdata at /pgdata not carried — a postgres slice gets one volume, " +
					"mounted at /var/lib/postgresql/data",
			},
			absent: []string{"# mount pgdata"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage, "the storage is real; only compose's path for it is not")
				require.Len(t, w, 1)
			},
		},
		{
			name: "both named volumes are accounted for, not just the first",
			input: `
services:
  web:
    image: myorg/web:1.0
    volumes:
      - appdata:/var/lib/app
      - uploads:/var/lib/uploads
volumes:
  appdata:
  uploads:
`,
			contains: []string{
				"# WARNING: volume appdata at /var/lib/app dropped",
				"# WARNING: volume uploads at /var/lib/uploads dropped",
			},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Mounts,
					"both were reported dropped, so neither may come back as a mount")
				require.Len(t, w, 2, "a second volume is as real as the first")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The text path: what mozza import writes to disk.
			text, err := ComposeToRecipe(tt.input)
			require.NoError(t, err)

			// Anchored to a directive line so no comment can satisfy it: there is
			// no "storage" directive in the grammar, whatever size follows it.
			assert.NotContains(t, text, "\n  storage ", "the parser has no storage directive")

			for _, want := range tt.contains {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range tt.absent {
				assert.NotContains(t, text, unwanted)
			}

			parsed, err := recipe.NewParser(text).Parse()
			require.NoError(t, err, "the written recipe must parse:\n%s", text)
			require.Len(t, parsed.Slices, 1)

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

			assert.Equal(t, parsed.Slices[0].Storage, r.Slices[0].Storage,
				"the two paths must agree on storage:\n%s", text)
			assert.Equal(t, parsed.Slices[0].Mounts, r.Slices[0].Mounts,
				"the two paths must agree on mounts:\n%s", text)

			tt.check(t, r.Slices[0], warnings)
		})
	}
}

// TestEngineMountPathsMatchWhereTheCompilerMounts holds the importer's idea of
// where an engine's volume lands against the plan the compiler actually builds.
// The importer needs the path to tell a volume it carries apart from one it does
// not, and a copied table that drifts would put that claim in a recipe wrongly.
func TestEngineMountPathsMatchWhereTheCompilerMounts(t *testing.T) {
	versions := map[string]string{"postgres": "16", "mysql": "8", "redis": "7"}

	for _, engine := range knownEngines {
		assert.Contains(t, engineMountPaths, engine,
			"the importer can write a %s shorthand, so it must know where %s mounts", engine, engine)
	}

	for engine, want := range engineMountPaths {
		t.Run(engine, func(t *testing.T) {
			version, ok := versions[engine]
			require.True(t, ok, "no version to build a %s shorthand with", engine)

			text := "App: sample\n\nData:\n  " + engine + " " + version + ", 10Gi\n"

			parsed, err := recipe.NewParser(text).Parse()
			require.NoError(t, err)

			ap, err := plan.Build(parsed)
			require.NoError(t, err)
			require.Len(t, ap.Slices, 1)

			got := ""
			switch {
			case ap.Slices[0].Database != nil:
				got = ap.Slices[0].Database.MountPath
			case ap.Slices[0].Cache != nil:
				got = ap.Slices[0].Cache.MountPath
			}

			assert.Equal(t, want, got, "the compiler decides where a %s volume goes", engine)
		})
	}
}

// TestTheStatefulRemedyMountsWhereItSaysItDoes follows the advice the importer
// writes into a recipe, exactly as a user would, and then reads where the volume
// really lands. A recipe cannot name a mount path for it (parseMount carries
// only file, secret and config), so the volume goes where the compiler puts it,
// and advice that named the compose path instead would hand back a clean recipe,
// a provisioned volume, and an app still writing where nothing is mounted.
func TestTheStatefulRemedyMountsWhereItSaysItDoes(t *testing.T) {
	text, err := ComposeToRecipe(`
services:
  web:
    image: myorg/web:1.0
    ports:
      - "3000:3000"
    volumes:
      - appdata:/var/lib/app
volumes:
  appdata:
`)
	require.NoError(t, err)
	require.Contains(t, text, "mounted at /data rather than /var/lib/app")

	// Take the advice literally: the quoted directive, on the slice it was
	// written for.
	remedied := text + "  each copy needs its own storage of 10Gi\n"

	parsed, err := recipe.NewParser(remedied).Parse()
	require.NoError(t, err, "advice that does not parse is not advice:\n%s", remedied)

	ap, err := plan.Build(parsed)
	require.NoError(t, err)
	require.NoError(t, plan.Validate(ap))
	require.Len(t, ap.Slices, 1)

	require.Equal(t, plan.SliceKindStateful, ap.Slices[0].Kind,
		"the terminal warning says this deploys as a StatefulSet")

	ss := k8s.BuildStatefulSet(ap.Slices[0], ap.Namespace, ap.Name)
	require.Len(t, ss.Spec.VolumeClaimTemplates, 1, "the advice promises a volume per copy")
	assert.Equal(t, "10Gi",
		ss.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().String(),
		"the advice promises 10Gi")

	require.NotEmpty(t, ss.Spec.Template.Spec.Containers)

	var mounted []string
	for _, vm := range ss.Spec.Template.Spec.Containers[0].VolumeMounts {
		mounted = append(mounted, vm.MountPath)
	}

	assert.Equal(t, []string{"/data"}, mounted,
		"the recipe says the volume mounts at /data, so /data is where it must mount")
}

// TestEveryDocumentedComposeShapeImports pins the import on-ramp: a compose file
// that `docker compose config` accepts is one mozza reads. Every row below was
// put through Docker Compose v5.1.2 to establish that compose really accepts the
// shape before it was asked of the importer. Every row marked as a tolerated
// shape is one a decoder reading only the short syntax would abort on with a raw
// YAML type error.
//
// Both authoring paths are driven, because `mozza import` drives both: the text
// path writes the file and the AST path supplies the warnings printed beside it.
// Where a shape has a short-syntax equivalent the two must import to the same
// recipe, which is the assertion that keeps the tolerated shapes from becoming a
// second conversion path free to drift away from the one short syntax uses.
func TestEveryDocumentedComposeShapeImports(t *testing.T) {
	tests := []struct {
		name string
		// input is the shape under test; equivalent, when set, is the short form
		// that must import to byte-identical output.
		input      string
		equivalent string
		contains   []string
		absent     []string
		check      func(t *testing.T, s recipe.Slice, w []Warning)
	}{
		{
			name: "a long-form port publishes what the short form publishes",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
`,
			contains: []string{"open to the public on port 80"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 80, s.Port)
				assert.True(t, s.Public)
			},
		},
		{
			name: "a long-form port reads a quoted published the same as a numeric one",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - target: 80
        published: "8080"
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
`,
			contains: []string{"open to the public on port 80"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 80, s.Port)
			},
		},
		{
			name: "a long-form port drops its protocol as the short form does",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - target: 53
        published: 5353
        protocol: udp
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "5353:53"
`,
			contains: []string{"open to the public on port 53"},
			absent:   []string{"udp"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 53, s.Port)
			},
		},
		{
			// No equivalence assertion here, and deliberately so. writePort splits
			// a short spec on its first colon only, so "127.0.0.1:8080:80" imports
			// as the port "8080:80". The long form is right and the short
			// form is wrong, and that short-form defect is filed on its own rather
			// than inherited here to keep the two agreeing.
			name: "a long-form port with a host interface publishes the container port",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - target: 80
        published: 8080
        host_ip: 127.0.0.1
`,
			contains: []string{"open to the public on port 80"},
			absent:   []string{"127.0.0.1", "8080:80"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 80, s.Port, "the container port is the one a recipe can name")
			},
		},
		{
			name: "a long-form port with no published port is not public",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - target: 3000
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "3000"
`,
			contains: []string{"on port 3000"},
			absent:   []string{"open to the public"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3000, s.Port)
				assert.False(t, s.Public, "compose published nothing, so neither does the recipe")
			},
		},
		{
			name: "a long-form mount carries what the short form carries",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - type: volume
        source: pgdata
        target: /var/lib/postgresql/data
volumes:
  pgdata:
`,
			equivalent: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{"postgres 16, 10Gi", "# mount pgdata at /var/lib/postgresql/data"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage)
				assert.Empty(t, w, "the shorthand carries the volume, so nothing was lost")
			},
		},
		{
			name: "a long-form mount's read_only and driver options change nothing it carries",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - type: volume
        source: pgdata
        target: /var/lib/postgresql/data
        read_only: true
        volume:
          nocopy: true
volumes:
  pgdata:
`,
			equivalent: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{"postgres 16, 10Gi", "# mount pgdata at /var/lib/postgresql/data"},
			absent:   []string{"nocopy", "read_only"},
		},
		{
			name: "a long-form bind mount is no more a volume than a short one",
			input: `
services:
  web:
    image: nginx:1.25
    volumes:
      - type: bind
        source: ./site
        target: /usr/share/nginx/html
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    volumes:
      - ./site:/usr/share/nginx/html
`,
			absent: []string{"# WARNING: volume", "# mount"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Storage)
				assert.Empty(t, w)
			},
		},
		{
			name: "a long-form tmpfs mount names a target and no source",
			input: `
services:
  web:
    image: nginx:1.25
    volumes:
      - type: tmpfs
        target: /tmp
`,
			absent: []string{"# WARNING: volume", "# mount"},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, s.Mounts, "an anonymous mount names no volume to carry")
				assert.Empty(t, w)
			},
		},
		{
			name: "short and long mounts mix in one list",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
      - type: volume
        source: other
        target: /extra
volumes:
  pgdata:
  other:
`,
			contains: []string{
				"postgres 16, 10Gi",
				"# mount pgdata at /var/lib/postgresql/data",
				"# WARNING: volume other at /extra not carried",
			},
			check: func(t *testing.T, s recipe.Slice, w []Warning) {
				t.Helper()

				assert.Equal(t, "10Gi", s.Storage)
				require.Len(t, w, 1, "the second volume rides on nothing and must say so")
				assert.Contains(t, w[0].Message, `volume other at /extra not carried`)
			},
		},
		{
			name: "a bare secret name is the secret the long form names",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      - api-key
secrets:
  api-key:
    file: ./api-key.txt
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    secrets:
      - source: api-key
secrets:
  api-key:
    file: ./api-key.txt
`,
			contains: []string{"secret API_KEY from api-key"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				require.Len(t, s.Secrets, 1)
				assert.Equal(t, "API_KEY", s.Secrets[0].EnvVar)
				assert.Equal(t, "api-key", s.Secrets[0].SecretName)
			},
		},
		{
			name: "short and long secrets mix in one list",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      - api-key
      - source: other
        target: /run/secrets/o
secrets:
  api-key:
    file: ./api-key.txt
  other:
    file: ./nginx.conf
`,
			contains: []string{"secret API_KEY from api-key", "secret OTHER from other"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				require.Len(t, s.Secrets, 2, "a bare name is as real a secret as a mapping")
			},
		},
		{
			name: "a top-level secret declared with the legacy external mapping imports",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      - source: api-key
secrets:
  api-key:
    external:
      name: real
`,
			contains: []string{"secret API_KEY from api-key"},
		},
		{
			name: "a top-level secret declared from the environment imports",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      - api-key
secrets:
  api-key:
    environment: API_KEY
`,
			contains: []string{"secret API_KEY from api-key"},
		},
		{
			name: "a health check written as a shell string is the shell list compose makes of it",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
    healthcheck:
      test: curl -f http://localhost:80/health
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:80/health"]
`,
			contains: []string{"health check /health"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, "/health", s.Health)
			},
		},
		{
			// The service does not join the network, so no boundary is derived here
			// and the one warning is the internal one. What is under test is that a
			// quoted flag reads as the flag it spells.
			name: "a quoted internal network is as internal as an unquoted one",
			input: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
networks:
  backend:
    internal: "true"
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
networks:
  backend:
    internal: true
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1)
				assert.Equal(t, "networks", w[0].Feature)
			},
		},
		{
			name: "a quoted false internal network is not internal",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: "FALSE"
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, w, "a network that is not internal has nothing to warn about")
			},
		},
		{
			name: "a quoted replica count is the count it spells",
			input: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    deploy:
      replicas: "3"
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    deploy:
      replicas: 3
`,
			contains: []string{"run 3 copies"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3, s.Replicas)
			},
		},
		{
			name: "a quoted zero replica count is not a copy count to write",
			input: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: "0"
`,
			absent: []string{"copies"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Zero(t, s.Replicas)
			},
		},
		{
			name: "a service config in short syntax imports and is called dropped",
			input: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    configs:
      - nginx-conf
configs:
  nginx-conf:
    file: ./nginx.conf
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    configs:
      - source: nginx-conf
        target: /etc/nginx/nginx.conf
configs:
  nginx-conf:
    file: ./nginx.conf
`,
			// The recipe carries no trace of the config, which is precisely what
			// the warning has to say.
			absent: []string{"nginx-conf", "mount", "set "},
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1)
				assert.Equal(t, "configs", w[0].Feature)
				assert.Equal(t, "warn", w[0].Severity)
				assert.Contains(t, w[0].Message, `config "nginx-conf" dropped`)
				assert.Contains(t, w[0].Message, "compose configs are not carried into the recipe")
				assert.Contains(t, w[0].Message, "mount config")
				assert.NotContains(t, w[0].Message, "environment reference",
					"nothing maps a config to an environment reference")
			},
		},

		// Sharing a value through an `x-` anchor and aliasing it into each service
		// is how a compose file avoids repeating itself, so the rows below are
		// mainstream input rather than an edge. They are also the rows that catch a
		// normaliser reading a node's own .Kind and .Value: an alias node's .Value
		// is the *anchor's name*, so anything that reads it without following the
		// alias first reads the label instead of the value, silently when the
		// label happens to parse.
		{
			name: "an aliased long-form port publishes what its anchor publishes",
			input: `
x-port: &portdef
  target: 80
  published: 8080
services:
  web:
    image: nginx:1.25
    ports:
      - *portdef
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
`,
			contains: []string{"open to the public on port 80"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 80, s.Port)
			},
		},
		{
			name: "an aliased long-form mount carries what its anchor names",
			input: `
x-mount: &mountdef
  type: volume
  source: pgdata
  target: /var/lib/postgresql/data
services:
  db:
    image: postgres:16
    volumes:
      - *mountdef
volumes:
  pgdata:
`,
			equivalent: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{"postgres 16, 10Gi", "# mount pgdata at /var/lib/postgresql/data"},
		},
		{
			// The anchor is named "5" and holds "3". Compose renders replicas: 3.
			// Reading the alias node's own .Value yields "5", which Atoi accepts,
			// so this imports as a wrong number rather than as an error, and only a
			// value whose anchor name also parses can catch that.
			name: "an aliased replica count is the count its anchor holds, not the anchor's name",
			input: `
x-replicas: &5 "3"
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: *5
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: 3
`,
			contains: []string{"run 3 copies"},
			absent:   []string{"run 5 copies"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3, s.Replicas, "the anchor holds 3; 5 is only its name")
			},
		},
		{
			// Same trap on the boolean side: an anchor named "off" holding "true".
			// Reading .Value gives "off", which is in compose's own vocabulary, so
			// the network silently stops being internal and its warning disappears.
			name: "an aliased internal flag is the flag its anchor holds, not the anchor's name",
			input: `
x-flag: &off "true"
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: *off
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: true
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1, "the anchor holds true, so the network is internal")
				assert.Equal(t, "networks", w[0].Feature)
			},
		},

		// The rows below are values the native decoder handles and a string-first
		// decoder would mangle. A careless change to the decoding order regresses
		// them, so they are asserted, not assumed.
		{
			name: "an unquoted replica count still takes the number path",
			input: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: 3
`,
			contains: []string{"run 3 copies"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3, s.Replicas)
			},
		},
		{
			name: "an integral float replica count still parses",
			input: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: 3.0
`,
			contains: []string{"run 3 copies"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3, s.Replicas, "compose accepts 3.0, and so did mozza before this change")
			},
		},
		{
			name: "yaml's own boolean vocabulary still reads as internal",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: yes
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1, "yes is a yaml boolean and was internal before this change")
			},
		},
		{
			name: "an uppercase unquoted boolean still reads as internal",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: TRUE
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				require.Len(t, w, 1)
			},
		},
		{
			name: "yaml's own negative boolean still reads as not internal",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: off
`,
			check: func(t *testing.T, _ recipe.Slice, w []Warning) {
				t.Helper()

				assert.Empty(t, w)
			},
		},
		{
			// A plain []string field would import an aliased short-syntax port,
			// because yaml's own sequence decoder follows aliases. Walking a sequence's Content by hand does not, so
			// this is the row that fails if the hand-rolled loop forgets them.
			name: "an aliased short-syntax port still imports",
			input: `
x-web-port: &webport "8080:80"
services:
  web:
    image: nginx:1.25
    ports:
      - *webport
`,
			equivalent: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
`,
			contains: []string{"open to the public on port 80"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 80, s.Port)
			},
		},
		{
			name: "an aliased short-syntax mount still imports",
			input: `
x-data: &datavol "pgdata:/var/lib/postgresql/data"
services:
  db:
    image: postgres:16
    volumes:
      - *datavol
volumes:
  pgdata:
`,
			equivalent: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata:
`,
			contains: []string{"postgres 16, 10Gi", "# mount pgdata at /var/lib/postgresql/data"},
		},
		{
			// An alias to a null is still a null, so it is dropped like a literal
			// one. That holds only if the alias is followed before the tag is
			// read, not after.
			name: "an aliased null port entry is still dropped rather than written empty",
			input: `
x-nothing: &nada ~
services:
  web:
    image: nginx:1.25
    ports:
      - 3000
      - *nada
`,
			contains: []string{"on port 3000"},
			absent:   []string{"on port \n"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3000, s.Port)
			},
		},
		{
			name: "a null port entry is still dropped rather than written empty",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - 3000
      - ~
`,
			contains: []string{"on port 3000"},
			// A null element decoded to "" would reach writePort and emit a bare
			// "on port " with no number, which is what yaml's own sequence decoder
			// avoids by dropping it.
			absent: []string{"on port \n"},
			check: func(t *testing.T, s recipe.Slice, _ []Warning) {
				t.Helper()

				assert.Equal(t, 3000, s.Port)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The text path: what mozza import writes to disk.
			text, err := ComposeToRecipe(tt.input)
			require.NoError(t, err, "compose accepts this file, so mozza must read it")

			for _, want := range tt.contains {
				assert.Contains(t, text, want, "full output:\n%s", text)
			}
			for _, unwanted := range tt.absent {
				assert.NotContains(t, text, unwanted, "full output:\n%s", text)
			}

			parsed, err := recipe.NewParser(text).Parse()
			require.NoError(t, err, "the written recipe must parse:\n%s", text)

			ap, err := plan.Build(parsed)
			require.NoError(t, err, "the written recipe must build:\n%s", text)
			require.NoError(t, plan.Validate(ap), "the written recipe must validate:\n%s", text)

			// The AST path: what the server reports and what the warnings come from.
			r, warnings, err := ComposeToRecipeAST([]byte(tt.input))
			require.NoError(t, err, "both entrypoints read the same types, so both must accept it")
			require.Len(t, r.Slices, 1)

			astPlan, err := plan.Build(r)
			require.NoError(t, err)
			require.NoError(t, plan.Validate(astPlan), "the imported AST must validate too")

			// A tolerated shape that imported to anything other than what its short
			// form imports to would be a second conversion path, free to drift,
			// so both authoring paths are held against the short form, not just one.
			if tt.equivalent != "" {
				shortText, shortErr := ComposeToRecipe(tt.equivalent)
				require.NoError(t, shortErr)
				assert.Equal(t, shortText, text, "the two shapes name the same thing")

				shortAST, _, shortASTErr := ComposeToRecipeAST([]byte(tt.equivalent))
				require.NoError(t, shortASTErr)
				assert.Equal(t, shortAST.Slices, r.Slices, "both paths agree on both shapes")
			}

			if tt.check != nil {
				tt.check(t, r.Slices[0], warnings)
			}
		})
	}
}

// TestAComposeNetworkIsReportedRatherThanCarried pins what a compose network
// imports to, on both paths: no network policy anywhere, and a warning naming the
// boundary that was not carried.
//
// The two vocabularies do not line up. A compose network is an undirected broadcast
// domain (everyone on it reaches everyone else), while a recipe's `only accept
// traffic from` is a directed ingress allow-list whose *presence* is a deny. So a
// derived policy is right only when its list is every source compose already allows,
// and membership read off the `networks:` keys is a lower bound rather than that set.
// `extends`, `include`, `network_mode`, `external_links`, an `external:` or
// `attachable:` network, an exotic driver, a network's `name:`, an unexpanded
// `${VAR}` and an override file compose loads without being asked all put a service
// on a network no key records, and the last two cannot be closed at all, because
// mozza expands nothing anywhere and the server entry point is handed content with no
// path to look beside. Every one missed is a policy that silently black-holes a
// service while the deploy reports success.
//
// So the boundary is named instead. Every row asserts the same two things (that
// neither path writes a directive, and that the warning is exactly what the file
// declares), and the shapes are all kept, because a warning that names the wrong
// thing is the failure that remains once no policy can be written.
//
// A row's warnings come in two kinds, and the split is the point of them. What the
// boundary is belongs to the network, so it is reported once per network; what to
// write instead belongs to the slice, which carries one directive however many
// networks it is on, so it is reported once per service. Following one instruction on
// its own has to be safe, and TestFollowingOneRemedyDeniesNothingComposeAllows is
// where that is held rather than described.
//
// The warnings are pinned in full and in order, so a boundary dropped without one is
// a boundary dropped in silence. Two rows read the compiled manifests rather than the
// recipe, because what a policy would have cost is only visible there.
func TestAComposeNetworkIsReportedRatherThanCarried(t *testing.T) {
	tests := []struct {
		name string
		// input is a file `docker compose config` v5.1.2 accepts.
		input string
		// equivalent is a spelling of the same file that must import identically.
		equivalent string
		check      func(t *testing.T, p *plan.AppPlan)
		// warnings is every networks warning, in full and in order.
		warnings []string
	}{
		{
			// The ordinary file, and the copy a user is most likely to read. It has
			// to be enough on its own to know what to write and where.
			name: "an ordinary shared network is named, with the directive that carries it",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  worker:
    image: myworker:1.0
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "worker"),
				remedy("api", []string{"backend"}, `"api" and "worker"`, "Api"),
				remedy("worker", []string{"backend"}, `"api" and "worker"`, "Worker"),
			},
		},
		{
			// The list inflects with its length, and the allow-list is every member
			// including the slice the directive is written under: compose lets a
			// service reach its own replicas over a shared network.
			name: "three services on one network name all three, each of them included",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  cache:
    image: redis:7
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "cache", "db"),
				remedy("api", []string{"backend"}, `"api" and "cache" and "db"`, "Api"),
				remedy("cache", []string{"backend"}, `"api" and "cache" and "db"`, "Cache"),
				remedy("db", []string{"backend"}, `"api" and "cache" and "db"`, "Db"),
			},
		},
		{
			// The row that keeps the self-peer in the remedy honest. Mozza gives a
			// multi-copy stateful slice a headless service so its pods can find each
			// other, and a directive that left db off its own list would deny the
			// replica traffic that service exists to route. So the list mozza
			// recommends has to contain db. And no policy is compiled either way,
			// which is the whole of what the importer guarantees.
			name: "a service reaching its own replicas is in the list mozza recommends",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    deploy:
      replicas: 3
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
			check: func(t *testing.T, p *plan.AppPlan) {
				t.Helper()

				db := compiledFor(t, p, "db")

				require.NotNil(t, db.service, "a multi-copy stateful slice gets a service")
				assert.Equal(t, "None", db.service.Spec.ClusterIP,
					"it is headless, and headless exists for pod-to-pod discovery")
				assert.Nil(t, db.policy,
					"and nothing compiled beside it can deny what that discovery routes")
			},
		},
		{
			name: "services on networks of their own are two boundaries, not one",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    networks: [frontend]
networks:
  backend:
  frontend:
`,
			warnings: []string{
				declares("backend", "api"),
				declares("frontend", "web"),
				remedy("api", []string{"backend"}, `"api"`, "Api"),
				remedy("web", []string{"frontend"}, `"web"`, "Web"),
			},
		},
		{
			// web bridges two networks that do not touch, and is a member of both.
			// Its one directive is the union of the two, because a slice carries one
			// directive and the presence of that directive is a deny: a list of
			// either network's members alone would shut out the other's.
			name: "a service on several networks gets one directive holding all of them",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  cdn:
    image: mycdn:1.0
    networks: [frontend]
  web:
    image: nginx:1.25
    networks: [frontend, backend]
networks:
  backend:
  frontend:
`,
			warnings: []string{
				declares("backend", "api", "web"),
				declares("frontend", "cdn", "web"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
				remedy("cdn", []string{"frontend"}, `"cdn" and "web"`, "Cdn"),
				remedy("web", []string{"backend", "frontend"},
					`"api" and "cdn" and "web"`, "Web"),
			},
		},
		{
			// One service and one boilerplate network, which is most of the compose
			// files that declare one at all. There is nothing here for the network to
			// keep out, so there is no boundary to report and nothing to say.
			name: "a lone service on a boilerplate network imports silently",
			input: `
services:
  web:
    image: nginx:1.25
    networks: [frontend]
networks:
  frontend:
`,
		},
		{
			// Compose puts api on the default network, not on backend, so the two
			// genuinely cannot reach each other. Nothing is derived from the default
			// network (it is where compose puts a service that asked for no
			// isolation), and the partition is reported all the same, by the clause
			// saying what backend keeps out.
			name: "a service that omits networks is what the other network keeps out",
			input: `
services:
  api:
    image: myapi:1.0
  web:
    image: nginx:1.25
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "web"),
				remedy("web", []string{"backend"}, `"web"`, "Web"),
			},
		},
		{
			// `networks: [default]` puts a service exactly where declaring nothing
			// puts it, so the two spellings describe the same file and neither draws
			// a boundary to report.
			name: "naming the default network puts a service where it already was",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default]
  worker:
    image: myworker:1.0
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
  worker:
    image: myworker:1.0
`,
		},
		{
			// The default network is a real network with real members, and a service
			// that names one of its own as well is on both. So monitoring's members
			// are not everything that reaches api (worker is there too, having
			// declared nothing), and the reader is told before being handed a list
			// that leaves worker out.
			name: "a service also on the default network makes the list a short one",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default, monitoring]
  metrics:
    image: mymetrics:1.0
    networks: [monitoring]
  worker:
    image: myworker:1.0
networks:
  monitoring:
`,
			warnings: []string{
				declares("monitoring", "api", "metrics") +
					mayBeShort(`service "api" also joins the default network, where compose puts`+
						" every service that declares no networks of its own"),
				remedy("api", []string{"monitoring"}, `"api" and "metrics"`, "Api") +
					denies("monitoring"),
				remedy("metrics", []string{"monitoring"}, `"api" and "metrics"`, "Metrics") +
					denies("monitoring"),
			},
		},
		{
			// Nothing but the default network anywhere, which is a file that asked
			// for no isolation at all.
			name: "a service whose only network is the default one draws no boundary",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [default]
  worker:
    image: myworker:1.0
    networks: [default]
`,
		},
		{
			name: "a file that declares no networks imports as it always did",
			input: `
services:
  api:
    image: myapi:1.0
  db:
    image: postgres:16
`,
		},
		{
			// A published port makes the slice reachable from outside the cluster,
			// and the compiler gives it an Ingress. The ingress controller joins no
			// compose network, so it is in no list read off one. A directive under
			// Web would deny it, the pods would stay up and the site would go dark.
			// So Web is left out of the remedy and told why, rather than the reader
			// noticing a name missing from it.
			name: "a public slice is left out of the remedy and told why",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "web"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
				exempt("web", []string{"backend"}, "Web"),
			},
			check: func(t *testing.T, p *plan.AppPlan) {
				t.Helper()

				web := compiledFor(t, p, "web")

				require.NotNil(t, web.ingress, "a public slice keeps its ingress")
				assert.Nil(t, web.policy, "and nothing is compiled that would deny it")
			},
		},
		{
			// Every member is public, so there is nowhere the directive belongs. Each
			// member says so for itself rather than the network reporting an absence:
			// a reader who is told nothing about a service cannot tell being passed
			// over from having nothing to do.
			name: "a network of nothing but public slices says so of every member",
			input: `
services:
  cdn:
    image: mycdn:1.0
    ports: ["9090:90"]
    networks: [edge]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [edge]
networks:
  edge:
`,
			warnings: []string{
				declares("edge", "cdn", "web"),
				exempt("cdn", []string{"edge"}, "Cdn"),
				exempt("web", []string{"edge"}, "Web"),
			},
		},
		{
			// An external network's members live in compose projects mozza cannot
			// enumerate, so what the file declares on it is certainly not all of it.
			// All three spellings compose accepts say the same thing.
			name: "an external network says its members live outside this file",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: true
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: "true"
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			name: "the legacy external mapping is external too",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external:
      name: real_legacy_net
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			// A network's `name:` is the network compose actually creates, and a
			// second key can resolve to the same one, so the key a service is
			// written under does not settle who else is on the network it names.
			name: "a network renamed by its `name:` may be reached under another key",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    name: shared_net
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this network sets a `name:` of its own, which is the network"+
						" compose actually creates, and a second key can resolve to that same"+
						" network"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// The `name:` is on some other key. A key that declares none still
			// resolves to <project>_<key>, which that one's `name:` is free to be, so
			// the question it raises is about this network as much as its own.
			name: "a network named elsewhere in the file is still a key that can collide",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
  ops:
    name: shared_ops
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`network "ops" sets a `+"`name:`"+` of its own, which is the network`+
						" compose actually creates, and a second key can resolve to that same"+
						" network"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// Both hold at once. External wins: that its members live outside the
			// file entirely is the larger fact, and a `name:` only says which key
			// reaches it.
			name: "a network both external and renamed reports the members outside the file",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [legacy]
  db:
    image: postgres:16
    networks: [legacy]
networks:
  legacy:
    external: true
    name: real_legacy_net
`,
			warnings: []string{
				declares("legacy", "api", "db") +
					mayBeShort("this network is declared `external:`, and its other members live"+
						" outside this compose file"),
				remedy("api", []string{"legacy"}, `"api" and "db"`, "Api") + denies("legacy"),
				remedy("db", []string{"legacy"}, `"api" and "db"`, "Db") + denies("legacy"),
			},
		},
		{
			// The mapping form is how a service declares an alias or an address on a
			// network, and it names the same networks the list form does.
			name: "the mapping form of a service's networks joins the same networks",
			input: `
services:
  api:
    image: myapi:1.0
    networks:
      backend:
        aliases: [api-internal]
  db:
    image: postgres:16
    networks:
      backend:
networks:
  backend:
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
		},
		{
			// A service taking its network namespace from another reaches whatever
			// that one reaches, over a network it never joined by name, so it is a
			// source no `networks:` key in the file records.
			name: "a service sharing another's network namespace is a source no key names",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  sidecar:
    image: busybox:1.36
    network_mode: "service:api"
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`service "sidecar" takes its network namespace from `+
						"`network_mode`, and reaches services over networks it never joined by name"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// `none` is the one value that reaches nothing. Measured against docker
			// 29.4.0 every other value reaches something (even `bridge` reaches a
			// service on a user-defined network at its address), so it is the only
			// one that leaves the file's memberships whole.
			name: "a service with no networking at all leaves the memberships whole",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  job:
    image: myjob:1.0
    network_mode: none
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db"),
			},
		},
		{
			// `extends` merges the base's `networks:` into this service, from this
			// file or one mozza was never handed. Compose renders the short and long
			// forms identically, so mozza must too.
			name: "a service inheriting its networks with `extends` is a member no key names",
			input: `
services:
  base:
    image: mybase:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    extends:
      service: base
`,
			equivalent: `
services:
  base:
    image: mybase:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    extends: base
`,
			warnings: []string{
				declares("backend", "api", "base") +
					mayBeShort(`service "web" takes its configuration from another with `+
						"`extends`, whose networks mozza does not read"),
				remedy("api", []string{"backend"}, `"api" and "base"`, "Api") + denies("backend"),
				remedy("base", []string{"backend"}, `"api" and "base"`, "Base") + denies("backend"),
			},
		},
		{
			name: "`extends` from another file is a base mozza cannot read at all",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
  web:
    image: nginx:1.25
    extends:
      file: base.yml
      service: base
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort(`service "web" takes its configuration from another with `+
						"`extends`, whose networks mozza does not read"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// `include` merges another file's services into this project, on these
			// networks. A reader who wrote the directive from the two names here
			// would be denying a service they never saw named.
			name: "a file that includes another may have members it never lists",
			input: `
include:
  - base.yml
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this file pulls services in with `include`, which mozza does not"+
						" resolve"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			name: "a service linked to a container outside the project names no key for it",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
    external_links:
      - legacy_db:db
  web:
    image: nginx:1.25
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "web") +
					mayBeShort(`service "api" declares `+"`external_links`"+", which reach"+
						" containers this file does not define over a network shared with them"),
				remedy("api", []string{"backend"}, `"api" and "web"`, "Api") + denies("backend"),
				remedy("web", []string{"backend"}, `"api" and "web"`, "Web") + denies("backend"),
			},
		},
		{
			// `docker run --network` may join an attachable network, so its members
			// are not only this file's services: the same failure as `external:`,
			// reached by a different key. Compose accepts the quoted form and
			// normalises it to the bare flag.
			name: "an attachable network is one containers outside compose may join",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    attachable: true
`,
			equivalent: `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
    attachable: "true"
`,
			warnings: []string{
				declares("backend", "api", "db") +
					mayBeShort("this network is declared `attachable:`, and containers compose did"+
						" not start may join it"),
				remedy("api", []string{"backend"}, `"api" and "db"`, "Api") + denies("backend"),
				remedy("db", []string{"backend"}, `"api" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// macvlan puts a container straight onto the physical network, so what
			// else is on it is nothing a compose file records. The plain network in
			// the same file keeps its own account, which is the per-network scope
			// visible in one row.
			name: "a network on a driver compose does not create is read per network",
			input: `
services:
  api:
    image: myapi:1.0
    networks: [phys]
  db:
    image: postgres:16
    networks: [phys]
  cache:
    image: redis:7
    networks: [ops]
  metrics:
    image: mymetrics:1.0
    networks: [ops]
networks:
  phys:
    driver: macvlan
  ops:
    driver: bridge
`,
			warnings: []string{
				declares("ops", "cache", "metrics"),
				declares("phys", "api", "db") +
					mayBeShort(`this network's "macvlan" driver puts it on a segment compose does`+
						" not create"),
				remedy("api", []string{"phys"}, `"api" and "db"`, "Api") + denies("phys"),
				remedy("cache", []string{"ops"}, `"cache" and "metrics"`, "Cache"),
				remedy("db", []string{"phys"}, `"api" and "db"`, "Db") + denies("phys"),
				remedy("metrics", []string{"ops"}, `"cache" and "metrics"`, "Metrics"),
			},
		},
		{
			// Compose expands ${VAR} before it reads the project and mozza expands
			// nothing anywhere, so this key is a literal token that may be any of the
			// networks declared here. No remedy is offered for it (a list drawn from
			// a network that does not exist is not one to recommend), and the network
			// it may really be is told that a member of it is unaccounted for.
			//
			// This is the shape that ended the derivation: compose resolves all three
			// services onto backend, and any policy written from these keys admits
			// two of them.
			name: "an unexpanded ${VAR} is a network mozza cannot name",
			input: `
services:
  api:
    image: myapi:1.0
    networks: ["${NET:-backend}"]
  cache:
    image: redis:7
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("${NET:-backend}", "api") +
					mayBeShort("this network's name holds a `${VAR}` mozza does not expand, and this"+
						" may be one of the other networks declared here"),
				declares("backend", "cache", "db") +
					mayBeShort(`service "api" joins a network whose name holds a `+
						"`${VAR}`"+" mozza does not expand, and it may be this one"),
				withheld("api", []string{"${NET:-backend}"}),
				remedy("cache", []string{"backend"}, `"cache" and "db"`, "Cache") +
					denies("backend"),
				remedy("db", []string{"backend"}, `"cache" and "db"`, "Db") + denies("backend"),
			},
		},
		{
			// A slice name is not always the compose service name: a service called
			// after a recipe keyword gets a suffix, and the directive has to name the
			// slice while the sentence names the service.
			name: "a service named after a reserved word is written as the slice it becomes",
			input: `
services:
  app:
    image: myapp:1.0
    networks: [backend]
  db:
    image: postgres:16
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "app", "db"),
				remedy("app", []string{"backend"}, `"app-svc" and "db"`, "App-svc"),
				remedy("db", []string{"backend"}, `"app-svc" and "db"`, "Db"),
			},
		},
		{
			// Every name in the directive is quoted, always: the grammar takes an
			// identifier or a string, and an unquoted name colliding with a keyword
			// lexes as that keyword instead.
			name: "a member named after a keyword is still quoted into a directive that parses",
			input: `
services:
  public:
    image: mypublic:1.0
    networks: [backend]
  api:
    image: myapi:1.0
    networks: [backend]
networks:
  backend:
`,
			warnings: []string{
				declares("backend", "api", "public"),
				remedy("api", []string{"backend"}, `"api" and "public"`, "Api"),
				remedy("public", []string{"backend"}, `"api" and "public"`, "Public"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The text path: what mozza import writes to disk, and what the server
			// deploys.
			text, err := ComposeToRecipe(tt.input)
			require.NoError(t, err, "compose accepts this file, so mozza must read it")

			assert.NotContains(t, text, "only accept traffic",
				"no compose file writes a network policy:\n%s", text)

			written, err := recipe.NewParser(text).Parse()
			require.NoError(t, err, "the written recipe must parse:\n%s", text)

			writtenPlan, err := plan.Build(written)
			require.NoError(t, err, "the written recipe must build:\n%s", text)
			require.NoError(t, plan.Validate(writtenPlan),
				"the written recipe must validate:\n%s", text)

			assert.Empty(t, boundaries(written), "what import wrote:\n%s", text)

			// The AST path: what the server reports and the warnings come from.
			imported, warnings, err := ComposeToRecipeAST([]byte(tt.input))
			require.NoError(t, err, "both entry points read the same types, so both must accept it")

			importedPlan, err := plan.Build(imported)
			require.NoError(t, err)
			require.NoError(t, plan.Validate(importedPlan), "the imported AST must validate too")

			assert.Empty(t, boundaries(imported),
				"the AST path has no production caller that compiles it, which is why it is held"+
					" to the same bar: a policy here is a trap for the first one to wire it up")

			assert.Equal(t, tt.warnings, networksWarnings(warnings))

			if tt.equivalent != "" {
				shortText, shortErr := ComposeToRecipe(tt.equivalent)
				require.NoError(t, shortErr)
				assert.Equal(t, shortText, text, "the two shapes name the same thing")

				shortAST, shortWarnings, shortASTErr := ComposeToRecipeAST([]byte(tt.equivalent))
				require.NoError(t, shortASTErr)
				assert.Equal(t, shortAST.Slices, imported.Slices, "both paths agree on both shapes")
				assert.Equal(t, tt.warnings, networksWarnings(shortWarnings),
					"and both shapes report the same boundary")
			}

			if tt.check != nil {
				tt.check(t, importedPlan)
			}
		})
	}
}

// declares is the sentence every network warning opens with: which network, and
// which services this file puts on it. Written out here in full rather than
// assembled from compose.go's own fragments, so that a rewording there fails a test
// instead of travelling silently into the assertion. The clauses a row adds are
// spelled out at the row or built by the two helpers below.
func declares(network string, members ...string) string {
	subject, keeps := "service", "compose keeps out everything that did not join"
	if len(members) > 1 {
		subject = "services"
		keeps = "compose lets them reach each other and keeps out everything that did not join"
	}

	return fmt.Sprintf(
		"network %s draws a boundary the recipe does not carry — this file declares %s %s on it,"+
			" and %s", quotedList(network), subject, quotedList(members...), keeps)
}

// remedy is the whole per-service warning where a directive is recommended: the
// networks the service joins, the one directive that carries all of them, and the
// section it goes under. A row states the allow-list itself, so a peer added or
// dropped fails here rather than being derived from the code under test.
func remedy(service string, nets []string, allow, under string) string {
	boundaries := "that boundary"
	if len(nets) > 1 {
		boundaries = "those boundaries"
	}

	return onNetworks(service, nets) + fmt.Sprintf(
		" — to carry %s by hand, add `only accept traffic from %s` under %s, which is every"+
			" service this file declares on a network with it", boundaries, allow, under)
}

// exempt is the whole per-service warning for a service the compiler gives an
// Ingress: no directive, and why not.
func exempt(service string, nets []string, under string) string {
	return onNetworks(service, nets) + fmt.Sprintf(
		" and publishes a port, so its slice is reachable from outside the cluster and a network"+
			" policy under %s would deny the ingress that serves it — no directive is recommended",
		under)
}

// withheld is the whole per-service warning for a service whose every network is
// named by a token mozza never expanded, so there is no membership to draw a list
// from at all.
func withheld(service string, nets []string) string {
	return onNetworks(service, nets) +
		" — no directive is recommended, because every network it joins is named by a `${VAR}`" +
		" mozza does not expand, and a list drawn from a network mozza cannot name is not one" +
		" to follow"
}

// denies is the clause a remedy carries when one of the service's networks may hold
// members this file never names. It is what stands between a reader and a directive
// that shuts out whatever those are.
func denies(nets ...string) string {
	return fmt.Sprintf("; %s may have members this file does not declare, and the directive"+
		" denies anything it leaves out", quotedList(nets...))
}

// onNetworks opens every per-service warning: the service, and the networks it is on.
func onNetworks(service string, nets []string) string {
	return fmt.Sprintf("service %s is on %s", quotedList(service), quotedList(nets...))
}

// quotedList is how the warnings quote a compose key in a sentence, spelled out here
// rather than borrowed from compose.go so that a change to the quoting there fails an
// assertion instead of travelling into it.
func quotedList(keys ...string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, `"`+key+`"`)
	}

	return strings.Join(quoted, " and ")
}

// mayBeShort is the clause a row carries when something in the file can put a service
// on the network without a `networks:` key recording it. It is what stands between a
// reader and a hand-written policy that denies whatever that construct brought in.
func mayBeShort(why string) string {
	return "; " + why + ", so the services named here may not be all of them"
}

// networksWarnings picks out what the import said about networks, leaving the
// warnings a row is not about (a build context, a dropped volume) alone.
func networksWarnings(warnings []Warning) []string {
	var said []string
	for _, w := range warnings {
		if w.Feature == "networks" {
			said = append(said, w.Message)
		}
	}

	return said
}

// boundaries reads the boundary every slice carries, keyed by the name plan.Build
// gives the slice. The two paths reach that name differently (the importer
// capitalizes a compose service into a section header, the parser lowercases a
// header it reads back), and the name an AllowFrom entry is checked against is
// the lowercased one, so that is the key both are held to.
//
// A slice carrying no boundary is absent rather than present and empty, because
// BuildNetworkPolicy turns an empty AllowFrom into `ingress: null`: a deny-all,
// and a different answer from no policy at all.
func boundaries(r *recipe.Recipe) map[string][]string {
	got := map[string][]string{}
	for _, s := range r.Slices {
		if s.NetworkPolicy != nil {
			got[strings.ToLower(s.Name)] = s.NetworkPolicy.AllowFrom
		}
	}

	return got
}

// compiled is what `mozza compile --target kubernetes` writes for one slice, as
// far as a boundary is concerned.
type compiled struct {
	policy  *networkingv1.NetworkPolicy
	service *corev1.Service
	ingress *networkingv1.Ingress
}

// compiledFor runs the Kubernetes compiler the way the command does, so a row
// reads the manifests a user would get rather than a reconstruction of them.
func compiledFor(t *testing.T, p *plan.AppPlan, slice string) compiled {
	t.Helper()

	objects, err := k8s.New().CompileObjects(p)
	require.NoError(t, err)

	var got compiled
	for _, object := range objects {
		switch o := object.(type) {
		case *networkingv1.NetworkPolicy:
			if o.Name == slice {
				got.policy = o
			}
		case *corev1.Service:
			if o.Name == slice {
				got.service = o
			}
		case *networkingv1.Ingress:
			if o.Name == slice {
				got.ingress = o
			}
		}
	}

	return got
}

// TestNoImportSetsADomain is the tripwire under the predicate remedyWarning uses to
// decide a slice is reachable from outside the cluster, and so to recommend it no
// directive at all.
//
// The compiler gives a slice an Ingress for `Public || Domain != ""`
// (k8s/compiler.go). publishesPort covers the first half only, and that is
// sufficient for exactly one reason: no import path writes Domain. A compose file
// carries no domain to read, so convertService never sets the field, and the second
// half is unreachable rather than merely unused.
//
// The day one is read (traefik's routing labels, below, are the obvious source), a
// slice would get an Ingress and a recommendation to write the policy that denies the
// controller serving it, and a reader following the advice would take the site dark
// with nothing to read anywhere. That change would be written in convertService,
// nowhere near the predicate. This fails on that commit and names what else has to
// move with it.
func TestNoImportSetsADomain(t *testing.T) {
	const composeFile = `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [backend]
    labels:
      traefik.enable: "true"
      traefik.http.routers.web.rule: "Host(web.example.com)"
networks:
  backend:
`

	imported, _, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)
	require.Len(t, imported.Slices, 2)

	for _, s := range imported.Slices {
		assert.Empty(t, s.Domain,
			"slice %q carries a domain, so publishesPort is no longer the whole of the compiler's"+
				" ingress guard — remedyWarning has to withhold the directive from a slice with a"+
				" Domain too, or it recommends the policy that denies this slice's ingress", s.Name)
	}
}

// TestFollowingOneRemedyDeniesNothingComposeAllows is what makes the remedy safe to
// act on, and the reason it is grouped by service rather than by network.
//
// A slice carries one `only accept traffic from`, and the presence of that directive
// is a deny. So an instruction naming one network's members is safe only when the
// service is on no other network. Grouped per network, a service on two would get two
// instructions that each read as complete and self-contained, and following either
// one on its own compiles a policy narrower than compose: validate passes, the deploy
// reports success, and a tier goes dark with nothing anywhere to read. No wording
// makes that safe; the instruction has to be whole.
//
// So each instruction is applied here on its own, as a reader would apply it, and the
// policy it compiles to is held against the reach compose gives that service. What
// compose allows is written out below rather than derived, because deriving it from
// the code under test would agree with any answer that code gave.
func TestFollowingOneRemedyDeniesNothingComposeAllows(t *testing.T) {
	// The textbook three tiers: web is public on frontend, api bridges frontend and
	// backend, db is alone with api on backend.
	const composeFile = `
services:
  web:
    image: nginx:1.25
    ports: ["8080:80"]
    networks: [frontend]
  api:
    image: myapi:1.0
    networks: [frontend, backend]
  db:
    image: postgres:16
    networks: [backend]
`

	// Everything sharing at least one network with the service, itself included:
	// compose lets a service reach its own replicas over a network it is on. web is
	// absent because it is public: the compiler gives it an Ingress, and a policy
	// under it would deny the controller serving it, so it is offered no directive.
	reach := map[string][]string{
		"api": {"api", "db", "web"},
		"db":  {"api", "db"},
	}

	text, err := ComposeToRecipe(composeFile)
	require.NoError(t, err)

	_, warnings, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)

	applied := map[string][]string{}
	for _, message := range networksWarnings(warnings) {
		directive, section, ok := recommendation(message)
		if !ok {
			continue
		}

		// One directive, under one section, into the recipe mozza actually wrote,
		// which is the whole of what the reader was told to do.
		parsed, parseErr := recipe.NewParser(
			strings.Replace(text, section+":\n", section+":\n  "+directive+"\n", 1)).Parse()
		require.NoError(t, parseErr, "the directive mozza recommends must parse")

		built, buildErr := plan.Build(parsed)
		require.NoError(t, buildErr)
		require.NoError(t, plan.Validate(built), "and must name slices that exist")

		slice := strings.ToLower(section)
		compiledSlice := compiledFor(t, built, slice)
		require.NotNil(t, compiledSlice.policy, "the directive compiles to a policy")

		applied[slice] = admits(compiledSlice.policy)
	}

	assert.Equal(t, reach, applied,
		"every instruction is one a reader can follow on its own: applied alone, it must"+
			" deny nothing compose allows, and every service compose isolates must get one")
}

// recommended matches the instruction a warning carries, so the test acts on what
// was written rather than on a reconstruction of it.
var recommended = regexp.MustCompile("add `(only accept traffic from [^`]+)` under ([^,]+),")

// recommendation reads the directive and the section out of a warning that carries
// one. A warning that recommends nothing returns false.
func recommendation(message string) (string, string, bool) {
	match := recommended.FindStringSubmatch(message)
	if match == nil {
		return "", "", false
	}

	return match[1], match[2], true
}

// admits reads the components a compiled policy lets through, which is the whole of
// what the slice carrying it is reachable from.
func admits(policy *networkingv1.NetworkPolicy) []string {
	var components []string
	for _, rule := range policy.Spec.Ingress {
		for _, from := range rule.From {
			components = append(components,
				from.PodSelector.MatchLabels["app.kubernetes.io/component"])
		}
	}
	sort.Strings(components)

	return components
}

// TestABoundaryIsReportedForAFileThatCannotBuildYet covers the files the table above
// cannot: a `build:` context imports to a slice with no kind, which fails plan.Build
// for a reason that predates any of this and is filed separately.
//
// The boundary is reported anyway, naming the stub as a member. The user's next move
// is to supply the image by hand, and a warning withheld until the recipe builds
// would be one they never see: the boundary would be gone from under the edit with
// nothing said about it.
func TestABoundaryIsReportedForAFileThatCannotBuildYet(t *testing.T) {
	const composeFile = `
services:
  api:
    image: myapi:1.0
    networks: [backend]
  web:
    build: .
    networks: [backend]
networks:
  backend:
`

	imported, warnings, err := ComposeToRecipeAST([]byte(composeFile))
	require.NoError(t, err)

	assert.Equal(t, []string{
		declares("backend", "api", "web"),
		remedy("api", []string{"backend"}, `"api" and "web"`, "Api"),
		remedy("web", []string{"backend"}, `"api" and "web"`, "Web"),
	}, networksWarnings(warnings), "the stub is a member like any other")
	assert.Empty(t, boundaries(imported))

	// The stub is what stops the recipe, and it stops it naming the missing image.
	_, err = plan.Build(imported)
	require.ErrorContains(t, err, `slice "web": could not determine kind`)

	// Supplying the image is the whole of the remedy, and the recipe it produces
	// still carries no policy. The warning is where the boundary was, and stays.
	text, err := ComposeToRecipe(composeFile)
	require.NoError(t, err)

	remedied := strings.Replace(text,
		"  # WARNING: build context detected, needs pre-built image\n",
		"  from image myweb:1.0\n", 1)

	parsed, err := recipe.NewParser(remedied).Parse()
	require.NoError(t, err)

	built, err := plan.Build(parsed)
	require.NoError(t, err)
	require.NoError(t, plan.Validate(built))

	assert.Empty(t, boundaries(parsed))
}

// TestUnreadableComposeShapesSayWhatTheyExpected covers the other half of the
// on-ramp: a shape mozza genuinely cannot read fails with a sentence a compose
// author can act on. yaml.v3 hands a custom unmarshaler's error back unchanged, so
// what is written here is what reaches the terminal, and YAML tags and Go type
// names are asserted absent from it.
// TestUnreadableComposeShapesSayWhatTheyExpected covers the shapes mozza's own
// unmarshalers catch, where mozza chooses the words rather than restating yaml's.
//
// Every want is the whole of what a compose author sees after the wrapping
// prefixes, so the copy is pinned rather than spot-checked, and every field path
// is the one `docker compose config` prints for the same file. An unmarshaler is
// handed a value and no path to it, so these names are read back off the document
// afterwards, which is what tells a service's own `volumes:` from the top-level
// one, and names the service that really holds the fault.
func TestUnreadableComposeShapesSayWhatTheyExpected(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "ports as a mapping",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      a: b
`,
			want: `compose services.web.ports (line 6): expected a list of ports such as "8080:80",` +
				` or a list of entries with target and published`,
		},
		{
			// A service's volumes and the top-level volumes: are different fields
			// with the same name. The path tells them apart:
			// compare "a declared volume is not a mapping" in the sibling test,
			// which reads volumes.pgdata.
			name: "volumes as a mapping",
			input: `
services:
  web:
    image: nginx:1.25
    volumes:
      a: b
`,
			want: `compose services.web.volumes (line 6): expected a list of mounts such as` +
				` "pgdata:/var/lib/postgresql/data", or a list of entries with source and target`,
		},
		{
			// The service named is the one that really holds the fault, not the
			// first in the file.
			name: "the second of two services holds the bad ports",
			input: `
services:
  api:
    image: nginx:1.25
  worker:
    image: busybox:1.36
    ports:
      a: b
`,
			want: `compose services.worker.ports (line 8): expected a list of ports such as "8080:80",` +
				` or a list of entries with target and published`,
		},
		{
			// Compose refuses this file ("is missing a target port"), and mozza
			// tolerating it would emit "open to the public on port" with no number,
			// in a recipe that then passes mozza validate. A wrong recipe that
			// validates travels further than an import that stops.
			name: "a long-form port with no target port",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - published: 8080
        protocol: tcp
`,
			want: `compose services.web.ports (line 6): expected a list of ports such as "8080:80",` +
				` or a list of entries with target and published`,
		},
		{
			// The entry at fault is named at its own line, not at the line the list
			// opened on.
			name: "the second entry of a ports list is the bad one",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
      - published: 8080
`,
			want: `compose services.web.ports (line 7): expected a list of ports such as "8080:80",` +
				` or a list of entries with target and published`,
		},
		{
			// The mapping mozza could not read and the one inside it share line 6.
			// The field's own value is the answer: anything deeper on that line
			// belongs to a structure mozza never looked inside, so the path stops at
			// ports rather than running on to ports.a.
			name: "a ports mapping with another mapping on its own line",
			input: `
services:
  web:
    image: nginx:1.25
    ports:
      a: {b: c}
`,
			want: `compose services.web.ports (line 6): expected a list of ports such as "8080:80",` +
				` or a list of entries with target and published`,
		},
		{
			// The second entry is the bad one, so the line is its own rather than
			// the line the list opened on.
			name: "a long-form mount with no target path",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - pgdata:/var/lib/postgresql/data
      - type: volume
        source: pgdata
volumes:
  pgdata:
`,
			want: `compose services.db.volumes (line 7): expected a list of mounts such as` +
				` "pgdata:/var/lib/postgresql/data", or a list of entries with source and target`,
		},
		{
			name: "a secret that is neither a name nor a mapping",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      - [1, 2]
`,
			want: "compose services.web.secrets (line 6): expected a secret name," +
				" or an entry with a source",
		},
		{
			name: "a health check test that is neither a string nor a list",
			input: `
services:
  web:
    image: nginx:1.25
    healthcheck:
      test:
        a: b
`,
			want: "compose services.web.healthcheck.test (line 7): expected a command string," +
				" or a list of command arguments",
		},
		{
			name: "deploy that is not a mapping at all",
			input: `
services:
  web:
    image: nginx:1.25
    deploy: hello
`,
			want: "compose services.web.deploy (line 5): expected a mapping of deploy settings",
		},
		{
			name: "a replica count mozza will not guess at",
			input: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: ${REPLICAS}
`,
			want: `compose services.web.deploy.replicas (line 6): expected a number of copies,` +
				` got "${REPLICAS}" — mozza does not expand ${VAR} in compose files`,
		},
		{
			name: "a declared network that is not a mapping",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend: bridge
`,
			want: "compose networks.backend (line 6): expected a mapping of network settings",
		},
		{
			name: "an internal flag outside compose's own vocabulary",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal: "maybe"
`,
			want: `compose networks.backend.internal (line 7): expected true or false, got "maybe"`,
		},
		{
			name: "an internal flag that is a mapping",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    internal:
      name: legacy_net
`,
			want: "compose networks.backend.internal (line 8): expected true or false," +
				" got a list or a mapping",
		},
		{
			// The same shape one field over, and deliberately not the same sentence.
			// external's mapping form is the legacy `external: {name: …}`, read before
			// this point, so a mapping never reaches the error, and naming it as a
			// shape the field rejects would describe a file mozza in fact accepts.
			name: "an external flag that is a list",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    external: [legacy_net]
`,
			want: "compose networks.backend.external (line 7): expected true or false, got a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for path, err := range map[string]error{
				"text": func() error { _, e := ComposeToRecipe(tt.input); return e }(),
				"ast":  func() error { _, _, e := ComposeToRecipeAST([]byte(tt.input)); return e }(),
			} {
				require.Error(t, err, "%s path", path)

				msg := err.Error()
				assert.Equal(t, "parsing compose YAML: "+tt.want, msg, "%s path", path)

				// The symptom this whole invariant exists to remove: a compose
				// author reading a Go type name or a yaml tag.
				assertNamesNoGoType(t, msg, path)
			}
		})
	}
}

// TestAnUnnamedComposeFieldSaysSoRatherThanGuess pins the name a message falls
// back to when the value an unmarshaler balked at cannot be found in the document
// again. yaml hands an unmarshaler a value and no path to it, so there is no
// service name to be had from inside one, and a vague "service volumes" is worth
// more to a reader than a confident "services.web.volumes" naming a service that
// is not the one at fault.
func TestAnUnnamedComposeFieldSaysSoRatherThanGuess(t *testing.T) {
	// A node from no document, so the search for it can find nothing.
	orphan := &yaml.Node{Kind: yaml.MappingNode, Line: 4}
	err := composeNamedShapeError(
		[]byte("services:\n  web:\n    image: nginx:1.25\n"),
		&composeShapeError{node: orphan, field: "service volumes", want: "a list of mounts"},
	)

	assert.Equal(t, "compose service volumes (line 4): expected a list of mounts", err.Error())
	assertNamesNoGoType(t, err.Error(), "fallback")
}

// assertNamesNoGoType holds the one line every import failure has to obey: what
// a compose author reads is compose, never the Go types and yaml tags mozza
// happens to be built from.
func assertNamesNoGoType(t *testing.T, msg, path string) {
	t.Helper()

	for _, leak := range []string{"!!", "importer.", "unmarshal", "struct {}", "map[string]", "interface {}"} {
		assert.NotContains(t, msg, leak, "%s path: %q may not reach the user", path, leak)
	}
}

// TestAMisshapenComposeValueNamesItsFieldNotItsGoType covers the shapes no
// unmarshaler of mozza's own catches, where yaml reports the fault itself and
// would otherwise report it as "cannot unmarshal !!str `web` into
// map[string]importer.composeService".
//
// Every want below is the whole of what a compose author sees after the wrapping
// prefixes, so the copy is pinned and not merely spot-checked. Each field name
// is the one `docker compose config` prints for the same file, verified against
// Compose v5.1.2, which answers the first row with "services must be a mapping"
// and the third with "services.web.image must be a string".
func TestAMisshapenComposeValueNamesItsFieldNotItsGoType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "services is not a mapping",
			input: `
services: web
`,
			want: `compose services (line 2): expected a mapping of service names to their settings, got "web"`,
		},
		{
			name: "a service is not a mapping",
			input: `
services:
  web: nginx:1.25
`,
			want: `compose services.web (line 3): expected a mapping of service settings, got "nginx:1.25"`,
		},
		{
			// A plain string field is the one shape mozza has no better word for
			// than "a single value", but the path still names it exactly, which is
			// the whole point of reading the field name off the document.
			name: "an image is a list",
			input: `
services:
  web:
    image: [nginx]
`,
			want: "compose services.web.image (line 4): expected a single value, got a list",
		},
		{
			name: "a health check is a bare command",
			input: `
services:
  web:
    image: nginx:1.25
    healthcheck: "curl -f http://localhost/health"
`,
			want: `compose services.web.healthcheck (line 5): expected a mapping with a test, got "curl -f http://localhost/health"`,
		},
		{
			// The mapping mozza could not read and the one inside it share line 6.
			// Reading into a list, the outer is the answer: anything deeper belongs
			// to a structure mozza never looked inside.
			name: "a service's secrets are a mapping",
			input: `
services:
  web:
    image: nginx:1.25
    secrets:
      api-key: {}
`,
			want: "compose services.web.secrets (line 6): expected a list of secrets, got a mapping",
		},
		{
			// The mirror of the row above: three mappings share line 3, and reading
			// into a single value the innermost is the answer, because every block
			// around it is a field mozza read quite happily.
			name: "a flow-style service with a misshapen setting inside it",
			input: `
services:
  web: {image: nginx:1.25, restart: {a: b}}
`,
			want: "compose services.web.restart (line 3): expected a single value, got a mapping",
		},
		{
			name: "a declared volume is not a mapping",
			input: `
services:
  web:
    image: nginx:1.25
volumes:
  pgdata: local
`,
			want: `compose volumes.pgdata (line 6): expected a mapping of volume settings, got "local"`,
		},
		{
			name: "a volume driver is a list",
			input: `
services:
  web:
    image: nginx:1.25
volumes:
  pgdata:
    driver: [local]
`,
			want: "compose volumes.pgdata.driver (line 7): expected a single value, got a list",
		},
		{
			name: "networks is not a mapping",
			input: `
services:
  web:
    image: nginx:1.25
networks: backend
`,
			want: `compose networks (line 5): expected a mapping of network names to their settings, got "backend"`,
		},
		{
			name: "a declared config names its file without a mapping",
			input: `
services:
  web:
    image: nginx:1.25
configs:
  nginx-conf: ./nginx.conf
`,
			want: `compose configs.nginx-conf (line 6): expected a mapping of settings, got "./nginx.conf"`,
		},
		{
			name: "secrets is not a mapping",
			input: `
services:
  web:
    image: nginx:1.25
secrets: hello
`,
			want: `compose secrets (line 5): expected a mapping of names to their settings, got "hello"`,
		},
		{
			// A key sits on its value's line, and here the key's own text begins
			// with the value's: the one arrangement where which of the two the
			// search settles on changes the answer. yaml reports the value, so the
			// value is what the message quotes back.
			name: "a key whose name begins with its own value",
			input: `
services:
  web:
    image: nginx:1.25
configs:
  conf: co
`,
			want: `compose configs.conf (line 6): expected a mapping of settings, got "co"`,
		},
		{
			// Nothing in the document answers to a field, so the message names the
			// file rather than inventing one.
			name: "the whole document is a scalar",
			input: `
hello
`,
			want: `compose file (line 2): expected a mapping of compose settings, got "hello"`,
		},
		{
			name: "a setting given twice in one service",
			input: `
services:
  web:
    image: nginx:1.25
    image: httpd:2.4
`,
			want: "compose services.web.image (line 5): set twice, first at line 4",
		},
		{
			name: "a service name used twice",
			input: `
services:
  web:
    image: nginx:1.25
  web:
    image: httpd:2.4
`,
			want: "compose services.web (line 5): set twice, first at line 3",
		},
		{
			// Inside a mapping mozza reads through a mirror of its own, yaml's
			// complaint is still the better one: it names the line the key was
			// repeated on, where the mapping's own line is where it opened. A
			// message from the mirror said neither, and called a duplicate a shape.
			name: "a setting given twice inside deploy",
			input: `
services:
  web:
    image: nginx:1.25
    deploy:
      replicas: 5
      replicas: 6
`,
			want: "compose services.web.deploy.replicas (line 7): set twice, first at line 6",
		},
		{
			name: "a setting given twice inside a declared network",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    driver: bridge
    driver: overlay
`,
			want: "compose networks.backend.driver (line 8): set twice, first at line 7",
		},
		{
			// The file does hold a mapping of network settings; the fault is one
			// level inside it, on the field that is really wrong.
			name: "a network driver is a mapping",
			input: `
services:
  web:
    image: nginx:1.25
networks:
  backend:
    driver:
      type: bridge
`,
			want: "compose networks.backend.driver (line 8): expected a single value, got a mapping",
		},
		{
			// A key that is a list names no field of its own, so the path stays the
			// one its parent gave it rather than trailing a dot with nothing after.
			name: "a list standing where a service name belongs",
			input: `
services:
  ? [a, b]
  : c
`,
			want: "compose services (line 3): expected a single value, got a list",
		},
		{
			// Two faults, both reported, in the order the file holds them.
			name: "more than one misshapen value",
			input: `
services:
  web:
    image: [nginx]
    restart: [always]
`,
			want: "compose services.web.image (line 4): expected a single value, got a list;" +
				" compose services.web.restart (line 5): expected a single value, got a list",
		},
		{
			// yaml quotes back only the first ten characters of a value it could not
			// read; the document holds the rest, and a message that reprinted all of
			// a certificate would be no more use than one naming no value at all.
			name: "a value too long to quote back whole",
			input: `
services:
  web:
    image: nginx:1.25
    healthcheck: "curl -f http://localhost:3000/health || exit 1 && echo done"
`,
			want: `compose services.web.healthcheck (line 5): expected a mapping with a test,` +
				` got a value beginning "curl -f http://localhost:3000/health || "`,
		},
		{
			// Not one of yaml's type errors but one of its outright refusals, which
			// names a Go value where compose would name a key.
			name: "a mapping key that is not a name",
			input: `
services:
  web:
    image: nginx:1.25
    environment:
      ? [a, b]
      : c
`,
			want: "compose file: a mapping key must be a name, not a list or a mapping",
		},
		{
			name: "a value tagged as binary that is not base64",
			input: `
services:
  web:
    image: !!binary "@@@"
`,
			want: "compose file: a value is tagged as binary but is not valid base64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for path, err := range map[string]error{
				"text": func() error { _, e := ComposeToRecipe(tt.input); return e }(),
				"ast":  func() error { _, _, e := ComposeToRecipeAST([]byte(tt.input)); return e }(),
			} {
				require.Error(t, err, "%s path", path)

				msg := err.Error()
				assert.Equal(t, "parsing compose YAML: "+tt.want, msg, "%s path", path)
				assertNamesNoGoType(t, msg, path)
			}
		})
	}
}

// TestComposeToRecipeRefusesInjectedValues holds one row per site where a
// compose value reaches recipe text. Without the guard, every payload here
// (except the two health-check rows, noted below) drives the built binary to a
// real Kubernetes manifest the compose file never asked for, so each row is a
// real exploit rather than a hypothetical.
//
// The wantField assertions matter as much as the errors: an import that refuses
// a file without naming the field is one the author cannot act on.
func TestComposeToRecipeRefusesInjectedValues(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantField string
		wantByte  string
	}{
		{
			// One compose service became two recipe slices, one of them with
			// 9 replicas.
			name: "service name breaks out of its header",
			input: `
services:
  aaa:
    image: nginx:1.24
  "evil:\n  from image nginx:1.26\n  run 9 copies\nX":
    image: nginx:1.25
`,
			wantField: "services",
			wantByte:  "a newline",
		},
		{
			// The engine shorthand's version is the tag, written unquoted.
			name: "engine version through the image tag",
			input: `
services:
  db:
    image: "postgres:16\n  run 33 copies\n  # "
`,
			wantField: "services.db.image",
			wantByte:  "a newline",
		},
		{
			// A space alone: no newline and no quote anywhere in the file.
			name: "image adds a directive with a space alone",
			input: `
services:
  api:
    image: "nginx:1.25 run 7 copies"
`,
			wantField: "services.api.image",
			wantByte:  "a space",
		},
		{
			name: "published port adds a directive",
			input: `
services:
  api:
    image: nginx:1.25
    ports:
      - "8080:80 run 3 copies"
`,
			wantField: "services.api.ports",
			wantByte:  "a space",
		},
		{
			name: "unpublished port adds a directive",
			input: `
services:
  api:
    image: nginx:1.25
    ports:
      - "80 run 4 copies"
`,
			wantField: "services.api.ports",
			wantByte:  "a space",
		},
		{
			// extractHealthCheck cuts the path at the first byte of " " (the
			// IndexAny at compose.go:2068), which is why this newline row cannot
			// carry a whole directive past it. That truncation is no defence:
			// its set omits the tab and the comma, both of which the lexer
			// breaks a word on, so the row below reaches the emitter whole.
			name: "health check path escapes its line",
			input: `
services:
  api:
    image: nginx:1.25
    ports:
      - "8080:80"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80/health\nX"]
`,
			wantField: "services.api.healthcheck.test",
			wantByte:  "a newline",
		},
		{
			// The injection the truncation lets through, and the row that pins
			// this field to the bare-word class: tabs alone, no newline in the
			// file, verified to compile to replicas: 5 on the unguarded binary.
			name: "health check path adds a directive with tabs alone",
			input: `
services:
  api:
    image: nginx:1.25
    ports:
      - "8080:80"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80/health\trun\t5\tcopies"]
`,
			wantField: "services.api.healthcheck.test",
			wantByte:  "a tab",
		},
		{
			// The same value in the dropped-check warning, where only a newline
			// escapes. Which of the two branches runs turns on the file's ports
			// rather than on the path, so the field answers to the stricter word
			// class either way: this shape with a port in the URL takes the
			// branch above and compiles the tab payload to replicas: 7.
			name: "health check path escapes the dropped-check comment",
			input: `
services:
  api:
    image: nginx:1.25
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health\n"]
`,
			wantField: "services.api.healthcheck.test",
			wantByte:  "a newline",
		},
		{
			// The reported shape of the defect: replicas: 5 with no newline and
			// no quote in the compose file at all.
			name: "restart adds a directive with a space alone",
			input: `
services:
  api:
    image: nginx:1.25
    restart: "always run 5 copies"
`,
			wantField: "services.api.restart",
			wantByte:  "a space",
		},
		{
			// Within-directive injection: no new directive at all, the argument
			// list itself widens and one dependency becomes two.
			name: "depends_on widens its own argument list",
			input: `
services:
  api:
    image: nginx:1.25
    depends_on: ["db and cache"]
  db:
    image: postgres:16
  cache:
    image: redis:7
`,
			wantField: "services.api.depends_on",
			wantByte:  "a space",
		},
		{
			// mountIsCarried pins the path to the engine's canonical one, so the
			// volume name is the only way into this comment.
			name: "carried mount comment through the volume name",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - "data\n  run 21 copies\n  # :/var/lib/postgresql/data"
volumes:
  "data\n  run 21 copies\n  # ":
`,
			wantField: "services.db.volumes",
			wantByte:  "a newline",
		},
		{
			name: "engine volume warning through the mount path",
			input: `
services:
  db:
    image: postgres:16
    volumes:
      - "vol:/elsewhere\n  run 22 copies\n  # "
volumes:
  vol:
`,
			wantField: "services.db.volumes",
			wantByte:  "a newline",
		},
		{
			name: "plain volume warning through the mount path",
			input: `
services:
  web:
    image: nginx:1.25
    volumes:
      - "vol:/mnt\n  run 23 copies\n  # "
volumes:
  vol:
`,
			wantField: "services.web.volumes",
			wantByte:  "a newline",
		},
		{
			name: "environment key is written unquoted",
			input: `
services:
  api:
    image: nginx:1.25
    environment:
      "K to \"x\"\n  run 8 copies\n  # ": v
`,
			wantField: "services.api.environment",
			wantByte:  "a space",
		},
		{
			// The reported instance. The quotes around the value do nothing:
			// scanString looks for its closing quote on one line only.
			name: "environment value closes its own quotes",
			input: `
services:
  a:
    image: nginx:1.25
    environment:
      EVIL: "x\"\n  run 11 copies\n  set Z to \"y"
`,
			wantField: "services.a.environment.EVIL",
			wantByte:  "a quote",
		},
		{
			// Written twice on one line; classifyWord lowercases, so the
			// uppercasing of the first spelling protects nothing.
			name: "secret source is written unquoted twice",
			input: `
services:
  api:
    image: nginx:1.25
    secrets:
      - source: "t from f\n  run 12 copies\n  # "
secrets:
  t:
    external: true
`,
			wantField: "services.api.secrets",
			wantByte:  "a space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComposeToRecipe(tt.input)

			require.Error(t, err, "the injected value should be refused")
			assert.Empty(t, got, "no recipe text should be produced")

			msg := err.Error()
			assert.Contains(t, msg, tt.wantField, "the error should name the compose field")
			assert.Contains(t, msg, tt.wantByte, "the error should name what could not be written")
		})
	}
}

// TestComposeToRecipeStillAcceptsValuesTheGrammarCanHold is the other half of
// the guarantee. Both values below work today and would be broken by a
// predicate that rejected the same characters everywhere: an env value holds a
// sentence because it is written between quotes, and a mount path holds spaces
// and quotes because it is written into a comment.
func TestComposeToRecipeStillAcceptsValuesTheGrammarCanHold(t *testing.T) {
	input := `
services:
  web:
    image: nginx:1.25
    environment:
      MESSAGE: "hello world from mozza"
    volumes:
      - 'data:/var/lib/my "app" data dir'
volumes:
  data:
`

	got, err := ComposeToRecipe(input)

	require.NoError(t, err, "a file that imports today must go on importing")
	assert.Contains(t, got, `set MESSAGE to "hello world from mozza"`)
	assert.Contains(t, got, `/var/lib/my "app" data dir`)
	mustParseRecipe(t, got)
}

// TestComposeToRecipeAcceptsOrdinaryCompose guards against the other failure
// mode: a predicate strict enough to reject the everyday file. Every value
// here is one a real compose file holds.
func TestComposeToRecipeAcceptsOrdinaryCompose(t *testing.T) {
	input := `
services:
  web:
    image: ghcr.io/owner/repo:v1.2.3-alpine
    ports:
      - "8080:80/tcp"
    restart: unless-stopped
    depends_on:
      - db
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80/healthz || exit 1"]
    environment:
      DATABASE_URL: "postgres://user:pw@db:5432/app?sslmode=disable"
      JSON_CONFIG: "{'k': 'v'}"
    secrets:
      - source: api-token
  db:
    image: postgres:16-alpine
    volumes:
      - "pgdata:/var/lib/postgresql/data"
volumes:
  pgdata:
secrets:
  api-token:
    external: true
`

	got, err := ComposeToRecipe(input)

	require.NoError(t, err)
	assert.Contains(t, got, "from image ghcr.io/owner/repo:v1.2.3-alpine")
	assert.Contains(t, got, "open to the public on port 80")
	assert.Contains(t, got, "restart unless-stopped")
	assert.Contains(t, got, "needs db")
	assert.Contains(t, got, "health check /healthz")
	assert.Contains(t, got, "secret API_TOKEN from api-token")
	mustParseRecipe(t, got)
}
