import { useState, useRef, useCallback } from "react";
import * as api from "../../api/client";
import type { ComposeServiceInfo, ComposeWarning } from "../../api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Upload, FileCode, AlertTriangle, Loader, ArrowLeft, ArrowRight,
  X, Server, Database, ChevronDown, ChevronRight, Copy, Check,
} from "lucide-react";

type Phase = "input" | "parsing" | "breakdown" | "preview";

interface ComposeImportProps {
  onRecipeReady: (source: string) => void;
}

export default function ComposeImport({ onRecipeReady }: ComposeImportProps) {
  const [phase, setPhase] = useState<Phase>("input");
  const [content, setContent] = useState("");
  const [services, setServices] = useState<ComposeServiceInfo[]>([]);
  const [warnings, setWarnings] = useState<ComposeWarning[]>([]);
  const [recipe, setRecipe] = useState("");
  const [error, setError] = useState("");
  const [expandedService, setExpandedService] = useState<string | null>(null);
  const [dismissedWarnings, setDismissedWarnings] = useState<Set<number>>(new Set());
  const [copied, setCopied] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > 1024 * 1024) {
      setError("File too large — max 1MB");
      return;
    }
    if (!file.name.match(/\.(ya?ml)$/i)) {
      setError("Please upload a .yml or .yaml file");
      return;
    }
    setError("");
    const reader = new FileReader();
    reader.onload = (ev) => {
      const text = ev.target?.result;
      if (typeof text === "string") {
        setContent(text);
      }
    };
    reader.readAsText(file);
  }, []);

  const handleParse = useCallback(async () => {
    if (!content.trim()) return;
    setPhase("parsing");
    setError("");
    try {
      const result = await api.parseCompose(content);
      setServices(result.services);
      setWarnings(result.warnings);
      setRecipe(result.recipe);
      setPhase("breakdown");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to parse compose file";
      const friendly = msg.includes("no services found")
        ? "No services found — make sure your file has a 'services:' section."
        : msg.includes("parsing compose YAML")
          ? "Invalid YAML syntax — check for indentation or formatting errors."
          : msg;
      setError(friendly);
      setPhase("input");
    }
  }, [content]);

  const handleDismissWarning = useCallback((idx: number) => {
    setDismissedWarnings((prev) => new Set(prev).add(idx));
  }, []);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(recipe);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }, [recipe]);

  const handleUseRecipe = useCallback(() => {
    onRecipeReady(recipe);
  }, [recipe, onRecipeReady]);

  const serviceIcon = (svc: ComposeServiceInfo) => {
    if (svc.engine === "postgres" || svc.engine === "mysql" || svc.engine === "redis") {
      return <Database size={16} />;
    }
    return <Server size={16} />;
  };

  const visibleWarnings = warnings.filter((_, i) => !dismissedWarnings.has(i));

  // ── Input phase ────────────────────────────────────────

  if (phase === "input" || phase === "parsing") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <div style={{ textAlign: "center", padding: "8px 0" }}>
          <p style={{ fontSize: 14, color: "rgba(255,255,255,0.5)", margin: 0 }}>
            Paste your docker-compose.yml or upload the file
          </p>
        </div>

        {/* Textarea */}
        <div style={{ position: "relative" }}>
          <textarea
            value={content}
            onChange={(e) => { setContent(e.target.value); setError(""); }}
            placeholder={`version: "3"\nservices:\n  web:\n    image: nginx:latest\n    ports:\n      - "80:80"\n  db:\n    image: postgres:16\n    environment:\n      POSTGRES_PASSWORD: secret`}
            spellCheck={false}
            style={{
              width: "100%",
              minHeight: 200,
              padding: 16,
              background: "rgba(255,255,255,0.03)",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 12,
              color: "#e0e0e0",
              fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
              fontSize: 13,
              lineHeight: 1.6,
              resize: "vertical",
              outline: "none",
              transition: "border-color 0.2s ease",
            }}
            onFocus={(e) => { e.currentTarget.style.borderColor = "rgba(255,107,53,0.4)"; }}
            onBlur={(e) => { e.currentTarget.style.borderColor = "rgba(255,255,255,0.1)"; }}
          />
        </div>

        {/* Divider */}
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <div style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.08)" }} />
          <span style={{ fontSize: 12, color: "rgba(255,255,255,0.3)", textTransform: "uppercase", letterSpacing: 1 }}>or</span>
          <div style={{ flex: 1, height: 1, background: "rgba(255,255,255,0.08)" }} />
        </div>

        {/* File upload */}
        <input
          ref={fileInputRef}
          type="file"
          accept=".yml,.yaml"
          onChange={handleFileUpload}
          style={{ display: "none" }}
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 8,
            padding: "12px 16px",
            background: "transparent",
            border: "1px dashed rgba(255,255,255,0.15)",
            borderRadius: 12,
            color: "rgba(255,255,255,0.5)",
            fontSize: 13,
            cursor: "pointer",
            transition: "all 0.2s ease",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = "rgba(255,107,53,0.4)";
            e.currentTarget.style.color = "rgba(255,255,255,0.8)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = "rgba(255,255,255,0.15)";
            e.currentTarget.style.color = "rgba(255,255,255,0.5)";
          }}
        >
          <Upload size={16} />
          Upload docker-compose.yml
        </button>

        {/* Error */}
        {error && (
          <div style={{
            display: "flex", alignItems: "flex-start", gap: 8,
            padding: "10px 14px", background: "rgba(239,68,68,0.08)",
            border: "1px solid rgba(239,68,68,0.2)", borderRadius: 10,
          }}>
            <AlertTriangle size={14} style={{ color: "#ef4444", marginTop: 2, flexShrink: 0 }} />
            <span style={{ fontSize: 13, color: "#fca5a5" }}>{error}</span>
          </div>
        )}

        {/* Parse button */}
        <Button
          className="bg-brand hover:bg-brand-hover text-primary-foreground w-full"
          disabled={!content.trim() || phase === "parsing"}
          onClick={handleParse}
        >
          {phase === "parsing" ? (
            <>
              <Loader size={16} className="mr-2 animate-spin" />
              Reading your compose file...
            </>
          ) : (
            <>
              <FileCode size={16} className="mr-2" />
              Import
            </>
          )}
        </Button>
      </div>
    );
  }

  // ── Breakdown phase ────────────────────────────────────

  if (phase === "breakdown") {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {/* Header */}
        <div style={{ textAlign: "center", padding: "4px 0" }}>
          <p style={{ fontSize: 15, fontWeight: 600, color: "#fff", margin: 0 }}>
            We found {services.length} slice{services.length !== 1 ? "s" : ""} in your pizza
          </p>
          <p style={{ fontSize: 13, color: "rgba(255,255,255,0.4)", margin: "4px 0 0" }}>
            Each compose service becomes a slice in your Mozza recipe
          </p>
        </div>

        {/* Warnings */}
        {visibleWarnings.length > 0 && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {visibleWarnings.map((w) => {
              const originalIdx = warnings.indexOf(w);
              return (
                <div
                  key={originalIdx}
                  style={{
                    display: "flex", alignItems: "flex-start", gap: 8,
                    padding: "10px 14px",
                    background: w.severity === "error"
                      ? "rgba(239,68,68,0.08)"
                      : w.severity === "warn"
                        ? "rgba(251,191,36,0.08)"
                        : "rgba(96,165,250,0.06)",
                    border: `1px solid ${
                      w.severity === "error" ? "rgba(239,68,68,0.2)"
                        : w.severity === "warn" ? "rgba(251,191,36,0.15)"
                          : "rgba(96,165,250,0.12)"
                    }`,
                    borderRadius: 10,
                  }}
                >
                  <AlertTriangle
                    size={14}
                    style={{
                      color: w.severity === "error" ? "#ef4444"
                        : w.severity === "warn" ? "#fbbf24" : "#60a5fa",
                      marginTop: 2, flexShrink: 0,
                    }}
                  />
                  <span style={{ fontSize: 12, color: "rgba(255,255,255,0.7)", flex: 1 }}>
                    {w.message}
                  </span>
                  <button
                    type="button"
                    onClick={() => handleDismissWarning(originalIdx)}
                    style={{ background: "none", border: "none", cursor: "pointer", padding: 2 }}
                  >
                    <X size={12} style={{ color: "rgba(255,255,255,0.3)" }} />
                  </button>
                </div>
              );
            })}
          </div>
        )}

        {/* Service cards */}
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {services.map((svc) => {
            const isExpanded = expandedService === svc.name;
            const envEntries = svc.env ? Object.entries(svc.env) : [];
            return (
              <div
                key={svc.name}
                style={{
                  background: "rgba(255,255,255,0.03)",
                  border: "1px solid rgba(255,255,255,0.08)",
                  borderRadius: 12,
                  overflow: "hidden",
                  transition: "border-color 0.2s ease",
                }}
                onMouseEnter={(e) => { e.currentTarget.style.borderColor = "rgba(255,107,53,0.25)"; }}
                onMouseLeave={(e) => { e.currentTarget.style.borderColor = "rgba(255,255,255,0.08)"; }}
              >
                {/* Card header */}
                <button
                  type="button"
                  onClick={() => setExpandedService(isExpanded ? null : svc.name)}
                  style={{
                    display: "flex", alignItems: "center", gap: 10,
                    width: "100%", padding: "12px 14px",
                    background: "none", border: "none", cursor: "pointer",
                    textAlign: "left",
                  }}
                >
                  <div style={{
                    display: "flex", alignItems: "center", justifyContent: "center",
                    width: 32, height: 32, borderRadius: 8,
                    background: svc.engine ? "rgba(96,165,250,0.12)" : "rgba(255,107,53,0.1)",
                    color: svc.engine ? "#60a5fa" : "#ff6b35",
                  }}>
                    {serviceIcon(svc)}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <p style={{ fontSize: 14, fontWeight: 600, color: "#fff", margin: 0 }}>
                      {svc.name}
                    </p>
                    <p style={{
                      fontSize: 12, color: "rgba(255,255,255,0.4)", margin: "2px 0 0",
                      fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}>
                      {svc.engine ? `${svc.engine} ${svc.version || ""}` : svc.image || "no image"}
                    </p>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    {svc.port ? (
                      <Badge variant="outline" className="text-[11px]">:{svc.port}</Badge>
                    ) : null}
                    {svc.public && (
                      <Badge variant="outline" className="text-[11px] border-brand/30 text-brand">public</Badge>
                    )}
                    {isExpanded ? <ChevronDown size={14} style={{ color: "rgba(255,255,255,0.3)" }} />
                      : <ChevronRight size={14} style={{ color: "rgba(255,255,255,0.3)" }} />}
                  </div>
                </button>

                {/* Expanded details */}
                {isExpanded && envEntries.length > 0 && (
                  <div style={{
                    padding: "0 14px 14px",
                    borderTop: "1px solid rgba(255,255,255,0.06)",
                  }}>
                    <p style={{
                      fontSize: 11, color: "rgba(255,255,255,0.35)",
                      textTransform: "uppercase", letterSpacing: 0.5,
                      margin: "10px 0 6px",
                    }}>
                      Environment
                    </p>
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      {envEntries.map(([key, val]) => (
                        <div key={key} style={{
                          display: "flex", gap: 8, alignItems: "baseline",
                          fontSize: 12, fontFamily: "monospace",
                        }}>
                          <span style={{ color: "#ff6b35", fontWeight: 500 }}>{key}</span>
                          <span style={{ color: "rgba(255,255,255,0.25)" }}>=</span>
                          <span style={{ color: "rgba(255,255,255,0.6)", wordBreak: "break-all" }}>{val}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>

        {/* Actions */}
        <div style={{ display: "flex", gap: 10 }}>
          <Button
            variant="outline"
            onClick={() => { setPhase("input"); setError(""); }}
            className="flex-1"
          >
            <ArrowLeft size={14} className="mr-2" />
            Back
          </Button>
          <Button
            className="bg-brand hover:bg-brand-hover text-primary-foreground flex-1"
            onClick={() => setPhase("preview")}
          >
            Preview recipe
            <ArrowRight size={14} className="ml-2" />
          </Button>
        </div>
      </div>
    );
  }

  // ── Preview phase ──────────────────────────────────────

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Header */}
      <div style={{ textAlign: "center", padding: "4px 0" }}>
        <p style={{ fontSize: 15, fontWeight: 600, color: "#fff", margin: 0 }}>
          Your Mozza recipe
        </p>
        <p style={{ fontSize: 13, color: "rgba(255,255,255,0.4)", margin: "4px 0 0" }}>
          Generated from your Docker Compose file
        </p>
      </div>

      {/* Recipe preview */}
      <div style={{ position: "relative" }}>
        <pre style={{
          padding: 16,
          background: "rgba(255,255,255,0.03)",
          border: "1px solid rgba(255,255,255,0.08)",
          borderRadius: 12,
          color: "#e0e0e0",
          fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
          fontSize: 13,
          lineHeight: 1.6,
          overflow: "auto",
          maxHeight: 360,
          margin: 0,
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
        }}>
          {recipe}
        </pre>
        <button
          type="button"
          onClick={handleCopy}
          style={{
            position: "absolute", top: 10, right: 10,
            display: "flex", alignItems: "center", gap: 4,
            padding: "4px 8px",
            background: "rgba(255,255,255,0.06)",
            border: "1px solid rgba(255,255,255,0.1)",
            borderRadius: 6, cursor: "pointer",
            fontSize: 11, color: "rgba(255,255,255,0.5)",
            transition: "all 0.15s ease",
          }}
          onMouseEnter={(e) => { e.currentTarget.style.borderColor = "rgba(255,107,53,0.3)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.borderColor = "rgba(255,255,255,0.1)"; }}
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? "Copied" : "Copy"}
        </button>
      </div>

      {/* Actions */}
      <div style={{ display: "flex", gap: 10 }}>
        <Button
          variant="outline"
          onClick={() => setPhase("breakdown")}
          className="flex-1"
        >
          <ArrowLeft size={14} className="mr-2" />
          Back
        </Button>
        <Button
          className="bg-brand hover:bg-brand-hover text-primary-foreground flex-1"
          onClick={handleUseRecipe}
        >
          Use this recipe
          <ArrowRight size={14} className="ml-2" />
        </Button>
      </div>
    </div>
  );
}
