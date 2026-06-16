import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { Template } from "../../api/types";
import { CategoryIcon } from "./RecipeCard";

interface RecipeDetailProps {
  template: Template;
  onClose: () => void;
}

/** Minimal keyword highlighter for .mozza recipe source. */
function HighlightedSource({ source }: { source: string }) {
  const keywords = ["name:", "slices:", "ingredients:", "kind:", "image:", "port:", "public:", "replicas:", "health:", "from:", "to:", "schedule:", "storage:", "engine:", "version:"];

  const lines = source.split("\n");
  return (
    <pre
      style={{
        margin: 0,
        padding: 16,
        fontSize: 12,
        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        lineHeight: 1.6,
        overflowX: "auto",
        color: "#ccc",
        background: "#0d0d0d",
        borderRadius: 8,
        border: "1px solid #1a1a1a",
      }}
    >
      {lines.map((line, i) => {
        const trimmed = line.trimStart();
        const indent = line.length - trimmed.length;
        const indentStr = line.slice(0, indent);

        // Comments
        if (trimmed.startsWith("#")) {
          return (
            <div key={i}>
              <span style={{ color: "#555" }}>{line}</span>
            </div>
          );
        }

        // Check for keyword at start of line
        const kw = keywords.find((k) => trimmed.startsWith(k));
        if (kw) {
          const rest = trimmed.slice(kw.length);
          return (
            <div key={i}>
              <span>{indentStr}</span>
              <span style={{ color: "#ff6b35" }}>{kw}</span>
              <span style={{ color: "#e5e5e5" }}>{rest}</span>
            </div>
          );
        }

        // List items (- prefix)
        if (trimmed.startsWith("- ")) {
          return (
            <div key={i}>
              <span>{indentStr}</span>
              <span style={{ color: "#666" }}>- </span>
              <span style={{ color: "#e5e5e5" }}>{trimmed.slice(2)}</span>
            </div>
          );
        }

        return <div key={i}>{line}</div>;
      })}
    </pre>
  );
}

export default function RecipeDetail({ template, onClose }: RecipeDetailProps) {
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  function handleDeploy() {
    // Navigate to deploy wizard with template pre-selected via query param
    navigate(`/app/deploy?marketplace=${encodeURIComponent(template.id)}`);
  }

  function handleCopy() {
    navigator.clipboard.writeText(template.source).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  }

  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        zIndex: 100,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
      }}
    >
      {/* Backdrop */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          background: "rgba(0,0,0,0.7)",
          backdropFilter: "blur(4px)",
        }}
        onClick={onClose}
      />

      {/* Modal */}
      <div
        style={{
          position: "relative",
          background: "#111",
          border: "1px solid #222",
          borderRadius: 12,
          width: "100%",
          maxWidth: 680,
          maxHeight: "85vh",
          display: "flex",
          flexDirection: "column",
          overflow: "hidden",
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: "20px 24px",
            borderBottom: "1px solid #1a1a1a",
            display: "flex",
            alignItems: "flex-start",
            gap: 14,
          }}
        >
          <span style={{ flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", width: 40, height: 40, borderRadius: 10, background: "rgba(255,107,53,0.08)" }}><CategoryIcon category={template.category} /></span>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              <h2 style={{ fontSize: 18, fontWeight: 700, color: "#e5e5e5", margin: 0 }}>
                {template.name}
              </h2>
              {template.official && (
                <span
                  style={{
                    fontSize: 10,
                    padding: "2px 8px",
                    borderRadius: 4,
                    border: "1px solid rgba(255,107,53,0.3)",
                    color: "rgba(255,107,53,0.8)",
                    fontWeight: 600,
                  }}
                >
                  Official
                </span>
              )}
            </div>
            <p style={{ fontSize: 13, color: "#888", margin: "6px 0 0", lineHeight: 1.5 }}>
              {template.description}
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            style={{
              background: "none",
              border: "none",
              color: "#666",
              cursor: "pointer",
              padding: 4,
              fontSize: 18,
              lineHeight: 1,
              flexShrink: 0,
            }}
            aria-label="Close"
          >
            &times;
          </button>
        </div>

        {/* Body — scrollable */}
        <div style={{ flex: 1, overflowY: "auto", padding: 24 }}>
          {/* Meta row */}
          <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", marginBottom: 16 }}>
            <span
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 4,
                fontSize: 11,
                padding: "3px 10px",
                borderRadius: 5,
                border: "1px solid #333",
                color: "#aaa",
              }}
            >
              <CategoryIcon category={template.category} />
              {template.category}
            </span>
            {template.tags.map((tag) => (
              <span
                key={tag}
                style={{
                  fontSize: 10,
                  padding: "3px 8px",
                  borderRadius: 5,
                  background: "#1a1a1a",
                  color: "#777",
                }}
              >
                {tag}
              </span>
            ))}
            {template.est_resources && (
              <span
                style={{
                  fontSize: 10,
                  padding: "3px 8px",
                  borderRadius: 5,
                  border: "1px solid rgba(250,204,21,0.3)",
                  color: "rgba(250,204,21,0.8)",
                }}
              >
                {template.est_resources}
              </span>
            )}
          </div>

          {/* Variables preview */}
          {template.variables.length > 0 && (
            <div style={{ marginBottom: 20 }}>
              <h3 style={{ fontSize: 12, fontWeight: 600, color: "#999", margin: "0 0 8px", textTransform: "uppercase", letterSpacing: 1 }}>
                Configuration
              </h3>
              <div
                style={{
                  display: "grid",
                  gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))",
                  gap: 8,
                }}
              >
                {template.variables.map((v) => (
                  <div
                    key={v.key}
                    style={{
                      padding: "8px 12px",
                      background: "#0d0d0d",
                      borderRadius: 6,
                      border: "1px solid #1a1a1a",
                    }}
                  >
                    <div style={{ fontSize: 11, fontWeight: 600, color: "#ccc" }}>{v.label}</div>
                    <div style={{ fontSize: 10, color: "#666", marginTop: 2 }}>
                      {v.required ? "Required" : `Default: ${v.default || "none"}`}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Recipe source */}
          <div>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 8 }}>
              <h3 style={{ fontSize: 12, fontWeight: 600, color: "#999", margin: 0, textTransform: "uppercase", letterSpacing: 1 }}>
                Recipe
              </h3>
              <button
                type="button"
                onClick={handleCopy}
                style={{
                  background: "none",
                  border: "1px solid #333",
                  borderRadius: 5,
                  padding: "3px 10px",
                  fontSize: 10,
                  color: copied ? "#ff6b35" : "#888",
                  cursor: "pointer",
                  transition: "color 0.2s",
                }}
              >
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
            <HighlightedSource source={template.source} />
          </div>
        </div>

        {/* Footer actions */}
        <div
          style={{
            padding: "16px 24px",
            borderTop: "1px solid #1a1a1a",
            display: "flex",
            alignItems: "center",
            justifyContent: "flex-end",
            gap: 10,
          }}
        >
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: "8px 20px",
              fontSize: 13,
              fontWeight: 600,
              color: "#888",
              background: "none",
              border: "1px solid #333",
              borderRadius: 8,
              cursor: "pointer",
            }}
          >
            Close
          </button>
          <button
            type="button"
            onClick={handleDeploy}
            style={{
              padding: "8px 24px",
              fontSize: 13,
              fontWeight: 700,
              color: "#fff",
              background: "#ff6b35",
              border: "none",
              borderRadius: 8,
              cursor: "pointer",
              transition: "opacity 0.2s",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.opacity = "0.9"; }}
            onMouseLeave={(e) => { e.currentTarget.style.opacity = "1"; }}
          >
            Deploy this recipe
          </button>
        </div>
      </div>
    </div>
  );
}
