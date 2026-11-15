package recipe

import (
	"strconv"
	"strings"
)

// Handlers for placement, scaling, network policy, security context and RBAC.

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

	if slice.Scheduling == nil {
		slice.Scheduling = &SchedulingSpec{}
	}
	slice.Scheduling.AntiAffinity = true

	p.consumeTrailing("run", "two", "copies", "copy", "on", "the", "same", "node")
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

	p.consumeTrailing("copies", "copy", "running", "during", "updates")
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
		p.consumeTrailing("copies", "copy", "down", "during", "updates")
	} else if tok.Type == TokenCopies || (tok.Type == TokenIdent && strings.ToLower(tok.Value) == "copies") {
		// "allow copies to find each other"
		slice.PeerDiscovery = true
		p.consumeTrailing("copies", "to", "find", "each", "other")
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

	p.consumeTrailing("at", "a", "time")
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
		p.consumeTrailing("copies", "copy", "in", "order")

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
