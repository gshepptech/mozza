package recipe

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parser transforms a token stream from the lexer into a Recipe AST.
// It implements a recursive descent strategy for the indentation-based
// .mozza format. Sections are delimited by headers at column 1 (no braces).
type Parser struct {
	source string
	tokens []Token
	pos    int
	errs   []error
}

// NewParser creates a Parser from source text. Call Parse to lex the
// source and produce the Recipe AST.
func NewParser(source string) *Parser {
	return &Parser{
		source: source,
	}
}

// Parse lexes the source and parses the resulting tokens into a Recipe AST.
// Multiple errors may be collected during parsing; they are returned as a
// joined error. A partial Recipe is always returned, even when errors occur.
func (p *Parser) Parse() (*Recipe, error) {
	tokens, lexErr := NewLexer(p.source).Tokens()
	p.tokens = tokens

	if lexErr != nil {
		p.errs = append(p.errs, lexErr)
	}

	recipe := p.parseRecipe()

	return recipe, errors.Join(p.errs...)
}

// parseRecipe is the top-level parsing loop that builds the Recipe AST.
func (p *Parser) parseRecipe() *Recipe {
	recipe := &Recipe{}

	p.skipInsignificant()

	for !p.atEnd() {
		tok := p.current()

		switch tok.Type { //nolint:exhaustive // parser handles specific top-level tokens
		case TokenApp:
			p.parseApp(recipe)
		case TokenNamespace:
			p.parseNamespace(recipe)
		case TokenImages:
			p.parseImages(recipe)
		case TokenCRDs:
			p.parseCRDs(recipe)
		case TokenSectionHeader:
			p.parseSection(recipe)
		default:
			p.addErrorf(tok.Line, tok.Col, "unexpected token %s", tok.Type)
			p.recover()
		}

		p.skipInsignificant()
	}

	return recipe
}

// parseApp parses an `App: name` declaration and sets recipe.Name.
func (p *Parser) parseApp(recipe *Recipe) {
	p.advance() // consume App token
	p.skipNewlines()

	// The app name follows as an Ident token (produced by tokenizeHeader).
	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString {
		recipe.Name = tok.Value
		p.advance()
	} else if tok.Type != TokenNewline && tok.Type != TokenEOF {
		// Allow any non-whitespace as app name.
		recipe.Name = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected app name after App:")
	}
}

// parseNamespace parses a `Namespace: name` declaration and sets recipe.Namespace.
func (p *Parser) parseNamespace(recipe *Recipe) {
	p.advance() // consume Namespace token
	p.skipNewlines()

	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString {
		recipe.Namespace = tok.Value
		p.advance()
	} else if tok.Type != TokenNewline && tok.Type != TokenEOF {
		recipe.Namespace = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected namespace name after Namespace:")
	}
}

// startsAlias reports whether tok begins an `Images:` entry — an alias name is
// an ident written with a trailing colon.
func startsAlias(tok Token) bool {
	return tok.Type == TokenIdent && strings.HasSuffix(tok.Value, ":")
}

// parseImages parses an `Images:` section containing alias definitions.
// Each indented line has the format `name: image-ref`.
func (p *Parser) parseImages(recipe *Recipe) {
	p.advance() // consume Images token
	consumedLine := p.lastConsumedLine(0)
	p.skipInsignificant()

	if recipe.Aliases == nil {
		recipe.Aliases = make(map[string]string)
	}

	for !p.atEnd() {
		tok := p.current()

		// Stop at the next section header, app, namespace, or images declaration.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp ||
			tok.Type == TokenNamespace || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
		}

		if p.reportOffLine(tok, consumedLine, "alias", startsAlias(tok)) {
			consumedLine = p.lastConsumedLine(consumedLine)
			p.skipInsignificant()

			continue
		}

		// Each alias line is an ident ending with ":" followed by the image ref.
		if startsAlias(tok) {
			name := strings.TrimSuffix(tok.Value, ":")
			p.advance()

			ref := p.current()
			if ref.Type == TokenIdent || ref.Type == TokenString {
				recipe.Aliases[name] = ref.Value
				p.advance()
			} else {
				p.addErrorf(ref.Line, ref.Col, "expected image reference after alias %q", name)
				p.recoverDirective()
			}
		} else {
			p.addErrorf(tok.Line, tok.Col, "expected alias definition (name: image-ref)")
			p.recoverDirective()
		}

		consumedLine = p.lastConsumedLine(consumedLine)
		p.skipInsignificant()
	}
}

// parseCRDs parses a `CRDs:` block collecting `from <url>` entries.
func (p *Parser) parseCRDs(recipe *Recipe) {
	p.advance() // consume CRDs token
	consumedLine := p.lastConsumedLine(0)
	p.skipInsignificant()

	for !p.atEnd() {
		tok := p.current()

		// Stop at the next section header or top-level keyword.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp ||
			tok.Type == TokenNamespace || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
		}

		if p.reportOffLine(tok, consumedLine, "entry", tok.Type == TokenFrom) {
			consumedLine = p.lastConsumedLine(consumedLine)
			p.skipInsignificant()

			continue
		}

		// Each line should be: from <url>
		if tok.Type == TokenFrom {
			p.advance()
			urlTok := p.current()
			if urlTok.Type == TokenIdent || urlTok.Type == TokenString {
				recipe.CRDs = append(recipe.CRDs, urlTok.Value)
				p.advance()
			} else {
				p.addErrorf(urlTok.Line, urlTok.Col, "expected URL after 'from' in CRDs section")
				p.recoverDirective()
			}
		} else {
			p.addErrorf(tok.Line, tok.Col, "expected 'from <url>' in CRDs section")
			p.recoverDirective()
		}

		consumedLine = p.lastConsumedLine(consumedLine)
		p.skipInsignificant()
	}
}

// parseSection parses a section header and its indented directives.
func (p *Parser) parseSection(recipe *Recipe) {
	headerTok := p.current()
	p.advance() // consume SectionHeader token

	slice := Slice{
		Name: strings.ToLower(headerTok.Value),
		Line: headerTok.Line,
	}

	p.skipInsignificant()

	consumedLine := headerTok.Line

	// Parse directives until we hit another section header, App, or EOF.
	for !p.atEnd() {
		tok := p.current()

		// Stop at the next section header, app, or images declaration.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
		}

		if !p.reportOffLine(tok, consumedLine, "directive", startsDirective(tok)) {
			p.parseDirective(&slice)
		}

		consumedLine = p.lastConsumedLine(consumedLine)
		p.skipInsignificant()
	}

	recipe.Slices = append(recipe.Slices, slice)
}

// directiveDispatch maps token types to their directive parser methods.
// Built once per Parser via initDirectiveDispatch.
type directiveHandler func(*Parser, *Slice)

// directiveTable is a package-level dispatch table for directive parsing.
// Each entry maps a token type to the parser method that handles it.
var directiveTable = map[TokenType]directiveHandler{
	TokenFrom:      (*Parser).parseFromImage,
	TokenOpen:      (*Parser).parseOpenPublic,
	TokenOn:        (*Parser).parseOnPort,
	TokenHealth:    (*Parser).parseHealthCheck,
	TokenRun:       (*Parser).parseRun,
	TokenNeeds:     (*Parser).parseNeeds,
	TokenPostgres:  (*Parser).parseDatabaseShorthand,
	TokenMysql:     (*Parser).parseDatabaseShorthand,
	TokenMongo:     (*Parser).parseDatabaseShorthand,
	TokenRedis:     (*Parser).parseCacheShorthand,
	TokenMemcached: (*Parser).parseCacheShorthand,
	TokenSet:       (*Parser).parseSet,
	TokenLimit:     (*Parser).parseLimit,
	TokenRestart:   (*Parser).parseRestart,
	TokenDomain:    (*Parser).parseDomain,
	TokenSecret:    (*Parser).parseSecret,
	TokenPull:      (*Parser).parsePullSecret,
	TokenSchedule:  (*Parser).parseScheduleRaw,
	TokenEach:      (*Parser).parseEach,
	TokenBefore:    (*Parser).parseBefore,
	TokenWith:      (*Parser).parseWith,
	TokenMount:     (*Parser).parseMount,
	TokenReadiness: (*Parser).parseProbeType,
	TokenLiveness:  (*Parser).parseProbeType,
	TokenStartup:   (*Parser).parseProbeType,
	TokenReachable: (*Parser).parseReachable,
	TokenKind:      (*Parser).parseKindOverride,
	TokenPrefer:    (*Parser).parsePrefer,
	TokenRequire:   (*Parser).parseRequire,
	TokenSpread:    (*Parser).parseSpread,
	TokenNever:     (*Parser).parseNever,
	TokenScale:     (*Parser).parseScale,
	TokenKeep:      (*Parser).parseKeep,
	TokenAllow:     (*Parser).parseAllow,
	TokenBlock:     (*Parser).parseBlock,
	TokenDrop:      (*Parser).parseDrop,
	TokenUpdate:    (*Parser).parseUpdate,
	TokenGraceful:  (*Parser).parseGraceful,
	TokenReadOnly:  (*Parser).parseReadOnlyFS,
	TokenIdent:     (*Parser).parseIdentDirective,
}

// identDirectiveWords mirrors the cases parseIdentDirective handles. TokenIdent
// is in directiveTable, but only these words begin a directive — every other
// ident reaching the table is an unknown directive.
var identDirectiveWords = map[string]bool{
	"only":  true,
	"after": true,
	"start": true,
	"add":   true,
	"use":   true,
}

// startsDirective reports whether tok is the first word of a directive.
func startsDirective(tok Token) bool {
	if tok.Type == TokenIdent {
		return identDirectiveWords[strings.ToLower(tok.Value)]
	}

	_, ok := directiveTable[tok.Type]

	return ok
}

// parseDirective dispatches a single directive inside a section body.
func (p *Parser) parseDirective(slice *Slice) {
	tok := p.current()

	if handler, ok := directiveTable[tok.Type]; ok {
		handler(p, slice)
		return
	}
	p.addErrorf(tok.Line, tok.Col, "unknown directive %q", tok.Value)
	p.recoverDirective()
}

// --- Helper functions ---

// plainEnglishToCron converts schedule text to a cron expression.
func plainEnglishToCron(text string) (string, error) {
	text = strings.TrimSpace(strings.ToLower(text))

	// "day at Xam" / "day at Xpm"
	if strings.HasPrefix(text, "day") {
		hour, err := parseTimeOfDay(text)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 %d * * *", hour), nil
	}

	// "hour"
	if text == "hour" {
		return "0 * * * *", nil
	}

	// "N minutes"
	if strings.HasSuffix(text, "minutes") {
		parts := strings.Fields(text)
		if len(parts) >= 1 {
			n := strings.TrimSpace(parts[0])
			if _, err := strconv.Atoi(n); err == nil {
				return fmt.Sprintf("*/%s * * * *", n), nil
			}
		}
	}

	// Weekday: "monday at Xam", etc.
	weekdays := map[string]int{
		"sunday": 0, "monday": 1, "tuesday": 2, "wednesday": 3,
		"thursday": 4, "friday": 5, "saturday": 6,
	}
	for day, num := range weekdays {
		if strings.HasPrefix(text, day) {
			hour, err := parseTimeOfDay(text)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("0 %d * * %d", hour, num), nil
		}
	}

	return "", fmt.Errorf("unrecognized schedule: %q", text)
}

// parseTimeOfDay extracts the hour from text containing "at Xam" or "at Xpm".
func parseTimeOfDay(text string) (int, error) {
	idx := strings.Index(text, "at ")
	if idx < 0 {
		return 0, fmt.Errorf("missing 'at' in schedule: %q", text)
	}
	timeStr := strings.TrimSpace(text[idx+3:])
	timeStr = strings.TrimSpace(timeStr)

	isPM := false
	if strings.HasSuffix(timeStr, "pm") {
		isPM = true
		timeStr = strings.TrimSuffix(timeStr, "pm")
	} else if strings.HasSuffix(timeStr, "am") {
		timeStr = strings.TrimSuffix(timeStr, "am")
	}

	hour, err := strconv.Atoi(timeStr)
	if err != nil {
		return 0, fmt.Errorf("invalid hour in schedule: %q", timeStr)
	}
	if isPM && hour < 12 {
		hour += 12
	}
	if !isPM && hour == 12 {
		hour = 0
	}
	return hour, nil
}

// mapVerb maps a plain English permission verb to Kubernetes API verbs.
func mapVerb(verb string) []string {
	switch verb {
	case "read":
		return []string{"get", "list", "watch"}
	case "write":
		return []string{"create", "update", "patch"}
	case "delete":
		return []string{"delete"}
	case "manage":
		return []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	default:
		return nil
	}
}

// dedup removes duplicate strings from a slice preserving order.
func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// parseLabelValue parses a "KEY=VAL" string from the current token.
func (p *Parser) parseLabelValue() LabelConstraint {
	tok := p.current()
	if tok.Type != TokenString && tok.Type != TokenIdent {
		return LabelConstraint{}
	}

	val := tok.Value
	p.advance()

	parts := strings.SplitN(val, "=", 2)
	if len(parts) == 2 {
		return LabelConstraint{Key: parts[0], Value: parts[1]}
	}
	return LabelConstraint{Key: val}
}

// parseTimeSuffix parses a duration value like "30s", "5m", "30" (default seconds).
func parseTimeSuffix(val string) int {
	if strings.HasSuffix(val, "m") {
		n, err := strconv.Atoi(strings.TrimSuffix(val, "m"))
		if err == nil {
			return n * 60
		}
	}
	if strings.HasSuffix(val, "s") {
		n, err := strconv.Atoi(strings.TrimSuffix(val, "s"))
		if err == nil {
			return n
		}
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return n
}

// matchIdent reports whether the current token is an ident with the given lowered value.
func (p *Parser) matchIdent(lower string) bool {
	if p.pos >= len(p.tokens) {
		return false
	}
	tok := p.tokens[p.pos]
	return (tok.Type == TokenIdent || tok.Type == TokenString) && strings.ToLower(tok.Value) == lower
}

// --- Token navigation helpers ---

// current returns the token at the current position.
func (p *Parser) current() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}

	return p.tokens[p.pos]
}

// advance moves to the next token and returns the consumed token.
func (p *Parser) advance() Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}

	return tok
}

// atEnd reports whether the parser has reached the end of input.
func (p *Parser) atEnd() bool {
	return p.pos >= len(p.tokens) || p.tokens[p.pos].Type == TokenEOF
}

// matchType reports whether the current token matches the given type.
func (p *Parser) matchType(tt TokenType) bool {
	return p.pos < len(p.tokens) && p.tokens[p.pos].Type == tt
}

// consumeTrailing advances past the words that finish a directive's phrasing,
// in the order given, skipping any the author left out — "keep at least 2" and
// "keep at least 2 copies running during updates" mean the same thing. It stops
// at the first token that is not the next word it expects, so anything else on
// the line reaches the caller's one-per-line check instead of being swallowed.
// Matching is on the word rather than the token type because these words lex as
// keywords in some phrasings and as plain idents in others; where a word varies
// ("copies", "copy") list both spellings, since only the one written matches.
func (p *Parser) consumeTrailing(words ...string) {
	for _, word := range words {
		if strings.ToLower(p.current().Value) == word {
			p.advance()
		}
	}
}

// lastConsumedLine reports the line the most recently consumed token sits on,
// or fallback when nothing has been consumed yet. Callers compare against the
// token actually consumed rather than an entry's first token because an init
// block spans lines, and a start-line comparison would wave through whatever
// follows its last step on the same line.
func (p *Parser) lastConsumedLine(fallback int) int {
	if p.pos == 0 || p.pos > len(p.tokens) {
		return fallback
	}

	return p.tokens[p.pos-1].Line
}

// reportOffLine reports tok when it continues a line an earlier entry already
// finished, and skips the rest of that line. Every entry — directive, alias,
// CRDs entry, init step — begins a line of its own; the lexer separates tokens
// by whitespace, so without this rule a value written into one entry can start
// the next one no matter who wrote the recipe. what names the kind of entry the
// caller reads.
//
// Which mistake this is turns on startsEntry — whether tok begins an entry of
// the kind the caller reads, which only the caller knows. If it does, the author
// wrote two entries on one line and the line rule is the fix. If it does not, no
// line break will help: the word is a misspelling, or an option the entry it
// follows does not have, and saying "put it on its own line" would name a cause
// that is not the cause.
func (p *Parser) reportOffLine(tok Token, consumedLine int, what string, startsEntry bool) bool {
	if tok.Line > consumedLine {
		return false
	}

	if startsEntry {
		p.addErrorf(tok.Line, tok.Col,
			"unexpected %q after a complete %s — each %s goes on its own line", tok.Value, what, what)
	} else {
		p.addErrorf(tok.Line, tok.Col,
			"unexpected %q — the %s on this line does not accept it, and no directive starts with it",
			tok.Value, what)
	}
	p.recoverDirective()

	return true
}

// skipNewlines advances past any newline tokens.
func (p *Parser) skipNewlines() {
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == TokenNewline {
		p.pos++
	}
}

// skipInsignificant advances past newlines and comments.
func (p *Parser) skipInsignificant() {
	for p.pos < len(p.tokens) {
		tt := p.tokens[p.pos].Type
		if tt != TokenNewline && tt != TokenComment {
			break
		}

		p.pos++
	}
}

// --- Error handling ---

// addErrorf records a parse error at the given position.
func (p *Parser) addErrorf(line, col int, format string, args ...any) {
	p.errs = append(p.errs, newParseErrorf(line, col, format, args...))
}

// recover advances the parser to the next section boundary: a section
// header, app declaration, or EOF.
func (p *Parser) recover() {
	for !p.atEnd() {
		tok := p.current()
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp || tok.Type == TokenImages || tok.Type == TokenCRDs {
			return
		}

		p.advance()
	}
}

// recoverDirective advances past the current directive line by skipping
// until a newline or section boundary is reached.
func (p *Parser) recoverDirective() {
	for !p.atEnd() {
		tok := p.current()
		if tok.Type == TokenNewline || tok.Type == TokenSectionHeader || tok.Type == TokenApp || tok.Type == TokenImages || tok.Type == TokenCRDs {
			return
		}

		p.advance()
	}
}
