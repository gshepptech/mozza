import { useState, useEffect, useRef, useCallback } from "react";
import * as api from "../../api/client";
import type { Template } from "../../api/types";
import RecipeCard from "./RecipeCard";
import RecipeDetail from "./RecipeDetail";

const CATEGORIES = [
  "All",
  "Web Frameworks",
  "Databases",
  "Monitoring",
  "CMS",
  "DevTools",
  "Communication",
] as const;

/** Featured recipe names shown at the top of the page. */
const FEATURED_IDS = ["wordpress", "grafana-stack", "ghost", "redis", "gitea"];

/** Skeleton card shown during loading. */
function SkeletonCard() {
  return (
    <div
      style={{
        background: "#111",
        border: "1px solid #1a1a1a",
        borderRadius: 10,
        padding: 20,
        display: "flex",
        flexDirection: "column",
        gap: 12,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: 6,
            background: "#1a1a1a",
            animation: "pulse 1.5s ease-in-out infinite",
          }}
        />
        <div style={{ flex: 1 }}>
          <div
            style={{
              width: "60%",
              height: 14,
              borderRadius: 4,
              background: "#1a1a1a",
              animation: "pulse 1.5s ease-in-out infinite",
            }}
          />
          <div
            style={{
              width: "90%",
              height: 10,
              borderRadius: 4,
              background: "#1a1a1a",
              marginTop: 6,
              animation: "pulse 1.5s ease-in-out infinite",
              animationDelay: "0.2s",
            }}
          />
        </div>
      </div>
      <div style={{ display: "flex", gap: 6 }}>
        <div
          style={{
            width: 70,
            height: 18,
            borderRadius: 4,
            background: "#1a1a1a",
            animation: "pulse 1.5s ease-in-out infinite",
            animationDelay: "0.4s",
          }}
        />
      </div>
    </div>
  );
}

export default function RecipeCatalog() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [activeCategory, setActiveCategory] = useState<string>("All");
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null);

  const debounceRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Debounce search input by 300ms.
  const handleSearchChange = useCallback((value: string) => {
    setSearchQuery(value);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setDebouncedQuery(value);
    }, 300);
  }, []);

  // Clean up debounce timer.
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  // Fetch recipes from marketplace API.
  useEffect(() => {
    setLoading(true);
    setError("");

    const params: Parameters<typeof api.listMarketplaceRecipes>[0] = {
      per_page: 100,
    };
    if (activeCategory !== "All") params.category = activeCategory;
    if (debouncedQuery) params.query = debouncedQuery;

    api
      .listMarketplaceRecipes(params)
      .then((result) => {
        setTemplates(result.recipes.map((r) => r.template));
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load recipes");
      })
      .finally(() => setLoading(false));
  }, [activeCategory, debouncedQuery]);

  // Split featured vs rest.
  const featured = templates.filter((t) => FEATURED_IDS.includes(t.id));
  const rest = templates.filter((t) => !FEATURED_IDS.includes(t.id));
  const showFeatured = activeCategory === "All" && !debouncedQuery && featured.length > 0;

  return (
    <div style={{ maxWidth: 1100, margin: "0 auto" }}>
      {/* Pulse keyframe for skeletons */}
      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 0.4; }
          50% { opacity: 0.8; }
        }
      `}</style>

      {/* Header */}
      <div style={{ marginBottom: 28 }}>
        <h1 style={{ fontSize: 22, fontWeight: 800, color: "#e5e5e5", margin: 0 }}>
          Recipe Marketplace
        </h1>
        <p style={{ fontSize: 13, color: "#666", marginTop: 6 }}>
          Production-ready recipes you can deploy in one click.
        </p>
      </div>

      {/* Search bar */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ position: "relative" }}>
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            style={{ position: "absolute", left: 12, top: "50%", transform: "translateY(-50%)" }}
          >
            <circle cx="11" cy="11" r="7" stroke="#555" strokeWidth="2" />
            <line x1="16" y1="16" x2="21" y2="21" stroke="#555" strokeWidth="2" strokeLinecap="round" />
          </svg>
          <input
            type="text"
            placeholder="Search recipes by name, description, or tag..."
            value={searchQuery}
            onChange={(e) => handleSearchChange(e.target.value)}
            style={{
              width: "100%",
              padding: "10px 14px 10px 36px",
              fontSize: 13,
              background: "#111",
              border: "1px solid #222",
              borderRadius: 8,
              color: "#e5e5e5",
              outline: "none",
              boxSizing: "border-box",
              transition: "border-color 0.2s",
            }}
            onFocus={(e) => { e.currentTarget.style.borderColor = "#ff6b35"; }}
            onBlur={(e) => { e.currentTarget.style.borderColor = "#222"; }}
          />
        </div>
      </div>

      {/* Category filter */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 24 }}>
        {CATEGORIES.map((cat) => (
          <button
            key={cat}
            type="button"
            onClick={() => setActiveCategory(cat)}
            style={{
              padding: "6px 14px",
              fontSize: 12,
              fontWeight: 600,
              borderRadius: 8,
              border: "none",
              cursor: "pointer",
              transition: "all 0.15s",
              background: activeCategory === cat ? "#ff6b35" : "#1a1a1a",
              color: activeCategory === cat ? "#fff" : "#888",
              boxShadow: activeCategory === cat ? "0 0 12px rgba(255,107,53,0.2)" : "none",
            }}
          >
            {cat}
          </button>
        ))}
      </div>

      {/* Loading state */}
      {loading && (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))",
            gap: 14,
          }}
        >
          {Array.from({ length: 6 }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      )}

      {/* Error state */}
      {!loading && error && (
        <div style={{ textAlign: "center", padding: "60px 0" }}>
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" style={{ margin: "0 auto 12px" }}>
            <path d="M12 2L2 22h20L12 2z" stroke="#ef4444" strokeWidth="1.5" fill="none" />
            <line x1="12" y1="10" x2="12" y2="15" stroke="#ef4444" strokeWidth="2" strokeLinecap="round" />
            <circle cx="12" cy="18" r="1" fill="#ef4444" />
          </svg>
          <p style={{ fontSize: 13, color: "#ef4444" }}>{error}</p>
          <button
            type="button"
            onClick={() => {
              setError("");
              setDebouncedQuery(debouncedQuery); // re-trigger effect
            }}
            style={{
              marginTop: 12,
              fontSize: 12,
              fontWeight: 600,
              color: "#ff6b35",
              background: "none",
              border: "none",
              cursor: "pointer",
              textDecoration: "underline",
            }}
          >
            Retry
          </button>
        </div>
      )}

      {/* Empty state */}
      {!loading && !error && templates.length === 0 && (
        <div style={{ textAlign: "center", padding: "60px 0" }}>
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" style={{ margin: "0 auto 12px" }}>
            <circle cx="11" cy="11" r="7" stroke="#444" strokeWidth="1.5" />
            <line x1="16" y1="16" x2="21" y2="21" stroke="#444" strokeWidth="1.5" strokeLinecap="round" />
            <line x1="8" y1="11" x2="14" y2="11" stroke="#444" strokeWidth="1.5" strokeLinecap="round" />
          </svg>
          <p style={{ fontSize: 13, color: "#666" }}>
            No recipes found
            {activeCategory !== "All" ? ` in ${activeCategory}` : ""}
            {debouncedQuery ? ` matching "${debouncedQuery}"` : ""}
            .
          </p>
          {(activeCategory !== "All" || debouncedQuery) && (
            <button
              type="button"
              onClick={() => {
                setActiveCategory("All");
                setSearchQuery("");
                setDebouncedQuery("");
              }}
              style={{
                marginTop: 10,
                fontSize: 12,
                fontWeight: 600,
                color: "#ff6b35",
                background: "none",
                border: "none",
                cursor: "pointer",
              }}
            >
              Clear filters
            </button>
          )}
        </div>
      )}

      {/* Featured section */}
      {!loading && !error && showFeatured && (
        <div style={{ marginBottom: 32 }}>
          <h2
            style={{
              fontSize: 13,
              fontWeight: 600,
              color: "#999",
              textTransform: "uppercase",
              letterSpacing: 1,
              margin: "0 0 12px",
            }}
          >
            Featured
          </h2>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))",
              gap: 14,
            }}
          >
            {featured.map((t) => (
              <RecipeCard key={t.id} template={t} onClick={() => setSelectedTemplate(t)} />
            ))}
          </div>
        </div>
      )}

      {/* Main grid */}
      {!loading && !error && (showFeatured ? rest : templates).length > 0 && (
        <div>
          {showFeatured && (
            <h2
              style={{
                fontSize: 13,
                fontWeight: 600,
                color: "#999",
                textTransform: "uppercase",
                letterSpacing: 1,
                margin: "0 0 12px",
              }}
            >
              All Recipes
            </h2>
          )}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))",
              gap: 14,
            }}
          >
            {(showFeatured ? rest : templates).map((t) => (
              <RecipeCard key={t.id} template={t} onClick={() => setSelectedTemplate(t)} />
            ))}
          </div>
        </div>
      )}

      {/* Detail modal */}
      {selectedTemplate && (
        <RecipeDetail
          template={selectedTemplate}
          onClose={() => setSelectedTemplate(null)}
        />
      )}
    </div>
  );
}
