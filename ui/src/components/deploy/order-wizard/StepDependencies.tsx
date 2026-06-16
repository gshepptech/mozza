import { Input } from "@/components/ui/input";
import { Database, Zap, Check } from "lucide-react";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import type { DependencyConfig, Trait } from "@/lib/interview-types";

interface StepDependenciesProps {
  traits: Trait[];
  dependencies: DependencyConfig[];
  onUpdate: (deps: DependencyConfig[]) => void;
}

const DB_ENGINES = [
  { id: "postgres", label: "PostgreSQL", defaultVersion: "16" },
  { id: "mysql", label: "MySQL", defaultVersion: "8" },
  { id: "mongo", label: "MongoDB", defaultVersion: "7" },
];

export default function StepDependencies({ traits, dependencies, onUpdate }: StepDependenciesProps) {
  const isStateful = traits.includes("stateful");

  const dbDep = dependencies.find((d) => d.type === "database") ?? {
    type: "database" as const,
    enabled: false,
    engine: "postgres",
    version: "16",
    storage: "10Gi",
  };
  const cacheDep = dependencies.find((d) => d.type === "cache") ?? {
    type: "cache" as const,
    enabled: false,
    engine: "redis",
    version: "7",
    storage: "",
  };

  // If stateful trait, pre-select database
  const effectiveDbEnabled = dbDep.enabled || isStateful;

  const updateDep = (type: "database" | "cache", patch: Partial<DependencyConfig>) => {
    const updated = dependencies.map((d) => (d.type === type ? { ...d, ...patch } : d));
    onUpdate(updated);
  };

  const toggleDb = () => {
    updateDep("database", { enabled: !effectiveDbEnabled });
  };

  const toggleCache = () => {
    updateDep("cache", { enabled: !cacheDep.enabled });
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <p style={{ color: "rgba(255,255,255,0.5)", fontSize: 14, margin: 0 }}>
        Toggle any services your app needs. We'll wire them up automatically.
      </p>

      {/* Database Card */}
      <button
        type="button"
        onClick={toggleDb}
        style={{
          display: "flex",
          flexDirection: "column",
          padding: 20,
          background: "rgba(255,255,255,0.03)",
          border: `1px solid ${effectiveDbEnabled ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
          borderRadius: 10,
          cursor: "pointer",
          position: "relative",
          transition: "border-color 0.15s ease",
          textAlign: "left",
        }}
      >
        {effectiveDbEnabled && (
          <span
            style={{
              position: "absolute",
              top: 8,
              right: 8,
              width: 20,
              height: 20,
              borderRadius: "50%",
              background: "#ff6b35",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Check size={12} color="#fff" />
          </span>
        )}
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <Database size={24} color={effectiveDbEnabled ? "#ff6b35" : "rgba(255,255,255,0.5)"} />
          <div>
            <span style={{ color: effectiveDbEnabled ? "#fff" : "rgba(255,255,255,0.7)", fontSize: 15, fontWeight: 500 }}>
              Database
            </span>
            <p style={{ color: "rgba(255,255,255,0.4)", fontSize: 13, margin: "4px 0 0" }}>
              Persistent data store for your app
            </p>
          </div>
        </div>
      </button>

      {/* Database expanded config */}
      {effectiveDbEnabled && (
        <div
          style={{
            marginTop: -8,
            padding: 20,
            background: "rgba(255,255,255,0.02)",
            border: "1px solid rgba(255,255,255,0.08)",
            borderRadius: 10,
            display: "flex",
            flexDirection: "column",
            gap: 16,
          }}
        >
          {isStateful && !dbDep.enabled && (
            <p style={{ color: "#ff6b35", fontSize: 13, margin: 0, fontStyle: "italic" }}>
              Since your app stores data, you probably need a database.
            </p>
          )}

          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 13, marginBottom: 8, display: "block" }}>
              Engine
            </label>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
              {DB_ENGINES.map((eng) => {
                const selected = dbDep.engine === eng.id;
                return (
                  <button
                    key={eng.id}
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      updateDep("database", { engine: eng.id, version: eng.defaultVersion });
                    }}
                    style={{
                      padding: "10px 12px",
                      background: selected ? "rgba(255,107,53,0.15)" : "rgba(255,255,255,0.03)",
                      border: `1px solid ${selected ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
                      borderRadius: 8,
                      color: selected ? "#ff6b35" : "rgba(255,255,255,0.7)",
                      fontSize: 14,
                      cursor: "pointer",
                      transition: "all 0.15s ease",
                    }}
                  >
                    {eng.label}
                  </button>
                );
              })}
            </div>
            <p className="text-[10px] text-muted-foreground/40 mt-1.5">Not sure? PostgreSQL works for almost everything.</p>
          </div>

          <div>
            <label style={{ color: "rgba(255,255,255,0.6)", fontSize: 13, marginBottom: 8, display: "block" }}>
              Storage size <HelpTooltip term="storage-gi" />
            </label>
            <div style={{ display: "flex", alignItems: "center" }}>
              <Input
                value={dbDep.storage}
                onClick={(e) => e.stopPropagation()}
                onChange={(e) => updateDep("database", { storage: e.target.value })}
                placeholder="10"
                style={{ maxWidth: 160 }}
              />
              <span className="text-xs text-muted-foreground ml-1">GB</span>
            </div>
            <p className="text-[10px] text-muted-foreground/40 mt-1">5 GB for prototypes, 10 GB for most apps, 50 GB for large datasets</p>
          </div>

          <p style={{ color: "rgba(255,255,255,0.35)", fontSize: 12, margin: 0 }}>
            A <code style={{ color: "rgba(255,255,255,0.5)" }}>DATABASE_URL</code> env var will be generated automatically.
          </p>
        </div>
      )}

      {/* Cache Card */}
      <button
        type="button"
        onClick={toggleCache}
        style={{
          display: "flex",
          flexDirection: "column",
          padding: 20,
          background: "rgba(255,255,255,0.03)",
          border: `1px solid ${cacheDep.enabled ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
          borderRadius: 10,
          cursor: "pointer",
          position: "relative",
          transition: "border-color 0.15s ease",
          textAlign: "left",
        }}
      >
        {cacheDep.enabled && (
          <span
            style={{
              position: "absolute",
              top: 8,
              right: 8,
              width: 20,
              height: 20,
              borderRadius: "50%",
              background: "#ff6b35",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Check size={12} color="#fff" />
          </span>
        )}
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <Zap size={24} color={cacheDep.enabled ? "#ff6b35" : "rgba(255,255,255,0.5)"} />
          <div>
            <span style={{ color: cacheDep.enabled ? "#fff" : "rgba(255,255,255,0.7)", fontSize: 15, fontWeight: 500 }}>
              Add fast storage (sessions, counters) <HelpTooltip term="cache-redis" />
            </span>
            <p style={{ color: "rgba(255,255,255,0.4)", fontSize: 13, margin: "4px 0 0" }}>
              Fast temporary storage — makes your app respond quicker by remembering recent data
            </p>
          </div>
        </div>
      </button>

      {/* Message Queue — hidden for now */}
      {/* AC-5.1: Queue card hidden for future implementation */}
    </div>
  );
}
