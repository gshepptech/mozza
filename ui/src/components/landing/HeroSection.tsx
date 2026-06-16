import { useState, useEffect, useRef, useCallback } from "react";
import { Link } from "react-router-dom";
import { COLORS, FONTS } from "@/pages/LandingPage";
import { MozzaMascot } from "./MozzaMascot";

// --- Terminal Typing Animation ---

const TERMINAL_LINES = [
  { type: "cmd" as const, text: "$ mozza init", delay: 0 },
  { type: "out" as const, text: 'Scanning project... detected Node.js + PostgreSQL', delay: 800 },
  { type: "out" as const, text: "Recipe created: my-app.mozza", delay: 1600 },
  { type: "cmd" as const, text: "$ mozza up", delay: 2800 },
  { type: "out" as const, text: "Building containers...", delay: 3400 },
  { type: "out" as const, text: "web       -> http://localhost:3000", delay: 4200 },
  { type: "out" as const, text: "postgres  -> ready", delay: 4800 },
  { type: "out" as const, text: "Your app is served!", delay: 5600 },
];

function TerminalAnimation() {
  const [visibleLines, setVisibleLines] = useState(0);
  const [typingIdx, setTypingIdx] = useState(0);
  const [typedChars, setTypedChars] = useState(0);
  const rafRef = useRef<number>(0);
  const startRef = useRef<number>(0);

  const animate = useCallback((timestamp: number) => {
    if (!startRef.current) startRef.current = timestamp;
    const elapsed = timestamp - startRef.current;

    let linesShown = 0;
    let currentTyping = -1;
    let charsTyped = 0;

    for (let i = 0; i < TERMINAL_LINES.length; i++) {
      const line = TERMINAL_LINES[i]!;
      if (elapsed < line.delay) break;

      if (line.type === "cmd") {
        const typeTime = elapsed - line.delay;
        const totalChars = line.text.length;
        const speed = 45; // ms per char
        charsTyped = Math.min(Math.floor(typeTime / speed), totalChars);
        if (charsTyped < totalChars) {
          currentTyping = i;
          linesShown = i;
          break;
        }
      }
      linesShown = i + 1;
    }

    setVisibleLines(linesShown);
    setTypingIdx(currentTyping);
    setTypedChars(charsTyped);

    if (linesShown < TERMINAL_LINES.length || currentTyping >= 0) {
      rafRef.current = requestAnimationFrame(animate);
    }
  }, []);

  useEffect(() => {
    // Reset on remount (React Strict Mode double-invokes effects in dev)
    startRef.current = 0;
    setVisibleLines(0);
    setTypingIdx(0);
    setTypedChars(0);

    const timeout = setTimeout(() => {
      rafRef.current = requestAnimationFrame(animate);
    }, 600);
    return () => {
      clearTimeout(timeout);
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, [animate]);

  return (
    <div
      style={{
        background: COLORS.terminal,
        borderRadius: 12,
        border: `1px solid ${COLORS.border}`,
        overflow: "hidden",
        width: "100%",
        maxWidth: 560,
      }}
    >
      {/* Title bar */}
      <div
        style={{
          padding: "10px 16px",
          display: "flex",
          alignItems: "center",
          gap: 8,
          borderBottom: `1px solid ${COLORS.border}`,
        }}
      >
        <div style={{ width: 12, height: 12, borderRadius: "50%", background: "#f85149" }} />
        <div style={{ width: 12, height: 12, borderRadius: "50%", background: "#d29922" }} />
        <div style={{ width: 12, height: 12, borderRadius: "50%", background: "#3fb950" }} />
        <span style={{ marginLeft: 8, fontSize: 12, color: COLORS.textDim, fontFamily: FONTS.mono }}>
          terminal
        </span>
      </div>

      {/* Content */}
      <div style={{ padding: "16px 20px", minHeight: 200, fontFamily: FONTS.mono, fontSize: 13, lineHeight: 1.7 }}>
        {TERMINAL_LINES.map((line, i) => {
          if (i === typingIdx) {
            // Currently typing this command
            const shown = line.text.slice(0, typedChars);
            return (
              <div key={i} style={{ color: COLORS.text }}>
                {shown}
                <span
                  style={{
                    display: "inline-block",
                    width: 8,
                    height: 16,
                    background: COLORS.accent,
                    marginLeft: 1,
                    verticalAlign: "text-bottom",
                    animation: "blink 1s step-end infinite",
                  }}
                />
              </div>
            );
          }
          if (i >= visibleLines) return null;
          const isCmd = line.type === "cmd";
          const isSuccess = line.text.includes("->") || line.text.includes("served") || line.text.includes("created");
          return (
            <div
              key={i}
              style={{
                color: isCmd ? COLORS.text : isSuccess ? COLORS.success : COLORS.textMuted,
                fontWeight: isCmd ? 600 : 400,
              }}
            >
              {line.text}
            </div>
          );
        })}
      </div>

      {/* Inject blink keyframes */}
      <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
    </div>
  );
}

// --- Hero Section ---

export function HeroSection() {
  return (
    <section
      style={{
        position: "relative",
        paddingTop: 140,
        paddingBottom: 100,
        paddingLeft: 24,
        paddingRight: 24,
        overflow: "hidden",
      }}
    >
      {/* Subtle radial glow behind content */}
      <div
        style={{
          position: "absolute",
          top: -100,
          left: "50%",
          transform: "translateX(-50%)",
          width: 800,
          height: 600,
          background: `radial-gradient(ellipse at center, ${COLORS.accentGlow} 0%, transparent 70%)`,
          pointerEvents: "none",
          opacity: 0.4,
        }}
      />

      <div
        style={{
          maxWidth: 1200,
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 64,
          flexWrap: "wrap",
        }}
      >
        {/* Left: copy */}
        <div style={{ flex: "1 1 480px", minWidth: 320, position: "relative", zIndex: 1 }}>
          <div
            style={{
              display: "inline-block",
              padding: "6px 14px",
              borderRadius: 999,
              background: COLORS.accentDim,
              border: `1px solid ${COLORS.accent}33`,
              fontSize: 13,
              fontWeight: 600,
              color: COLORS.accent,
              marginBottom: 24,
              fontFamily: FONTS.sans,
            }}
          >
            Open Source Container Orchestrator
          </div>

          <h1
            style={{
              fontSize: "clamp(36px, 5vw, 56px)",
              fontWeight: 800,
              lineHeight: 1.1,
              letterSpacing: -1.5,
              margin: "0 0 20px 0",
              fontFamily: FONTS.sans,
              color: COLORS.text,
            }}
          >
            Deploy like you're{" "}
            <span style={{ color: COLORS.accent }}>ordering pizza</span>
          </h1>

          <p
            style={{
              fontSize: 18,
              lineHeight: 1.6,
              color: COLORS.textMuted,
              margin: "0 0 36px 0",
              maxWidth: 480,
              fontFamily: FONTS.sans,
            }}
          >
            Write a recipe. Mozza handles containers, networking, secrets, and scaling.
            Production deployments accessible to everyone, not just platform engineers.
          </p>

          <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
            <Link
              to="/register"
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 8,
                padding: "14px 28px",
                borderRadius: 10,
                background: COLORS.accent,
                color: "#fff",
                fontSize: 16,
                fontWeight: 600,
                textDecoration: "none",
                fontFamily: FONTS.sans,
                transition: "transform 0.15s, box-shadow 0.15s",
                boxShadow: `0 0 24px ${COLORS.accentGlow}`,
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.transform = "translateY(-1px)";
                e.currentTarget.style.boxShadow = `0 4px 32px ${COLORS.accentGlow}`;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.transform = "translateY(0)";
                e.currentTarget.style.boxShadow = `0 0 24px ${COLORS.accentGlow}`;
              }}
            >
              Get Started
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path d="M3 8h10M9 4l4 4-4 4" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </Link>
            <a
              href="https://github.com/gshepptech/mozza"
              target="_blank"
              rel="noopener noreferrer"
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 8,
                padding: "14px 28px",
                borderRadius: 10,
                background: "transparent",
                border: `1px solid ${COLORS.border}`,
                color: COLORS.text,
                fontSize: 16,
                fontWeight: 500,
                textDecoration: "none",
                fontFamily: FONTS.sans,
                transition: "border-color 0.2s",
              }}
              onMouseEnter={(e) => (e.currentTarget.style.borderColor = COLORS.borderHover)}
              onMouseLeave={(e) => (e.currentTarget.style.borderColor = COLORS.border)}
            >
              <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
              </svg>
              GitHub
            </a>
          </div>
        </div>

        {/* Right: terminal + mascot */}
        <div style={{ flex: "1 1 480px", minWidth: 320, display: "flex", justifyContent: "center", position: "relative" }}>
          <TerminalAnimation />
          {/* Mascot peeking from behind terminal */}
          <div
            style={{
              position: "absolute",
              bottom: -16,
              right: 20,
              transform: "rotate(8deg)",
              opacity: 0.9,
              pointerEvents: "none",
            }}
          >
            <MozzaMascot size={64} mood="wink" />
          </div>
        </div>
      </div>
    </section>
  );
}
