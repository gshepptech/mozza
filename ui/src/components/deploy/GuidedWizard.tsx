import { useState, useCallback } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  Globe, Server, Cog, Clock, Database as DatabaseIcon, Layers,
  ArrowLeft, ArrowRight, Check, Settings, Plus, Trash2,
  CheckCircle, Copy,
} from "lucide-react";

// ─── Types ───────────────────────────────────────────────────

type DeployType = "webapp" | "api" | "worker" | "scheduled" | "database" | "fullstack";

interface GuidedService {
  name: string;
  image: string;
  port: number;
  isPublic: boolean;
  kind: string;
  schedule: string;
  engine: string;
  storage: string;
  backups: boolean;
  needsDb: boolean;
  dbEngine: string;
  needsCache: boolean;
  grpcEnabled: boolean;
  grpcPort: number;
  runMode: "forever" | "schedule";
}

interface GuidedWizardProps {
  onComplete: (recipeName: string, recipeSource: string) => void;
  onAdvancedMode: () => void;
}

// ─── Constants ───────────────────────────────────────────────

const DEPLOY_TYPES: Array<{
  id: DeployType;
  icon: typeof Globe;
  title: string;
  description: string;
}> = [
  {
    id: "fullstack",
    icon: Layers,
    title: "App with services",
    description: "A typical app — web frontend, API, database, workers. Pick what you need in the next step.",
  },
  {
    id: "webapp",
    icon: Globe,
    title: "Single web service",
    description: "One container that serves HTTP traffic (website, API, or both)",
  },
  {
    id: "database",
    icon: DatabaseIcon,
    title: "Database only",
    description: "Spin up PostgreSQL, MySQL, or MongoDB",
  },
  {
    id: "worker",
    icon: Cog,
    title: "Background process",
    description: "A worker or scheduled job that runs behind the scenes",
  },
];

const DB_OPTIONS = [
  { id: "postgres", label: "PostgreSQL", image: "postgres:16-alpine", port: 5432 },
  { id: "mysql", label: "MySQL", image: "mysql:8", port: 3306 },
  { id: "mongo", label: "MongoDB", image: "mongo:7", port: 27017 },
];

const SCHEDULE_PRESETS = [
  { id: "hourly", label: "Every hour", cron: "0 * * * *" },
  { id: "daily", label: "Every day at midnight", cron: "0 0 * * *" },
  { id: "weekly", label: "Every Monday at midnight", cron: "0 0 * * 1" },
  { id: "custom", label: "Custom cron", cron: "" },
];

// ─── Helpers ─────────────────────────────────────────────────

function makeDefaultService(deployType: DeployType): GuidedService {
  const base: GuidedService = {
    name: "",
    image: "",
    port: 8080,
    isPublic: true,
    kind: "web",
    schedule: "",
    engine: "",
    storage: "",
    backups: false,
    needsDb: false,
    dbEngine: "postgres",
    needsCache: false,
    grpcEnabled: false,
    grpcPort: 9090,
    runMode: "forever",
  };

  switch (deployType) {
    case "webapp":
    case "api":
      return { ...base, kind: "web", isPublic: true, port: 8080 };
    case "worker":
      return { ...base, kind: "worker", port: 0, isPublic: false, runMode: "forever" };
    case "scheduled":
      return { ...base, kind: "scheduled", port: 0, isPublic: false, schedule: "0 * * * *" };
    case "database":
      return { ...base, kind: "database", engine: "postgres", storage: "10Gi", isPublic: false, port: 5432, image: "postgres:16-alpine" };
    case "fullstack":
      return { ...base, kind: "web", isPublic: true };
    default:
      return base;
  }
}

function generateRecipeSource(appName: string, services: GuidedService[]): string {
  const lines: string[] = [`App: ${appName}`, ""];

  for (const svc of services) {
    const svcName = svc.name || svc.kind;
    lines.push(`${svcName.charAt(0).toUpperCase() + svcName.slice(1)}:`);

    if (svc.kind === "database" && svc.engine) {
      const parts = [svc.engine];
      if (svc.storage) parts.push(svc.storage);
      if (svc.backups) parts.push("daily backups");
      lines.push(`  ${parts.join(", ")}`);
    } else if (svc.image) {
      lines.push(`  from image ${svc.image}`);
    }

    if (svc.isPublic && svc.port) {
      lines.push(`  open to the public on port ${svc.port}`);
    } else if (svc.port && !["worker", "scheduled", "database"].includes(svc.kind)) {
      lines.push(`  on port ${svc.port}`);
    }

    if (svc.grpcEnabled && svc.grpcPort) {
      lines.push(`  on port ${svc.grpcPort} as grpc`);
    }

    if (svc.kind === "scheduled" && svc.schedule) {
      lines.push(`  run every "${svc.schedule}"`);
    }

    if (svc.kind === "worker" && svc.runMode === "schedule" && svc.schedule) {
      lines.push(`  run every "${svc.schedule}"`);
    }

    // Dependencies
    const needs: string[] = [];
    if (svc.needsDb) needs.push(svc.dbEngine || "database");
    if (svc.needsCache) needs.push("cache");
    if (needs.length > 0) {
      lines.push(`  needs ${needs.join(" and ")}`);
    }

    lines.push("");
  }

  // Add extra services for databases/caches that were requested
  const addedExtras = new Set<string>();
  for (const svc of services) {
    if (svc.needsDb && !addedExtras.has(svc.dbEngine)) {
      const db = DB_OPTIONS.find((d) => d.id === svc.dbEngine);
      if (db) {
        addedExtras.add(svc.dbEngine);
        lines.push(`${db.label.charAt(0).toUpperCase() + db.label.slice(1)}:`);
        lines.push(`  ${db.id}, 10Gi`);
        lines.push("");
      }
    }
    if (svc.needsCache && !addedExtras.has("redis")) {
      addedExtras.add("redis");
      lines.push("Cache:");
      lines.push("  redis");
      lines.push("");
    }
  }

  return lines.join("\n").trimEnd();
}

// ─── Sub-components ──────────────────────────────────────────

function TypeCard({
  item,
  active,
  onClick,
}: {
  item: typeof DEPLOY_TYPES[number];
  active: boolean;
  onClick: () => void;
}) {
  const Icon = item.icon;
  return (
    <Card
      className={cn(
        "oven-card feature-card bg-card cursor-pointer transition-all duration-200",
        active
          ? "border-brand/50 shadow-[0_0_25px_rgba(255,107,53,0.12)] bg-brand-muted"
          : "border-border/50 hover:border-brand/25 hover:shadow-[0_0_15px_rgba(255,107,53,0.06)]",
      )}
      onClick={onClick}
    >
      <CardContent className="p-6 flex items-start gap-5">
        <div
          className={cn(
            "flex h-12 w-12 shrink-0 items-center justify-center rounded-xl transition-colors",
            active
              ? "bg-brand text-primary-foreground"
              : "bg-muted text-muted-foreground",
          )}
        >
          <Icon className="h-6 w-6" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-bold text-foreground">{item.title}</p>
          <p className="text-xs text-muted-foreground mt-1.5 leading-relaxed">
            {item.description}
          </p>
        </div>
        {active && (
          <CheckCircle className="h-5 w-5 shrink-0 text-brand mt-1" />
        )}
      </CardContent>
    </Card>
  );
}

function ToggleSwitch({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      className="flex items-center justify-between w-full py-3 px-4 rounded-xl border border-border/50 bg-card hover:border-brand/25 transition-colors"
      onClick={() => onChange(!checked)}
    >
      <span className="text-sm text-foreground">{label}</span>
      <div
        className={cn(
          "relative w-11 h-6 rounded-full transition-colors",
          checked ? "bg-brand" : "bg-muted",
        )}
      >
        <div
          className={cn(
            "absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform shadow-sm",
            checked ? "translate-x-[22px]" : "translate-x-0.5",
          )}
        />
      </div>
    </button>
  );
}

function DbEnginePicker({
  selected,
  onSelect,
}: {
  selected: string;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="grid grid-cols-3 gap-2">
      {DB_OPTIONS.map((db) => (
        <button
          key={db.id}
          type="button"
          className={cn(
            "rounded-xl border px-3 py-3 text-center transition-all",
            selected === db.id
              ? "border-brand/50 bg-brand-muted text-foreground shadow-[0_0_15px_rgba(255,107,53,0.08)]"
              : "border-border/50 bg-card text-muted-foreground hover:border-brand/25",
          )}
          onClick={() => onSelect(db.id)}
        >
          <DatabaseIcon className="h-4 w-4 mx-auto mb-1.5" />
          <span className="text-xs font-semibold block">{db.label}</span>
        </button>
      ))}
    </div>
  );
}

function SchedulePicker({
  schedule,
  onChange,
}: {
  schedule: string;
  onChange: (s: string) => void;
}) {
  const activePreset = SCHEDULE_PRESETS.find((p) => p.cron === schedule);
  const isCustom = !activePreset || activePreset.id === "custom";

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2">
        {SCHEDULE_PRESETS.map((preset) => (
          <button
            key={preset.id}
            type="button"
            className={cn(
              "rounded-xl border px-3 py-3 text-left transition-all",
              (preset.id === "custom" ? isCustom && schedule !== "" && !SCHEDULE_PRESETS.slice(0, -1).some((p) => p.cron === schedule) : preset.cron === schedule)
                ? "border-brand/50 bg-brand-muted text-foreground"
                : "border-border/50 bg-card text-muted-foreground hover:border-brand/25",
            )}
            onClick={() => {
              if (preset.id === "custom") {
                onChange("");
              } else {
                onChange(preset.cron);
              }
            }}
          >
            <Clock className="h-3.5 w-3.5 mb-1" />
            <span className="text-xs font-semibold block">{preset.label}</span>
          </button>
        ))}
      </div>
      {(isCustom || (!activePreset && schedule !== "")) && (
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Cron expression</Label>
          <Input
            value={schedule}
            onChange={(e) => onChange(e.target.value)}
            placeholder="e.g. */30 * * * *"
            className="h-10 text-sm bg-elevated font-mono"
          />
        </div>
      )}
    </div>
  );
}

// ─── Step content renderers ──────────────────────────────────

function StepDetails({
  deployType,
  service,
  onChange,
}: {
  deployType: DeployType;
  service: GuidedService;
  onChange: (s: GuidedService) => void;
}) {
  const update = (patch: Partial<GuidedService>) => onChange({ ...service, ...patch });

  if (deployType === "database") {
    return (
      <div className="space-y-5">
        <div>
          <Label className="text-sm text-foreground font-medium mb-3 block">
            Which database?
          </Label>
          <DbEnginePicker
            selected={service.engine}
            onSelect={(id) => {
              const db = DB_OPTIONS.find((d) => d.id === id);
              update({
                engine: id,
                image: db?.image || "",
                port: db?.port || 5432,
                name: id,
              });
            }}
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Storage size</Label>
          <Input
            value={service.storage}
            onChange={(e) => update({ storage: e.target.value })}
            placeholder="e.g. 10Gi"
            className="h-10 text-sm bg-elevated"
          />
        </div>
        <ToggleSwitch
          label="Enable daily backups?"
          checked={service.backups}
          onChange={(backups) => update({ backups })}
        />
      </div>
    );
  }

  if (deployType === "webapp" || deployType === "api") {
    return (
      <div className="space-y-5">
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Container image</Label>
          <Input
            value={service.image}
            onChange={(e) => update({ image: e.target.value })}
            placeholder="e.g. nginx:alpine, myorg/api:latest"
            className="h-10 text-sm bg-elevated font-mono"
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">What port does it listen on?</Label>
          <Input
            type="number"
            value={service.port || ""}
            onChange={(e) => update({ port: parseInt(e.target.value) || 0 })}
            placeholder="8080"
            className="h-10 text-sm bg-elevated"
          />
        </div>
        <ToggleSwitch
          label="Should it be accessible from the internet?"
          checked={service.isPublic}
          onChange={(isPublic) => update({ isPublic })}
        />
        <ToggleSwitch
          label="Need a database?"
          checked={service.needsDb}
          onChange={(needsDb) => update({ needsDb })}
        />
        {service.needsDb && (
          <DbEnginePicker
            selected={service.dbEngine}
            onSelect={(id) => update({ dbEngine: id })}
          />
        )}
        <ToggleSwitch
          label="Need a cache (Redis)?"
          checked={service.needsCache}
          onChange={(needsCache) => update({ needsCache })}
        />
      </div>
    );
  }

  if (deployType === "worker") {
    return (
      <div className="space-y-5">
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Container image</Label>
          <Input
            value={service.image}
            onChange={(e) => update({ image: e.target.value })}
            placeholder="e.g. myorg/worker:latest"
            className="h-10 text-sm bg-elevated font-mono"
          />
        </div>
        <div>
          <Label className="text-sm text-foreground font-medium mb-3 block">
            Runs forever or on a schedule?
          </Label>
          <div className="grid grid-cols-2 gap-3">
            {(["forever", "schedule"] as const).map((mode) => (
              <Card
                key={mode}
                className={cn(
                  "oven-card cursor-pointer transition-all",
                  service.runMode === mode
                    ? "border-brand/50 bg-brand-muted"
                    : "border-border/50 hover:border-brand/25",
                )}
                onClick={() => update({ runMode: mode, kind: mode === "schedule" ? "scheduled" : "worker" })}
              >
                <CardContent className="p-4 text-center">
                  {mode === "forever" ? (
                    <Cog className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
                  ) : (
                    <Clock className="h-5 w-5 mx-auto mb-2 text-muted-foreground" />
                  )}
                  <p className="text-sm font-semibold text-foreground">
                    {mode === "forever" ? "Runs forever" : "On a schedule"}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
        {service.runMode === "schedule" && (
          <SchedulePicker
            schedule={service.schedule}
            onChange={(schedule) => update({ schedule })}
          />
        )}
      </div>
    );
  }

  if (deployType === "scheduled") {
    return (
      <div className="space-y-5">
        <div className="space-y-1.5">
          <Label className="text-xs text-muted-foreground">Container image</Label>
          <Input
            value={service.image}
            onChange={(e) => update({ image: e.target.value })}
            placeholder="e.g. myorg/cron-job:latest"
            className="h-10 text-sm bg-elevated font-mono"
          />
        </div>
        <div>
          <Label className="text-sm text-foreground font-medium mb-3 block">
            How often should it run?
          </Label>
          <SchedulePicker
            schedule={service.schedule}
            onChange={(schedule) => update({ schedule })}
          />
        </div>
        <ToggleSwitch
          label="Need a database?"
          checked={service.needsDb}
          onChange={(needsDb) => update({ needsDb })}
        />
        {service.needsDb && (
          <DbEnginePicker
            selected={service.dbEngine}
            onSelect={(id) => update({ dbEngine: id })}
          />
        )}
      </div>
    );
  }

  // fullstack — handled separately
  return null;
}

function FullStackDetails({
  services,
  onChange,
}: {
  services: GuidedService[];
  onChange: (svcs: GuidedService[]) => void;
}) {
  const SERVICE_TYPES = [
    { id: "web", label: "Web App", icon: Globe },
    { id: "api", label: "API", icon: Server },
    { id: "worker", label: "Worker", icon: Cog },
  ];

  const addService = () => {
    const idx = services.length + 1;
    onChange([
      ...services,
      {
        ...makeDefaultService("api"),
        name: `service-${idx}`,
        kind: "web",
      },
    ]);
  };

  const removeService = (index: number) => {
    onChange(services.filter((_, i) => i !== index));
  };

  const updateService = (index: number, patch: Partial<GuidedService>) => {
    onChange(services.map((s, i) => (i === index ? { ...s, ...patch } : s)));
  };

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">
        Add the services that make up your app. You can configure each one individually.
      </p>

      {services.map((svc, idx) => (
        <Card key={idx} className="oven-card bg-card border-border/50">
          <CardContent className="p-4 space-y-3">
            <div className="flex items-center justify-between">
              <span className="text-sm font-bold text-foreground">
                Service {idx + 1}
              </span>
              {services.length > 1 && (
                <button
                  type="button"
                  className="text-muted-foreground/50 hover:text-red-400 transition-colors"
                  onClick={() => removeService(idx)}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              )}
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Service name</Label>
              <Input
                value={svc.name}
                onChange={(e) => updateService(idx, { name: e.target.value })}
                placeholder="e.g. frontend, api, worker"
                className="h-9 text-sm bg-elevated"
              />
            </div>

            <div className="flex gap-2">
              {SERVICE_TYPES.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className={cn(
                    "flex-1 rounded-lg border px-2 py-2 text-center transition-all text-xs",
                    svc.kind === t.id
                      ? "border-brand/50 bg-brand-muted text-foreground"
                      : "border-border/50 text-muted-foreground hover:border-brand/25",
                  )}
                  onClick={() => updateService(idx, {
                    kind: t.id,
                    isPublic: t.id === "web",
                    port: t.id === "worker" ? 0 : 8080,
                  })}
                >
                  <t.icon className="h-3.5 w-3.5 mx-auto mb-1" />
                  {t.label}
                </button>
              ))}
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Container image</Label>
              <Input
                value={svc.image}
                onChange={(e) => updateService(idx, { image: e.target.value })}
                placeholder="e.g. myorg/service:latest"
                className="h-9 text-sm bg-elevated font-mono"
              />
            </div>

            {svc.kind !== "worker" && (
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Port</Label>
                <Input
                  type="number"
                  value={svc.port || ""}
                  onChange={(e) => updateService(idx, { port: parseInt(e.target.value) || 0 })}
                  placeholder="8080"
                  className="h-9 text-sm bg-elevated"
                />
              </div>
            )}
          </CardContent>
        </Card>
      ))}

      {services.length < 10 && (
        <Button
          variant="outline"
          className="w-full border-dashed"
          onClick={addService}
        >
          <Plus className="mr-2 h-4 w-4" />
          Add another service
        </Button>
      )}
    </div>
  );
}

function StepExtras({
  services,
  onChange,
  deployType,
}: {
  services: GuidedService[];
  onChange: (svcs: GuidedService[]) => void;
  deployType: DeployType;
}) {
  // Determine what's already present
  const hasDb = services.some((s) => s.needsDb || s.kind === "database");
  const hasCache = services.some((s) => s.needsCache);
  const hasWorker = services.some((s) => s.kind === "worker");
  const hasScheduled = services.some((s) => s.kind === "scheduled");

  const updateFirstService = (patch: Partial<GuidedService>) => {
    onChange(services.map((s, i) => (i === 0 ? { ...s, ...patch } : s)));
  };

  const addExtraService = (kind: string) => {
    const extra = makeDefaultService(kind === "worker" ? "worker" : "scheduled");
    extra.name = kind === "worker" ? "worker" : "cron";
    onChange([...services, extra]);
  };

  const removeExtraByKind = (kind: string) => {
    // Remove last service of this kind
    const idx = [...services].reverse().findIndex((s) => s.kind === kind);
    if (idx >= 0) {
      const realIdx = services.length - 1 - idx;
      onChange(services.filter((_, i) => i !== realIdx));
    }
  };

  // Hide extras step for database-only deployments
  if (deployType === "database") return null;

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground mb-2">
        Add any extras your app needs. These are optional.
      </p>

      {!hasDb && (
        <div className="space-y-3">
          <ToggleSwitch
            label="Add a database"
            checked={false}
            onChange={() => updateFirstService({ needsDb: true })}
          />
        </div>
      )}
      {hasDb && (
        <div className="rounded-xl border border-brand/20 bg-brand-muted p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-foreground flex items-center gap-2">
              <DatabaseIcon className="h-4 w-4 text-brand" />
              Database
            </span>
            <button
              type="button"
              className="text-xs text-muted-foreground hover:text-red-400"
              onClick={() => updateFirstService({ needsDb: false })}
            >
              Remove
            </button>
          </div>
          <DbEnginePicker
            selected={services[0]?.dbEngine || "postgres"}
            onSelect={(id) => updateFirstService({ dbEngine: id })}
          />
        </div>
      )}

      {!hasCache && (
        <ToggleSwitch
          label="Add a cache (Redis)"
          checked={false}
          onChange={() => updateFirstService({ needsCache: true })}
        />
      )}
      {hasCache && (
        <div className="flex items-center justify-between rounded-xl border border-brand/20 bg-brand-muted p-3">
          <span className="text-sm font-medium text-foreground flex items-center gap-2">
            <Badge variant="outline" className="text-xs">Redis</Badge>
            Cache
          </span>
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-red-400"
            onClick={() => updateFirstService({ needsCache: false })}
          />
        </div>
      )}

      {!hasWorker && deployType !== "worker" && (
        <ToggleSwitch
          label="Add background workers"
          checked={false}
          onChange={() => addExtraService("worker")}
        />
      )}
      {hasWorker && deployType !== "worker" && (
        <div className="flex items-center justify-between rounded-xl border border-brand/20 bg-brand-muted p-3">
          <span className="text-sm font-medium text-foreground flex items-center gap-2">
            <Cog className="h-4 w-4 text-brand" />
            Background worker
          </span>
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-red-400"
            onClick={() => removeExtraByKind("worker")}
          >
            Remove
          </button>
        </div>
      )}

      {!hasScheduled && deployType !== "scheduled" && (
        <ToggleSwitch
          label="Add scheduled jobs"
          checked={false}
          onChange={() => addExtraService("scheduled")}
        />
      )}
      {hasScheduled && deployType !== "scheduled" && (
        <div className="flex items-center justify-between rounded-xl border border-brand/20 bg-brand-muted p-3">
          <span className="text-sm font-medium text-foreground flex items-center gap-2">
            <Clock className="h-4 w-4 text-brand" />
            Scheduled job
          </span>
          <button
            type="button"
            className="text-xs text-muted-foreground hover:text-red-400"
            onClick={() => removeExtraByKind("scheduled")}
          >
            Remove
          </button>
        </div>
      )}
    </div>
  );
}

function StepReview({
  recipeSource,
  onEdit,
}: {
  recipeSource: string;
  onEdit: (s: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState(recipeSource);
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(recipeSource).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (editing) {
    return (
      <div className="space-y-3">
        <Label className="text-xs text-muted-foreground">Edit recipe</Label>
        <textarea
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          className="w-full h-64 rounded-xl border border-border/50 bg-elevated p-4 font-mono text-xs text-foreground resize-none focus:outline-none focus:border-brand/50"
        />
        <div className="flex gap-2">
          <Button
            className="bg-brand hover:bg-brand-hover text-primary-foreground"
            onClick={() => {
              onEdit(editValue);
              setEditing(false);
            }}
          >
            Save changes
          </Button>
          <Button
            variant="outline"
            onClick={() => {
              setEditValue(recipeSource);
              setEditing(false);
            }}
          >
            Cancel
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="relative rounded-xl border border-border/50 bg-elevated overflow-hidden">
        <div className="flex items-center justify-between px-4 py-2 border-b border-border/50 bg-muted/30">
          <span className="text-xs font-medium text-muted-foreground">
            Generated recipe
          </span>
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
              onClick={handleCopy}
            >
              {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
              {copied ? "Copied" : "Copy"}
            </button>
          </div>
        </div>
        <pre className="p-4 font-mono text-xs text-foreground leading-relaxed whitespace-pre-wrap overflow-auto max-h-72">
          {recipeSource}
        </pre>
      </div>

      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setEditValue(recipeSource);
            setEditing(true);
          }}
        >
          <Settings className="mr-1.5 h-3.5 w-3.5" />
          Edit recipe
        </Button>
      </div>
    </div>
  );
}

// ─── Main GuidedWizard ──────────────────────────────────────

export default function GuidedWizard({ onComplete, onAdvancedMode }: GuidedWizardProps) {
  const [step, setStep] = useState(1);
  const [deployType, setDeployType] = useState<DeployType | null>(null);
  const [appName, setAppName] = useState("");
  const [services, setServices] = useState<GuidedService[]>([]);
  const [recipeSource, setRecipeSource] = useState("");

  // Determine total steps based on deploy type
  const isDatabase = deployType === "database";
  const isFullStack = deployType === "fullstack";
  // database: type -> details -> review (3 steps)
  // fullstack: type -> services -> extras -> review (4 steps)
  // others: type -> details -> extras -> review (4 steps)
  const totalSteps = isDatabase ? 3 : 4;

  const handleTypeSelect = useCallback((type: DeployType) => {
    setDeployType(type);
    const defaultSvc = makeDefaultService(type);
    defaultSvc.name = type === "fullstack" ? "frontend" : type;
    if (type === "fullstack") {
      setServices([
        { ...defaultSvc, name: "frontend", kind: "web", isPublic: true, port: 3000 },
        { ...makeDefaultService("api"), name: "api", kind: "api", port: 8080 },
      ]);
    } else {
      setServices([defaultSvc]);
    }
    setAppName(type === "fullstack" ? "my-app" : `my-${type}`);
  }, []);

  const isStepValid = (): boolean => {
    switch (step) {
      case 1: return deployType !== null;
      case 2:
        if (isFullStack) return services.length > 0 && services.every((s) => s.image.trim() !== "");
        if (isDatabase) return services[0]?.engine !== "";
        return services[0]?.image?.trim() !== "";
      case 3:
        if (isDatabase) return true; // review step
        return true; // extras are optional
      case 4: return true; // review step
      default: return false;
    }
  };

  const handleNext = () => {
    if (step === totalSteps) {
      // Generate recipe and complete
      const source = recipeSource || generateRecipeSource(appName, services);
      onComplete(appName, source);
      return;
    }

    // Before advancing to review, generate the recipe
    if (step === totalSteps - 1) {
      setRecipeSource(generateRecipeSource(appName, services));
    }

    setStep((s) => Math.min(s + 1, totalSteps));
  };

  const handleBack = () => {
    setStep((s) => Math.max(s - 1, 1));
  };

  const renderStepContent = () => {
    switch (step) {
      case 1:
        return (
          <div className="space-y-4">
            <div className="space-y-1.5 mb-4">
              <Label className="text-xs text-muted-foreground">App name</Label>
              <Input
                value={appName}
                onChange={(e) => setAppName(e.target.value)}
                placeholder="e.g. my-web-app"
                className="h-10 text-sm bg-elevated"
              />
            </div>
            <div className="grid grid-cols-1 gap-3">
              {DEPLOY_TYPES.map((type) => (
                <TypeCard
                  key={type.id}
                  item={type}
                  active={deployType === type.id}
                  onClick={() => handleTypeSelect(type.id)}
                />
              ))}
            </div>
          </div>
        );

      case 2:
        if (!deployType) return null;
        if (isFullStack) {
          return (
            <FullStackDetails
              services={services}
              onChange={setServices}
            />
          );
        }
        return (
          <StepDetails
            deployType={deployType}
            service={services[0]!}
            onChange={(svc) => setServices([svc, ...services.slice(1)])}
          />
        );

      case 3:
        if (isDatabase) {
          // Review step for database
          return (
            <StepReview
              recipeSource={recipeSource || generateRecipeSource(appName, services)}
              onEdit={(src) => setRecipeSource(src)}
            />
          );
        }
        return (
          <StepExtras
            services={services}
            onChange={setServices}
            deployType={deployType!}
          />
        );

      case 4:
        return (
          <StepReview
            recipeSource={recipeSource || generateRecipeSource(appName, services)}
            onEdit={(src) => setRecipeSource(src)}
          />
        );

      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {renderStepContent()}

      {/* Internal navigation for guided sub-steps */}
      {step > 1 && (
        <div className="flex items-center justify-between pt-4 border-t border-border/30">
          <Button
            variant="ghost"
            size="sm"
            onClick={handleBack}
            className="text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft className="mr-2 h-4 w-4" />
            Previous question
          </Button>
          {step < totalSteps && (
            <Button
              onClick={handleNext}
              disabled={!isStepValid()}
              size="sm"
              className="bg-brand hover:bg-brand-hover text-primary-foreground"
            >
              Next
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
          )}
          {step === totalSteps && (
            <Button
              onClick={handleNext}
              disabled={!isStepValid()}
              size="sm"
              className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_15px_rgba(255,107,53,0.2)]"
            >
              <CheckCircle className="mr-2 h-4 w-4" />
              Looks good!
            </Button>
          )}
        </div>
      )}

      {step === 1 && deployType !== null && (
        <div className="flex justify-end pt-4 border-t border-border/30">
          <Button
            onClick={handleNext}
            disabled={!isStepValid()}
            size="sm"
            className="bg-brand hover:bg-brand-hover text-primary-foreground"
          >
            Next
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        </div>
      )}

      {/* Escape hatch */}
      <div className="text-center">
        <button
          type="button"
          className="text-xs text-muted-foreground/60 hover:text-muted-foreground transition-colors"
          onClick={onAdvancedMode}
        >
          I know what I'm doing → Advanced mode
        </button>
      </div>
    </div>
  );
}
