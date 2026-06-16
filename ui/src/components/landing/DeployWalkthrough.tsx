import { useState, useEffect, useCallback } from "react";
import { Link } from "react-router-dom";
import { COLORS, FONTS, useScrollReveal, useMediaQuery } from "@/pages/LandingPage";
import { MozzaMascot } from "./MozzaMascot";

// --- Walkthrough Steps ---

const STEPS = [
  {
    id: "pick",
    label: "Pick a Recipe",
    mascotMood: "happy" as const,
    mascotSays: "What are we cooking today?",
  },
  {
    id: "configure",
    label: "Configure",
    mascotMood: "thinking" as const,
    mascotSays: "Let me check your ingredients...",
  },
  {
    id: "deploy",
    label: "Deploy",
    mascotMood: "wink" as const,
    mascotSays: "Order's in! Sit tight...",
  },
  {
    id: "track",
    label: "Track",
    mascotMood: "celebrate" as const,
    mascotSays: "Your app is served!",
  },
] as const;

// --- Recipe Options for Step 1 ---

const RECIPE_OPTIONS = [
  { name: "Next.js App", stack: "Node + Postgres", icon: "web" },
  { name: "REST API", stack: "Go + Redis", icon: "api" },
  { name: "Static Site", stack: "Vite + CDN", icon: "static" },
];

// --- Mini Icons ---

function RecipeMiniIcon({ type }: { type: string }) {
  if (type === "api") {
    return (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <circle cx="10" cy="10" r="6" stroke={COLORS.accent} strokeWidth="1.5" />
        <path d="M8 10h4M10 8v4" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    );
  }
  if (type === "static") {
    return (
      <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
        <rect x="3" y="4" width="14" height="12" rx="2" stroke={COLORS.accent} strokeWidth="1.5" />
        <line x1="3" y1="8" x2="17" y2="8" stroke={COLORS.accent} strokeWidth="1" opacity="0.4" />
      </svg>
    );
  }
  // web (default)
  return (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
      <rect x="2" y="3" width="16" height="11" rx="2" stroke={COLORS.accent} strokeWidth="1.5" />
      <line x1="2" y1="7" x2="18" y2="7" stroke={COLORS.accent} strokeWidth="1" opacity="0.4" />
      <line x1="8" y1="14" x2="12" y2="14" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.4" />
      <line x1="10" y1="14" x2="10" y2="17" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.4" />
    </svg>
  );
}

// --- Step 1: Pick Recipe ---

function PickStep({ onPick }: { onPick: (name: string) => void }) {
  const [hovered, setHovered] = useState<number | null>(null);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {RECIPE_OPTIONS.map((r, i) => (
        <button
          key={i}
          onClick={() => onPick(r.name)}
          onMouseEnter={() => setHovered(i)}
          onMouseLeave={() => setHovered(null)}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            padding: "12px 16px",
            background: hovered === i ? COLORS.bgCard : "transparent",
            border: `1px solid ${hovered === i ? COLORS.accent + "44" : COLORS.border}`,
            borderRadius: 10,
            cursor: "pointer",
            transition: "all 0.15s ease",
            fontFamily: FONTS.sans,
            textAlign: "left",
          }}
        >
          <RecipeMiniIcon type={r.icon} />
          <div>
            <div style={{ fontSize: 14, fontWeight: 600, color: COLORS.text }}>{r.name}</div>
            <div style={{ fontSize: 12, color: COLORS.textDim }}>{r.stack}</div>
          </div>
        </button>
      ))}
    </div>
  );
}

// --- Step 2: Configure ---

function ConfigureStep({ recipeName, onContinue }: { recipeName: string; onContinue: () => void }) {
  const [env, setEnv] = useState("staging");
  const [replicas, setReplicas] = useState(2);

  const slug = recipeName.toLowerCase().replace(/\s+/g, "-").replace(/\./g, "");

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      {/* Recipe file preview */}
      <div
        style={{
          background: COLORS.terminal,
          borderRadius: 8,
          border: `1px solid ${COLORS.border}`,
          padding: "12px 16px",
          fontFamily: FONTS.mono,
          fontSize: 12,
          lineHeight: 1.7,
        }}
      >
        <div><span style={{ color: COLORS.accent }}>name:</span> <span style={{ color: COLORS.text }}>{slug}</span></div>
        <div><span style={{ color: COLORS.accent }}>env:</span> <span style={{ color: "#79c0ff" }}>{env}</span></div>
        <div><span style={{ color: COLORS.accent }}>replicas:</span> <span style={{ color: "#79c0ff" }}>{replicas}</span></div>
      </div>

      {/* Environment toggle */}
      <div style={{ display: "flex", gap: 8 }}>
        {["dev", "staging", "production"].map((e) => (
          <button
            key={e}
            onClick={() => setEnv(e)}
            style={{
              flex: 1,
              padding: "8px 0",
              borderRadius: 6,
              border: `1px solid ${env === e ? COLORS.accent + "66" : COLORS.border}`,
              background: env === e ? COLORS.accentDim : "transparent",
              color: env === e ? COLORS.accent : COLORS.textMuted,
              fontSize: 12,
              fontWeight: 600,
              fontFamily: FONTS.sans,
              cursor: "pointer",
              transition: "all 0.15s",
            }}
          >
            {e}
          </button>
        ))}
      </div>

      {/* Replicas slider */}
      <div>
        <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 6 }}>
          <span style={{ fontSize: 12, color: COLORS.textMuted, fontFamily: FONTS.sans }}>Replicas</span>
          <span style={{ fontSize: 12, color: COLORS.accent, fontFamily: FONTS.mono, fontWeight: 600 }}>{replicas}</span>
        </div>
        <input
          type="range"
          min={1}
          max={5}
          value={replicas}
          onChange={(e) => setReplicas(Number(e.target.value))}
          style={{
            width: "100%",
            accentColor: COLORS.accent,
            cursor: "pointer",
          }}
        />
      </div>

      {/* Deploy button */}
      <button
        onClick={onContinue}
        style={{
          padding: "10px 0",
          borderRadius: 8,
          border: "none",
          background: COLORS.accent,
          color: "#fff",
          fontSize: 14,
          fontWeight: 600,
          fontFamily: FONTS.sans,
          cursor: "pointer",
          transition: "opacity 0.15s",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.9")}
        onMouseLeave={(e) => (e.currentTarget.style.opacity = "1")}
      >
        Place Order
      </button>
    </div>
  );
}

// --- Step 3: Deploy Animation ---

function DeployStep({ onComplete }: { onComplete: () => void }) {
  const [progress, setProgress] = useState(0);
  const [lines, setLines] = useState<string[]>([]);

  const deployLines = [
    "Building containers...",
    "Pushing images...",
    "Configuring networking...",
    "Injecting secrets...",
    "Starting health checks...",
    "Rolling out replicas...",
  ];

  useEffect(() => {
    let idx = 0;
    const interval = setInterval(() => {
      if (idx < deployLines.length) {
        setLines((prev) => [...prev, deployLines[idx]!]);
        setProgress(Math.round(((idx + 1) / deployLines.length) * 100));
        idx++;
      } else {
        clearInterval(interval);
        setTimeout(onComplete, 600);
      }
    }, 700);
    return () => clearInterval(interval);
  }, [onComplete]);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {/* Progress bar */}
      <div
        style={{
          height: 6,
          borderRadius: 3,
          background: COLORS.border,
          overflow: "hidden",
        }}
      >
        <div
          style={{
            height: "100%",
            width: `${progress}%`,
            background: COLORS.accent,
            borderRadius: 3,
            transition: "width 0.4s ease",
          }}
        />
      </div>
      <div style={{ fontSize: 11, color: COLORS.textDim, fontFamily: FONTS.mono, textAlign: "right" }}>
        {progress}%
      </div>

      {/* Deploy log */}
      <div
        style={{
          background: COLORS.terminal,
          borderRadius: 8,
          border: `1px solid ${COLORS.border}`,
          padding: "10px 14px",
          fontFamily: FONTS.mono,
          fontSize: 11,
          lineHeight: 1.8,
          minHeight: 120,
        }}
      >
        {lines.map((line, i) => (
          <div key={i} style={{ color: i === lines.length - 1 ? COLORS.success : COLORS.textMuted }}>
            <span style={{ color: COLORS.textDim }}>{">"}</span> {line}
            {i === lines.length - 1 && progress < 100 && (
              <span
                style={{
                  display: "inline-block",
                  width: 6,
                  height: 12,
                  background: COLORS.accent,
                  marginLeft: 4,
                  verticalAlign: "text-bottom",
                  animation: "blink 1s step-end infinite",
                }}
              />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// --- Step 4: Track / Done ---

function TrackStep() {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {/* Fake order card */}
      <div
        style={{
          background: COLORS.terminal,
          borderRadius: 10,
          border: `1px solid ${COLORS.success}33`,
          padding: "16px 18px",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: COLORS.text, fontFamily: FONTS.sans }}>
            Order #MZ-4821
          </span>
          <span
            style={{
              fontSize: 11,
              fontWeight: 600,
              color: COLORS.success,
              fontFamily: FONTS.mono,
              padding: "3px 8px",
              background: `${COLORS.success}15`,
              border: `1px solid ${COLORS.success}33`,
              borderRadius: 4,
            }}
          >
            LIVE
          </span>
        </div>

        {/* Services */}
        {[
          { name: "web", status: "healthy", cpu: "12%", mem: "156MB" },
          { name: "db", status: "healthy", cpu: "8%", mem: "312MB" },
          { name: "cache", status: "healthy", cpu: "3%", mem: "64MB" },
        ].map((svc) => (
          <div
            key={svc.name}
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "6px 0",
              borderTop: `1px solid ${COLORS.border}`,
              fontSize: 12,
              fontFamily: FONTS.mono,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <div
                style={{
                  width: 6,
                  height: 6,
                  borderRadius: "50%",
                  background: COLORS.success,
                }}
              />
              <span style={{ color: COLORS.text }}>{svc.name}</span>
            </div>
            <div style={{ display: "flex", gap: 16 }}>
              <span style={{ color: COLORS.textDim }}>CPU {svc.cpu}</span>
              <span style={{ color: COLORS.textDim }}>MEM {svc.mem}</span>
            </div>
          </div>
        ))}
      </div>

      {/* CTA */}
      <Link
        to="/register"
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          gap: 8,
          padding: "10px 0",
          borderRadius: 8,
          background: COLORS.accent,
          color: "#fff",
          fontSize: 14,
          fontWeight: 600,
          fontFamily: FONTS.sans,
          textDecoration: "none",
          transition: "opacity 0.15s",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.9")}
        onMouseLeave={(e) => (e.currentTarget.style.opacity = "1")}
      >
        Try it for real
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M2 7h10M8 3l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </Link>
    </div>
  );
}

// --- Main Walkthrough Component ---

export function DeployWalkthrough() {
  const [ref, visible] = useScrollReveal();
  const isMobile = useMediaQuery("(max-width: 768px)");
  const [step, setStep] = useState(0);
  const [selectedRecipe, setSelectedRecipe] = useState("");
  const [hasInteracted, setHasInteracted] = useState(false);

  const handlePickRecipe = useCallback((name: string) => {
    setSelectedRecipe(name);
    setStep(1);
    setHasInteracted(true);
  }, []);

  const handleConfigure = useCallback(() => {
    setStep(2);
  }, []);

  const handleDeployComplete = useCallback(() => {
    setStep(3);
  }, []);

  const handleRestart = useCallback(() => {
    setStep(0);
    setSelectedRecipe("");
  }, []);

  const currentStep = STEPS[step]!;

  return (
    <section
      ref={ref}
      style={{
        padding: "100px 24px",
        position: "relative",
        overflow: "hidden",
      }}
    >
      {/* Subtle glow */}
      <div
        style={{
          position: "absolute",
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
          width: 700,
          height: 500,
          background: `radial-gradient(ellipse at center, ${COLORS.accentGlow} 0%, transparent 70%)`,
          pointerEvents: "none",
          opacity: 0.15,
        }}
      />

      <div style={{ maxWidth: 1200, margin: "0 auto", position: "relative", zIndex: 1 }}>
        {/* Section header */}
        <div style={{ textAlign: "center", marginBottom: 56 }}>
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
            Try it <span style={{ color: COLORS.accent }}>right now</span>
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
            Walk through a deploy — no account needed. Click to experience the flow.
          </p>
        </div>

        {/* Walkthrough card */}
        <div
          style={{
            maxWidth: 680,
            margin: "0 auto",
            display: "flex",
            flexDirection: isMobile ? "column" : "row",
            gap: 0,
            background: COLORS.bgSubtle,
            border: `1px solid ${COLORS.border}`,
            borderRadius: 16,
            overflow: "hidden",
            opacity: visible ? 1 : 0,
            transform: visible ? "translateY(0)" : "translateY(24px)",
            transition: "opacity 0.6s ease 0.2s, transform 0.6s ease 0.2s",
          }}
        >
          {/* Left: Step indicators + mascot */}
          <div
            style={{
              width: isMobile ? "100%" : 220,
              flexShrink: 0,
              padding: "28px 24px",
              background: COLORS.bgCard,
              borderRight: isMobile ? "none" : `1px solid ${COLORS.border}`,
              borderBottom: isMobile ? `1px solid ${COLORS.border}` : "none",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              gap: 24,
            }}
          >
            {/* Mascot */}
            <div style={{ position: "relative" }}>
              <MozzaMascot size={80} mood={currentStep.mascotMood} />
              {/* Speech bubble */}
              <div
                style={{
                  position: "absolute",
                  top: -8,
                  right: -100,
                  background: COLORS.terminal,
                  border: `1px solid ${COLORS.border}`,
                  borderRadius: 8,
                  padding: "6px 10px",
                  fontSize: 11,
                  color: COLORS.text,
                  fontFamily: FONTS.sans,
                  fontWeight: 500,
                  whiteSpace: "nowrap",
                  maxWidth: isMobile ? 140 : 120,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}
              >
                {currentStep.mascotSays}
                {/* Arrow */}
                <div
                  style={{
                    position: "absolute",
                    left: -6,
                    top: 12,
                    width: 0,
                    height: 0,
                    borderTop: "6px solid transparent",
                    borderBottom: "6px solid transparent",
                    borderRight: `6px solid ${COLORS.border}`,
                  }}
                />
              </div>
            </div>

            {/* Step indicators */}
            <div style={{ display: "flex", flexDirection: "column", gap: 4, width: "100%" }}>
              {STEPS.map((s, i) => {
                const isActive = i === step;
                const isDone = i < step;
                return (
                  <div
                    key={s.id}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 10,
                      padding: "8px 12px",
                      borderRadius: 8,
                      background: isActive ? COLORS.accentDim : "transparent",
                      transition: "background 0.2s",
                    }}
                  >
                    {/* Step dot */}
                    <div
                      style={{
                        width: 20,
                        height: 20,
                        borderRadius: "50%",
                        border: `2px solid ${isDone ? COLORS.success : isActive ? COLORS.accent : COLORS.border}`,
                        background: isDone ? COLORS.success : "transparent",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                        flexShrink: 0,
                        transition: "all 0.2s",
                      }}
                    >
                      {isDone && (
                        <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                          <path d="M2 5.5L4 7.5L8 3" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                        </svg>
                      )}
                    </div>
                    <span
                      style={{
                        fontSize: 13,
                        fontWeight: isActive ? 600 : 400,
                        color: isActive ? COLORS.accent : isDone ? COLORS.text : COLORS.textDim,
                        fontFamily: FONTS.sans,
                        transition: "color 0.2s",
                      }}
                    >
                      {s.label}
                    </span>
                  </div>
                );
              })}
            </div>

            {/* Restart */}
            {hasInteracted && (
              <button
                onClick={handleRestart}
                style={{
                  fontSize: 12,
                  color: COLORS.textDim,
                  background: "none",
                  border: "none",
                  cursor: "pointer",
                  fontFamily: FONTS.sans,
                  padding: "4px 8px",
                  transition: "color 0.15s",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.color = COLORS.textMuted)}
                onMouseLeave={(e) => (e.currentTarget.style.color = COLORS.textDim)}
              >
                Start over
              </button>
            )}
          </div>

          {/* Right: Step content */}
          <div
            style={{
              flex: 1,
              padding: "28px 24px",
              minHeight: isMobile ? "auto" : 340,
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
            }}
          >
            {/* Step title */}
            <div style={{ marginBottom: 20 }}>
              <div
                style={{
                  fontSize: 11,
                  fontWeight: 700,
                  color: COLORS.accent,
                  fontFamily: FONTS.mono,
                  textTransform: "uppercase",
                  letterSpacing: 1,
                  marginBottom: 6,
                }}
              >
                Step {step + 1} of {STEPS.length}
              </div>
              <h3
                style={{
                  fontSize: 20,
                  fontWeight: 700,
                  color: COLORS.text,
                  margin: 0,
                  fontFamily: FONTS.sans,
                }}
              >
                {currentStep.label}
                {selectedRecipe && step >= 1 && (
                  <span style={{ fontSize: 14, fontWeight: 400, color: COLORS.textMuted, marginLeft: 8 }}>
                    — {selectedRecipe}
                  </span>
                )}
              </h3>
            </div>

            {/* Step body */}
            {step === 0 && <PickStep onPick={handlePickRecipe} />}
            {step === 1 && <ConfigureStep recipeName={selectedRecipe} onContinue={handleConfigure} />}
            {step === 2 && <DeployStep onComplete={handleDeployComplete} />}
            {step === 3 && <TrackStep />}
          </div>
        </div>
      </div>

      {/* Blink animation for deploy cursor */}
      <style>{`@keyframes blink { 50% { opacity: 0; } }`}</style>
    </section>
  );
}
