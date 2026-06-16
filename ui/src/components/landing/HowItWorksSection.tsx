import { COLORS, FONTS, useScrollReveal, useMediaQuery } from "@/pages/LandingPage";

// --- Custom SVG Icons ---

function RecipeIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
      <rect x="8" y="4" width="24" height="32" rx="3" stroke={COLORS.accent} strokeWidth="2" />
      <line x1="13" y1="12" x2="27" y2="12" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.7" />
      <line x1="13" y1="18" x2="24" y2="18" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.5" />
      <line x1="13" y1="24" x2="21" y2="24" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.5" />
      <circle cx="27" cy="28" r="5" fill={COLORS.accent} opacity="0.2" />
      <path d="M25 28l1.5 1.5L29 27" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function DetectIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
      <circle cx="20" cy="20" r="14" stroke={COLORS.accent} strokeWidth="2" opacity="0.3" />
      <circle cx="20" cy="20" r="9" stroke={COLORS.accent} strokeWidth="2" opacity="0.6" />
      <circle cx="20" cy="20" r="4" fill={COLORS.accent} />
      <line x1="20" y1="2" x2="20" y2="8" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <line x1="20" y1="32" x2="20" y2="38" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <line x1="2" y1="20" x2="8" y2="20" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <line x1="32" y1="20" x2="38" y2="20" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
    </svg>
  );
}

function BuildIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
      <rect x="6" y="22" width="10" height="14" rx="2" stroke={COLORS.accent} strokeWidth="2" opacity="0.6" />
      <rect x="15" y="14" width="10" height="22" rx="2" stroke={COLORS.accent} strokeWidth="2" opacity="0.8" />
      <rect x="24" y="6" width="10" height="30" rx="2" stroke={COLORS.accent} strokeWidth="2" />
      <line x1="9" y1="28" x2="13" y2="28" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <line x1="18" y1="22" x2="22" y2="22" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <line x1="27" y1="14" x2="31" y2="14" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
    </svg>
  );
}

function DeployIcon() {
  return (
    <svg width="40" height="40" viewBox="0 0 40 40" fill="none">
      <path d="M20 4L34 14V30L20 36L6 30V14L20 4Z" stroke={COLORS.accent} strokeWidth="2" />
      <path d="M20 4V36" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.3" />
      <path d="M6 14L20 20L34 14" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.3" />
      <circle cx="20" cy="20" r="4" fill={COLORS.accent} opacity="0.6" />
      <path d="M18 20l1.5 1.5L22 19" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// --- Connector Arrow SVG ---

function ConnectorArrow({ isMobile }: { isMobile: boolean }) {
  if (isMobile) return null;
  return (
    <svg width="48" height="24" viewBox="0 0 48 24" fill="none" style={{ flexShrink: 0 }}>
      <line x1="0" y1="12" x2="38" y2="12" stroke={COLORS.border} strokeWidth="2" strokeDasharray="4 4" />
      <path d="M36 6l8 6-8 6" stroke={COLORS.accent} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" opacity="0.6" />
    </svg>
  );
}

// --- Step Data ---

const STEPS = [
  {
    icon: RecipeIcon,
    title: "Write a Recipe",
    desc: "Describe your app in a simple YAML file. Name, image, ports, dependencies.",
  },
  {
    icon: DetectIcon,
    title: "Auto-Detect",
    desc: "Mozza scans your project, detects frameworks, databases, and services.",
  },
  {
    icon: BuildIcon,
    title: "Build & Configure",
    desc: "Containers built, networking wired, secrets injected, health checks set.",
  },
  {
    icon: DeployIcon,
    title: "Deploy Anywhere",
    desc: "Ship to local dev, staging, or production with the same recipe.",
  },
];

// --- Section ---

export function HowItWorksSection() {
  const [ref, visible] = useScrollReveal();
  const isMobile = useMediaQuery("(max-width: 768px)");

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
      <div style={{ maxWidth: 1200, margin: "0 auto" }}>
        {/* Section header */}
        <div style={{ textAlign: "center", marginBottom: 64 }}>
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
            From code to production in{" "}
            <span style={{ color: COLORS.accent }}>four steps</span>
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
            No Helm charts. No service meshes. No YAML sprawl.
          </p>
        </div>

        {/* Steps */}
        <div
          style={{
            display: "flex",
            flexDirection: isMobile ? "column" : "row",
            alignItems: "center",
            justifyContent: "center",
            gap: isMobile ? 32 : 0,
          }}
        >
          {STEPS.map((step, i) => {
            const delay = 0.15 + i * 0.1;
            return (
              <div
                key={i}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: isMobile ? 0 : 0,
                }}
              >
                {/* Step card */}
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    alignItems: "center",
                    textAlign: "center",
                    padding: "28px 20px",
                    width: isMobile ? "100%" : 220,
                    opacity: visible ? 1 : 0,
                    transform: visible ? "translateY(0)" : "translateY(24px)",
                    transition: `opacity 0.5s ease ${delay}s, transform 0.5s ease ${delay}s`,
                  }}
                >
                  {/* Step number */}
                  <div
                    style={{
                      width: 28,
                      height: 28,
                      borderRadius: "50%",
                      background: COLORS.accentDim,
                      border: `1px solid ${COLORS.accent}44`,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      fontSize: 12,
                      fontWeight: 700,
                      color: COLORS.accent,
                      fontFamily: FONTS.mono,
                      marginBottom: 16,
                    }}
                  >
                    {i + 1}
                  </div>

                  {/* Icon */}
                  <div style={{ marginBottom: 16 }}>
                    <step.icon />
                  </div>

                  <h3
                    style={{
                      fontSize: 16,
                      fontWeight: 600,
                      color: COLORS.text,
                      margin: "0 0 8px 0",
                      fontFamily: FONTS.sans,
                    }}
                  >
                    {step.title}
                  </h3>
                  <p
                    style={{
                      fontSize: 14,
                      color: COLORS.textMuted,
                      margin: 0,
                      lineHeight: 1.5,
                      fontFamily: FONTS.sans,
                    }}
                  >
                    {step.desc}
                  </p>
                </div>

                {/* Connector */}
                {i < STEPS.length - 1 && <ConnectorArrow isMobile={isMobile} />}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
