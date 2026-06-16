import { useState, useCallback } from "react";
import { COLORS, FONTS, useScrollReveal } from "@/pages/LandingPage";

const INSTALL_CMD = "go install github.com/gshepptech/mozza/cmd/mozza@latest";

export function InstallCTA() {
  const [ref, visible] = useScrollReveal();
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(INSTALL_CMD);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers
      const textarea = document.createElement("textarea");
      textarea.value = INSTALL_CMD;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand("copy");
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, []);

  return (
    <section ref={ref} style={{ padding: "100px 24px", position: "relative", overflow: "hidden" }}>
      {/* Background glow */}
      <div
        style={{
          position: "absolute",
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
          width: 600,
          height: 400,
          background: `radial-gradient(ellipse at center, ${COLORS.accentGlow} 0%, transparent 70%)`,
          pointerEvents: "none",
          opacity: 0.25,
        }}
      />

      <div
        style={{
          maxWidth: 640,
          margin: "0 auto",
          textAlign: "center",
          position: "relative",
          zIndex: 1,
        }}
      >
        {/* Heading */}
        <h2
          style={{
            fontSize: "clamp(28px, 4vw, 40px)",
            fontWeight: 700,
            letterSpacing: -0.8,
            margin: "0 0 12px 0",
            fontFamily: FONTS.sans,
            color: COLORS.text,
            opacity: visible ? 1 : 0,
            transform: visible ? "translateY(0)" : "translateY(20px)",
            transition: "opacity 0.6s ease, transform 0.6s ease",
          }}
        >
          Ready to <span style={{ color: COLORS.accent }}>ship</span>?
        </h2>
        <p
          style={{
            fontSize: 16,
            color: COLORS.textMuted,
            margin: "0 0 40px 0",
            fontFamily: FONTS.sans,
            opacity: visible ? 1 : 0,
            transform: visible ? "translateY(0)" : "translateY(20px)",
            transition: "opacity 0.6s ease 0.1s, transform 0.6s ease 0.1s",
          }}
        >
          Install Mozza in one command. Works on macOS, Linux, and WSL.
        </p>

        {/* Terminal install block */}
        <div
          style={{
            background: COLORS.terminal,
            borderRadius: 12,
            border: `1px solid ${COLORS.border}`,
            overflow: "hidden",
            opacity: visible ? 1 : 0,
            transform: visible ? "translateY(0)" : "translateY(20px)",
            transition: "opacity 0.6s ease 0.2s, transform 0.6s ease 0.2s",
          }}
        >
          {/* Title bar */}
          <div
            style={{
              padding: "10px 16px",
              borderBottom: `1px solid ${COLORS.border}`,
              display: "flex",
              alignItems: "center",
              gap: 8,
            }}
          >
            <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#f85149" }} />
            <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#d29922" }} />
            <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#3fb950" }} />
            <span style={{ marginLeft: 8, fontSize: 12, color: COLORS.textDim, fontFamily: FONTS.mono }}>
              install
            </span>
          </div>

          {/* Command + copy button */}
          <div
            style={{
              padding: "20px 24px",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 16,
            }}
          >
            <code
              style={{
                fontFamily: FONTS.mono,
                fontSize: 14,
                color: COLORS.text,
                fontWeight: 500,
                whiteSpace: "nowrap",
                overflow: "auto",
                flex: 1,
              }}
            >
              <span style={{ color: COLORS.textDim }}>$ </span>
              {INSTALL_CMD}
            </code>

            <button
              onClick={handleCopy}
              style={{
                flexShrink: 0,
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                padding: "8px 16px",
                borderRadius: 8,
                border: `1px solid ${copied ? `${COLORS.success}44` : COLORS.border}`,
                background: copied ? `${COLORS.success}11` : "transparent",
                color: copied ? COLORS.success : COLORS.textMuted,
                fontSize: 13,
                fontWeight: 500,
                fontFamily: FONTS.sans,
                cursor: "pointer",
                transition: "all 0.2s ease",
              }}
              onMouseEnter={(e) => {
                if (!copied) {
                  e.currentTarget.style.borderColor = COLORS.borderHover;
                  e.currentTarget.style.color = COLORS.text;
                }
              }}
              onMouseLeave={(e) => {
                if (!copied) {
                  e.currentTarget.style.borderColor = COLORS.border;
                  e.currentTarget.style.color = COLORS.textMuted;
                }
              }}
            >
              {copied ? (
                <>
                  <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                    <path d="M3 7.5l3 3 5-6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                  Copied
                </>
              ) : (
                <>
                  <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
                    <rect x="4" y="4" width="9" height="9" rx="1.5" stroke="currentColor" strokeWidth="1.2" />
                    <path d="M10 4V2.5A1.5 1.5 0 008.5 1h-6A1.5 1.5 0 001 2.5v6A1.5 1.5 0 002.5 10H4" stroke="currentColor" strokeWidth="1.2" />
                  </svg>
                  Copy
                </>
              )}
            </button>
          </div>
        </div>

        {/* Sub-text */}
        <p
          style={{
            fontSize: 13,
            color: COLORS.textDim,
            marginTop: 20,
            fontFamily: FONTS.sans,
            opacity: visible ? 1 : 0,
            transition: "opacity 0.6s ease 0.4s",
          }}
        >
          Requires Docker. Verified checksums. No sudo needed.
        </p>
      </div>
    </section>
  );
}
