import type { Template } from "../../api/types";

/** Simple geometric SVG icons per category. */
function CategoryIcon({ category }: { category: string }) {
  const size = 18;
  const color = "#ff6b35";
  switch (category.toLowerCase()) {
    case "web frameworks":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <circle cx="12" cy="12" r="10" stroke={color} strokeWidth="1.5" />
          <ellipse cx="12" cy="12" rx="4" ry="10" stroke={color} strokeWidth="1.5" />
          <line x1="2" y1="12" x2="22" y2="12" stroke={color} strokeWidth="1.5" />
        </svg>
      );
    case "databases":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <ellipse cx="12" cy="6" rx="8" ry="3" stroke={color} strokeWidth="1.5" />
          <path d="M4 6v12c0 1.66 3.58 3 8 3s8-1.34 8-3V6" stroke={color} strokeWidth="1.5" />
          <path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" stroke={color} strokeWidth="1.5" />
        </svg>
      );
    case "monitoring":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <polyline points="4,18 8,12 12,15 16,8 20,10" stroke={color} strokeWidth="1.5" strokeLinejoin="round" />
          <rect x="3" y="3" width="18" height="18" rx="2" stroke={color} strokeWidth="1.5" />
        </svg>
      );
    case "cms":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <rect x="3" y="3" width="18" height="18" rx="2" stroke={color} strokeWidth="1.5" />
          <line x1="3" y1="9" x2="21" y2="9" stroke={color} strokeWidth="1.5" />
          <line x1="9" y1="9" x2="9" y2="21" stroke={color} strokeWidth="1.5" />
        </svg>
      );
    case "devtools":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <polyline points="8,6 2,12 8,18" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          <polyline points="16,6 22,12 16,18" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
          <line x1="14" y1="4" x2="10" y2="20" stroke={color} strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      );
    case "communication":
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <path d="M4 4h16v12H8l-4 4V4z" stroke={color} strokeWidth="1.5" strokeLinejoin="round" />
          <circle cx="9" cy="10" r="1" fill={color} />
          <circle cx="12" cy="10" r="1" fill={color} />
          <circle cx="15" cy="10" r="1" fill={color} />
        </svg>
      );
    default:
      return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
          <rect x="3" y="3" width="18" height="18" rx="3" stroke={color} strokeWidth="1.5" />
          <circle cx="12" cy="12" r="4" stroke={color} strokeWidth="1.5" />
        </svg>
      );
  }
}

interface RecipeCardProps {
  template: Template;
  onClick: () => void;
}

export default function RecipeCard({ template, onClick }: RecipeCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      style={{
        background: "#111111",
        border: "1px solid #222",
        borderRadius: 10,
        padding: 20,
        cursor: "pointer",
        textAlign: "left",
        display: "flex",
        flexDirection: "column",
        gap: 12,
        transition: "border-color 0.2s, box-shadow 0.2s",
        width: "100%",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.borderColor = "#ff6b35";
        e.currentTarget.style.boxShadow = "0 0 20px rgba(255,107,53,0.08)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.borderColor = "#222";
        e.currentTarget.style.boxShadow = "none";
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <span style={{ flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", width: 32, height: 32, borderRadius: 8, background: "rgba(255,107,53,0.08)" }}><CategoryIcon category={template.category} /></span>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span
              style={{
                fontSize: 14,
                fontWeight: 700,
                color: "#e5e5e5",
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {template.name}
            </span>
            {template.official && (
              <span
                style={{
                  fontSize: 9,
                  padding: "1px 6px",
                  borderRadius: 4,
                  border: "1px solid rgba(255,107,53,0.3)",
                  color: "rgba(255,107,53,0.8)",
                  fontWeight: 600,
                  flexShrink: 0,
                }}
              >
                Official
              </span>
            )}
          </div>
          <p
            style={{
              fontSize: 11,
              color: "#888",
              marginTop: 3,
              lineHeight: 1.5,
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {template.description}
          </p>
        </div>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
        <span
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 4,
            fontSize: 10,
            padding: "2px 8px",
            borderRadius: 4,
            border: "1px solid #333",
            color: "#aaa",
          }}
        >
          <CategoryIcon category={template.category} />
          {template.category}
        </span>
        {template.est_resources && (
          <span
            style={{
              fontSize: 10,
              padding: "2px 8px",
              borderRadius: 4,
              border: "1px solid rgba(250,204,21,0.3)",
              color: "rgba(250,204,21,0.8)",
            }}
          >
            {template.est_resources}
          </span>
        )}
      </div>
    </button>
  );
}

export { CategoryIcon };
