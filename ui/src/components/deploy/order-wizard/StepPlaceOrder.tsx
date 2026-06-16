import { useState, useEffect, useRef, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Search, Plus, Minus, AlertTriangle, ExternalLink } from "lucide-react";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import { listAliases, createAlias, listClusters } from "@/api/client";
import type { ImageAlias } from "@/api/types";
import type { ServiceInterviewState, WizardTarget } from "@/lib/interview-types";

interface StepPlaceOrderProps {
  service: ServiceInterviewState;
  target: WizardTarget;
  teamId: string;
  onUpdateService: (updates: Partial<ServiceInterviewState>) => void;
  onSetTarget: (target: WizardTarget) => void;
}

export function StepPlaceOrder({
  service,
  target,
  teamId,
  onUpdateService,
  onSetTarget,
}: StepPlaceOrderProps) {
  const [aliases, setAliases] = useState<ImageAlias[]>([]);
  const [clusters, setClusters] = useState<{ id: string; name: string }[]>([]);
  const [query, setQuery] = useState(service.aliasName);
  const [showDropdown, setShowDropdown] = useState(false);
  const [showInlineCreate, setShowInlineCreate] = useState(false);
  const [newImageRef, setNewImageRef] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState("");
  const [loadingAliases, setLoadingAliases] = useState(true);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Fetch aliases and clusters on mount
  useEffect(() => {
    let cancelled = false;
    setLoadingAliases(true);
    listAliases(teamId)
      .then((data) => {
        if (!cancelled) setAliases(data ?? []);
      })
      .catch(() => {
        if (!cancelled) setAliases([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingAliases(false);
      });

    listClusters()
      .then((data) => {
        if (!cancelled) {
          const list = (data.clusters ?? []).map((c: { id: string; name: string }) => ({
            id: c.id,
            name: c.name,
          }));
          setClusters(list);
          // Default target: kitchen if clusters exist, local if not
          if (list.length > 0 && !service.aliasName) {
            onSetTarget("kitchen");
          } else if (list.length === 0 && !service.aliasName) {
            onSetTarget("local");
          }
        }
      })
      .catch(() => {
        if (!cancelled) setClusters([]);
      });

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [teamId]);

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  // Filter aliases by query (name OR image URL)
  const filtered = aliases.filter((a) => {
    if (!query) return true;
    const q = query.toLowerCase();
    return a.name.toLowerCase().includes(q) || a.image_ref.toLowerCase().includes(q);
  });

  const exactMatch = aliases.find((a) => a.name.toLowerCase() === query.toLowerCase());

  const handleSelectAlias = useCallback(
    (alias: ImageAlias) => {
      setQuery(alias.name);
      onUpdateService({ aliasName: alias.name, aliasImage: alias.image_ref });
      setShowDropdown(false);
      setShowInlineCreate(false);
    },
    [onUpdateService],
  );

  const handleQueryChange = useCallback(
    (value: string) => {
      setQuery(value);
      setShowDropdown(true);
      setShowInlineCreate(false);
      setCreateError("");
      // Clear selection if query doesn't match current alias
      if (value !== service.aliasName) {
        onUpdateService({ aliasName: "", aliasImage: "" });
      }
    },
    [onUpdateService, service.aliasName],
  );

  const handleShowInlineCreate = useCallback(() => {
    setShowInlineCreate(true);
    setShowDropdown(false);
    setNewImageRef("");
    setCreateError("");
  }, []);

  const handleCreateAlias = useCallback(async () => {
    if (!query.trim() || !newImageRef.trim()) return;
    setCreating(true);
    setCreateError("");
    try {
      await createAlias(teamId, query.trim(), newImageRef.trim());
      const newAlias: ImageAlias = {
        id: "",
        team_id: teamId,
        name: query.trim(),
        image_ref: newImageRef.trim(),
        created_at: new Date().toISOString(),
      };
      setAliases((prev) => [...prev, newAlias]);
      handleSelectAlias(newAlias);
      setShowInlineCreate(false);
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create alias");
    } finally {
      setCreating(false);
    }
  }, [teamId, query, newImageRef, handleSelectAlias]);

  const handleReplicaChange = useCallback(
    (delta: number) => {
      const next = Math.max(1, Math.min(100, service.replicas + delta));
      onUpdateService({ replicas: next });
    },
    [service.replicas, onUpdateService],
  );

  const handleReplicaInput = useCallback(
    (value: string) => {
      const n = parseInt(value, 10);
      if (!isNaN(n)) {
        onUpdateService({ replicas: Math.max(1, Math.min(100, n)) });
      }
    },
    [onUpdateService],
  );

  const hasNoClusters = clusters.length === 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 28 }}>
      {/* Subtitle — title rendered by parent OrderWizard */}
      <p style={{ fontSize: 14, color: "rgba(255,255,255,0.5)", margin: 0 }}>
        Pick an image from the menu or add a new one.
      </p>

      {/* Alias Autocomplete */}
      <div style={{ position: "relative" }} ref={dropdownRef}>
        <label
          style={{
            display: "block",
            fontSize: 13,
            fontWeight: 500,
            color: "rgba(255,255,255,0.7)",
            marginBottom: 6,
          }}
        >
          Saved app
        </label>
        <div style={{ position: "relative" }}>
          <Search
            size={16}
            style={{
              position: "absolute",
              left: 12,
              top: "50%",
              transform: "translateY(-50%)",
              color: "rgba(255,255,255,0.35)",
              pointerEvents: "none",
            }}
          />
          <Input
            ref={inputRef}
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            onFocus={() => setShowDropdown(true)}
            placeholder="Search your saved apps..."
            style={{
              paddingLeft: 36,
              background: "rgba(255,255,255,0.05)",
              border: "1px solid rgba(255,255,255,0.1)",
              color: "#fff",
              fontSize: 14,
            }}
          />
        </div>

        {/* Dropdown */}
        {showDropdown && !showInlineCreate && (
          <div
            style={{
              position: "absolute",
              top: "100%",
              left: 0,
              right: 0,
              marginTop: 4,
              background: "#1a1a1a",
              border: "1px solid rgba(255,255,255,0.1)",
              borderRadius: 8,
              maxHeight: 240,
              overflowY: "auto",
              zIndex: 50,
            }}
          >
            {loadingAliases ? (
              <div
                style={{
                  padding: "12px 16px",
                  color: "rgba(255,255,255,0.4)",
                  fontSize: 13,
                }}
              >
                Loading...
              </div>
            ) : filtered.length > 0 ? (
              filtered.map((alias) => (
                <button
                  key={alias.name}
                  onClick={() => handleSelectAlias(alias)}
                  style={{
                    display: "block",
                    width: "100%",
                    textAlign: "left",
                    padding: "10px 16px",
                    background: "transparent",
                    border: "none",
                    cursor: "pointer",
                    transition: "background 0.15s",
                  }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background =
                      "rgba(255,107,53,0.1)";
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = "transparent";
                  }}
                >
                  <div style={{ fontWeight: 600, color: "#fff", fontSize: 14 }}>
                    {alias.name}
                  </div>
                  <div
                    style={{
                      fontSize: 12,
                      color: "rgba(255,255,255,0.4)",
                      marginTop: 2,
                    }}
                  >
                    {alias.image_ref}
                  </div>
                </button>
              ))
            ) : query.trim() ? (
              <div style={{ padding: "12px 16px" }}>
                <div style={{ color: "rgba(255,255,255,0.5)", fontSize: 13 }}>
                  No matches found.
                </div>
                {!exactMatch && (
                  <button
                    onClick={handleShowInlineCreate}
                    style={{
                      marginTop: 8,
                      background: "transparent",
                      border: "none",
                      color: "#ff6b35",
                      fontSize: 13,
                      cursor: "pointer",
                      padding: 0,
                      fontWeight: 500,
                    }}
                  >
                    + Add &quot;{query.trim()}&quot; as a new alias
                  </button>
                )}
              </div>
            ) : (
              <div
                style={{
                  padding: "12px 16px",
                  color: "rgba(255,255,255,0.4)",
                  fontSize: 13,
                }}
              >
                No saved apps yet. Type a name to create one.
              </div>
            )}
          </div>
        )}
      </div>

      {/* Inline Alias Creation */}
      {showInlineCreate && (
        <div
          style={{
            background: "rgba(255,107,53,0.05)",
            border: "1px solid rgba(255,107,53,0.2)",
            borderRadius: 8,
            padding: 16,
          }}
        >
          <p style={{ color: "rgba(255,255,255,0.7)", fontSize: 13, margin: 0, marginBottom: 12 }}>
            We don&apos;t have <strong style={{ color: "#ff6b35" }}>{query.trim()}</strong> on the
            menu yet &mdash; what&apos;s the full image?
          </p>
          <Input
            value={newImageRef}
            onChange={(e) => setNewImageRef(e.target.value)}
            placeholder="e.g. docker.io/myorg/myapp:latest"
            style={{
              background: "rgba(255,255,255,0.05)",
              border: "1px solid rgba(255,255,255,0.1)",
              color: "#fff",
              fontSize: 14,
              marginBottom: 12,
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreateAlias();
            }}
          />
          {createError && (
            <p style={{ color: "#ef4444", fontSize: 12, margin: "0 0 8px" }}>{createError}</p>
          )}
          <div style={{ display: "flex", gap: 8 }}>
            <Button
              onClick={handleCreateAlias}
              disabled={creating || !newImageRef.trim()}
              style={{
                background: "#ff6b35",
                color: "#fff",
                border: "none",
                fontSize: 13,
                padding: "6px 16px",
              }}
            >
              {creating ? "Adding..." : "Add to menu"}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                setShowInlineCreate(false);
                setShowDropdown(true);
              }}
              style={{ color: "rgba(255,255,255,0.5)", fontSize: 13 }}
            >
              Cancel
            </Button>
          </div>
        </div>
      )}

      {/* Replica Stepper */}
      <div>
        <label
          style={{
            display: "block",
            fontSize: 13,
            fontWeight: 500,
            color: "rgba(255,255,255,0.7)",
            marginBottom: 6,
          }}
        >
          Running copies <HelpTooltip term="replica" />
        </label>
        <div style={{ display: "flex", alignItems: "center", gap: 0 }}>
          <Button
            variant="outline"
            onClick={() => handleReplicaChange(-1)}
            disabled={service.replicas <= 1}
            style={{
              width: 36,
              height: 36,
              padding: 0,
              borderRadius: "8px 0 0 8px",
              borderRight: "none",
              background: "rgba(255,255,255,0.05)",
              borderColor: "rgba(255,255,255,0.1)",
              color: "#fff",
            }}
          >
            <Minus size={14} />
          </Button>
          <Input
            type="number"
            value={service.replicas}
            onChange={(e) => handleReplicaInput(e.target.value)}
            min={1}
            max={100}
            style={{
              width: 64,
              textAlign: "center",
              borderRadius: 0,
              background: "rgba(255,255,255,0.05)",
              border: "1px solid rgba(255,255,255,0.1)",
              color: "#fff",
              fontSize: 14,
              height: 36,
            }}
          />
          <Button
            variant="outline"
            onClick={() => handleReplicaChange(1)}
            disabled={service.replicas >= 100}
            style={{
              width: 36,
              height: 36,
              padding: 0,
              borderRadius: "0 8px 8px 0",
              borderLeft: "none",
              background: "rgba(255,255,255,0.05)",
              borderColor: "rgba(255,255,255,0.1)",
              color: "#fff",
            }}
          >
            <Plus size={14} />
          </Button>
        </div>
      </div>

      {/* Target Toggle */}
      <div>
        <label
          style={{
            display: "block",
            fontSize: 13,
            fontWeight: 500,
            color: "rgba(255,255,255,0.7)",
            marginBottom: 6,
          }}
        >
          Deploy target
        </label>
        <div
          style={{
            display: "flex",
            gap: 0,
            borderRadius: 8,
            overflow: "hidden",
            border: "1px solid rgba(255,255,255,0.1)",
          }}
        >
          <button
            onClick={() => onSetTarget("local")}
            style={{
              flex: 1,
              padding: "10px 16px",
              background:
                target === "local" ? "rgba(255,107,53,0.15)" : "rgba(255,255,255,0.03)",
              border: "none",
              borderRight: "1px solid rgba(255,255,255,0.1)",
              color: target === "local" ? "#ff6b35" : "rgba(255,255,255,0.5)",
              fontSize: 14,
              fontWeight: target === "local" ? 600 : 400,
              cursor: "pointer",
              transition: "all 0.15s",
            }}
          >
            Counter{" "}
            <span style={{ fontSize: 12, opacity: 0.6 }}>(local)</span>
          </button>
          <button
            onClick={() => onSetTarget("kitchen")}
            style={{
              flex: 1,
              padding: "10px 16px",
              background:
                target === "kitchen"
                  ? "rgba(255,107,53,0.15)"
                  : "rgba(255,255,255,0.03)",
              border: "none",
              color: target === "kitchen" ? "#ff6b35" : "rgba(255,255,255,0.5)",
              fontSize: 14,
              fontWeight: target === "kitchen" ? 600 : 400,
              cursor: "pointer",
              transition: "all 0.15s",
            }}
          >
            Kitchen{" "}
            <span style={{ fontSize: 12, opacity: 0.6 }}>(kubernetes)</span>
          </button>
        </div>

        {/* Warning: no clusters */}
        {target === "kitchen" && hasNoClusters && (
          <div
            style={{
              marginTop: 10,
              display: "flex",
              alignItems: "flex-start",
              gap: 8,
              padding: "10px 14px",
              background: "rgba(234,179,8,0.08)",
              border: "1px solid rgba(234,179,8,0.2)",
              borderRadius: 8,
            }}
          >
            <AlertTriangle
              size={16}
              style={{ color: "#eab308", flexShrink: 0, marginTop: 1 }}
            />
            <div style={{ fontSize: 13 }}>
              <span style={{ color: "rgba(255,255,255,0.7)" }}>
                You need to connect a kitchen first.
              </span>
              <a
                href="/app/clusters"
                style={{
                  color: "#ff6b35",
                  marginLeft: 6,
                  textDecoration: "none",
                  fontWeight: 500,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 4,
                }}
              >
                Add cluster <ExternalLink size={12} />
              </a>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
