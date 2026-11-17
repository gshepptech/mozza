package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
