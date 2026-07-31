package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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

// TestImportedStorageIsAlwaysAFormTheParserAccepts pins the storage dimension of
// the same import invariant. A recipe states storage in exactly one place — the
// engine shorthand's size argument, "postgres 16, 10Gi" — so that is the only
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
// really lands. A recipe cannot name a mount path for it — parseMount carries
// only file, secret and config — so the volume goes where the compiler puts it,
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
// shape before it was asked of the importer, and every row marked as a tolerated
// shape aborted the parse with a raw YAML type error before this change.
//
// Both authoring paths are driven, because `mozza import` drives both — the text
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
			// today as the port "8080:80" — the long form is right and the short
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
			// The service deliberately does not join the network. Importing a
			// service that joins one produces a NetworkPolicy naming a compose
			// network rather than a slice, which mozza validate rejects — a
			// separate defect, filed on its own, and one this row would otherwise
			// be quietly asserting the absence of.
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
			// the warning has to say and what the old wording denied.
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
		// alias first reads the label instead of the value — silently, when the
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
			// Reading the alias node's own .Value yields "5", which Atoi accepts —
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

		// The rows below already parsed before this change. They are the ones a
		// careless fix regresses — each is a value the first decoder handles and a
		// string-first decoder would mangle — so they are asserted, not assumed.
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
			// An aliased short-syntax port imported before this change, because a
			// plain []string field went through yaml's own sequence decoder, which
			// follows aliases. Walking a sequence's Content by hand does not, so
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
			// one — which is only true if the alias is followed before the tag is
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
			// form imports to would be a second conversion path, free to drift —
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

// TestUnreadableComposeShapesSayWhatTheyExpected covers the other half of the
// on-ramp: a shape mozza genuinely cannot read fails with a sentence a compose
// author can act on. yaml.v3 hands a custom unmarshaler's error back verbatim, so
// what is written here is what reaches the terminal, and the tags and Go type
// names that used to reach it are asserted absent rather than merely improved on.
// TestUnreadableComposeShapesSayWhatTheyExpected covers the shapes mozza's own
// unmarshalers catch, where mozza chooses the words rather than restating yaml's.
//
// Every want is the whole of what a compose author sees after the wrapping
// prefixes, so the copy is pinned rather than spot-checked, and every field path
// is the one `docker compose config` prints for the same file. An unmarshaler is
// handed a value and no path to it, so these names are read back off the document
// afterwards — which is what tells a service's own `volumes:` from the top-level
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
			// that used to answer to the same name. The path tells them apart —
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
			// Compose refuses this file — "is missing a target port" — and mozza
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
// service name to be had from inside one — and a vague "service volumes" is worth
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
// is the one `docker compose config` prints for the same file — verified against
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
			// than "a single value" — but the path still names it exactly, which is
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
			// with the value's — the one arrangement where which of the two the
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
