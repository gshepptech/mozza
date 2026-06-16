import { useState, useEffect, useRef } from "react";
import { Link } from "react-router-dom";
import { HeroSection } from "@/components/landing/HeroSection";
import { HowItWorksSection } from "@/components/landing/HowItWorksSection";
import { FeatureShowcase } from "@/components/landing/FeatureShowcase";
import { RecipeComparison } from "@/components/landing/RecipeComparison";
import { DeployWalkthrough } from "@/components/landing/DeployWalkthrough";
import { MarketplacePreview } from "@/components/landing/MarketplacePreview";
import { CommunitySection } from "@/components/landing/CommunitySection";
import { InstallCTA } from "@/components/landing/InstallCTA";
import { MozzaMascotInline } from "@/components/landing/MozzaMascot";

// --- Design Tokens ---

export const COLORS = {
  bg: "#0a0a0a",
  bgSubtle: "#111111",
  bgCard: "#151515",
  border: "#222222",
  borderHover: "#333333",
  text: "#e0e0e0",
  textMuted: "#888888",
  textDim: "#666666",
  accent: "#ff6b35",
  accentDim: "rgba(255, 107, 53, 0.15)",
  accentGlow: "rgba(255, 107, 53, 0.3)",
  success: "#3fb950",
  terminal: "#0d1117",
} as const;

// Inline styles are intentional: LandingPage is a standalone marketing page
// that does not use Tailwind, so all styling is done via inline style objects.
export const FONTS = {
  sans: '-apple-system, BlinkMacSystemFont, "Inter", "Segoe UI", Roboto, Helvetica, Arial, sans-serif',
  mono: '"Geist Mono", "JetBrains Mono", "SF Mono", "Fira Code", monospace',
} as const;

// --- Scroll Animation Hook ---

export function useScrollReveal(): [React.RefObject<HTMLDivElement | null>, boolean] {
  const ref = useRef<HTMLDivElement | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const observer = new IntersectionObserver(
      ([entry]: IntersectionObserverEntry[]) => {
        if (entry?.isIntersecting) {
          setVisible(true);
          observer.disconnect();
        }
      },
      { threshold: 0.15 }
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return [ref, visible];
}

// --- Responsive Hook ---

export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia(query).matches;
  });

  useEffect(() => {
    const mql = window.matchMedia(query);
    const handler = (e: MediaQueryListEvent) => setMatches(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, [query]);

  return matches;
}

// --- Nav ---

function Nav() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 40);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <nav
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        zIndex: 100,
        padding: "0 24px",
        height: 64,
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        background: scrolled ? "rgba(10, 10, 10, 0.92)" : "transparent",
        backdropFilter: scrolled ? "blur(12px)" : "none",
        borderBottom: scrolled ? `1px solid ${COLORS.border}` : "1px solid transparent",
        transition: "all 0.3s ease",
        fontFamily: FONTS.sans,
      }}
    >
      <Link to="/" style={{ textDecoration: "none", display: "flex", alignItems: "center", gap: 10 }}>
        <MozzaMascotInline size={28} />
        <span style={{ fontSize: 20, fontWeight: 700, color: COLORS.text, letterSpacing: -0.5 }}>
          Mozza
        </span>
      </Link>

      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <Link
          to="/login"
          style={{
            color: COLORS.textMuted,
            textDecoration: "none",
            fontSize: 14,
            fontWeight: 500,
            padding: "8px 16px",
            borderRadius: 8,
            transition: "color 0.2s",
          }}
          onMouseEnter={(e) => (e.currentTarget.style.color = COLORS.text)}
          onMouseLeave={(e) => (e.currentTarget.style.color = COLORS.textMuted)}
        >
          Sign in
        </Link>
        <Link
          to="/register"
          style={{
            color: "#fff",
            textDecoration: "none",
            fontSize: 14,
            fontWeight: 600,
            padding: "8px 20px",
            borderRadius: 8,
            background: COLORS.accent,
            transition: "opacity 0.2s",
          }}
          onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.9")}
          onMouseLeave={(e) => (e.currentTarget.style.opacity = "1")}
        >
          Get Started
        </Link>
      </div>
    </nav>
  );
}

// --- Footer ---

function Footer() {
  return (
    <footer
      style={{
        padding: "48px 24px 32px",
        borderTop: `1px solid ${COLORS.border}`,
        fontFamily: FONTS.sans,
      }}
    >
      <div
        style={{
          maxWidth: 1200,
          margin: "0 auto",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 16,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <MozzaMascotInline size={20} />
          <span style={{ fontSize: 14, color: COLORS.textMuted }}>
            Mozza — Deploy like you order pizza.
          </span>
        </div>
        <div style={{ fontSize: 13, color: COLORS.textDim }}>
          Built by gshepptech
        </div>
      </div>
    </footer>
  );
}

// --- Main Page ---

export default function LandingPage() {
  return (
    <div
      style={{
        minHeight: "100vh",
        background: COLORS.bg,
        color: COLORS.text,
        fontFamily: FONTS.sans,
        overflowX: "hidden",
      }}
    >
      <Nav />
      <main>
        <HeroSection />
        <HowItWorksSection />
        <FeatureShowcase />
        <RecipeComparison />
        <DeployWalkthrough />
        <MarketplacePreview />
        <CommunitySection />
        <InstallCTA />
      </main>
      <Footer />
    </div>
  );
}
