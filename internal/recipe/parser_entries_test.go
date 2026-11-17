package recipe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
