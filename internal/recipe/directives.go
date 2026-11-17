package recipe

import (
	"strconv"
	"strings"
)

// Handlers for the directives that describe a container: its image, ports,
// health check, command, dependencies, environment, limits and secrets.

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

// portNumber reads tok as a port number, reporting a parse error at tok's
// position and returning false when it is not one Kubernetes will accept.
//
// The range is judged here as well as in plan.Validate because this is the last
// point that still knows the author wrote the number: plan.Slice uses zero for
// "this slice declares no port", so once a plan exists a written 0 and an
// omitted port are the same value.
//
// The lexer classifies any Unicode digit as a number token (isInteger uses
// unicode.IsDigit), so Atoi here fails on "٥" as readily as on a value too large
// for an int. One wording is true of both, and of 0 and 70000.
func (p *Parser) portNumber(tok Token) (int, bool) {
	n, err := strconv.Atoi(tok.Value)
	if err != nil || n < 1 || n > 65535 {
		p.addErrorf(tok.Line, tok.Col, "port %q is not a usable port — a port must be a number between 1 and 65535", tok.Value)

		return 0, false
	}

	return n, true
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
		if n, ok := p.portNumber(tok); ok {
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

	n, ok := p.portNumber(tok)
	if !ok {
		p.recoverDirective()
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
		// "health check by running CMD" should not reach here, because the "health" dispatch
		// handles it first.
		p.addErrorf(tok.Line, tok.Col, "expected health check path")
		p.recoverDirective()
	} else {
		p.addErrorf(tok.Line, tok.Col, "expected health check path")
		p.recoverDirective()
	}
}

// parseRun dispatches all "run ..." directives:
//   - "run N copies": replicas
//   - "run once [with N parallel]": one-shot job
//   - "run to completion": one-shot job
//   - "run every ...": schedule (cron)
//   - "run on every node [labeled ...]": daemon
//   - "run as user N" / "run as group N": security
//   - "run once, retry up to N times": retries
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
		// "run every ...": schedule
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
		switch tok.Type {
		case TokenWith:
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
		case TokenRetry:
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
		default:
			return
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
		switch tok.Type {
		case TokenDaily:
			p.advance()
			if p.matchType(TokenBackups) {
				p.advance()
			}
			slice.Backups = "daily"
		case TokenIdent, TokenNumber:
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
