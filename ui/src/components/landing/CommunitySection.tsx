import { useState } from "react";
import { COLORS, FONTS, useScrollReveal } from "@/pages/LandingPage";

// --- Community Cards ---

function DiscordCard({ visible }: { visible: boolean }) {
  const [hovered, setHovered] = useState(false);

  return (
    <a
      href="https://discord.gg/mozza"
      target="_blank"
      rel="noopener noreferrer"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        flex: "1 1 340px",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        textAlign: "center",
        padding: "48px 32px",
        background: hovered ? COLORS.bgCard : COLORS.terminal,
        border: `1px solid ${hovered ? COLORS.borderHover : COLORS.border}`,
        borderRadius: 16,
        textDecoration: "none",
        transition: "all 0.2s ease",
        opacity: visible ? 1 : 0,
        transform: visible ? "translateY(0)" : "translateY(24px)",
        transitionProperty: "opacity, transform, background, border-color",
        transitionDuration: "0.6s, 0.6s, 0.2s, 0.2s",
        transitionDelay: "0.15s, 0.15s, 0s, 0s",
      }}
    >
      {/* Discord icon */}
      <svg width="48" height="48" viewBox="0 0 48 48" fill="none" style={{ marginBottom: 20 }}>
        <rect x="4" y="10" width="40" height="28" rx="8" stroke="#5865F2" strokeWidth="2" />
        <circle cx="17" cy="24" r="3" fill="#5865F2" />
        <circle cx="31" cy="24" r="3" fill="#5865F2" />
        <path d="M14 10c0 0 2-4 10-4s10 4 10 4" stroke="#5865F2" strokeWidth="2" strokeLinecap="round" opacity="0.5" />
        <path d="M16 38l-2 4" stroke="#5865F2" strokeWidth="2" strokeLinecap="round" opacity="0.5" />
        <path d="M32 38l2 4" stroke="#5865F2" strokeWidth="2" strokeLinecap="round" opacity="0.5" />
      </svg>
      <h3
        style={{
          fontSize: 20,
          fontWeight: 700,
          color: COLORS.text,
          margin: "0 0 8px 0",
          fontFamily: FONTS.sans,
        }}
      >
        Join the Community
      </h3>
      <p
        style={{
          fontSize: 15,
          color: COLORS.textMuted,
          margin: "0 0 20px 0",
          lineHeight: 1.5,
          fontFamily: FONTS.sans,
        }}
      >
        Ask questions, share recipes, and help shape the roadmap. Friendly humans, zero gatekeeping.
      </p>
      <span
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          fontSize: 14,
          fontWeight: 600,
          color: "#5865F2",
          fontFamily: FONTS.sans,
        }}
      >
        Join Discord
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M2 7h10M8 3l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
    </a>
  );
}

function GitHubCard({ visible }: { visible: boolean }) {
  const [hovered, setHovered] = useState(false);

  return (
    <a
      href="https://github.com/gshepptech/mozza"
      target="_blank"
      rel="noopener noreferrer"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        flex: "1 1 340px",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        textAlign: "center",
        padding: "48px 32px",
        background: hovered ? COLORS.bgCard : COLORS.terminal,
        border: `1px solid ${hovered ? COLORS.borderHover : COLORS.border}`,
        borderRadius: 16,
        textDecoration: "none",
        transition: "all 0.2s ease",
        opacity: visible ? 1 : 0,
        transform: visible ? "translateY(0)" : "translateY(24px)",
        transitionProperty: "opacity, transform, background, border-color",
        transitionDuration: "0.6s, 0.6s, 0.2s, 0.2s",
        transitionDelay: "0.25s, 0.25s, 0s, 0s",
      }}
    >
      {/* GitHub icon */}
      <svg width="48" height="48" viewBox="0 0 48 48" fill="none" style={{ marginBottom: 20 }}>
        <path
          d="M24 4C12.96 4 4 12.96 4 24c0 8.84 5.74 16.33 13.71 18.97.5.09.68-.22.68-.48l-.01-3.72c-5.57 1.21-6.75-1.33-6.75-1.33-.91-2.32-2.22-2.94-2.22-2.94-1.82-1.24.14-1.22.14-1.22 2.01.14 3.07 2.07 3.07 2.07 1.79 3.06 4.69 2.18 5.83 1.67.18-1.29.7-2.18 1.27-2.68-4.45-.5-9.13-2.22-9.13-9.88 0-2.18.78-3.97 2.05-5.37-.2-.5-.89-2.54.2-5.3 0 0 1.68-.54 5.5 2.05A19.1 19.1 0 0124 14.28c1.7.01 3.41.23 5.01.68 3.81-2.59 5.49-2.05 5.49-2.05 1.1 2.76.41 4.8.2 5.3 1.28 1.4 2.05 3.19 2.05 5.37 0 7.69-4.69 9.37-9.15 9.87.72.62 1.36 1.84 1.36 3.7l-.01 5.5c0 .27.18.58.69.48C38.27 40.32 44 32.83 44 24 44 12.96 35.04 4 24 4z"
          fill={COLORS.text}
        />
      </svg>
      <h3
        style={{
          fontSize: 20,
          fontWeight: 700,
          color: COLORS.text,
          margin: "0 0 8px 0",
          fontFamily: FONTS.sans,
        }}
      >
        Open Source
      </h3>
      <p
        style={{
          fontSize: 15,
          color: COLORS.textMuted,
          margin: "0 0 20px 0",
          lineHeight: 1.5,
          fontFamily: FONTS.sans,
        }}
      >
        Mozza is open source. Read the code, report bugs, contribute features, fork it.
      </p>
      <span
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 6,
          fontSize: 14,
          fontWeight: 600,
          color: COLORS.text,
          fontFamily: FONTS.sans,
        }}
      >
        Star on GitHub
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M2 7h10M8 3l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </span>
    </a>
  );
}

// --- Section ---

export function CommunitySection() {
  const [ref, visible] = useScrollReveal();

  return (
    <section
      ref={ref}
      style={{
        padding: "100px 24px",
        background: COLORS.bgSubtle,
        borderTop: `1px solid ${COLORS.border}`,
        borderBottom: `1px solid ${COLORS.border}`,
      }}
    >
      <div style={{ maxWidth: 800, margin: "0 auto" }}>
        {/* Header */}
        <div style={{ textAlign: "center", marginBottom: 48 }}>
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
            Built in the open
          </h2>
          <p
            style={{
              fontSize: 16,
              color: COLORS.textMuted,
              margin: 0,
              fontFamily: FONTS.sans,
              opacity: visible ? 1 : 0,
              transform: visible ? "translateY(0)" : "translateY(20px)",
              transition: "opacity 0.6s ease 0.1s, transform 0.6s ease 0.1s",
            }}
          >
            Mozza is shaped by its users. Come say hello.
          </p>
        </div>

        {/* Cards */}
        <div style={{ display: "flex", gap: 20, flexWrap: "wrap" }}>
          <DiscordCard visible={visible} />
          <GitHubCard visible={visible} />
        </div>
      </div>
    </section>
  );
}
