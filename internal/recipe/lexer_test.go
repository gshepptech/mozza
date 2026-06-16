package recipe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tok is a shorthand helper for building expected Token values in tests.
func tok(tt TokenType, val string, line, col int) Token {
	return Token{Type: tt, Value: val, Line: line, Col: col}
}

func TestLexer_Tokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      []Token
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "empty input",
			input: "",
			want: []Token{
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "comment only",
			input: "# This is a comment",
			want: []Token{
				tok(TokenComment, "# This is a comment", 1, 1),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "comment with leading whitespace",
			input: "  # indented comment",
			want: []Token{
				tok(TokenComment, "# indented comment", 1, 3),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "app declaration",
			input: "App: simple-api",
			want: []Token{
				tok(TokenApp, "App", 1, 1),
				tok(TokenIdent, "simple-api", 1, 6),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "section header",
			input: "Storefront:",
			want: []Token{
				tok(TokenSectionHeader, "Storefront", 1, 1),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "from image directive",
			input: "  from image acme/storefront:latest",
			want: []Token{
				tok(TokenFrom, "from", 1, 3),
				tok(TokenImage, "image", 1, 8),
				tok(TokenIdent, "acme/storefront:latest", 1, 14),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "open to the public on port",
			input: "  open to the public on port 3000",
			want: []Token{
				tok(TokenOpen, "open", 1, 3),
				tok(TokenTo, "to", 1, 8),
				tok(TokenThe, "the", 1, 11),
				tok(TokenPublic, "public", 1, 15),
				tok(TokenOn, "on", 1, 22),
				tok(TokenPort, "port", 1, 25),
				tok(TokenNumber, "3000", 1, 30),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "on port directive",
			input: "  on port 8080",
			want: []Token{
				tok(TokenOn, "on", 1, 3),
				tok(TokenPort, "port", 1, 6),
				tok(TokenNumber, "8080", 1, 11),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "health check directive",
			input: "  health check /healthz",
			want: []Token{
				tok(TokenHealth, "health", 1, 3),
				tok(TokenCheck, "check", 1, 10),
				tok(TokenIdent, "/healthz", 1, 16),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "run copies directive",
			input: "  run 3 copies",
			want: []Token{
				tok(TokenRun, "run", 1, 3),
				tok(TokenNumber, "3", 1, 7),
				tok(TokenCopies, "copies", 1, 9),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "run 1 copy",
			input: "  run 1 copy",
			want: []Token{
				tok(TokenRun, "run", 1, 3),
				tok(TokenNumber, "1", 1, 7),
				tok(TokenCopy, "copy", 1, 9),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "needs with and separator",
			input: "  needs db and cache",
			want: []Token{
				tok(TokenNeeds, "needs", 1, 3),
				tok(TokenIdent, "db", 1, 9),
				tok(TokenAnd, "and", 1, 12),
				tok(TokenIdent, "cache", 1, 16),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "needs with comma separator",
			input: "  needs db, cache",
			want: []Token{
				tok(TokenNeeds, "needs", 1, 3),
				tok(TokenIdent, "db", 1, 9),
				tok(TokenComma, ",", 1, 11),
				tok(TokenIdent, "cache", 1, 13),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "postgres shorthand",
			input: "  postgres 16, 20Gi, daily backups",
			want: []Token{
				tok(TokenPostgres, "postgres", 1, 3),
				tok(TokenNumber, "16", 1, 12),
				tok(TokenComma, ",", 1, 14),
				tok(TokenIdent, "20Gi", 1, 16),
				tok(TokenComma, ",", 1, 20),
				tok(TokenDaily, "daily", 1, 22),
				tok(TokenBackups, "backups", 1, 28),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "redis shorthand",
			input: "  redis 7",
			want: []Token{
				tok(TokenRedis, "redis", 1, 3),
				tok(TokenNumber, "7", 1, 9),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "blank lines ignored",
			input: "App: x\n\n\nApi:",
			want: []Token{
				tok(TokenApp, "App", 1, 1),
				tok(TokenIdent, "x", 1, 6),
				tok(TokenNewline, "\n", 1, 7),
				tok(TokenNewline, "\n", 2, 1),
				tok(TokenNewline, "\n", 3, 1),
				tok(TokenSectionHeader, "Api", 4, 1),
				tok(TokenEOF, "", 4, 1),
			},
		},
		{
			name:      "unterminated string",
			input:     `  from image "missing-close`,
			wantErr:   true,
			errSubstr: "unterminated string",
		},
		{
			name:  "multiple comments",
			input: "# first\n# second",
			want: []Token{
				tok(TokenComment, "# first", 1, 1),
				tok(TokenNewline, "\n", 1, 8),
				tok(TokenComment, "# second", 2, 1),
				tok(TokenEOF, "", 2, 1),
			},
		},
		{
			name:  "tab indentation",
			input: "\tfrom image test:latest",
			want: []Token{
				tok(TokenFrom, "from", 1, 2),
				tok(TokenImage, "image", 1, 7),
				tok(TokenIdent, "test:latest", 1, 13),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "set directive",
			input: `  set DATABASE_URL to "postgres://db:5432/app"`,
			want: []Token{
				tok(TokenSet, "set", 1, 3),
				tok(TokenIdent, "DATABASE_URL", 1, 7),
				tok(TokenTo, "to", 1, 20),
				tok(TokenString, "postgres://db:5432/app", 1, 23),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "limit cpu directive",
			input: `  limit cpu to "500m"`,
			want: []Token{
				tok(TokenLimit, "limit", 1, 3),
				tok(TokenCpu, "cpu", 1, 9),
				tok(TokenTo, "to", 1, 13),
				tok(TokenString, "500m", 1, 16),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "limit memory directive",
			input: `  limit memory to "256Mi"`,
			want: []Token{
				tok(TokenLimit, "limit", 1, 3),
				tok(TokenMemory, "memory", 1, 9),
				tok(TokenTo, "to", 1, 16),
				tok(TokenString, "256Mi", 1, 19),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "restart always directive",
			input: "  restart always",
			want: []Token{
				tok(TokenRestart, "restart", 1, 3),
				tok(TokenAlways, "always", 1, 11),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "restart unless-stopped directive",
			input: "  restart unless-stopped",
			want: []Token{
				tok(TokenRestart, "restart", 1, 3),
				tok(TokenUnlessStopped, "unless-stopped", 1, 11),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "domain directive",
			input: `  domain "api.example.com"`,
			want: []Token{
				tok(TokenDomain, "domain", 1, 3),
				tok(TokenString, "api.example.com", 1, 10),
				tok(TokenEOF, "", 1, 1),
			},
		},
		{
			name:  "namespace declaration",
			input: "Namespace: production",
			want: []Token{
				tok(TokenNamespace, "Namespace", 1, 1),
				tok(TokenIdent, "production", 1, 12),
				tok(TokenEOF, "", 1, 1),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lexer := NewLexer(tt.input)
			got, err := lexer.Tokens()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLexer_ErrorPosition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		errLine int
		errCol  int
	}{
		{
			name:    "unterminated string on first line",
			input:   `  from image "oops`,
			errLine: 1,
			errCol:  14,
		},
		{
			name:    "unterminated string on second line",
			input:   "# comment\n  from image \"oops",
			errLine: 2,
			errCol:  14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lexer := NewLexer(tt.input)
			_, err := lexer.Tokens()
			require.Error(t, err)

			expected := newParseError(tt.errLine, tt.errCol, "unterminated string literal")
			assert.Contains(t, err.Error(), expected.Error())
		})
	}
}

func TestLexer_ErrorRecovery(t *testing.T) {
	t.Parallel()

	// An error on line 1 should not prevent tokens from line 2.
	input := "  from image \"unclosed\nApi:"
	lexer := NewLexer(input)
	tokens, err := lexer.Tokens()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unterminated string")

	// Line 2 should still produce tokens despite line 1 error.
	var sectionFound bool

	for _, tok := range tokens {
		if tok.Type == TokenSectionHeader {
			sectionFound = true

			break
		}
	}

	assert.True(t, sectionFound, "expected SectionHeader from line 2 after error on line 1")
}

func TestLexer_SimpleAPIExample(t *testing.T) {
	t.Parallel()

	input := `# Simple API — single web service with a database

App: simple-api

Api:
  from image myorg/api:latest
  open to the public on port 8080
  health check /healthz
  run 2 copies
  needs db

Db:
  postgres 16, 10Gi`

	lexer := NewLexer(input)
	tokens, err := lexer.Tokens()
	require.NoError(t, err)

	// Verify key structural tokens are present and in order.
	typeSequence := filterSignificant(tokens)

	expected := []TokenType{
		TokenComment,
		TokenApp, TokenIdent,
		TokenSectionHeader,
		TokenFrom, TokenImage, TokenIdent,
		TokenOpen, TokenTo, TokenThe, TokenPublic, TokenOn, TokenPort, TokenNumber,
		TokenHealth, TokenCheck, TokenIdent,
		TokenRun, TokenNumber, TokenCopies,
		TokenNeeds, TokenIdent,
		TokenSectionHeader,
		TokenPostgres, TokenNumber, TokenComma, TokenIdent,
		TokenEOF,
	}

	assert.Equal(t, expected, typeSequence)
}

// filterSignificant returns token types, excluding newlines.
func filterSignificant(tokens []Token) []TokenType {
	var types []TokenType

	for _, t := range tokens {
		if t.Type == TokenNewline {
			continue
		}

		types = append(types, t.Type)
	}

	return types
}

func TestTokenType_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tt   TokenType
		want string
	}{
		{TokenComment, "Comment"},
		{TokenApp, "App"},
		{TokenEOF, "EOF"},
		{TokenType(999), "TokenType(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.tt.String())
		})
	}
}

func TestToken_String(t *testing.T) {
	t.Parallel()

	token := Token{Type: TokenApp, Value: "App", Line: 1, Col: 1}
	assert.Equal(t, `App("App")@1:1`, token.String())
}

func TestParseError_Error(t *testing.T) {
	t.Parallel()

	err := newParseError(5, 10, "something went wrong")
	assert.Equal(t, "line 5, col 10: something went wrong", err.Error())
}

func TestParseErrorf(t *testing.T) {
	t.Parallel()

	err := newParseErrorf(1, 1, "bad token %q", "@@")
	assert.Equal(t, `line 1, col 1: bad token "@@"`, err.Error())
}

func TestLexer_EnvLimitsExample(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "env-limits.mozza"))
	require.NoError(t, err)

	lexer := NewLexer(string(data))
	tokens, err := lexer.Tokens()
	require.NoError(t, err)

	typeSequence := filterSignificant(tokens)

	// Verify key token types from Phase 2 extensions are present.
	assert.Contains(t, typeSequence, TokenSet)
	assert.Contains(t, typeSequence, TokenLimit)
	assert.Contains(t, typeSequence, TokenRestart)
	assert.Contains(t, typeSequence, TokenDomain)
	assert.Contains(t, typeSequence, TokenNamespace)
}

func TestLexer_NewKeywords(t *testing.T) {
	t.Parallel()

	// Verify every new keyword maps to the correct token type.
	tests := []struct {
		keyword string
		want    TokenType
	}{
		{"as", TokenAs},
		{"using", TokenUsing},
		{"every", TokenEvery},
		{"at", TokenAt},
		{"schedule", TokenSchedule},
		{"once", TokenOnce},
		{"completion", TokenCompletion},
		{"parallel", TokenParallel},
		{"retry", TokenRetry},
		{"node", TokenNode},
		{"labeled", TokenLabeled},
		{"except", TokenExcept},
		{"each", TokenEach},
		{"own", TokenOwn},
		{"ordered", TokenOrdered},
		{"find", TokenFind},
		{"before", TokenBefore},
		{"starting", TokenStarting},
		{"sidecar", TokenSidecar},
		{"with", TokenWith},
		{"mount", TokenMount},
		{"file", TokenFile},
		{"config", TokenConfig},
		{"permission", TokenPermission},
		{"manage", TokenManage},
		{"account", TokenAccount},
		{"read", TokenRead},
		{"write", TokenWrite},
		{"read-only", TokenReadOnly},
		{"readiness", TokenReadiness},
		{"liveness", TokenLiveness},
		{"startup", TokenStartup},
		{"running", TokenRunning},
		{"stopping", TokenStopping},
		{"wait", TokenWait},
		{"seconds", TokenSeconds},
		{"prefer", TokenPrefer},
		{"require", TokenRequire},
		{"avoid", TokenAvoid},
		{"spread", TokenSpread},
		{"across", TokenAcross},
		{"zones", TokenZones},
		{"never", TokenNever},
		{"accept", TokenAccept},
		{"traffic", TokenTraffic},
		{"block", TokenBlock},
		{"reachable", TokenReachable},
		{"kind", TokenKind},
		{"scale", TokenScale},
		{"between", TokenBetween},
		{"based", TokenBased},
		{"keep", TokenKeep},
		{"during", TokenDuring},
		{"updates", TokenUpdates},
		{"allow", TokenAllow},
		{"down", TokenDown},
		{"user", TokenUser},
		{"group", TokenGroup},
		{"drop", TokenDrop},
		{"capability", TokenCapability},
		{"filesystem", TokenFilesystem},
		{"update", TokenUpdate},
		{"graceful", TokenGraceful},
		{"shutdown", TokenShutdown},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			t.Parallel()

			input := "  " + tt.keyword
			lexer := NewLexer(input)
			tokens, err := lexer.Tokens()
			require.NoError(t, err)

			// First significant token should be the keyword.
			significant := filterSignificant(tokens)
			require.NotEmpty(t, significant)
			assert.Equal(t, tt.want, significant[0], "keyword %q should lex as %s", tt.keyword, tt.want)
		})
	}
}

func TestLexer_NewKeywordsCaseInsensitive(t *testing.T) {
	t.Parallel()

	// The lexer lowercases words, so mixed-case should still match.
	tests := []struct {
		input string
		want  TokenType
	}{
		{"  Sidecar", TokenSidecar},
		{"  MOUNT", TokenMount},
		{"  Scale", TokenScale},
		{"  GRACEFUL", TokenGraceful},
		{"  Readiness", TokenReadiness},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokens()
			require.NoError(t, err)

			significant := filterSignificant(tokens)
			require.NotEmpty(t, significant)
			assert.Equal(t, tt.want, significant[0])
		})
	}
}

func TestLexer_NewDirectiveSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantSeq []TokenType
	}{
		{
			name:  "scale between directive",
			input: "  scale between 2 and 10 based on cpu",
			wantSeq: []TokenType{
				TokenScale, TokenBetween, TokenNumber, TokenAnd, TokenNumber,
				TokenBased, TokenOn, TokenCpu, TokenEOF,
			},
		},
		{
			name:  "graceful shutdown directive",
			input: "  graceful shutdown 30 seconds",
			wantSeq: []TokenType{
				TokenGraceful, TokenShutdown, TokenNumber, TokenSeconds, TokenEOF,
			},
		},
		{
			name:  "mount config directive",
			input: `  mount config "app.yaml" to "/etc/app"`,
			wantSeq: []TokenType{
				TokenMount, TokenConfig, TokenString, TokenTo, TokenString, TokenEOF,
			},
		},
		{
			name:  "accept traffic directive",
			input: "  accept traffic from storefront",
			wantSeq: []TokenType{
				TokenAccept, TokenTraffic, TokenFrom, TokenIdent, TokenEOF,
			},
		},
		{
			name:  "run once on schedule",
			input: `  run once every "1h"`,
			wantSeq: []TokenType{
				TokenRun, TokenOnce, TokenEvery, TokenString, TokenEOF,
			},
		},
		{
			name:  "before starting directive",
			input: "  before starting run migrate",
			wantSeq: []TokenType{
				TokenBefore, TokenStarting, TokenRun, TokenIdent, TokenEOF,
			},
		},
		{
			name:  "spread across zones",
			input: "  spread across zones",
			wantSeq: []TokenType{
				TokenSpread, TokenAcross, TokenZones, TokenEOF,
			},
		},
		{
			name:  "keep during updates",
			input: "  keep 2 during updates",
			wantSeq: []TokenType{
				TokenKeep, TokenNumber, TokenDuring, TokenUpdates, TokenEOF,
			},
		},
		{
			name:  "drop capability directive",
			input: "  drop capability ALL",
			wantSeq: []TokenType{
				TokenDrop, TokenCapability, TokenIdent, TokenEOF,
			},
		},
		{
			name:  "read-only filesystem",
			input: "  read-only filesystem",
			wantSeq: []TokenType{
				TokenReadOnly, TokenFilesystem, TokenEOF,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokens()
			require.NoError(t, err)

			seq := filterSignificant(tokens)
			assert.Equal(t, tt.wantSeq, seq)
		})
	}
}

func TestLexer_NewTokensDontBreakExisting(t *testing.T) {
	t.Parallel()

	// Verify the original simple-api example still works unchanged.
	input := `App: simple-api

Api:
  from image myorg/api:latest
  open to the public on port 8080
  health check /healthz
  run 2 copies
  needs db
  restart always

Db:
  postgres 16, 10Gi, daily backups`

	lexer := NewLexer(input)
	tokens, err := lexer.Tokens()
	require.NoError(t, err)

	seq := filterSignificant(tokens)
	expected := []TokenType{
		TokenApp, TokenIdent,
		TokenSectionHeader,
		TokenFrom, TokenImage, TokenIdent,
		TokenOpen, TokenTo, TokenThe, TokenPublic, TokenOn, TokenPort, TokenNumber,
		TokenHealth, TokenCheck, TokenIdent,
		TokenRun, TokenNumber, TokenCopies,
		TokenNeeds, TokenIdent,
		TokenRestart, TokenAlways,
		TokenSectionHeader,
		TokenPostgres, TokenNumber, TokenComma, TokenIdent, TokenComma, TokenDaily, TokenBackups,
		TokenEOF,
	}
	assert.Equal(t, expected, seq)
}
