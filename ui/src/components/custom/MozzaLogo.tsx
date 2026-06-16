// Mozza brand icon — mozzarella ball for the internal UI.
// Tailwind-compatible. Use instead of lucide Pizza icon or emoji.

interface MozzaLogoProps {
  className?: string;
  size?: number;
}

export function MozzaLogo({ className = "", size = 20 }: MozzaLogoProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      className={className}
    >
      <defs>
        <radialGradient id="mz-body" cx="0.45" cy="0.4" r="0.55">
          <stop offset="0%" stopColor="#fffef8" />
          <stop offset="50%" stopColor="#f5f0e8" />
          <stop offset="100%" stopColor="#e8ddd0" />
        </radialGradient>
        <linearGradient id="mz-hat" x1="16" y1="2" x2="16" y2="12" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#ffffff" />
          <stop offset="100%" stopColor="#f0ede8" />
        </linearGradient>
      </defs>
      {/* Feet */}
      <ellipse cx="12.5" cy="27" rx="3.5" ry="1.8" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.5" />
      <ellipse cx="19.5" cy="27" rx="3.5" ry="1.8" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.5" />
      {/* Body */}
      <ellipse cx="16" cy="18" rx="10" ry="9.5" fill="url(#mz-body)" />
      <ellipse cx="16" cy="18" rx="10" ry="9.5" fill="none" stroke="#d8d0c4" strokeWidth="0.5" />
      <ellipse cx="13" cy="14" rx="4" ry="3" fill="#fff" opacity="0.3" transform="rotate(-15, 13, 14)" />
      {/* Eyes */}
      <ellipse cx="13" cy="17.5" rx="1.8" ry="2" fill="#1a1208" />
      <circle cx="13.5" cy="16.8" r="0.7" fill="#fff" opacity="0.85" />
      <ellipse cx="19.5" cy="17.5" rx="1.8" ry="2" fill="#1a1208" />
      <circle cx="20" cy="16.8" r="0.7" fill="#fff" opacity="0.85" />
      {/* Smile */}
      <path d="M13.5 20.5Q16 23 18.5 20.5" stroke="#1a1208" strokeWidth="0.9" strokeLinecap="round" fill="none" />
      {/* Cheeks */}
      <circle cx="10.5" cy="20" r="2" fill="currentColor" opacity="0.12" />
      <circle cx="21.5" cy="20" r="2" fill="currentColor" opacity="0.12" />
      {/* Chef hat */}
      <ellipse cx="16" cy="10" rx="7" ry="2.2" fill="#e8e4dc" stroke="#d0ccc4" strokeWidth="0.4" />
      <path d="M10 10Q8 5 12 4Q14 1 16 3Q18 1 20 4Q24 5 22 10Z" fill="url(#mz-hat)" />
      <path d="M10 10Q8 5 12 4Q14 1 16 3Q18 1 20 4Q24 5 22 10" fill="none" stroke="#d8d4cc" strokeWidth="0.4" />
      <path d="M11.5 7Q13 4.5 15 4" fill="none" stroke="#fff" strokeWidth="0.8" strokeLinecap="round" opacity="0.5" />
      {/* Band accent — uses currentColor so it picks up text-brand */}
      <path d="M11 11Q16 12.5 21 11" fill="none" stroke="currentColor" strokeWidth="0.6" opacity="0.4" />
    </svg>
  );
}

// Larger version for auth pages / hero moments
export function MozzaLogoLarge({ className = "", size = 80 }: MozzaLogoProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 120 120"
      fill="none"
      className={className}
    >
      <defs>
        <radialGradient id="mzl-body" cx="0.45" cy="0.38" r="0.55">
          <stop offset="0%" stopColor="#fffef8" />
          <stop offset="50%" stopColor="#f5f0e8" />
          <stop offset="100%" stopColor="#e8ddd0" />
        </radialGradient>
        <radialGradient id="mzl-shadow" cx="0.5" cy="0.7" r="0.5">
          <stop offset="0%" stopColor="#ddd4c4" stopOpacity="0.5" />
          <stop offset="100%" stopColor="#ddd4c4" stopOpacity="0" />
        </radialGradient>
        <linearGradient id="mzl-hat" x1="60" y1="5" x2="60" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#ffffff" />
          <stop offset="100%" stopColor="#f0ede8" />
        </linearGradient>
        <linearGradient id="mzl-band" x1="40" y1="30" x2="80" y2="30" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#e0dbd2" />
          <stop offset="50%" stopColor="#d4cec4" />
          <stop offset="100%" stopColor="#e0dbd2" />
        </linearGradient>
        <radialGradient id="mzl-blush" cx="0.5" cy="0.5" r="0.5">
          <stop offset="0%" stopColor="currentColor" stopOpacity="0.2" />
          <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
        </radialGradient>
      </defs>

      {/* Ground shadow */}
      <ellipse cx="60" cy="110" rx="28" ry="5" fill="#000" opacity="0.06" />

      {/* Legs + feet */}
      <ellipse cx="47" cy="96" rx="8" ry="10" fill="#f5f0e8" stroke="#ddd4c4" strokeWidth="0.8" />
      <ellipse cx="45" cy="104" rx="10" ry="5" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.8" />
      <ellipse cx="43" cy="102" rx="4" ry="2" fill="#fff" opacity="0.3" />
      <ellipse cx="73" cy="96" rx="8" ry="10" fill="#f5f0e8" stroke="#ddd4c4" strokeWidth="0.8" />
      <ellipse cx="75" cy="104" rx="10" ry="5" fill="#f0ebe2" stroke="#d8d0c4" strokeWidth="0.8" />
      <ellipse cx="73" cy="102" rx="4" ry="2" fill="#fff" opacity="0.3" />

      {/* Body */}
      <ellipse cx="60" cy="62" rx="34" ry="32" fill="url(#mzl-body)" />
      <ellipse cx="60" cy="62" rx="34" ry="32" fill="url(#mzl-shadow)" />
      <ellipse cx="60" cy="62" rx="34" ry="32" fill="none" stroke="#d8d0c4" strokeWidth="0.8" />
      {/* Highlights */}
      <ellipse cx="48" cy="46" rx="14" ry="10" fill="#fff" opacity="0.35" transform="rotate(-15, 48, 46)" />
      <ellipse cx="42" cy="42" rx="6" ry="4" fill="#fff" opacity="0.2" transform="rotate(-20, 42, 42)" />
      {/* Texture */}
      <path d="M38 56Q42 54 46 56" stroke="#e8e0d4" strokeWidth="1" strokeLinecap="round" fill="none" opacity="0.5" />
      <path d="M64 72Q70 70 76 73" stroke="#e8e0d4" strokeWidth="0.8" strokeLinecap="round" fill="none" opacity="0.4" />

      {/* Arms — friendly open */}
      <path d="M28 62Q14 60 8 64" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
      <path d="M28 62Q14 60 8 64" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
      <circle cx="6" cy="63" r="5" fill="#f5f0e8" stroke="#d8d0c4" strokeWidth="0.8" />
      <ellipse cx="4" cy="61" rx="2" ry="1.2" fill="#fff" opacity="0.3" />
      <path d="M92 62Q106 60 112 64" stroke="#e8ddd0" strokeWidth="5" strokeLinecap="round" />
      <path d="M92 62Q106 60 112 64" stroke="#f5f0e8" strokeWidth="3.5" strokeLinecap="round" />
      <circle cx="114" cy="63" r="5" fill="#f5f0e8" stroke="#d8d0c4" strokeWidth="0.8" />
      <ellipse cx="112" cy="61" rx="2" ry="1.2" fill="#fff" opacity="0.3" />

      {/* Eyes */}
      <ellipse cx="48" cy="58" rx="5.5" ry="6" fill="#1a1208" />
      <ellipse cx="48" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
      <circle cx="49.5" cy="56" r="2.2" fill="#fff" opacity="0.9" />
      <circle cx="46.5" cy="59.5" r="1" fill="#fff" opacity="0.35" />
      <ellipse cx="72" cy="58" rx="5.5" ry="6" fill="#1a1208" />
      <ellipse cx="72" cy="58" rx="5" ry="5.5" fill="#2a1e10" />
      <circle cx="73.5" cy="56" r="2.2" fill="#fff" opacity="0.9" />
      <circle cx="70.5" cy="59.5" r="1" fill="#fff" opacity="0.35" />

      {/* Smile */}
      <path d="M50 67Q60 76 70 67" stroke="#1a1208" strokeWidth="2.2" strokeLinecap="round" fill="none" />

      {/* Cheeks */}
      <ellipse cx="38" cy="66" rx="6" ry="4.5" fill="url(#mzl-blush)" />
      <ellipse cx="82" cy="66" rx="6" ry="4.5" fill="url(#mzl-blush)" />

      {/* Chef hat */}
      <ellipse cx="60" cy="34" rx="22" ry="6" fill="url(#mzl-band)" stroke="#c8c2b8" strokeWidth="0.8" />
      <path d="M40 34Q35 18 46 12Q50 3 60 6Q70 3 74 12Q85 18 80 34Z" fill="url(#mzl-hat)" />
      <path d="M40 34Q35 18 46 12Q50 3 60 6Q70 3 74 12Q85 18 80 34" fill="none" stroke="#d8d4cc" strokeWidth="0.8" />
      <path d="M43 22Q46 14 52 13" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" opacity="0.5" />
      <path d="M68 13Q74 14 77 22" fill="none" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" opacity="0.35" />
      {/* Band accent */}
      <path d="M42 36Q60 40 78 36" fill="none" stroke="currentColor" strokeWidth="1.5" opacity="0.35" />

      {/* Cheese string detail */}
      <path d="M86 68Q90 72 88 80Q87 84 85 82Q86 78 84 74" stroke="#e8ddd0" strokeWidth="2" strokeLinecap="round" fill="none" opacity="0.5" />
      <circle cx="85" cy="82" r="2" fill="#ede6dc" opacity="0.5" />
    </svg>
  );
}
