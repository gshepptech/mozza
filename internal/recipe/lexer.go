package recipe

import (
	"errors"
	"strings"
	"unicode"
)

// Lexer tokenizes .mozza recipe source text line by line.
// It is indentation-aware: lines starting at column 1 that end with ":"
// are treated as section headers; indented lines are directives.
type Lexer struct {
	source string
	lines  []string
}

// NewLexer creates a Lexer from source text.
func NewLexer(source string) *Lexer {
	return &Lexer{
		source: source,
		lines:  strings.Split(source, "\n"),
	}
}

// Tokens returns all tokens from the source. Lexing errors are collected
// and returned as a joined error; partial results are still returned.
func (l *Lexer) Tokens() ([]Token, error) {
	var tokens []Token
	var errs []error

	for i, line := range l.lines {
		lineNum := i + 1
		lineTokens, lineErrs := l.tokenizeLine(line, lineNum)
		tokens = append(tokens, lineTokens...)
		errs = append(errs, lineErrs...)

		if i < len(l.lines)-1 {
			tokens = append(tokens, Token{Type: TokenNewline, Value: "\n", Line: lineNum, Col: len(line) + 1})
		}
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: "", Line: len(l.lines), Col: 1})

	return tokens, errors.Join(errs...)
}

// tokenizeLine processes a single line and returns its tokens and any errors.
func (l *Lexer) tokenizeLine(line string, lineNum int) ([]Token, []error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, nil
	}

	if strings.HasPrefix(trimmed, "#") {
		col := strings.Index(line, "#") + 1
		return []Token{{Type: TokenComment, Value: trimmed, Line: lineNum, Col: col}}, nil
	}

	// Detect headers: starts at col 1 (no leading whitespace) and contains
	// a ":" where everything before the colon is a single word (no spaces).
	// This matches "App: name", "Storefront:", "Api:", etc.
	if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx > 0 && !strings.ContainsAny(trimmed[:colonIdx], " \t") {
			return l.tokenizeHeader(trimmed, lineNum)
		}
	}

	return l.tokenizeWords(line, lineNum)
}

// tokenizeHeader processes a header line like "App: name" or "Storefront:".
func (l *Lexer) tokenizeHeader(trimmed string, lineNum int) ([]Token, []error) {
	// Split on first ":"
	colonIdx := strings.Index(trimmed, ":")
	label := strings.TrimSpace(trimmed[:colonIdx])
	rest := strings.TrimSpace(trimmed[colonIdx+1:])

	lowered := strings.ToLower(label)

	if lowered == "app" {
		tokens := []Token{{Type: TokenApp, Value: label, Line: lineNum, Col: 1}}
		if rest != "" {
			tokens = append(tokens, Token{Type: TokenIdent, Value: rest, Line: lineNum, Col: colonIdx + 3})
		}
		return tokens, nil
	}

	if lowered == "namespace" {
		tokens := []Token{{Type: TokenNamespace, Value: label, Line: lineNum, Col: 1}}
		if rest != "" {
			tokens = append(tokens, Token{Type: TokenIdent, Value: rest, Line: lineNum, Col: colonIdx + 3})
		}
		return tokens, nil
	}

	if lowered == "images" {
		tokens := []Token{{Type: TokenImages, Value: label, Line: lineNum, Col: 1}}
		return tokens, nil
	}

	if lowered == "crds" {
		tokens := []Token{{Type: TokenCRDs, Value: label, Line: lineNum, Col: 1}}
		return tokens, nil
	}

	// Section header for slices.
	tokens := []Token{{Type: TokenSectionHeader, Value: label, Line: lineNum, Col: 1}}
	return tokens, nil
}

// tokenizeWords scans through a line producing tokens from each word or symbol.
func (l *Lexer) tokenizeWords(line string, lineNum int) ([]Token, []error) {
	var tokens []Token
	var errs []error

	pos := 0
	for pos < len(line) {
		if line[pos] == ' ' || line[pos] == '\t' {
			pos++
			continue
		}

		tok, newPos, err := l.scanToken(line, lineNum, pos)
		if err != nil {
			errs = append(errs, err)
			pos = len(line)
		} else {
			tokens = append(tokens, tok)
			pos = newPos
		}
	}

	return tokens, errs
}

// scanToken reads a single token starting at the given position in the line.
func (l *Lexer) scanToken(line string, lineNum, pos int) (Token, int, error) {
	ch := line[pos]
	col := pos + 1

	switch {
	case ch == ',':
		return Token{Type: TokenComma, Value: ",", Line: lineNum, Col: col}, pos + 1, nil
	case ch == '"':
		return l.scanString(line, lineNum, pos)
	default:
		return l.scanWord(line, lineNum, pos)
	}
}

// scanString reads a quoted string starting at pos (which must be a double quote).
func (l *Lexer) scanString(line string, lineNum, pos int) (Token, int, error) {
	col := pos + 1
	end := strings.Index(line[pos+1:], "\"")

	if end == -1 {
		return Token{}, 0, newParseError(lineNum, col, "unterminated string literal")
	}

	closePos := pos + 1 + end
	value := line[pos+1 : closePos]

	return Token{Type: TokenString, Value: value, Line: lineNum, Col: col}, closePos + 1, nil
}

// scanWord reads an unquoted word and classifies it as keyword, number, bool, or ident.
func (l *Lexer) scanWord(line string, lineNum, pos int) (Token, int, error) {
	start := pos
	for pos < len(line) && !isDelimiter(line[pos]) {
		pos++
	}

	word := line[start:pos]
	col := start + 1

	return classifyWord(word, lineNum, col), pos, nil
}

// classifyWord determines the token type for an unquoted word.
func classifyWord(word string, lineNum, col int) Token {
	lowered := strings.ToLower(word)

	if tt, ok := keywords[lowered]; ok {
		return Token{Type: tt, Value: word, Line: lineNum, Col: col}
	}

	if lowered == "true" || lowered == "false" {
		return Token{Type: TokenBool, Value: lowered, Line: lineNum, Col: col}
	}

	if isInteger(word) {
		return Token{Type: TokenNumber, Value: word, Line: lineNum, Col: col}
	}

	return Token{Type: TokenIdent, Value: word, Line: lineNum, Col: col}
}

// isDelimiter reports whether a byte is a token boundary.
func isDelimiter(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == ',' || ch == '"'
}

// isInteger reports whether the word consists entirely of ASCII digits.
func isInteger(word string) bool {
	if word == "" {
		return false
	}

	for _, r := range word {
		if !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}
