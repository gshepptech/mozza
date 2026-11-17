package recipe

import "strings"

// Handlers for schedules, init steps, sidecars, mounts and probes.

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
//   - "before starting:": init block
//   - "before stopping, wait Ns": lifecycle pre-stop wait
//   - "before stopping, run CMD": lifecycle pre-stop command
func (p *Parser) parseBefore(slice *Slice) {
	p.advance() // consume 'before'

	tok := p.current()
	isStarting := tok.Type == TokenStarting ||
		(tok.Type == TokenIdent && strings.ToLower(strings.TrimSuffix(tok.Value, ":")) == "starting")
	isStopping := tok.Type == TokenStopping ||
		(tok.Type == TokenIdent && strings.ToLower(strings.TrimSuffix(tok.Value, ":")) == "stopping")

	if isStarting {
		// "before starting:": init block mode
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
		switch next.Type {
		case TokenWait:
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
		case TokenRun:
			p.advance() // consume 'run'
			cmdTok := p.current()
			if cmdTok.Type == TokenString {
				if slice.Lifecycle == nil {
					slice.Lifecycle = &LifecycleSpec{}
				}
				slice.Lifecycle.PreStopCommand = cmdTok.Value
				p.advance()
			}
		default:
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
	consumedLine := p.lastConsumedLine(0)

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

		// An init step must begin a line of its own. Handing a mid-line 'run'
		// back to parseSection keeps one reporting point for a section body.
		if tok.Line <= consumedLine {
			break
		}
		p.advance() // consume 'run'

		// Expect "image".
		if !p.matchType(TokenImage) {
			// Not an init step line. Step back over 'run' so the caller sees
			// the line from its first token.
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
		consumedLine = p.lastConsumedLine(consumedLine)
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
			n, ok := p.portNumber(numTok)
			if !ok {
				p.recoverDirective()
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
			n, ok := p.portNumber(numTok)
			if !ok {
				p.recoverDirective()
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
