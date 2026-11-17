package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
