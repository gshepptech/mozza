import { useState } from "react";
import { Link } from "react-router-dom";
import { COLORS, FONTS, useScrollReveal, useMediaQuery } from "@/pages/LandingPage";

// --- Recipe Card SVG Icons ---

function WebAppIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <rect x="3" y="5" width="26" height="18" rx="3" stroke={COLORS.accent} strokeWidth="1.5" />
      <line x1="3" y1="11" x2="29" y2="11" stroke={COLORS.accent} strokeWidth="1" opacity="0.4" />
      <circle cx="7" cy="8" r="1" fill={COLORS.accent} opacity="0.5" />
      <circle cx="10.5" cy="8" r="1" fill={COLORS.accent} opacity="0.5" />
      <rect x="7" y="14" width="8" height="5" rx="1" fill={COLORS.accent} opacity="0.15" />
      <line x1="18" y1="15" x2="25" y2="15" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
      <line x1="18" y1="18" x2="23" y2="18" stroke={COLORS.accent} strokeWidth="1" opacity="0.2" />
      <line x1="12" y1="23" x2="20" y2="23" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.3" />
      <line x1="14" y1="26" x2="18" y2="26" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" opacity="0.2" />
    </svg>
  );
}

function ApiIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <path d="M8 8h16v16H8z" stroke={COLORS.accent} strokeWidth="1.5" strokeDasharray="3 2" opacity="0.3" />
      <circle cx="16" cy="16" r="6" stroke={COLORS.accent} strokeWidth="1.5" />
      <path d="M13 16h6M16 13v6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="6" cy="6" r="2" fill={COLORS.accent} opacity="0.4" />
      <circle cx="26" cy="6" r="2" fill={COLORS.accent} opacity="0.4" />
      <circle cx="6" cy="26" r="2" fill={COLORS.accent} opacity="0.4" />
      <circle cx="26" cy="26" r="2" fill={COLORS.accent} opacity="0.4" />
      <line x1="8" y1="7" x2="12" y2="12" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
      <line x1="24" y1="7" x2="20" y2="12" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
    </svg>
  );
}

function DatabaseIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <ellipse cx="16" cy="8" rx="10" ry="4" stroke={COLORS.accent} strokeWidth="1.5" />
      <path d="M6 8v8c0 2.2 4.5 4 10 4s10-1.8 10-4V8" stroke={COLORS.accent} strokeWidth="1.5" />
      <path d="M6 16v8c0 2.2 4.5 4 10 4s10-1.8 10-4v-8" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.5" />
      <ellipse cx="16" cy="16" rx="10" ry="4" stroke={COLORS.accent} strokeWidth="1" opacity="0.2" />
    </svg>
  );
}

function WorkerIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <circle cx="16" cy="16" r="10" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.4" />
      <circle cx="16" cy="16" r="5" stroke={COLORS.accent} strokeWidth="1.5" />
      <circle cx="16" cy="16" r="2" fill={COLORS.accent} />
      <line x1="16" y1="2" x2="16" y2="6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
      <line x1="16" y1="26" x2="16" y2="30" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
      <line x1="2" y1="16" x2="6" y2="16" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
      <line x1="26" y1="16" x2="30" y2="16" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function StaticSiteIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <rect x="4" y="6" width="24" height="20" rx="2" stroke={COLORS.accent} strokeWidth="1.5" />
      <line x1="4" y1="12" x2="28" y2="12" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
      <rect x="8" y="15" width="7" height="7" rx="1" fill={COLORS.accent} opacity="0.15" />
      <line x1="19" y1="16" x2="25" y2="16" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
      <line x1="19" y1="19" x2="24" y2="19" stroke={COLORS.accent} strokeWidth="1" opacity="0.2" />
      <line x1="19" y1="22" x2="22" y2="22" stroke={COLORS.accent} strokeWidth="1" opacity="0.15" />
    </svg>
  );
}

function CronIcon() {
  return (
    <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
      <circle cx="16" cy="16" r="12" stroke={COLORS.accent} strokeWidth="1.5" />
      <line x1="16" y1="8" x2="16" y2="16" stroke={COLORS.accent} strokeWidth="2" strokeLinecap="round" />
      <line x1="16" y1="16" x2="22" y2="20" stroke={COLORS.accent} strokeWidth="2" strokeLinecap="round" />
      <circle cx="16" cy="16" r="1.5" fill={COLORS.accent} />
      {[0, 30, 60, 90, 120, 150, 180, 210, 240, 270, 300, 330].map((deg) => {
        const rad = (deg * Math.PI) / 180;
        const x = 16 + 11 * Math.sin(rad);
        const y = 16 - 11 * Math.cos(rad);
        return <circle key={deg} cx={x} cy={y} r="0.8" fill={COLORS.accent} opacity="0.3" />;
      })}
    </svg>
  );
}

// --- Recipe Data ---

const RECIPES = [
  { icon: WebAppIcon, name: "Next.js App", stack: "Node.js + PostgreSQL", slices: 3 },
  { icon: ApiIcon, name: "REST API", stack: "Go + Redis + Postgres", slices: 4 },
  { icon: DatabaseIcon, name: "Data Pipeline", stack: "Python + Kafka + ClickHouse", slices: 5 },
  { icon: WorkerIcon, name: "Background Worker", stack: "Node.js + Redis + S3", slices: 3 },
  { icon: StaticSiteIcon, name: "Static Site", stack: "Vite + CDN", slices: 1 },
  { icon: CronIcon, name: "Cron Service", stack: "Go + PostgreSQL", slices: 2 },
];

// --- Recipe Card ---

function RecipeCard({
  recipe,
  index,
  visible,
}: {
  recipe: (typeof RECIPES)[0];
  index: number;
  visible: boolean;
}) {
  const [hovered, setHovered] = useState(false);
  const delay = 0.1 + index * 0.06;

  return (
    <div
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        background: hovered ? COLORS.bgCard : COLORS.terminal,
        border: `1px solid ${hovered ? COLORS.borderHover : COLORS.border}`,
        borderRadius: 12,
        padding: 24,
        cursor: "pointer",
        transition: "all 0.2s ease",
        opacity: visible ? 1 : 0,
        transform: visible ? "translateY(0)" : "translateY(20px)",
        transitionProperty: "opacity, transform, background, border-color",
        transitionDuration: "0.5s, 0.5s, 0.2s, 0.2s",
        transitionDelay: `${delay}s, ${delay}s, 0s, 0s`,
      }}
    >
      <div style={{ marginBottom: 16 }}>
        <recipe.icon />
      </div>
      <h4
        style={{
          fontSize: 16,
          fontWeight: 600,
          color: COLORS.text,
          margin: "0 0 6px 0",
          fontFamily: FONTS.sans,
        }}
      >
        {recipe.name}
      </h4>
      <p
        style={{
          fontSize: 13,
          color: COLORS.textMuted,
          margin: "0 0 14px 0",
          fontFamily: FONTS.sans,
        }}
      >
        {recipe.stack}
      </p>
      <div
        style={{
          fontSize: 12,
          color: COLORS.textDim,
          fontFamily: FONTS.mono,
          display: "flex",
          alignItems: "center",
          gap: 6,
        }}
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
          <rect x="1" y="1" width="5" height="5" rx="1" stroke={COLORS.accent} strokeWidth="1" opacity="0.5" />
          <rect x="8" y="1" width="5" height="5" rx="1" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
          <rect x="1" y="8" width="5" height="5" rx="1" stroke={COLORS.accent} strokeWidth="1" opacity="0.3" />
        </svg>
        {recipe.slices} {recipe.slices === 1 ? "slice" : "slices"}
      </div>
    </div>
  );
}

// --- Section ---

export function MarketplacePreview() {
  const [ref, visible] = useScrollReveal();
  const isMobile = useMediaQuery("(max-width: 768px)");

  return (
    <section ref={ref} style={{ padding: "100px 24px" }}>
      <div style={{ maxWidth: 1200, margin: "0 auto" }}>
        {/* Header */}
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
            Start with a{" "}
            <span style={{ color: COLORS.accent }}>recipe</span>
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
            Pre-built recipes for common stacks. Clone, customize, deploy.
          </p>
        </div>

        {/* Recipe grid */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: isMobile ? "1fr" : "repeat(3, 1fr)",
            gap: 16,
            marginBottom: 40,
          }}
        >
          {RECIPES.map((recipe, i) => (
            <RecipeCard key={i} recipe={recipe} index={i} visible={visible} />
          ))}
        </div>

        {/* Browse all link */}
        <div
          style={{
            textAlign: "center",
            opacity: visible ? 1 : 0,
            transition: "opacity 0.6s ease 0.5s",
          }}
        >
          <Link
            to="/app/recipes"
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 8,
              color: COLORS.accent,
              textDecoration: "none",
              fontSize: 15,
              fontWeight: 600,
              fontFamily: FONTS.sans,
              padding: "10px 24px",
              borderRadius: 8,
              border: `1px solid ${COLORS.accent}33`,
              background: COLORS.accentDim,
              transition: "background 0.2s",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = `${COLORS.accent}22`)}
            onMouseLeave={(e) => (e.currentTarget.style.background = COLORS.accentDim)}
          >
            Browse All Recipes
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path d="M3 8h10M9 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </Link>
        </div>
      </div>
    </section>
  );
}
