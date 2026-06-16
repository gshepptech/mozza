import { COLORS, FONTS, useScrollReveal, useMediaQuery } from "@/pages/LandingPage";

// --- SVG Icons for Features ---

function DockerImportIcon() {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
      <rect x="4" y="20" width="8" height="6" rx="1" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.5" />
      <rect x="14" y="20" width="8" height="6" rx="1" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.7" />
      <rect x="24" y="20" width="8" height="6" rx="1" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.5" />
      <rect x="14" y="12" width="8" height="6" rx="1" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <rect x="24" y="12" width="8" height="6" rx="1" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <path d="M36 23h6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.3" />
      <path d="M36 20c4-2 6 0 8-1" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.3" />
      <path d="M34 30l5 5M39 30l-5 5" stroke={COLORS.accent} strokeWidth="2" strokeLinecap="round" />
      <path d="M37 35v6" stroke={COLORS.accent} strokeWidth="2" strokeLinecap="round" />
      <circle cx="37" cy="44" r="2" fill={COLORS.accent} opacity="0.4" />
    </svg>
  );
}

function FrameworkDetectIcon() {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
      <circle cx="24" cy="24" r="18" stroke={COLORS.accent} strokeWidth="1.5" strokeDasharray="3 3" opacity="0.3" />
      <circle cx="24" cy="24" r="10" stroke={COLORS.accent} strokeWidth="2" opacity="0.6" />
      <circle cx="24" cy="24" r="3" fill={COLORS.accent} />
      <path d="M24 6v6M24 36v6M6 24h6M36 24h6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.4" />
      <path d="M11.5 11.5l4.2 4.2M32.3 32.3l4.2 4.2M11.5 36.5l4.2-4.2M32.3 15.7l4.2-4.2" stroke={COLORS.accent} strokeWidth="1" strokeLinecap="round" opacity="0.25" />
    </svg>
  );
}

function GitPushIcon() {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
      <circle cx="24" cy="38" r="4" stroke={COLORS.accent} strokeWidth="2" opacity="0.5" />
      <circle cx="24" cy="24" r="4" stroke={COLORS.accent} strokeWidth="2" opacity="0.7" />
      <circle cx="24" cy="10" r="4" fill={COLORS.accent} opacity="0.8" />
      <line x1="24" y1="14" x2="24" y2="20" stroke={COLORS.accent} strokeWidth="2" />
      <line x1="24" y1="28" x2="24" y2="34" stroke={COLORS.accent} strokeWidth="2" opacity="0.5" />
      <path d="M36 24c0 0-4 4-12 4" stroke={COLORS.accent} strokeWidth="1.5" strokeDasharray="3 2" opacity="0.3" />
      <circle cx="36" cy="24" r="3" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <path d="M20 6l4-2 4 2" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function MonitoringIcon() {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
      <rect x="4" y="8" width="40" height="28" rx="4" stroke={COLORS.accent} strokeWidth="2" opacity="0.5" />
      <line x1="4" y1="40" x2="44" y2="40" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.3" />
      <polyline
        points="10,28 16,22 22,26 28,16 34,20 40,12"
        stroke={COLORS.accent}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        fill="none"
      />
      <circle cx="40" cy="12" r="2.5" fill={COLORS.accent} />
    </svg>
  );
}

// --- Feature Data ---

const FEATURES = [
  {
    icon: DockerImportIcon,
    title: "Docker Compose Import",
    desc: "Already have a docker-compose.yml? Import it directly. Mozza converts your existing setup into a recipe with zero manual rewriting.",
    code: "$ mozza import docker-compose.yml\nImported 4 services -> my-app.mozza",
  },
  {
    icon: FrameworkDetectIcon,
    title: "Framework Detection",
    desc: "Point Mozza at your repo. It detects your language, framework, databases, and build tools automatically. Node, Go, Python, Rails, and more.",
    code: "$ mozza init .\nDetected: Next.js + PostgreSQL + Redis\nGenerated: my-app.mozza",
  },
  {
    icon: GitPushIcon,
    title: "Git Push Deploy",
    desc: "Push to your branch and Mozza builds, tests, and deploys. No CI config required. Same workflow from local dev to production.",
    code: "$ git push origin main\n-> Building containers...\n-> Deployed to production",
  },
  {
    icon: MonitoringIcon,
    title: "Built-in Monitoring",
    desc: "Health checks, resource usage, logs, and alerts built in. When something breaks, mozza doctor tells you why and offers a fix.",
    code: "$ mozza doctor\nAll 6 services healthy\nMemory: 42% | CPU: 18%",
  },
];

// --- Single Feature Row ---

function FeatureRow({
  feature,
  index,
  visible,
  isMobile,
}: {
  feature: (typeof FEATURES)[0];
  index: number;
  visible: boolean;
  isMobile: boolean;
}) {
  const reversed = index % 2 === 1;
  const delay = 0.1 + index * 0.08;

  const contentBlock = (
    <div style={{ flex: "1 1 400px", minWidth: 280 }}>
      <div style={{ marginBottom: 20 }}>
        <feature.icon />
      </div>
      <h3
        style={{
          fontSize: "clamp(22px, 3vw, 28px)",
          fontWeight: 700,
          color: COLORS.text,
          margin: "0 0 12px 0",
          fontFamily: FONTS.sans,
          letterSpacing: -0.5,
        }}
      >
        {feature.title}
      </h3>
      <p
        style={{
          fontSize: 16,
          lineHeight: 1.65,
          color: COLORS.textMuted,
          margin: 0,
          maxWidth: 440,
          fontFamily: FONTS.sans,
        }}
      >
        {feature.desc}
      </p>
    </div>
  );

  const codeBlock = (
    <div style={{ flex: "1 1 400px", minWidth: 280 }}>
      <div
        style={{
          background: COLORS.terminal,
          borderRadius: 12,
          border: `1px solid ${COLORS.border}`,
          overflow: "hidden",
        }}
      >
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
        </div>
        <pre
          style={{
            padding: "20px 24px",
            margin: 0,
            fontFamily: FONTS.mono,
            fontSize: 13,
            lineHeight: 1.7,
            color: COLORS.textMuted,
            overflow: "auto",
          }}
        >
          {feature.code.split("\n").map((line, li) => (
            <div
              key={li}
              style={{
                color: line.startsWith("$")
                  ? COLORS.text
                  : line.startsWith("->")
                    ? COLORS.success
                    : line.includes("Detected") || line.includes("Imported") || line.includes("Generated")
                      ? COLORS.success
                      : COLORS.textMuted,
                fontWeight: line.startsWith("$") ? 600 : 400,
              }}
            >
              {line}
            </div>
          ))}
        </pre>
      </div>
    </div>
  );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: isMobile ? "column" : reversed ? "row-reverse" : "row",
        alignItems: "center",
        gap: isMobile ? 32 : 64,
        opacity: visible ? 1 : 0,
        transform: visible ? "translateY(0)" : "translateY(30px)",
        transition: `opacity 0.6s ease ${delay}s, transform 0.6s ease ${delay}s`,
      }}
    >
      {contentBlock}
      {codeBlock}
    </div>
  );
}

// --- Section ---

export function FeatureShowcase() {
  const [ref, visible] = useScrollReveal();
  const isMobile = useMediaQuery("(max-width: 768px)");

  return (
    <section ref={ref} style={{ padding: "100px 24px" }}>
      <div style={{ maxWidth: 1200, margin: "0 auto" }}>
        {/* Section header */}
        <div style={{ textAlign: "center", marginBottom: 80 }}>
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
            Everything you need,{" "}
            <span style={{ color: COLORS.accent }}>nothing you don't</span>
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
            Batteries included. Complexity excluded.
          </p>
        </div>

        {/* Feature rows */}
        <div style={{ display: "flex", flexDirection: "column", gap: 80 }}>
          {FEATURES.map((feature, i) => (
            <FeatureRow key={i} feature={feature} index={i} visible={visible} isMobile={isMobile} />
          ))}
        </div>
      </div>
    </section>
  );
}
