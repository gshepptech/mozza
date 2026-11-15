package importer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/recipe"
)

// The tests below assert the property each predicate exists to guarantee —
// that an accepted value still occupies exactly the one position it was written
// into — by lexing the emitted line with the real lexer. Asserting the property
// rather than the character set is what keeps these honest: a set restated in a
// test drifts away from the grammar the moment the grammar moves, and passes
// while it drifts.

// significantTokens lexes source and returns its tokens with the trailing EOF
// dropped, so a count is a count of what the value actually became.
func significantTokens(t *testing.T, source string) []recipe.Token {
	t.Helper()

	tokens, err := recipe.NewLexer(source).Tokens()
	require.NoError(t, err, "emitted line should lex without error:\n%s", source)
	require.NotEmpty(t, tokens)
	require.Equal(t, recipe.TokenEOF, tokens[len(tokens)-1].Type, "last token should be EOF")

	return tokens[:len(tokens)-1]
}

// --- bare word ---

// wordsAcceptedInRecipes are values a bare-word site must keep accepting. Each
// is adversarial in some way the grammar does not care about — a colon, a hash,
// a single quote — and each is a shape a real compose file holds.
var wordsAcceptedInRecipes = []string{
	"nginx",
	"nginx:1.25",
	"ghcr.io/owner/repo:v1.2.3",
	"docker.io/library/postgres:16-alpine",
	"8080",
	"/health",
	"/healthz?ready=1",
	"always",
	"unless-stopped",
	"POSTGRES_PASSWORD",
	"db",
	"a:b",
	"a#b",
	"a'b",
	"a|b",
	"a=b",
	"a(b)",
	"100%",
	"a\\b",
	"a{b}",
	"a$b",
	"café",
	"-alpine",
	"5",
}

func TestRecipeSafeWordAcceptsValuesThatLexToOneToken(t *testing.T) {
	for _, value := range wordsAcceptedInRecipes {
		t.Run(value, func(t *testing.T) {
			require.NoError(t, recipeSafeWord("field", value))

			// Wrapped the way an emitter writes one: indented, after a directive
			// keyword, unquoted.
			tokens := significantTokens(t, "  restart "+value)

			require.Len(t, tokens, 2, "value should add exactly one token to the directive")
			assert.Equal(t, value, tokens[1].Value, "the token should be the whole value")
		})
	}
}

// TestRecipeSafeWordRejectsEveryByteTheLexerBreaksAWordOn is the test that makes
// the bare-word set a guarantee rather than a list. It sweeps every byte, and
// for each one the predicate accepts it proves the lexer really does keep the
// value whole. A byte added to the lexer's isDelimiter and not to
// structuralInWord fails here.
func TestRecipeSafeWordRejectsEveryByteTheLexerBreaksAWordOn(t *testing.T) {
	for b := range 256 {
		value := "x" + string([]byte{byte(b)}) + "y"

		if err := recipeSafeWord("field", value); err != nil {
			// Rejected. The lexer is allowed to disagree only in the direction
			// of safety, and the one byte it does is documented below.
			continue
		}

		tokens := significantTokens(t, "  restart "+value)
		require.Lenf(t, tokens, 2,
			"byte %#x is accepted by recipeSafeWord but splits a bare word — add it to structuralInWord", b)
		assert.Equalf(t, value, tokens[1].Value,
			"byte %#x is accepted by recipeSafeWord but changes the token's value", b)
	}
}

// TestRecipeSafeWordRejectsCarriageReturnAheadOfTheLexer records the one place
// the predicate is deliberately stricter than today's grammar. The lexer splits
// on "\n" alone and isDelimiter omits '\r', so a bare CR survives a round trip
// right now — but a CRLF-authored recipe does not parse at all, so normalising
// line endings is an ordinary future fix, and it would make '\r' structural at
// every site at once.
func TestRecipeSafeWordRejectsCarriageReturnAheadOfTheLexer(t *testing.T) {
	require.Error(t, recipeSafeWord("field", "x\ry"))

	tokens := significantTokens(t, "  restart x\ry")
	require.Len(t, tokens, 2, "today's lexer keeps a CR inside the word — the rejection is forward-looking")
}

// --- quoted string ---

// stringsAcceptedInRecipes are values a quoted site must keep accepting. Spaces,
// tabs and commas are inert inside a string, and an env value holding a sentence
// is ordinary — `MESSAGE: "hello world from mozza"` works today and has to keep
// working.
var stringsAcceptedInRecipes = []string{
	"hello world from mozza",
	"db:3306",
	"a,b,c",
	"a\tb",
	"postgres://user:pw@host:5432/db?sslmode=disable",
	"{'k':'v'}",
	"  leading and trailing  ",
	"# not a comment here",
	"line one; line two",
}

func TestRecipeSafeStringAcceptsValuesThatLexToOneString(t *testing.T) {
	for _, value := range stringsAcceptedInRecipes {
		t.Run(value, func(t *testing.T) {
			require.NoError(t, recipeSafeString("field", value))

			// Wrapped exactly as the env emitter writes one.
			tokens := significantTokens(t, `  set KEY to "`+value+`"`)

			require.Len(t, tokens, 4, "value should add exactly one token to the directive")
			assert.Equal(t, recipe.TokenString, tokens[3].Type, "the value should lex as one string")
			assert.Equal(t, value, tokens[3].Value, "the string should be the whole value")
		})
	}
}

// TestRecipeSafeStringRejectsEveryByteThatEndsAString sweeps the string position
// the same way the bare-word sweep does. It is a separate sweep because the
// string position accepts bytes the bare-word position cannot — that difference
// is the reason the predicate is position-aware at all, and a test that shared
// one sweep would not notice if it stopped being true.
func TestRecipeSafeStringRejectsEveryByteThatEndsAString(t *testing.T) {
	for b := range 256 {
		value := "x" + string([]byte{byte(b)}) + "y"

		if err := recipeSafeString("field", value); err != nil {
			continue
		}

		tokens := significantTokens(t, `  set KEY to "`+value+`"`)
		require.Lenf(t, tokens, 4,
			"byte %#x is accepted by recipeSafeString but ends the quoted value — add it to structuralInString", b)
		require.Equalf(t, recipe.TokenString, tokens[3].Type,
			"byte %#x is accepted by recipeSafeString but stops the value being one string", b)
		assert.Equalf(t, value, tokens[3].Value,
			"byte %#x is accepted by recipeSafeString but changes the string's value", b)
	}
}

// TestRecipeSafeStringAcceptsWhatBareWordsRefuse holds the two positions apart.
// A predicate that rejected spaces everywhere would pass every rejection test
// and quietly break the env values this asserts.
func TestRecipeSafeStringAcceptsWhatBareWordsRefuse(t *testing.T) {
	for _, value := range []string{"hello world", "a,b", "a\tb"} {
		require.NoError(t, recipeSafeString("field", value), "a string holds what a word cannot")
		require.Error(t, recipeSafeWord("field", value), "a bare word cannot hold it")
	}
}

// --- comment ---

// commentsAcceptedInRecipes are values a comment site must keep accepting. A
// mount path with spaces *and* quotes imports and compiles today; the volume
// warnings report it verbatim and nothing in a comment can end the comment.
var commentsAcceptedInRecipes = []string{
	`/var/lib/my "app" data dir`,
	"/data",
	"/var/lib/postgresql/data",
	"a b c",
	"x,y",
	`path with 'quotes'",`,
	"# another hash",
	"tab\tseparated",
}

func TestRecipeSafeCommentAcceptsValuesThatStayInOneComment(t *testing.T) {
	for _, value := range commentsAcceptedInRecipes {
		t.Run(value, func(t *testing.T) {
			require.NoError(t, recipeSafeComment("field", value))

			// Wrapped as writeVolumes writes one: the value sits mid-comment,
			// with the warning's own text following it.
			tokens := significantTokens(t, "  # WARNING: volume data at "+value+" dropped")

			require.Len(t, tokens, 1, "the whole line should stay one comment")
			assert.Equal(t, recipe.TokenComment, tokens[0].Type)
			assert.Contains(t, tokens[0].Value, value, "the comment should still hold the value")
		})
	}
}

// TestRecipeSafeCommentRejectsEveryByteThatEndsAComment sweeps the comment
// position. It is the loosest of the three — only a line ending escapes a
// comment — so it is also where a position-blind predicate does the most damage.
func TestRecipeSafeCommentRejectsEveryByteThatEndsAComment(t *testing.T) {
	for b := range 256 {
		value := "x" + string([]byte{byte(b)}) + "y"

		if err := recipeSafeComment("field", value); err != nil {
			continue
		}

		tokens := significantTokens(t, "  # WARNING: volume data at "+value+" dropped")
		require.Lenf(t, tokens, 1,
			"byte %#x is accepted by recipeSafeComment but escapes the comment — add it to structuralInComment", b)
		assert.Equalf(t, recipe.TokenComment, tokens[0].Type,
			"byte %#x is accepted by recipeSafeComment but stops the line being a comment", b)
	}
}

// TestRecipeSafeCommentAcceptsWhatBareWordsAndStringsRefuse is the negative side
// of the comment class: a quote is not structural here, and a mount path holding
// one has to go on importing.
func TestRecipeSafeCommentAcceptsWhatBareWordsAndStringsRefuse(t *testing.T) {
	value := `/var/lib/my "app" data dir`

	require.NoError(t, recipeSafeComment("field", value), "a comment holds quotes and spaces")
	require.Error(t, recipeSafeString("field", value), "a quoted value cannot hold a quote")
	require.Error(t, recipeSafeWord("field", value), "a bare word cannot hold a space")
}

// --- the predicates' own bookkeeping ---

// TestEveryStructuralByteIsNamed holds the rejected sets against the names a
// message uses for them, so the fallback in recipeSafe cannot quietly become the
// path a real message takes.
func TestEveryStructuralByteIsNamed(t *testing.T) {
	for _, set := range []string{structuralInWord, structuralInString, structuralInComment} {
		for i := 0; i < len(set); i++ {
			assert.Containsf(t, structuralByteNames, set[i],
				"byte %#x is rejected but has no name for a message", set[i])
		}
	}
}

// TestRecipeSafeNamesTheFieldAndTheByte asserts the message is one an author can
// act on: which field, which value, and what about it could not be written.
func TestRecipeSafeNamesTheFieldAndTheByte(t *testing.T) {
	err := recipeSafeWord("services.api.restart", "always run 5 copies")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "services.api.restart")
	assert.Contains(t, err.Error(), `"always run 5 copies"`)
	assert.Contains(t, err.Error(), "a space")
}

// TestRecipeSafeTruncatesALongValue keeps a message shorter than the certificate
// a compose scalar can hold, the way composeReadError's messages already do.
func TestRecipeSafeTruncatesALongValue(t *testing.T) {
	value := strings.Repeat("a", maxReportedValue) + " and more"

	err := recipeSafeWord("field", value)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "a value beginning")
	assert.NotContains(t, err.Error(), "and more")
}

// --- Helm and Dockerfile ---
//
// Both are pure functions, so they are driven directly. Neither can be reached
// offline through Scan: it fetches over hardcoded GitHub hosts.

func TestHelmToRecipeRefusesInjectedValues(t *testing.T) {
	tests := []struct {
		name      string
		values    string
		chartName string
		wantField string
	}{
		{
			name:      "chart name from the URL path",
			values:    "image:\n  repository: nginx\n  tag: \"1.25\"\n",
			chartName: "chart\n  run 9 copies\nX",
			wantField: "chart name",
		},
		{
			name:      "image repository from values.yaml",
			values:    "image:\n  repository: \"nginx run 9 copies\"\n  tag: \"1.25\"\n",
			chartName: "chart",
			wantField: "values.yaml image.repository",
		},
		{
			name:      "image tag from values.yaml",
			values:    "image:\n  repository: nginx\n  tag: \"1.25 run 9 copies\"\n",
			chartName: "chart",
			wantField: "values.yaml image.tag",
		},
		{
			// extractHelmPort formats whatever node it finds with %v, so the
			// port is a string vector even though a port is a number.
			name:      "service port is formatted from an arbitrary node",
			values:    "service:\n  port: \"80 run 9 copies\"\n",
			chartName: "chart",
			wantField: "values.yaml service.port",
		},
		{
			// The fallback key, and the reason the field travels out of
			// extractHelmPort rather than being named at the check: a
			// values.yaml holding only containerPort has no service block, so
			// a message naming service.port would send its author to a field
			// that is not in their file.
			name:      "container port is formatted from an arbitrary node",
			values:    "containerPort: \"80 run 9 copies\"\n",
			chartName: "chart",
			wantField: "values.yaml containerPort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := HelmToRecipe(tt.values, tt.chartName)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantField)
		})
	}
}

func TestHelmToRecipeStillConvertsOrdinaryCharts(t *testing.T) {
	values := "image:\n  repository: nginx\n  tag: \"1.25\"\nservice:\n  port: 8080\nreplicaCount: 3\n"

	got, err := HelmToRecipe(values, "my-chart")

	require.NoError(t, err)
	assert.Contains(t, got, "from image nginx:1.25")
	assert.Contains(t, got, "on port 8080")
	assert.Contains(t, got, "run 3 copies")
	mustParseRecipe(t, got)
}

func TestDockerfileToRecipeRefusesInjectedValues(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
		repoName   string
		wantField  string
	}{
		{
			name:       "repo name from the URL path",
			dockerfile: "FROM nginx\nEXPOSE 8080\n",
			repoName:   "repo\n  run 9 copies\nX",
			wantField:  "repository name",
		},
		{
			name:       "port from the EXPOSE line",
			dockerfile: "FROM nginx\nEXPOSE 8080\"\n",
			repoName:   "repo",
			wantField:  "Dockerfile EXPOSE port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DockerfileToRecipe(tt.dockerfile, tt.repoName)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantField)
		})
	}
}

func TestDockerfileToRecipeStillConvertsOrdinaryDockerfiles(t *testing.T) {
	got, err := DockerfileToRecipe("FROM nginx\nEXPOSE 8080/tcp\n", "my-repo")

	require.NoError(t, err)
	assert.Contains(t, got, "from image my-repo:latest")
	assert.Contains(t, got, "open to the public on port 8080")
	mustParseRecipe(t, got)
}

// TestGuardedImportersProduceParseableRecipes is the round-trip the guards
// exist to protect: whatever these importers accept has to parse back, and the
// accepted values below are the adversarial ones the predicates let through.
func TestGuardedImportersProduceParseableRecipes(t *testing.T) {
	for _, name := range []string{"my-chart", "chart.with.dots", "chart_underscore"} {
		t.Run("helm/"+name, func(t *testing.T) {
			got, err := HelmToRecipe("image:\n  repository: nginx\n  tag: \"1.25\"\n", name)
			require.NoError(t, err)
			mustParseRecipe(t, got)
		})

		t.Run("dockerfile/"+name, func(t *testing.T) {
			got, err := DockerfileToRecipe("FROM nginx\nEXPOSE 8080\n", name)
			require.NoError(t, err)
			mustParseRecipe(t, got)
		})
	}
}
