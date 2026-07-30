package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
