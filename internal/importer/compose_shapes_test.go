package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

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
