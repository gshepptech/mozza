package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

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
