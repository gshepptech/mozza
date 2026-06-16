import { COLORS } from "@/pages/LandingPage";

// --- Mozza Mascot: "Mozzi" the Mozzarella Ball ---
// A soft, round, friendly mozzarella ball with a chef hat.
// Smooth white body with creamy highlights, expressive face, little arms/legs.

interface MozzaMascotProps {
  size?: number;
  mood?: "happy" | "wink" | "thinking" | "celebrate";
  style?: React.CSSProperties;
}

let _idCounter = 0;
function useUniqueId() {
  return `mz${++_idCounter}`;
}

export function MozzaMascot({ size = 120, mood = "wink", style }: MozzaMascotProps) {
  const id = useUniqueId();

  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      fill="none"
      style={{ display: "block", ...style }}
    >
      <defs>
        {/* Body gradient — creamy white mozzarella */}
        <radialGradient id={`${id}-body`} cx="0.45" cy="0.38" r="0.55">
          <stop offset="0%" stopColor="#fffef8" />
          <stop offset="50%" stopColor="#f5f0e8" />
          <stop offset="100%" stopColor="#e8ddd0" />
        </radialGradient>
        {/* Body shadow — subtle warm undertone */}
        <radialGradient id={`${id}-bodyShadow`} cx="0.5" cy="0.7" r="0.5">
          <stop offset="0%" stopColor="#ddd4c4" stopOpacity="0.5" />
          <stop offset="100%" stopColor="#ddd4c4" stopOpacity="0" />
        </radialGradient>
        {/* Chef hat */}
        <linearGradient id={`${id}-hat`} x1="60" y1="5" x2="60" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#ffffff" />
          <stop offset="100%" stopColor="#f0ede8" />
        </linearGradient>
        <linearGradient id={`${id}-hatBand`} x1="40" y1="30" x2="80" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#e0dbd2" />
          <stop offset="50%" stopColor="#d4cec4" />
          <stop offset="100%" stopColor="#e0dbd2" />
        </linearGradient>
        {/* Drop shadow */}
        <filter id={`${id}-shadow`} x="-15%" y="-10%" width="140%" height="140%">
          <feDropShadow dx="0" dy="4" stdDeviation="5" floodColor="#000" floodOpacity="0.12" />
        </filter>
        {/* Soft body glow */}
        <filter id={`${id}-glow`} x="-10%" y="-10%" width="120%" height="120%">
          <feGaussianBlur in="SourceGraphic" stdDeviation="1" />
        </filter>
        {/* Cheek blush */}
        <radialGradient id={`${id}-blush`} cx="0.5" cy="0.5" r="0.5">
          <stop offset="0%" stopColor={COLORS.accent} stopOpacity="0.2" />
          <stop offset="100%" stopColor={COLORS.accent} stopOpacity="0" />
        </radialGradient>
      </defs>

      {/* === GROUND SHADOW === */}
      <ellipse cx="60" cy="110" rx="28" ry="5" fill="#000" opacity="0.06" />

      {/* === LEGS === */}
      <g>
        {/* Left leg */}
        <ellipse cx="47" cy="96" rx="8" ry="10" fill="#f5f0e8" />
        <ellipse cx="47" cy="96" rx="8" ry="10" fill="none" stroke="#ddd4c4" strokeWidth="0.8" />
        {/* Left foot */}
        <ellipse cx="45" cy="104" rx="10" ry="5" fill="#f0ebe2" />
        <ellipse cx="45" cy="104" rx="10" ry="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
        {/* Foot highlight */}
        <ellipse cx="43" cy="102" rx="4" ry="2" fill="#fff" opacity="0.3" />

        {/* Right leg */}
        <ellipse cx="73" cy="96" rx="8" ry="10" fill="#f5f0e8" />
        <ellipse cx="73" cy="96" rx="8" ry="10" fill="none" stroke="#ddd4c4" strokeWidth="0.8" />
        {/* Right foot */}
        <ellipse cx="75" cy="104" rx="10" ry="5" fill="#f0ebe2" />
        <ellipse cx="75" cy="104" rx="10" ry="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
        <ellipse cx="73" cy="102" rx="4" ry="2" fill="#fff" opacity="0.3" />
      </g>

      {/* === MAIN BODY === */}
      <g filter={`url(#${id}-shadow)`}>
        {/* Body — large soft circle, slightly squished */}
        <ellipse cx="60" cy="62" rx="34" ry="32" fill={`url(#${id}-body)`} />
        {/* Warm shadow on bottom */}
        <ellipse cx="60" cy="62" rx="34" ry="32" fill={`url(#${id}-bodyShadow)`} />
        {/* Subtle outline */}
        <ellipse cx="60" cy="62" rx="34" ry="32" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
      </g>

      {/* Body highlight — top-left shine */}
      <ellipse cx="48" cy="46" rx="14" ry="10" fill="#fff" opacity="0.35" transform="rotate(-15, 48, 46)" />
      {/* Secondary smaller highlight */}
      <ellipse cx="42" cy="42" rx="6" ry="4" fill="#fff" opacity="0.2" transform="rotate(-20, 42, 42)" />

      {/* Mozzarella texture — soft organic streaks */}
      <path d="M38 56Q42 54 46 56" stroke="#e8e0d4" strokeWidth="1" strokeLinecap="round" fill="none" opacity="0.5" />
      <path d="M64 72Q70 70 76 73" stroke="#e8e0d4" strokeWidth="0.8" strokeLinecap="round" fill="none" opacity="0.4" />
      <path d="M44 76Q50 74 54 76" stroke="#e8e0d4" strokeWidth="0.8" strokeLinecap="round" fill="none" opacity="0.3" />

      {/* === ARMS === */}
      {mood === "celebrate" ? (
        <>
          {/* Left arm — raised up */}
          <path d="M28 58Q16 46 12 36" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M28 58Q16 46 12 36" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          {/* Left hand */}
          <circle cx="11" cy="34" r="5" fill="#f5f0e8" />
          <circle cx="11" cy="34" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
          <ellipse cx="9" cy="32" rx="2" ry="1.2" fill="#fff" opacity="0.3" />

          {/* Right arm — raised up */}
          <path d="M92 58Q104 46 108 36" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M92 58Q104 46 108 36" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          {/* Right hand */}
          <circle cx="109" cy="34" r="5" fill="#f5f0e8" />
          <circle cx="109" cy="34" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
          <ellipse cx="107" cy="32" rx="2" ry="1.2" fill="#fff" opacity="0.3" />

          {/* Sparkles */}
          <g opacity="0.7">
            <path d="M5 28V22M2 25h6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
            <path d="M115 28V22M112 25h6" stroke={COLORS.accent} strokeWidth="1.5" strokeLinecap="round" />
            <circle cx="8" cy="20" r="1" fill={COLORS.accent} opacity="0.5" />
            <circle cx="114" cy="20" r="1" fill={COLORS.accent} opacity="0.5" />
          </g>
        </>
      ) : mood === "wink" ? (
        <>
          {/* Left arm — resting */}
          <path d="M28 64Q16 68 10 66" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M28 64Q16 68 10 66" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="8" cy="65" r="5" fill="#f5f0e8" />
          <circle cx="8" cy="65" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />

          {/* Right arm — waving */}
          <path d="M92 58Q104 50 110 42" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M92 58Q104 50 110 42" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="111" cy="40" r="5" fill="#f5f0e8" />
          <circle cx="111" cy="40" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
          <ellipse cx="109" cy="38" rx="2" ry="1.2" fill="#fff" opacity="0.3" />
        </>
      ) : mood === "thinking" ? (
        <>
          {/* Left arm — resting */}
          <path d="M28 64Q16 68 10 66" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M28 64Q16 68 10 66" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="8" cy="65" r="5" fill="#f5f0e8" />
          <circle cx="8" cy="65" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />

          {/* Right arm — hand on chin thinking */}
          <path d="M92 62Q98 56 96 48" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M92 62Q98 56 96 48" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="96" cy="46" r="5" fill="#f5f0e8" />
          <circle cx="96" cy="46" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
        </>
      ) : (
        <>
          {/* Both arms — friendly open pose */}
          <path d="M28 62Q14 60 8 64" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M28 62Q14 60 8 64" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="6" cy="63" r="5" fill="#f5f0e8" />
          <circle cx="6" cy="63" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
          <ellipse cx="4" cy="61" rx="2" ry="1.2" fill="#fff" opacity="0.3" />

          <path d="M92 62Q106 60 112 64" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
          <path d="M92 62Q106 60 112 64" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
          <circle cx="114" cy="63" r="5" fill="#f5f0e8" />
          <circle cx="114" cy="63" r="5" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
          <ellipse cx="112" cy="61" rx="2" ry="1.2" fill="#fff" opacity="0.3" />
        </>
      )}

      {/* === FACE === */}

      {/* Eyes */}
      {mood === "wink" ? (
        <>
          {/* Left eye — large, round, expressive */}
          <ellipse cx="48" cy="58" rx="5.5" ry="6" fill="#1a1208" />
          <ellipse cx="48" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
          <circle cx="49.5" cy="56" r="2.2" fill="#fff" opacity="0.9" />
          <circle cx="46.5" cy="59.5" r="1" fill="#fff" opacity="0.35" />
          {/* Right eye — wink */}
          <path d="M66 58Q71 62.5 76 58" stroke="#1a1208" strokeWidth="2.8" strokeLinecap="round" fill="none" />
          <path d="M67 57L65 54.5" stroke="#1a1208" strokeWidth="1.3" strokeLinecap="round" />
          <path d="M75 57L77 54.5" stroke="#1a1208" strokeWidth="1.3" strokeLinecap="round" />
        </>
      ) : mood === "thinking" ? (
        <>
          {/* Both eyes looking up-right */}
          <ellipse cx="48" cy="58" rx="5.5" ry="6" fill="#1a1208" />
          <ellipse cx="48" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
          <circle cx="50" cy="55.5" r="2.2" fill="#fff" opacity="0.9" />
          <circle cx="47" cy="58.5" r="1" fill="#fff" opacity="0.35" />
          <ellipse cx="72" cy="58" rx="5.5" ry="6" fill="#1a1208" />
          <ellipse cx="72" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
          <circle cx="74" cy="55.5" r="2.2" fill="#fff" opacity="0.9" />
          <circle cx="71" cy="58.5" r="1" fill="#fff" opacity="0.35" />
          {/* Raised eyebrow */}
          <path d="M66 50Q71 47 77 49" stroke="#1a1208" strokeWidth="1.5" strokeLinecap="round" fill="none" />
        </>
      ) : mood === "celebrate" ? (
        <>
          {/* Joyful closed eyes — happy arcs */}
          <path d="M43 58Q48 52.5 53 58" stroke="#1a1208" strokeWidth="2.8" strokeLinecap="round" fill="none" />
          <path d="M67 58Q72 52.5 77 58" stroke="#1a1208" strokeWidth="2.8" strokeLinecap="round" fill="none" />
          {/* Tiny lash accents */}
          <path d="M43 56L41.5 54" stroke="#1a1208" strokeWidth="1" strokeLinecap="round" />
          <path d="M53 56L54.5 54" stroke="#1a1208" strokeWidth="1" strokeLinecap="round" />
          <path d="M67 56L65.5 54" stroke="#1a1208" strokeWidth="1" strokeLinecap="round" />
          <path d="M77 56L78.5 54" stroke="#1a1208" strokeWidth="1" strokeLinecap="round" />
        </>
      ) : (
        <>
          {/* Default happy — both eyes open */}
          <ellipse cx="48" cy="58" rx="5.5" ry="6" fill="#1a1208" />
          <ellipse cx="48" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
          <circle cx="49.5" cy="56" r="2.2" fill="#fff" opacity="0.9" />
          <circle cx="46.5" cy="59.5" r="1" fill="#fff" opacity="0.35" />
          <ellipse cx="72" cy="58" rx="5.5" ry="6" fill="#1a1208" />
          <ellipse cx="72" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
          <circle cx="73.5" cy="56" r="2.2" fill="#fff" opacity="0.9" />
          <circle cx="70.5" cy="59.5" r="1" fill="#fff" opacity="0.35" />
        </>
      )}

      {/* Mouth */}
      {mood === "celebrate" ? (
        <g>
          {/* Big open joyful grin */}
          <path d="M50 68Q60 79 70 68" stroke="#1a1208" strokeWidth="2.2" strokeLinecap="round" fill="none" />
          <path d="M52 69Q60 77 68 69" fill="#c44020" opacity="0.45" />
          {/* Tongue hint */}
          <ellipse cx="60" cy="74" rx="4" ry="2.5" fill="#e06848" opacity="0.4" />
        </g>
      ) : mood === "thinking" ? (
        <g>
          {/* Small "hmm" mouth */}
          <ellipse cx="60" cy="70" rx="3.5" ry="4" fill="#1a1208" opacity="0.6" />
          <ellipse cx="60" cy="69.5" rx="2.2" ry="2.5" fill="#3a2a18" opacity="0.4" />
        </g>
      ) : (
        <g>
          {/* Friendly smile */}
          <path d="M50 67Q60 76 70 67" stroke="#1a1208" strokeWidth="2.2" strokeLinecap="round" fill="none" />
        </g>
      )}

      {/* Rosy cheeks */}
      <ellipse cx="38" cy="66" rx="6" ry="4.5" fill={`url(#${id}-blush)`} />
      <ellipse cx="82" cy="66" rx="6" ry="4.5" fill={`url(#${id}-blush)`} />

      {/* === CHEF HAT === */}
      <g>
        {/* Hat band — sits on head */}
        <ellipse cx="60" cy="34" rx="22" ry="6" fill={`url(#${id}-hatBand)`} />
        <ellipse cx="60" cy="34" rx="22" ry="6" fill="none" stroke="#c8c2b8" strokeWidth="0.8" />

        {/* Hat body — three tall puffy lobes */}
        <path
          d="M40 34Q35 18 46 12Q50 3 60 6Q70 3 74 12Q85 18 80 34Z"
          fill={`url(#${id}-hat)`}
        />
        <path
          d="M40 34Q35 18 46 12Q50 3 60 6Q70 3 74 12Q85 18 80 34"
          fill="none"
          stroke="#d8d4cc"
          strokeWidth="0.8"
        />

        {/* Hat fold lines — adds dimension */}
        <path d="M48 14Q54 10 60 12" stroke="#e8e4dc" strokeWidth="1" strokeLinecap="round" fill="none" opacity="0.5" />
        <path d="M60 12Q66 10 72 14" stroke="#e8e4dc" strokeWidth="1" strokeLinecap="round" fill="none" opacity="0.4" />

        {/* Hat shine — left lobe */}
        <path d="M43 22Q46 14 52 13" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" opacity="0.5" />
        {/* Hat shine — right lobe */}
        <path d="M68 13Q74 14 77 22" fill="none" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" opacity="0.35" />

        {/* Hat band accent stripe */}
        <path d="M42 36Q60 40 78 36" fill="none" stroke={COLORS.accent} strokeWidth="1.5" opacity="0.35" />
      </g>

      {/* === TINY MOZZARELLA STRETCH DETAIL === */}
      {/* A little cheese string hanging off one side — adds character */}
      <path
        d="M86 68Q90 72 88 80Q87 84 85 82Q86 78 84 74"
        stroke="#e8ddd0"
        strokeWidth="2"
        strokeLinecap="round"
        fill="none"
        opacity="0.5"
      />
      <circle cx="85" cy="82" r="2" fill="#ede6dc" opacity="0.5" />
    </svg>
  );
}

// --- Small inline mascot for nav/footer ---
export function MozzaMascotInline({ size = 28 }: { size?: number }) {
  const id = useUniqueId();

  return (
    <svg width={size} height={size} viewBox="0 0 32 32" fill="none">
      <defs>
        <radialGradient id={`${id}-b`} cx="0.45" cy="0.4" r="0.55">
          <stop offset="0%" stopColor="#fffef8" />
          <stop offset="50%" stopColor="#f5f0e8" />
          <stop offset="100%" stopColor="#e8ddd0" />
        </radialGradient>
        <linearGradient id={`${id}-h`} x1="16" y1="2" x2="16" y2="12" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#ffffff" />
          <stop offset="100%" stopColor="#f0ede8" />
        </linearGradient>
        <radialGradient id={`${id}-bl`} cx="0.5" cy="0.5" r="0.5">
          <stop offset="0%" stopColor={COLORS.accent} stopOpacity="0.18" />
          <stop offset="100%" stopColor={COLORS.accent} stopOpacity="0" />
        </radialGradient>
      </defs>

      {/* Ground shadow */}
      <ellipse cx="16" cy="29" rx="8" ry="1.5" fill="#000" opacity="0.05" />

      {/* Feet */}
      <ellipse cx="12.5" cy="27" rx="3.5" ry="1.8" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.5" />
      <ellipse cx="19.5" cy="27" rx="3.5" ry="1.8" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.5" />

      {/* Body */}
      <ellipse cx="16" cy="18" rx="10" ry="9.5" fill={`url(#${id}-b)`} />
      <ellipse cx="16" cy="18" rx="10" ry="9.5" fill="none" stroke="#d8d0c4" strokeWidth="0.5" />
      {/* Highlight */}
      <ellipse cx="13" cy="14" rx="4" ry="3" fill="#fff" opacity="0.3" transform="rotate(-15, 13, 14)" />

      {/* Eyes */}
      <ellipse cx="13" cy="17.5" rx="1.8" ry="2" fill="#1a1208" />
      <circle cx="13.5" cy="16.8" r="0.7" fill="#fff" opacity="0.85" />
      <ellipse cx="19.5" cy="17.5" rx="1.8" ry="2" fill="#1a1208" />
      <circle cx="20" cy="16.8" r="0.7" fill="#fff" opacity="0.85" />

      {/* Smile */}
      <path d="M13.5 20.5Q16 23 18.5 20.5" stroke="#1a1208" strokeWidth="0.9" strokeLinecap="round" fill="none" />

      {/* Cheeks */}
      <ellipse cx="10.5" cy="20" rx="2" ry="1.5" fill={`url(#${id}-bl)`} />
      <ellipse cx="21.5" cy="20" rx="2" ry="1.5" fill={`url(#${id}-bl)`} />

      {/* Chef hat */}
      <ellipse cx="16" cy="10" rx="7" ry="2.2" fill="#e8e4dc" stroke="#d0ccc4" strokeWidth="0.4" />
      <path d="M10 10Q8 5 12 4Q14 1 16 3Q18 1 20 4Q24 5 22 10Z" fill={`url(#${id}-h)`} />
      <path d="M10 10Q8 5 12 4Q14 1 16 3Q18 1 20 4Q24 5 22 10" fill="none" stroke="#d8d4cc" strokeWidth="0.4" />
      {/* Hat shine */}
      <path d="M11.5 7Q13 4.5 15 4" fill="none" stroke="#fff" strokeWidth="0.8" strokeLinecap="round" opacity="0.5" />
      {/* Band accent */}
      <path d="M11 11Q16 12.5 21 11" fill="none" stroke={COLORS.accent} strokeWidth="0.6" opacity="0.3" />
    </svg>
  );
}
