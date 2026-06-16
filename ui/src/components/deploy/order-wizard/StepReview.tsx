import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { RotateCcw, Rocket, Code, FileText } from "lucide-react";
import type { ServiceInterviewState, WizardTarget } from "@/lib/interview-types";
import { generateOrderRecipe } from "@/lib/recipe-generator";
import { validateRecipe } from "@/api/client";

// ─── Props ──────────────────────────────────────────────────

interface StepReviewProps {
  services: ServiceInterviewState[];
  target: WizardTarget;
  teamId: string;
  wizardState: Parameters<typeof generateOrderRecipe>[0];
  onPlaceOrder: (recipeSource: string) => void;
}

// ─── Component ──────────────────────────────────────────────

export default function StepReview({
  services,
  target,
  wizardState,
  onPlaceOrder,
}: StepReviewProps) {
  const generatedSource = generateOrderRecipe(wizardState);
  const [recipeSource, setRecipeSource] = useState(generatedSource);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [validating, setValidating] = useState(false);
  const [viewMode, setViewMode] = useState<"recipe" | "yaml">("recipe");

  const isModified = recipeSource !== generatedSource;

  // Re-generate when wizard state changes
  useEffect(() => {
    if (!isModified) {
      setRecipeSource(generatedSource);
    }
  }, [generatedSource, isModified]);

  // Validate on blur or on initial load
  const validate = useCallback(async (source: string) => {
    setValidating(true);
    try {
      const result = await validateRecipe(source);
      setValidationErrors(result.valid ? [] : (result.errors || ["Invalid recipe syntax"]));
    } catch {
      // If validation endpoint fails, allow anyway
      setValidationErrors([]);
    } finally {
      setValidating(false);
    }
  }, []);

  useEffect(() => {
    validate(recipeSource);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleReset = () => {
    setRecipeSource(generatedSource);
    setValidationErrors([]);
    validate(generatedSource);
  };

  // Summary stats
  const totalReplicas = services.reduce((sum, s) => sum + s.replicas, 0);
  const deps = services
    .flatMap((s) => s.dependencies.filter((d) => d.enabled))
    .map((d) => d.type === "database" ? d.engine : d.type);
  const uniqueDeps = [...new Set(deps)];

  const canPlaceOrder = validationErrors.length === 0 && !validating;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Summary card */}
      <div
        style={{
          display: "flex",
          gap: 16,
          padding: "12px 16px",
          background: "rgba(255,255,255,0.03)",
          borderRadius: 8,
          border: "1px solid rgba(255,255,255,0.06)",
          fontSize: 13,
          color: "rgba(255,255,255,0.6)",
          flexWrap: "wrap",
        }}
      >
        <span>
          <strong style={{ color: "#fff" }}>{services.length}</strong>{" "}
          {services.length === 1 ? "service" : "services"}
        </span>
        <span style={{ color: "rgba(255,255,255,0.15)" }}>|</span>
        <span>
          <strong style={{ color: "#fff" }}>{totalReplicas}</strong> total{" "}
          {totalReplicas === 1 ? "copy" : "copies"}
        </span>
        <span style={{ color: "rgba(255,255,255,0.15)" }}>|</span>
        <span>{target === "kitchen" ? "Kitchen (Kubernetes)" : "Counter (Local)"}</span>
        {uniqueDeps.length > 0 && (
          <>
            <span style={{ color: "rgba(255,255,255,0.15)" }}>|</span>
            <span>{uniqueDeps.join(", ")}</span>
          </>
        )}
      </div>

      {/* Editor toolbar */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <div style={{ display: "flex", gap: 4 }}>
          <button
            onClick={() => setViewMode("recipe")}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 5,
              padding: "5px 12px",
              background: viewMode === "recipe" ? "rgba(255,107,53,0.12)" : "transparent",
              border: viewMode === "recipe"
                ? "1px solid rgba(255,107,53,0.3)"
                : "1px solid rgba(255,255,255,0.08)",
              borderRadius: 6,
              color: viewMode === "recipe" ? "#ff6b35" : "rgba(255,255,255,0.4)",
              cursor: "pointer",
              fontSize: 12,
              fontWeight: 500,
            }}
          >
            <FileText style={{ width: 13, height: 13 }} />
            Recipe
          </button>
          <button
            onClick={() => setViewMode("yaml")}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 5,
              padding: "5px 12px",
              background: viewMode === "yaml" ? "rgba(255,107,53,0.12)" : "transparent",
              border: viewMode === "yaml"
                ? "1px solid rgba(255,107,53,0.3)"
                : "1px solid rgba(255,255,255,0.08)",
              borderRadius: 6,
              color: viewMode === "yaml" ? "#ff6b35" : "rgba(255,255,255,0.4)",
              cursor: "pointer",
              fontSize: 12,
              fontWeight: 500,
            }}
          >
            <Code style={{ width: 13, height: 13 }} />
            View the blueprint
          </button>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          {isModified && (
            <>
              <Badge variant="outline" style={{ fontSize: 11 }}>Modified</Badge>
              <button
                onClick={handleReset}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 4,
                  background: "none",
                  border: "none",
                  color: "rgba(255,255,255,0.4)",
                  cursor: "pointer",
                  fontSize: 12,
                }}
              >
                <RotateCcw style={{ width: 12, height: 12 }} />
                Reset
              </button>
            </>
          )}
        </div>
      </div>

      {/* Recipe editor / YAML view */}
      {viewMode === "recipe" ? (
        <textarea
          value={recipeSource}
          onChange={(e) => setRecipeSource(e.target.value)}
          onBlur={() => validate(recipeSource)}
          spellCheck={false}
          style={{
            width: "100%",
            minHeight: 280,
            padding: 16,
            fontFamily: "'JetBrains Mono', 'Fira Code', 'SF Mono', monospace",
            fontSize: 13,
            lineHeight: 1.6,
            background: "rgba(0,0,0,0.3)",
            border: validationErrors.length > 0
              ? "1px solid rgba(239,68,68,0.4)"
              : "1px solid rgba(255,255,255,0.08)",
            borderRadius: 8,
            color: "#e0e0e0",
            resize: "vertical",
            outline: "none",
            tabSize: 2,
          }}
        />
      ) : (
        <div
          style={{
            minHeight: 280,
            padding: 16,
            fontFamily: "'JetBrains Mono', 'Fira Code', 'SF Mono', monospace",
            fontSize: 13,
            lineHeight: 1.6,
            background: "rgba(0,0,0,0.3)",
            border: "1px solid rgba(255,255,255,0.08)",
            borderRadius: 8,
            color: "rgba(255,255,255,0.5)",
            whiteSpace: "pre-wrap",
            overflow: "auto",
          }}
        >
          {target === "kitchen"
            ? "# Compiled Kubernetes YAML will appear here after deployment\n# This view shows what Mozza generates from your recipe"
            : "# Compiled docker-compose.yml will appear here after deployment\n# This view shows what Mozza generates from your recipe"}
        </div>
      )}

      {/* Validation errors */}
      {validationErrors.length > 0 && (
        <div
          style={{
            padding: "10px 14px",
            background: "rgba(239,68,68,0.08)",
            border: "1px solid rgba(239,68,68,0.2)",
            borderRadius: 6,
            fontSize: 12,
            color: "#f87171",
          }}
        >
          {validationErrors.map((err, i) => (
            <div key={i}>{err}</div>
          ))}
        </div>
      )}

      {/* Place Order button */}
      <div style={{ display: "flex", justifyContent: "flex-end", paddingTop: 8 }}>
        <Button
          onClick={() => onPlaceOrder(recipeSource)}
          disabled={!canPlaceOrder}
          style={{
            background: canPlaceOrder ? "#ff6b35" : "rgba(255,255,255,0.1)",
            color: canPlaceOrder ? "#fff" : "rgba(255,255,255,0.3)",
            border: "none",
            fontWeight: 600,
            fontSize: 15,
            padding: "12px 32px",
            borderRadius: 8,
            cursor: canPlaceOrder ? "pointer" : "not-allowed",
          }}
        >
          <Rocket style={{ width: 16, height: 16, marginRight: 8 }} />
          Place Order
        </Button>
      </div>
    </div>
  );
}
