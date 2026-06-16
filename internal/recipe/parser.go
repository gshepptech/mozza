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

// parseImages parses an `Images:` section containing alias definitions.
// Each indented line has the format `name: image-ref`.
func (p *Parser) parseImages(recipe *Recipe) {
	p.advance() // consume Images token
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

		// Each alias line is an ident ending with ":" followed by the image ref.
		if tok.Type == TokenIdent && strings.HasSuffix(tok.Value, ":") {
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

		p.skipInsignificant()
	}
}

// parseCRDs parses a `CRDs:` block collecting `from <url>` entries.
func (p *Parser) parseCRDs(recipe *Recipe) {
	p.advance() // consume CRDs token
	p.skipInsignificant()

	for !p.atEnd() {
		tok := p.current()

		// Stop at the next section header or top-level keyword.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp ||
			tok.Type == TokenNamespace || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
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

	// Parse directives until we hit another section header, App, or EOF.
	for !p.atEnd() {
		tok := p.current()

		// Stop at the next section header, app, or images declaration.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
		}

		p.parseDirective(&slice)
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

// parseFromImage parses "from image <ref>" or "from <ref>".
// Both "from image myorg/api:latest" and "from frontend" are valid.
func (p *Parser) parseFromImage(slice *Slice) {
	p.advance() // consume 'from'

	// Consume optional 'image' keyword.
	if tok := p.current(); tok.Type == TokenImage {
		p.advance() // consume 'image'
	}

	// The image reference is the next token. Accept ident, string, or any
	// keyword token so that alias names like "redis" or "postgres" work.
	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString || isImageRefToken(tok.Type) {
		slice.Image = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected image reference after 'from'")
		p.recoverDirective()
	}
}

// isImageRefToken reports whether the token type can be used as an image
// reference or alias name in a "from" directive.
func isImageRefToken(tt TokenType) bool {
	switch tt { //nolint:exhaustive // only keyword tokens usable as image refs
	case TokenPostgres, TokenMysql, TokenMongo, TokenRedis, TokenMemcached,
		TokenDaily, TokenBackups, TokenSet, TokenLimit, TokenCpu, TokenMemory,
		TokenRestart, TokenDomain, TokenSecret, TokenPull, TokenKey,
		TokenAlways, TokenUnlessStopped, TokenNumber:
		return true
	default:
		return false
	}
}

// parseOpenPublic parses "open to the public on port <N>".
func (p *Parser) parseOpenPublic(slice *Slice) {
	p.advance() // consume 'open'
	slice.Public = true

	// Consume optional "to the public" keywords.
	if p.matchType(TokenTo) {
		p.advance()
	}
	if p.matchType(TokenThe) {
		p.advance()
	}
	if p.matchType(TokenPublic) {
		p.advance()
	}

	// Expect "on port <N>".
	if p.matchType(TokenOn) {
		p.advance()
	}
	if p.matchType(TokenPort) {
		p.advance()
	}

	tok := p.current()
	if tok.Type == TokenNumber {
		n, err := strconv.Atoi(tok.Value)
		if err != nil {
			p.addErrorf(tok.Line, tok.Col, "invalid port number %q: %v", tok.Value, err)
		} else {
			slice.Port = n
		}
		p.advance()
	}
}

// parseOnPort parses "on port <N> [as NAME] [using PROTO]" (internal port, not public).
// When "as NAME" is present, appends to Ports slice (multi-port). Otherwise sets Port.
func (p *Parser) parseOnPort(slice *Slice) {
	p.advance() // consume 'on'

	if p.matchType(TokenPort) {
		p.advance()
	}

	tok := p.current()
	if tok.Type != TokenNumber {
		p.addErrorf(tok.Line, tok.Col, "expected port number after 'on port'")
		p.recoverDirective()
		return
	}

	n, err := strconv.Atoi(tok.Value)
	if err != nil {
		p.addErrorf(tok.Line, tok.Col, "invalid port number %q: %v", tok.Value, err)
		p.advance()
		return
	}
	p.advance()

	// Check for "as NAME" (multi-port).
	if p.matchType(TokenAs) {
		p.advance() // consume 'as'
		spec := PortSpec{Port: n}
		nameTok := p.current()
		if nameTok.Type == TokenIdent || nameTok.Type == TokenString {
			spec.Name = nameTok.Value
			p.advance()
		}
		// Optional "using PROTO".
		if p.matchType(TokenUsing) {
			p.advance()
			protoTok := p.current()
			if protoTok.Type == TokenIdent || protoTok.Type == TokenString {
				spec.Protocol = protoTok.Value
				p.advance()
			}
		}
		slice.Ports = append(slice.Ports, spec)
	} else {
		slice.Port = n
	}
}

// parseHealthCheck parses "health check <path> [every Ns, timeout Ns, wait Ns before starting]".
// Creates BOTH readiness and liveness probes for backward compatibility.
func (p *Parser) parseHealthCheck(slice *Slice) {
	p.advance() // consume 'health'

	if p.matchType(TokenCheck) {
		p.advance()
	}

	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString {
		path := tok.Value
		slice.Health = path
		p.advance()

		// Parse optional timing params.
		interval, timeout, delay := p.parseProbeTiming()

		readiness := ProbeSpec{Type: "readiness", HTTPPath: path, Interval: interval, Timeout: timeout, Delay: delay}
		liveness := ProbeSpec{Type: "liveness", HTTPPath: path, Interval: interval, Timeout: timeout, Delay: delay}
		slice.Probes = append(slice.Probes, readiness, liveness)
	} else if tok.Type == TokenIdent && strings.ToLower(tok.Value) == "by" {
		// "health check by running CMD" — shouldn't happen since "health" dispatch
		p.addErrorf(tok.Line, tok.Col, "expected health check path")
		p.recoverDirective()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected health check path")
		p.recoverDirective()
	}
}

// parseRun dispatches all "run ..." directives:
//   - "run N copies" — replicas
//   - "run once [with N parallel]" — one-shot job
//   - "run to completion" — one-shot job
//   - "run every ..." — schedule (cron)
//   - "run on every node [labeled ...]" — daemon
//   - "run as user N" / "run as group N" — security
//   - "run once, retry up to N times" — retries
func (p *Parser) parseRun(slice *Slice) {
	p.advance() // consume 'run'

	tok := p.current()

	switch tok.Type { //nolint:exhaustive // parser handles specific run modifiers
	case TokenNumber:
		// "run N copies"
		n, err := strconv.Atoi(tok.Value)
		if err != nil {
			p.addErrorf(tok.Line, tok.Col, "invalid replicas value %q: %v", tok.Value, err)
		} else {
			slice.Replicas = n
		}
		p.advance()
		if p.matchType(TokenCopies) || p.matchType(TokenCopy) {
			p.advance()
		}

	case TokenOnce:
		// "run once [with N parallel]" or "run once, retry up to N times"
		p.advance() // consume 'once'
		slice.RunOnce = true
		p.parseRunOnceModifiers(slice)

	case TokenTo:
		// "run to completion"
		p.advance() // consume 'to'
		if p.matchType(TokenCompletion) {
			p.advance()
		}
		slice.RunOnce = true

	case TokenEvery:
		// "run every ..." — schedule
		p.advance() // consume 'every'
		p.parseRunEvery(slice)

	case TokenOn:
		// "run on every node [labeled ...]"
		p.advance() // consume 'on'
		if p.matchType(TokenEvery) {
			p.advance() // consume 'every'
		}
		if p.matchType(TokenNode) {
			p.advance() // consume 'node'
		}
		slice.DaemonMode = true
		// Optional "labeled KEY=VAL"
		if p.matchType(TokenLabeled) {
			p.advance()
			lc := p.parseLabelValue()
			if lc.Key != "" {
				if slice.Scheduling == nil {
					slice.Scheduling = &SchedulingSpec{}
				}
				slice.Scheduling.NodeRequirements = append(
					slice.Scheduling.NodeRequirements, lc)
			}
		}

	case TokenAs:
		// "run as user N" or "run as group N"
		p.advance() // consume 'as'
		p.parseRunAs(slice)

	default:
		p.addErrorf(tok.Line, tok.Col, "expected number, 'once', 'every', 'on', 'to', or 'as' after 'run'")
		p.recoverDirective()
	}
}

// parseRunOnceModifiers handles optional modifiers after "run once":
// "with N parallel" and/or ", retry up to N times".
func (p *Parser) parseRunOnceModifiers(slice *Slice) {
	// Check for comma separator (for "run once, retry up to N times").
	if p.matchType(TokenComma) {
		p.advance()
	}

	for !p.atEnd() && !p.matchType(TokenNewline) {
		tok := p.current()
		if tok.Type == TokenWith {
			// "with N parallel"
			p.advance()
			if numTok := p.current(); numTok.Type == TokenNumber {
				n, atoiErr := strconv.Atoi(numTok.Value)
				if atoiErr != nil {
					p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
					return
				}
				slice.Parallelism = n
				p.advance()
			}
			if p.matchType(TokenParallel) {
				p.advance()
			}
		} else if tok.Type == TokenRetry {
			// "retry up to N times"
			p.advance() // consume 'retry'
			// consume optional "up"
			if p.matchIdent("up") {
				p.advance()
			}
			// consume optional "to"
			if p.matchType(TokenTo) {
				p.advance()
			}
			if numTok := p.current(); numTok.Type == TokenNumber {
				n, atoiErr := strconv.Atoi(numTok.Value)
				if atoiErr != nil {
					p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
					return
				}
				slice.Retries = n
				p.advance()
			}
			// consume optional "times"
			if p.matchIdent("times") {
				p.advance()
			}
		} else {
			break
		}
		if p.matchType(TokenComma) {
			p.advance()
		}
	}
}

// parseRunEvery handles schedule directives after "run every":
//   - "run every day at Xam" → cron
//   - "run every hour" → cron
//   - "run every N minutes" → cron
//   - "run every WEEKDAY at Xam" → cron
func (p *Parser) parseRunEvery(slice *Slice) {
	tok := p.current()

	// Collect remaining tokens on this line as text for plainEnglishToCron.
	var parts []string
	for !p.atEnd() && !p.matchType(TokenNewline) {
		parts = append(parts, p.current().Value)
		p.advance()
	}

	text := strings.Join(parts, " ")
	cron, err := plainEnglishToCron(text)
	if err != nil {
		p.addErrorf(tok.Line, tok.Col, "invalid schedule: %v", err)
		return
	}
	slice.Schedule = cron
}

// parseRunAs handles "run as user N" and "run as group N".
func (p *Parser) parseRunAs(slice *Slice) {
	tok := p.current()
	switch tok.Type { //nolint:exhaustive // only user/group expected
	case TokenUser:
		p.advance()
		if numTok := p.current(); numTok.Type == TokenNumber {
			n, atoiErr := strconv.Atoi(numTok.Value)
			if atoiErr != nil {
				p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
				return
			}
			if slice.Security == nil {
				slice.Security = &SecuritySpec{}
			}
			slice.Security.RunAsUser = n
			p.advance()
		}
	case TokenGroup:
		p.advance()
		if numTok := p.current(); numTok.Type == TokenNumber {
			n, atoiErr := strconv.Atoi(numTok.Value)
			if atoiErr != nil {
				p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
				return
			}
			if slice.Security == nil {
				slice.Security = &SecuritySpec{}
			}
			slice.Security.RunAsGroup = n
			p.advance()
		}
	default:
		p.addErrorf(tok.Line, tok.Col, "expected 'user' or 'group' after 'run as'")
		p.recoverDirective()
	}
}

// parseNeeds parses "needs <name> [and <name>]*", comma-separated names,
// or "needs [cluster-wide] permission to VERB RESOURCE".
func (p *Parser) parseNeeds(slice *Slice) {
	p.advance() // consume 'needs'

	tok := p.current()

	// Check for "needs permission ..." or "needs cluster-wide permission ..."
	if tok.Type == TokenPermission ||
		(tok.Type == TokenIdent && strings.ToLower(tok.Value) == "cluster-wide") {
		p.parseNeedsPermission(slice)
		return
	}

	for !p.atEnd() {
		tok := p.current()
		if tok.Type == TokenIdent || tok.Type == TokenString {
			slice.Needs = append(slice.Needs, tok.Value)
			p.advance()
		} else {
			break
		}

		// Consume separators: "and" or ","
		if p.matchType(TokenAnd) {
			p.advance()
		} else if p.matchType(TokenComma) {
			p.advance()
		} else {
			break
		}
	}
}

// parseDatabaseShorthand parses "postgres <ver>, <size>[, daily backups]".
func (p *Parser) parseDatabaseShorthand(slice *Slice) {
	engineTok := p.advance() // consume engine token
	slice.Engine = strings.ToLower(engineTok.Value)

	// Parse version number.
	if tok := p.current(); tok.Type == TokenNumber || tok.Type == TokenIdent {
		slice.Version = tok.Value
		p.advance()
	}

	// Parse optional comma-separated options: size, daily backups.
	for p.matchType(TokenComma) {
		p.advance() // consume comma

		tok := p.current()
		if tok.Type == TokenDaily {
			p.advance()
			if p.matchType(TokenBackups) {
				p.advance()
			}
			slice.Backups = "daily"
		} else if tok.Type == TokenIdent || tok.Type == TokenNumber {
			// Size value like "20Gi".
			slice.Storage = tok.Value
			p.advance()
		}
	}
}

// parseCacheShorthand parses "redis <ver>[, <size>]".
func (p *Parser) parseCacheShorthand(slice *Slice) {
	engineTok := p.advance() // consume engine token
	slice.Engine = strings.ToLower(engineTok.Value)

	// Parse version.
	if tok := p.current(); tok.Type == TokenNumber || tok.Type == TokenIdent {
		slice.Version = tok.Value
		p.advance()
	}

	// Parse optional comma-separated size.
	if p.matchType(TokenComma) {
		p.advance()
		if tok := p.current(); tok.Type == TokenIdent || tok.Type == TokenNumber {
			slice.Storage = tok.Value
			p.advance()
		}
	}
}

// parseSet parses "set KEY to <value>".
func (p *Parser) parseSet(slice *Slice) {
	p.advance() // consume 'set'

	tok := p.current()
	if tok.Type != TokenIdent {
		p.addErrorf(tok.Line, tok.Col, "expected environment variable name after 'set'")
		p.recoverDirective()
		return
	}

	key := tok.Value
	p.advance()

	// Consume optional 'to'.
	if p.matchType(TokenTo) {
		p.advance()
	}

	// Expect value (string, ident, or number).
	tok = p.current()
	if tok.Type == TokenString || tok.Type == TokenIdent || tok.Type == TokenNumber {
		if slice.Env == nil {
			slice.Env = make(map[string]string)
		}
		slice.Env[key] = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected value after 'set %s to'", key)
		p.recoverDirective()
	}
}

// parseLimit parses "limit cpu to <value>" or "limit memory to <value>".
func (p *Parser) parseLimit(slice *Slice) {
	p.advance() // consume 'limit'

	tok := p.current()

	switch tok.Type { //nolint:exhaustive // only cpu/memory expected
	case TokenCpu:
		p.advance()
		if p.matchType(TokenTo) {
			p.advance()
		}
		val := p.current()
		if val.Type == TokenString || val.Type == TokenIdent || val.Type == TokenNumber {
			slice.CPULimit = val.Value
			p.advance()
		} else {
			p.addErrorf(val.Line, val.Col, "expected CPU limit value")
			p.recoverDirective()
		}
	case TokenMemory:
		p.advance()
		if p.matchType(TokenTo) {
			p.advance()
		}
		val := p.current()
		if val.Type == TokenString || val.Type == TokenIdent || val.Type == TokenNumber {
			slice.MemoryLimit = val.Value
			p.advance()
		} else {
			p.addErrorf(val.Line, val.Col, "expected memory limit value")
			p.recoverDirective()
		}
	default:
		p.addErrorf(tok.Line, tok.Col, "expected 'cpu' or 'memory' after 'limit'")
		p.recoverDirective()
	}
}

// parseRestart parses "restart always" or "restart unless-stopped".
func (p *Parser) parseRestart(slice *Slice) {
	p.advance() // consume 'restart'

	tok := p.current()

	switch tok.Type { //nolint:exhaustive // only restart policy tokens expected
	case TokenAlways:
		slice.RestartPolicy = "always"
		p.advance()
	case TokenUnlessStopped:
		slice.RestartPolicy = "unless-stopped"
		p.advance()
	case TokenIdent:
		slice.RestartPolicy = tok.Value
		p.advance()
	default:
		p.addErrorf(tok.Line, tok.Col, "expected restart policy (always, unless-stopped)")
		p.recoverDirective()
	}
}

// parseDomain parses "domain <name>".
func (p *Parser) parseDomain(slice *Slice) {
	p.advance() // consume 'domain'

	tok := p.current()
	if tok.Type == TokenString || tok.Type == TokenIdent {
		slice.Domain = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected domain name after 'domain'")
		p.recoverDirective()
	}
}

// parseSecret parses "secret KEY from NAME [key KEYNAME]".
func (p *Parser) parseSecret(slice *Slice) {
	p.advance() // consume 'secret'

	tok := p.current()
	if tok.Type != TokenIdent {
		p.addErrorf(tok.Line, tok.Col, "expected environment variable name after 'secret'")
		p.recoverDirective()
		return
	}

	ref := SecretRef{EnvVar: tok.Value}
	p.advance()

	// Expect 'from'.
	if !p.matchType(TokenFrom) {
		p.addErrorf(p.current().Line, p.current().Col, "expected 'from' after 'secret %s'", ref.EnvVar)
		p.recoverDirective()
		return
	}
	p.advance() // consume 'from'

	// Expect secret name.
	tok = p.current()
	if tok.Type != TokenIdent && tok.Type != TokenString {
		p.addErrorf(tok.Line, tok.Col, "expected secret name after 'from'")
		p.recoverDirective()
		return
	}
	ref.SecretName = tok.Value
	ref.Key = ref.EnvVar // default key = env var name
	p.advance()

	// Optional "key KEYNAME".
	if p.matchType(TokenKey) {
		p.advance() // consume 'key'
		tok = p.current()
		if tok.Type == TokenIdent || tok.Type == TokenString {
			ref.Key = tok.Value
			p.advance()
		} else {
			p.addErrorf(tok.Line, tok.Col, "expected key name after 'key'")
			p.recoverDirective()
			return
		}
	}

	slice.Secrets = append(slice.Secrets, ref)
}

// parsePullSecret parses "pull secret NAME".
func (p *Parser) parsePullSecret(slice *Slice) {
	p.advance() // consume 'pull'

	if !p.matchType(TokenSecret) {
		p.addErrorf(p.current().Line, p.current().Col, "expected 'secret' after 'pull'")
		p.recoverDirective()
		return
	}
	p.advance() // consume 'secret'

	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString {
		slice.PullSecret = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected secret name after 'pull secret'")
		p.recoverDirective()
	}
}

// --- New directive parsers (Items 3, 4, 5) ---

// parseScheduleRaw parses `schedule "CRON"` (raw cron expression).
func (p *Parser) parseScheduleRaw(slice *Slice) {
	p.advance() // consume 'schedule'

	tok := p.current()
	if tok.Type == TokenString {
		slice.Schedule = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected quoted cron expression after 'schedule'")
		p.recoverDirective()
	}
}

// parseEach handles "each copy needs its own storage of SIZE",
// and other "each" directives.
func (p *Parser) parseEach(slice *Slice) {
	p.advance() // consume 'each'

	// Consume "copy needs its own storage of SIZE"
	if p.matchType(TokenCopy) || p.matchIdent("copy") {
		p.advance()
	}
	// consume "needs"
	if p.matchType(TokenNeeds) {
		p.advance()
	}
	// consume filler words: "its", "own", "storage", "of"
	for p.matchIdent("its") || p.matchType(TokenOwn) || p.matchIdent("storage") || p.matchIdent("of") {
		p.advance()
	}

	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenString || tok.Type == TokenNumber {
		slice.StatefulStorage = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected storage size after 'each copy needs its own storage of'")
		p.recoverDirective()
	}
}

// parseBefore handles:
//   - "before starting:" — init block
//   - "before stopping, wait Ns" — lifecycle pre-stop wait
//   - "before stopping, run CMD" — lifecycle pre-stop command
func (p *Parser) parseBefore(slice *Slice) {
	p.advance() // consume 'before'

	tok := p.current()
	isStarting := tok.Type == TokenStarting ||
		(tok.Type == TokenIdent && strings.ToLower(strings.TrimSuffix(tok.Value, ":")) == "starting")
	isStopping := tok.Type == TokenStopping ||
		(tok.Type == TokenIdent && strings.ToLower(strings.TrimSuffix(tok.Value, ":")) == "stopping")

	if isStarting {
		// "before starting:" — init block mode
		p.advance() // consume 'starting' or 'starting:'
		p.parseInitBlock(slice)
	} else if isStopping {
		p.advance() // consume 'stopping'
		// consume optional comma
		if p.matchType(TokenComma) {
			p.advance()
		}
		// "wait Ns" or "run CMD"
		next := p.current()
		if next.Type == TokenWait {
			p.advance()
			valTok := p.current()
			if valTok.Type == TokenNumber || valTok.Type == TokenIdent {
				n := parseTimeSuffix(valTok.Value)
				if slice.Lifecycle == nil {
					slice.Lifecycle = &LifecycleSpec{}
				}
				slice.Lifecycle.PreStopWait = n
				p.advance()
				// consume optional "seconds"
				if p.matchType(TokenSeconds) || p.matchIdent("s") {
					p.advance()
				}
			}
		} else if next.Type == TokenRun {
			p.advance() // consume 'run'
			cmdTok := p.current()
			if cmdTok.Type == TokenString {
				if slice.Lifecycle == nil {
					slice.Lifecycle = &LifecycleSpec{}
				}
				slice.Lifecycle.PreStopCommand = cmdTok.Value
				p.advance()
			}
		} else {
			p.addErrorf(next.Line, next.Col, "expected 'wait' or 'run' after 'before stopping'")
			p.recoverDirective()
		}
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected 'starting' or 'stopping' after 'before'")
		p.recoverDirective()
	}
}

// parseInitBlock reads subsequent lines with "run image IMG [with CMD]" until
// a non-matching line is encountered.
func (p *Parser) parseInitBlock(slice *Slice) {
	// Skip to next line.
	p.skipInsignificant()

	for !p.atEnd() {
		tok := p.current()

		// Stop at section boundaries.
		if tok.Type == TokenSectionHeader || tok.Type == TokenApp || tok.Type == TokenImages || tok.Type == TokenCRDs {
			break
		}

		// Each init line starts with "run".
		if tok.Type != TokenRun {
			break
		}
		p.advance() // consume 'run'

		// Expect "image".
		if !p.matchType(TokenImage) {
			// Not an init step line, put back by breaking.
			// We already consumed 'run' — this is a problem. Let's rewind.
			p.pos-- // unconsume 'run'
			break
		}
		p.advance() // consume 'image'

		step := InitStep{}
		imgTok := p.current()
		if imgTok.Type == TokenIdent || imgTok.Type == TokenString {
			step.Image = imgTok.Value
			p.advance()
		}

		// Optional 'with "CMD"'.
		if p.matchType(TokenWith) {
			p.advance()
			cmdTok := p.current()
			if cmdTok.Type == TokenString {
				step.Command = cmdTok.Value
				p.advance()
			}
		}

		slice.InitSteps = append(slice.InitSteps, step)
		p.skipInsignificant()
	}
}

// parseWith handles "with sidecar NAME from IMG [on port N]".
func (p *Parser) parseWith(slice *Slice) {
	p.advance() // consume 'with'

	if !p.matchType(TokenSidecar) {
		// Could be other "with" usage; for now only sidecar.
		p.addErrorf(p.current().Line, p.current().Col, "expected 'sidecar' after 'with'")
		p.recoverDirective()
		return
	}
	p.advance() // consume 'sidecar'

	sc := Sidecar{}

	// Name.
	nameTok := p.current()
	if nameTok.Type == TokenIdent || nameTok.Type == TokenString {
		sc.Name = nameTok.Value
		p.advance()
	}

	// "from IMG"
	if p.matchType(TokenFrom) {
		p.advance()
		imgTok := p.current()
		if imgTok.Type == TokenIdent || imgTok.Type == TokenString {
			sc.Image = imgTok.Value
			p.advance()
		}
	}

	// Optional "on port N"
	if p.matchType(TokenOn) {
		p.advance()
		if p.matchType(TokenPort) {
			p.advance()
		}
		if numTok := p.current(); numTok.Type == TokenNumber {
			n, atoiErr := strconv.Atoi(numTok.Value)
			if atoiErr != nil {
				p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
				return
			}
			sc.Ports = append(sc.Ports, PortSpec{Port: n})
			p.advance()
		}
	}

	slice.Sidecars = append(slice.Sidecars, sc)
}

// parseMount handles:
//   - `mount file "PATH" at TARGET`
//   - `mount secret "NAME" at TARGET`
//   - `mount config "PATH" at TARGET`
func (p *Parser) parseMount(slice *Slice) {
	p.advance() // consume 'mount'

	tok := p.current()
	m := MountSpec{}

	switch tok.Type { //nolint:exhaustive // only mount type tokens expected
	case TokenFile:
		m.Type = "file"
		p.advance()
	case TokenSecret:
		m.Type = "secret"
		p.advance()
	case TokenConfig:
		m.Type = "config-dir"
		p.advance()
	default:
		p.addErrorf(tok.Line, tok.Col, "expected 'file', 'secret', or 'config' after 'mount'")
		p.recoverDirective()
		return
	}

	// Source (quoted).
	srcTok := p.current()
	if srcTok.Type == TokenString || srcTok.Type == TokenIdent {
		m.Source = srcTok.Value
		p.advance()
	}

	// "at TARGET"
	if p.matchType(TokenAt) {
		p.advance()
	}
	tgtTok := p.current()
	if tgtTok.Type == TokenString || tgtTok.Type == TokenIdent {
		m.Target = tgtTok.Value
		p.advance()
	}

	slice.Mounts = append(slice.Mounts, m)
}

// parseProbeType handles "readiness check ...", "liveness check ...", "startup check ...".
func (p *Parser) parseProbeType(slice *Slice) {
	typeTok := p.advance() // consume readiness/liveness/startup

	probeType := strings.ToLower(typeTok.Value)

	if p.matchType(TokenCheck) {
		p.advance()
	}

	probe := ProbeSpec{Type: probeType}

	tok := p.current()
	if p.matchIdent("by") {
		// "readiness check by running CMD"
		p.advance() // consume 'by'
		if p.matchType(TokenRunning) {
			p.advance()
		}
		cmdTok := p.current()
		if cmdTok.Type == TokenString {
			probe.Command = cmdTok.Value
			p.advance()
		}
	} else if tok.Type == TokenOn {
		// "liveness check on tcp port N"
		p.advance() // consume 'on'
		// consume "tcp"
		if p.matchIdent("tcp") {
			p.advance()
		}
		if p.matchType(TokenPort) {
			p.advance()
		}
		if numTok := p.current(); numTok.Type == TokenNumber {
			n, atoiErr := strconv.Atoi(numTok.Value)
			if atoiErr != nil {
				p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
				return
			}
			probe.TCPPort = n
			p.advance()
		}
	} else if tok.Type == TokenIdent || tok.Type == TokenString {
		// HTTP path
		probe.HTTPPath = tok.Value
		p.advance()
		// Parse optional timing.
		interval, timeout, delay := p.parseProbeTiming()
		probe.Interval = interval
		probe.Timeout = timeout
		probe.Delay = delay
	}

	slice.Probes = append(slice.Probes, probe)
}

// parseProbeTiming parses optional "every Ns, timeout Ns, wait Ns before starting"
// and returns (interval, timeout, delay). Returns (0,0,0) if no timing found.
func (p *Parser) parseProbeTiming() (int, int, int) {
	var interval, timeout, delay int

	for !p.atEnd() && !p.matchType(TokenNewline) {
		tok := p.current()
		if tok.Type == TokenEvery {
			p.advance()
			if numTok := p.current(); numTok.Type == TokenIdent || numTok.Type == TokenNumber {
				interval = parseTimeSuffix(numTok.Value)
				p.advance()
			}
			if p.matchType(TokenComma) {
				p.advance()
			}
		} else if p.matchIdent("timeout") {
			p.advance()
			if numTok := p.current(); numTok.Type == TokenIdent || numTok.Type == TokenNumber {
				timeout = parseTimeSuffix(numTok.Value)
				p.advance()
			}
			if p.matchType(TokenComma) {
				p.advance()
			}
		} else if tok.Type == TokenWait {
			p.advance()
			if numTok := p.current(); numTok.Type == TokenIdent || numTok.Type == TokenNumber {
				delay = parseTimeSuffix(numTok.Value)
				p.advance()
			}
			// consume optional "before starting"
			if p.matchType(TokenBefore) {
				p.advance()
			}
			if p.matchType(TokenStarting) {
				p.advance()
			}
		} else {
			break
		}
	}

	return interval, timeout, delay
}

// parseReachable parses `reachable as "NAME"` (DNS name).
func (p *Parser) parseReachable(slice *Slice) {
	p.advance() // consume 'reachable'

	if p.matchType(TokenAs) {
		p.advance()
	}

	tok := p.current()
	if tok.Type == TokenString || tok.Type == TokenIdent {
		slice.DNSName = tok.Value
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected DNS name after 'reachable as'")
		p.recoverDirective()
	}
}

// parseKindOverride parses `kind: NAME`.
func (p *Parser) parseKindOverride(slice *Slice) {
	p.advance() // consume 'kind'

	// The lexer may produce "kind:" as TokenKind if it's at column 1, but
	// inside a section body it's an indented line. The colon might be part
	// of the next ident if the lexer kept it. Handle both.
	tok := p.current()
	val := tok.Value
	// If the value has a leading colon (from "kind: X" tokenized as "kind" ":X"), strip it.
	val = strings.TrimPrefix(val, ":")
	val = strings.TrimSpace(val)
	if val == "" {
		// The colon was separate, move to next token.
		if tok.Type == TokenIdent && tok.Value == ":" {
			p.advance()
			tok = p.current()
			val = tok.Value
		}
	}

	if tok.Type == TokenIdent || tok.Type == TokenString {
		slice.Kind = val
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected kind name")
		p.recoverDirective()
	}
}

// parsePrefer parses "prefer nodes labeled KEY=VAL".
func (p *Parser) parsePrefer(slice *Slice) {
	p.advance() // consume 'prefer'
	// consume optional "nodes"
	if p.matchType(TokenNode) || p.matchIdent("nodes") {
		p.advance()
	}
	if p.matchType(TokenLabeled) {
		p.advance()
	}
	lc := p.parseLabelValue()
	if lc.Key != "" {
		if slice.Scheduling == nil {
			slice.Scheduling = &SchedulingSpec{}
		}
		slice.Scheduling.NodePreferences = append(slice.Scheduling.NodePreferences, lc)
	}
}

// parseRequire parses "require nodes labeled KEY=VAL".
func (p *Parser) parseRequire(slice *Slice) {
	p.advance() // consume 'require'
	if p.matchType(TokenNode) || p.matchIdent("nodes") {
		p.advance()
	}
	if p.matchType(TokenLabeled) {
		p.advance()
	}
	lc := p.parseLabelValue()
	if lc.Key != "" {
		if slice.Scheduling == nil {
			slice.Scheduling = &SchedulingSpec{}
		}
		slice.Scheduling.NodeRequirements = append(slice.Scheduling.NodeRequirements, lc)
	}
}

// parseSpread parses "spread copies across nodes|zones".
func (p *Parser) parseSpread(slice *Slice) {
	p.advance() // consume 'spread'
	// consume "copies"
	if p.matchType(TokenCopies) {
		p.advance()
	}
	if p.matchType(TokenAcross) {
		p.advance()
	}

	if slice.Scheduling == nil {
		slice.Scheduling = &SchedulingSpec{}
	}

	tok := p.current()
	if tok.Type == TokenZones || (tok.Type == TokenIdent && strings.ToLower(tok.Value) == "zones") {
		slice.Scheduling.SpreadTopology = "zones"
		p.advance()
	} else if tok.Type == TokenNode || (tok.Type == TokenIdent && strings.ToLower(tok.Value) == "nodes") {
		slice.Scheduling.SpreadTopology = "nodes"
		p.advance()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected 'nodes' or 'zones' after 'spread copies across'")
		p.recoverDirective()
	}
}

// parseNever parses "never run two copies on the same node".
func (p *Parser) parseNever(slice *Slice) {
	p.advance() // consume 'never'
	// consume rest of line tokens until newline
	if slice.Scheduling == nil {
		slice.Scheduling = &SchedulingSpec{}
	}
	slice.Scheduling.AntiAffinity = true
	p.recoverDirective()
}

// parseScale parses "scale between MIN and MAX copies based on cpu|memory PCT%".
func (p *Parser) parseScale(slice *Slice) {
	p.advance() // consume 'scale'

	if p.matchType(TokenBetween) {
		p.advance()
	}

	as := AutoScaleSpec{}

	// MIN
	if numTok := p.current(); numTok.Type == TokenNumber {
		n, atoiErr := strconv.Atoi(numTok.Value)
		if atoiErr != nil {
			p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
			return
		}
		as.MinReplicas = n
		p.advance()
	}

	if p.matchType(TokenAnd) {
		p.advance()
	}

	// MAX
	if numTok := p.current(); numTok.Type == TokenNumber {
		n, atoiErr := strconv.Atoi(numTok.Value)
		if atoiErr != nil {
			p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
			return
		}
		as.MaxReplicas = n
		p.advance()
	}

	// consume "copies"
	if p.matchType(TokenCopies) {
		p.advance()
	}

	// consume "based"
	if p.matchType(TokenBased) {
		p.advance()
	}
	// consume "on"
	if p.matchType(TokenOn) {
		p.advance()
	}

	// cpu or memory
	metricTok := p.current()
	isMemory := false
	if metricTok.Type == TokenCpu {
		p.advance()
	} else if metricTok.Type == TokenMemory {
		isMemory = true
		p.advance()
	}

	// PCT% (comes as ident like "80%")
	if pctTok := p.current(); pctTok.Type == TokenIdent || pctTok.Type == TokenNumber {
		val := strings.TrimSuffix(pctTok.Value, "%")
		n, atoiErr := strconv.Atoi(val)
		if atoiErr != nil {
			p.addErrorf(pctTok.Line, pctTok.Col, "invalid number %q: %v", val, atoiErr)
			return
		}
		if isMemory {
			as.MemoryTarget = n
		} else {
			as.CPUTarget = n
		}
		p.advance()
	}

	slice.AutoScale = &as
}

// parseKeep parses "keep at least N copies running during updates".
func (p *Parser) parseKeep(slice *Slice) {
	p.advance() // consume 'keep'

	// consume "at"
	if p.matchType(TokenAt) {
		p.advance()
	}
	// consume "least"
	if p.matchIdent("least") {
		p.advance()
	}

	if numTok := p.current(); numTok.Type == TokenNumber {
		n, atoiErr := strconv.Atoi(numTok.Value)
		if atoiErr != nil {
			p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
			return
		}
		if slice.DisruptionBudget == nil {
			slice.DisruptionBudget = &DisruptionBudgetSpec{}
		}
		slice.DisruptionBudget.MinAvailable = n
		p.advance()
	}

	// consume remaining: "copies running during updates"
	p.recoverDirective()
}

// parseAllow handles:
//   - "allow at most N copies down during updates" — disruption budget
//   - "allow copies to find each other" — peer discovery
func (p *Parser) parseAllow(slice *Slice) {
	p.advance() // consume 'allow'

	tok := p.current()
	if tok.Type == TokenAt {
		// "allow at most N copies down during updates"
		p.advance() // consume 'at'
		// consume "most"
		if p.matchIdent("most") {
			p.advance()
		}
		if numTok := p.current(); numTok.Type == TokenNumber {
			n, atoiErr := strconv.Atoi(numTok.Value)
			if atoiErr != nil {
				p.addErrorf(numTok.Line, numTok.Col, "invalid number %q: %v", numTok.Value, atoiErr)
				return
			}
			if slice.DisruptionBudget == nil {
				slice.DisruptionBudget = &DisruptionBudgetSpec{}
			}
			slice.DisruptionBudget.MaxUnavailable = n
			p.advance()
		}
		p.recoverDirective()
	} else if tok.Type == TokenCopies || (tok.Type == TokenIdent && strings.ToLower(tok.Value) == "copies") {
		// "allow copies to find each other"
		slice.PeerDiscovery = true
		p.recoverDirective()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected 'at' or 'copies' after 'allow'")
		p.recoverDirective()
	}
}

// parseBlock handles "block all traffic except from namespace NS".
func (p *Parser) parseBlock(slice *Slice) {
	p.advance() // consume 'block'

	// consume "all"
	if p.matchIdent("all") {
		p.advance()
	}
	if p.matchType(TokenTraffic) {
		p.advance()
	}
	if p.matchType(TokenExcept) {
		p.advance()
	}
	if p.matchType(TokenFrom) {
		p.advance()
	}

	// "namespace NS"
	if p.matchIdent("namespace") {
		p.advance()
		tok := p.current()
		if tok.Type == TokenString || tok.Type == TokenIdent {
			if slice.NetworkPolicy == nil {
				slice.NetworkPolicy = &NetworkPolicySpec{}
			}
			slice.NetworkPolicy.AllowNamespace = append(slice.NetworkPolicy.AllowNamespace, tok.Value)
			p.advance()
		}
	}
}

// parseDrop parses "drop all capabilities".
func (p *Parser) parseDrop(slice *Slice) {
	p.advance() // consume 'drop'

	// consume "all"
	if p.matchIdent("all") {
		p.advance()
	}
	// consume "capabilities"
	if p.matchIdent("capabilities") {
		p.advance()
	}

	if slice.Security == nil {
		slice.Security = &SecuritySpec{}
	}
	slice.Security.DropCapabilities = append(slice.Security.DropCapabilities, "ALL")
}

// parseUpdate parses "update one at a time" or "update N% at a time".
func (p *Parser) parseUpdate(slice *Slice) {
	p.advance() // consume 'update'

	tok := p.current()
	us := UpdateStrategySpec{}

	if tok.Type == TokenIdent && strings.ToLower(tok.Value) == "one" {
		// "update one at a time"
		us.MaxSurge = "1"
		us.MaxUnavailable = "0"
		p.advance()
	} else if tok.Type == TokenIdent || tok.Type == TokenNumber {
		// "update N% at a time"
		us.MaxSurge = tok.Value
		us.MaxUnavailable = "0"
		p.advance()
	}

	// consume "at a time"
	p.recoverDirective()
	slice.UpdateStrategy = &us
}

// parseGraceful parses "graceful shutdown Ns" or "graceful shutdown Nm".
func (p *Parser) parseGraceful(slice *Slice) {
	p.advance() // consume 'graceful'

	if p.matchType(TokenShutdown) {
		p.advance()
	}

	tok := p.current()
	if tok.Type == TokenIdent || tok.Type == TokenNumber {
		val := tok.Value
		p.advance()

		seconds := parseTimeSuffix(val)
		slice.GracefulShutdown = seconds
	}
}

// parseReadOnlyFS parses "read-only filesystem".
func (p *Parser) parseReadOnlyFS(slice *Slice) {
	p.advance() // consume 'read-only'

	if p.matchType(TokenFilesystem) {
		p.advance()
	}

	if slice.Security == nil {
		slice.Security = &SecuritySpec{}
	}
	slice.Security.ReadOnlyRoot = true
}

// parseIdentDirective handles directives that start with unrecognized idents:
//   - "only accept traffic from ..."
//   - "after starting, run CMD"
//   - "start copies in order"
//   - "add capability NAME"
//   - "use account NAME"
func (p *Parser) parseIdentDirective(slice *Slice) {
	tok := p.current()
	word := strings.ToLower(tok.Value)

	switch word {
	case "only":
		// "only accept traffic from SLICE [and SLICE...]"
		p.advance() // consume 'only'
		if p.matchType(TokenAccept) {
			p.advance()
		}
		if p.matchType(TokenTraffic) {
			p.advance()
		}
		if p.matchType(TokenFrom) {
			p.advance()
		}
		if slice.NetworkPolicy == nil {
			slice.NetworkPolicy = &NetworkPolicySpec{}
		}
		// Parse slice names separated by "and".
		for !p.atEnd() && !p.matchType(TokenNewline) {
			nameTok := p.current()
			if nameTok.Type == TokenIdent || nameTok.Type == TokenString {
				slice.NetworkPolicy.AllowFrom = append(slice.NetworkPolicy.AllowFrom, nameTok.Value)
				p.advance()
			} else {
				break
			}
			if p.matchType(TokenAnd) {
				p.advance()
			} else {
				break
			}
		}

	case "after":
		// "after starting, run CMD"
		p.advance() // consume 'after'
		if p.matchType(TokenStarting) {
			p.advance()
		}
		if p.matchType(TokenComma) {
			p.advance()
		}
		if p.matchType(TokenRun) {
			p.advance()
		}
		cmdTok := p.current()
		if cmdTok.Type == TokenString {
			if slice.Lifecycle == nil {
				slice.Lifecycle = &LifecycleSpec{}
			}
			slice.Lifecycle.PostStartCommand = cmdTok.Value
			p.advance()
		}

	case "start":
		// "start copies in order"
		p.advance()
		slice.OrderedStartup = true
		p.recoverDirective()

	case "add":
		// "add capability NAME"
		p.advance() // consume 'add'
		if p.matchType(TokenCapability) {
			p.advance()
		}
		capTok := p.current()
		if capTok.Type == TokenIdent || capTok.Type == TokenString {
			if slice.Security == nil {
				slice.Security = &SecuritySpec{}
			}
			slice.Security.AddCapabilities = append(slice.Security.AddCapabilities, capTok.Value)
			p.advance()
		}

	case "use":
		// "use account NAME"
		p.advance() // consume 'use'
		if p.matchType(TokenAccount) {
			p.advance()
		}
		acctTok := p.current()
		if acctTok.Type == TokenString || acctTok.Type == TokenIdent {
			slice.ServiceAccount = acctTok.Value
			p.advance()
		}

	default:
		p.addErrorf(tok.Line, tok.Col, "unknown directive %q", tok.Value)
		p.recoverDirective()
	}
}

// parseNeedsPermission is dispatched from parseNeeds when the next token is "permission".
// It handles "needs permission to VERB [and VERB] RESOURCE"
// and "needs cluster-wide permission to VERB RESOURCE".
func (p *Parser) parseNeedsPermission(slice *Slice) {
	clusterWide := false

	// Check for "cluster-wide"
	tok := p.current()
	if tok.Type == TokenIdent && strings.ToLower(tok.Value) == "cluster-wide" {
		clusterWide = true
		p.advance()
	}

	if p.matchType(TokenPermission) {
		p.advance()
	}

	// consume "to"
	if p.matchType(TokenTo) {
		p.advance()
	}

	perm := Permission{ClusterWide: clusterWide}

	// Parse verbs until we hit a non-verb token.
	for !p.atEnd() && !p.matchType(TokenNewline) {
		verbTok := p.current()
		verb := strings.ToLower(verbTok.Value)
		mapped := mapVerb(verb)
		if len(mapped) > 0 {
			perm.Verbs = append(perm.Verbs, mapped...)
			p.advance()
			if p.matchType(TokenAnd) {
				p.advance()
				continue
			}
			continue
		}
		// Not a verb — must be a resource.
		break
	}

	// Remaining ident is the resource.
	resTok := p.current()
	if resTok.Type == TokenIdent || resTok.Type == TokenString {
		perm.Resources = append(perm.Resources, resTok.Value)
		p.advance()
	}

	// Deduplicate verbs.
	perm.Verbs = dedup(perm.Verbs)
	slice.Permissions = append(slice.Permissions, perm)
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

// peek returns the token at offset ahead of current position without advancing.
func (p *Parser) peek(offset int) Token {
	idx := p.pos + offset
	if idx >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[idx]
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
