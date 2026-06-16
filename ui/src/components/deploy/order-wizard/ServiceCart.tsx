import { useState } from "react";
import type { ServiceInterviewState, DependencyConfig } from "@/lib/interview-types";
import { Database, Zap, Trash2, Plus, Server, Globe, Cog } from "lucide-react";

// ─── Props ──────────────────────────────────────────────────

interface ServiceCartProps {
  services: ServiceInterviewState[];
  activeServiceIndex: number;
  onSelectService: (index: number) => void;
  onAddService: () => void;
  onDeleteService: (index: number) => void;
  readOnly: boolean;
}

// ─── Helpers ────────────────────────────────────────────────

function getServiceIcon(service: ServiceInterviewState) {
  if (service.traits.includes("web-facing")) return Globe;
  if (service.traits.includes("stateful")) return Database;
  if (service.traits.includes("worker")) return Cog;
  return Server;
}

function getEnabledDeps(service: ServiceInterviewState): DependencyConfig[] {
  return service.dependencies.filter((d) => d.enabled);
}

function depIcon(type: string) {
  if (type === "database") return Database;
  if (type === "cache") return Zap;
  return Server;
}

function depLabel(dep: DependencyConfig): string {
  if (dep.type === "database") return dep.engine || "Database";
  if (dep.type === "cache") return "Redis";
  return dep.engine || dep.type;
}

// ─── Component ──────────────────────────────────────────────

export function ServiceCart({
  services,
  activeServiceIndex,
  onSelectService,
  onAddService,
  onDeleteService,
  readOnly,
}: ServiceCartProps) {
  const [confirmDelete, setConfirmDelete] = useState<number | null>(null);

  // Collect all enabled dependencies across all services
  const allDeps: Array<{ dep: DependencyConfig; serviceIdx: number }> = [];
  services.forEach((svc, i) => {
    getEnabledDeps(svc).forEach((dep) => {
      // Avoid duplicate deps (shared DB across services)
      const isDuplicate = allDeps.some(
        (d) => d.dep.type === dep.type && d.dep.engine === dep.engine,
      );
      if (!isDuplicate) allDeps.push({ dep, serviceIdx: i });
    });
  });

  const totalItems = services.length;

  return (
    <div
      style={{
        width: 200,
        flexShrink: 0,
        display: "flex",
        flexDirection: "column",
        gap: 0,
      }}
    >
      {/* Header */}
      <div
        style={{
          fontSize: 13,
          fontWeight: 600,
          color: "rgba(255,255,255,0.6)",
          padding: "0 4px 10px",
        }}
      >
        Your order ({totalItems} {totalItems === 1 ? "item" : "items"})
      </div>

      {/* Service cards */}
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        {services.map((svc, i) => {
          const Icon = getServiceIcon(svc);
          const isActive = i === activeServiceIndex;

          return (
            <div
              key={i}
              onClick={() => !readOnly && onSelectService(i)}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                padding: "8px 10px",
                borderRadius: 8,
                background: isActive ? "rgba(255,107,53,0.1)" : "rgba(255,255,255,0.02)",
                border: isActive
                  ? "1px solid rgba(255,107,53,0.3)"
                  : "1px solid rgba(255,255,255,0.06)",
                cursor: readOnly ? "default" : "pointer",
                transition: "all 0.15s ease",
              }}
            >
              <Icon
                style={{
                  width: 14,
                  height: 14,
                  color: isActive ? "#ff6b35" : "rgba(255,255,255,0.35)",
                  flexShrink: 0,
                }}
              />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div
                  style={{
                    fontSize: 13,
                    fontWeight: isActive ? 600 : 400,
                    color: isActive ? "#fff" : "rgba(255,255,255,0.65)",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {svc.aliasName || `Service ${i + 1}`}
                </div>
                {svc.replicas > 1 && (
                  <div style={{ fontSize: 11, color: "rgba(255,255,255,0.35)" }}>
                    ×{svc.replicas} copies
                  </div>
                )}
              </div>

              {/* Delete button */}
              {!readOnly && services.length > 1 && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    if (confirmDelete === i) {
                      onDeleteService(i);
                      setConfirmDelete(null);
                    } else {
                      setConfirmDelete(i);
                    }
                  }}
                  onBlur={() => setConfirmDelete(null)}
                  style={{
                    background: "none",
                    border: "none",
                    padding: 2,
                    cursor: "pointer",
                    color: confirmDelete === i ? "#ef4444" : "rgba(255,255,255,0.2)",
                    flexShrink: 0,
                  }}
                  title={confirmDelete === i ? "Click again to confirm" : "Remove service"}
                >
                  <Trash2 style={{ width: 12, height: 12 }} />
                </button>
              )}
            </div>
          );
        })}
      </div>

      {/* Dependency "sides" */}
      {allDeps.length > 0 && (
        <div style={{ marginTop: 8 }}>
          <div
            style={{
              fontSize: 11,
              color: "rgba(255,255,255,0.3)",
              padding: "0 4px 4px",
              textTransform: "uppercase",
              letterSpacing: "0.5px",
            }}
          >
            Sides
          </div>
          {allDeps.map(({ dep }, i) => {
            const DepIcon = depIcon(dep.type);
            return (
              <div
                key={`dep-${i}`}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  padding: "5px 10px",
                  borderRadius: 6,
                  background: "rgba(255,255,255,0.01)",
                  border: "1px solid rgba(255,255,255,0.04)",
                  marginBottom: 3,
                }}
              >
                <DepIcon
                  style={{ width: 12, height: 12, color: "rgba(255,255,255,0.25)" }}
                />
                <span style={{ fontSize: 12, color: "rgba(255,255,255,0.4)", flex: 1 }}>
                  {depLabel(dep)}
                </span>
                <span
                  style={{
                    fontSize: 10,
                    color: "rgba(255,255,255,0.25)",
                    background: "rgba(255,255,255,0.05)",
                    padding: "1px 5px",
                    borderRadius: 3,
                  }}
                >
                  side
                </span>
              </div>
            );
          })}
        </div>
      )}

      {/* Add service button */}
      {!readOnly && (
        <button
          onClick={onAddService}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 6,
            marginTop: 10,
            padding: "8px 0",
            background: "none",
            border: "1px dashed rgba(255,255,255,0.12)",
            borderRadius: 8,
            color: "rgba(255,255,255,0.35)",
            cursor: "pointer",
            fontSize: 12,
            transition: "all 0.15s ease",
          }}
        >
          <Plus style={{ width: 12, height: 12 }} />
          Add another item
        </button>
      )}
    </div>
  );
}
