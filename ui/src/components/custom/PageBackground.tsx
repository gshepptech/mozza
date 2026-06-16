import { useLocation } from "react-router-dom";

// Subtle SVG background illustrations per page section.
// Rendered as a fixed overlay behind page content — purely decorative.

const ACCENT = "rgba(255, 107, 53, 0.04)";
const ACCENT_MED = "rgba(255, 107, 53, 0.06)";
const BLUE = "rgba(96, 165, 250, 0.03)";
const GREEN = "rgba(74, 222, 128, 0.03)";

function getSceneForPath(path: string): React.ReactNode {
  // Deploy pages — kitchen/oven warmth
  if (path.includes("/deploy")) {
    return (
      <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.6">
        {/* Warm radial from top-right */}
        <circle cx="1100" cy="100" r="400" fill={ACCENT_MED} />
        <circle cx="1050" cy="150" r="200" fill={ACCENT} />
        {/* Oven arch hint */}
        <path d="M950 600Q1050 400 1150 600" stroke="rgba(255,107,53,0.05)" strokeWidth="1.5" fill="none" />
        <path d="M970 590Q1050 420 1130 590" stroke="rgba(255,107,53,0.03)" strokeWidth="1" fill="none" />
        {/* Steam wisps */}
        <path d="M1000 350Q1010 320 1000 290Q990 260 1000 230" stroke="rgba(255,255,255,0.03)" strokeWidth="2" strokeLinecap="round" fill="none" />
        <path d="M1040 360Q1050 330 1040 300Q1030 270 1040 240" stroke="rgba(255,255,255,0.02)" strokeWidth="2" strokeLinecap="round" fill="none" />
      </svg>
    );
  }

  // Monitoring — heartbeat/pulse
  if (path.includes("/monitoring")) {
    return (
      <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.5">
        <circle cx="1100" cy="150" r="300" fill={GREEN} />
        {/* Pulse line */}
        <polyline
          points="600,400 700,400 720,400 740,350 760,450 780,380 800,420 820,400 900,400 1000,400 1050,400 1070,350 1090,450 1110,380 1130,420 1150,400 1200,400"
          stroke="rgba(74,222,128,0.06)"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          fill="none"
        />
      </svg>
    );
  }

  // Environments — layers/clouds
  if (path.includes("/environments")) {
    return (
      <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.5">
        <circle cx="1050" cy="200" r="350" fill={BLUE} />
        {/* Stacked layers */}
        <ellipse cx="1050" cy="350" rx="180" ry="30" stroke="rgba(96,165,250,0.05)" strokeWidth="1.5" fill="none" />
        <ellipse cx="1050" cy="310" rx="160" ry="28" stroke="rgba(96,165,250,0.04)" strokeWidth="1.5" fill="none" />
        <ellipse cx="1050" cy="270" rx="140" ry="26" stroke="rgba(96,165,250,0.03)" strokeWidth="1.5" fill="none" />
      </svg>
    );
  }

  // Recipes / Marketplace — recipe cards floating
  if (path.includes("/recipes") || path.includes("/marketplace")) {
    return (
      <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.5">
        <circle cx="1100" cy="120" r="250" fill={ACCENT} />
        {/* Floating recipe card outlines */}
        <rect x="980" y="200" width="120" height="80" rx="8" stroke="rgba(255,107,53,0.04)" strokeWidth="1" />
        <rect x="1030" y="160" width="120" height="80" rx="8" stroke="rgba(255,107,53,0.03)" strokeWidth="1" transform="rotate(5, 1090, 200)" />
        <rect x="1060" y="300" width="100" height="70" rx="8" stroke="rgba(255,107,53,0.03)" strokeWidth="1" transform="rotate(-3, 1110, 335)" />
      </svg>
    );
  }

  // Doctor — stethoscope/heartbeat
  if (path.includes("/doctor")) {
    return (
      <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.5">
        <circle cx="1080" cy="180" r="280" fill={GREEN} />
        {/* Cross shape */}
        <rect x="1050" y="250" width="60" height="140" rx="6" fill="rgba(74,222,128,0.03)" />
        <rect x="1010" y="290" width="140" height="60" rx="6" fill="rgba(74,222,128,0.03)" />
      </svg>
    );
  }

  // Overview / default — warm corner glow
  return (
    <svg width="100%" height="100%" viewBox="0 0 1200 800" preserveAspectRatio="xMaxYMax slice" fill="none" opacity="0.5">
      <circle cx="1100" cy="100" r="350" fill={ACCENT} />
      <circle cx="100" cy="700" r="250" fill={BLUE} />
    </svg>
  );
}

export function PageBackground() {
  const { pathname } = useLocation();

  return (
    <div
      style={{
        position: "absolute",
        inset: 0,
        overflow: "hidden",
        pointerEvents: "none",
        zIndex: 0,
      }}
    >
      {getSceneForPath(pathname)}
    </div>
  );
}
