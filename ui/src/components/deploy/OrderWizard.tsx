import { useState, useEffect, useCallback } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Search, Plus, ArrowRight, ArrowLeft, Check, CheckCircle,
  Globe, Server, Cog, Database as DatabaseIcon, Settings,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { MozzaLogo } from "@/components/custom/MozzaLogo";
import {
  FAST_LANE_STEPS,
  RESOURCE_SIZE_MAP,
} from "@/lib/interview-engine";
import type { ResourceSize } from "@/lib/interview-types";
import {
  type WizardState,
  type TraitCombo,
  type FastLaneSource,
  createDefaultServiceState,
} from "@/lib/interview-types";
import { listAliases, createAlias, scanNamespace, type RegistryImage } from "@/api/client";
import type { ImageAlias, Template } from "@/api/types";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import { InlineHelp } from "@/components/ui/InlineHelp";
import TemplateCatalog from "./TemplateCatalog";
import GitHubImport from "./GitHubImport";
import FrameworkDetect from "./FrameworkDetect";
import FrameworkWizard from "./FrameworkWizard";
import type { DetectResult } from "@/api/types";
import { detectFramework } from "@/api/client";
import ComposeImport from "./ComposeImport";

// ─── Props ──────────────────────────────────────────────────

interface OrderWizardProps {
  onComplete: (recipeName: string, recipeSource: string) => void;
  onAdvancedMode: () => void;
  teamId?: string;
}

// ─── Fast Lane Step Indicator ───────────────────────────────

const STEP_LABELS = ["Pick", "Customize", "Order"];

function FastLaneIndicator({ currentStep }: { currentStep: number }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      {STEP_LABELS.map((label, i) => {
        const stepNum = i + 1;
        const isCompleted = stepNum < currentStep;
        const isCurrent = stepNum === currentStep;
        return (
          <div key={label} style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div
                style={{
                  width: 28,
                  height: 28,
                  borderRadius: "50%",
                  background: isCompleted || isCurrent ? "#ff6b35" : "rgba(255,255,255,0.08)",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 12,
                  fontWeight: 700,
                  color: isCompleted || isCurrent ? "#fff" : "rgba(255,255,255,0.35)",
                  boxShadow: isCurrent ? "0 0 12px rgba(255,107,53,0.4)" : "none",
                  transition: "all 0.2s ease",
                }}
              >
                {isCompleted ? <Check size={14} /> : stepNum}
              </div>
              <span
                style={{
                  fontSize: 13,
                  fontWeight: isCurrent ? 600 : 400,
                  color: isCurrent ? "#fff" : "rgba(255,255,255,0.45)",
                }}
              >
                {label}
              </span>
            </div>
            {i < STEP_LABELS.length - 1 && (
              <div
                style={{
                  width: 32,
                  height: 2,
                  background: isCompleted ? "#ff6b35" : "rgba(255,255,255,0.1)",
                  borderRadius: 1,
                }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

// ─── Alias Card ─────────────────────────────────────────────

function aliasColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  const hues = [0, 30, 60, 120, 180, 210, 270, 330];
  return `hsl(${hues[Math.abs(hash) % hues.length]}, 60%, 45%)`;
}

function AliasCard({
  alias,
  selected,
  onClick,
}: {
  alias: ImageAlias;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "relative rounded-xl border p-4 text-left transition-all w-full",
        selected
          ? "border-brand/50 bg-brand-muted shadow-[0_0_20px_rgba(255,107,53,0.1)]"
          : "border-border/50 bg-card hover:border-brand/25",
      )}
    >
      {selected && (
        <div className="absolute top-3 right-3">
          <CheckCircle className="h-4 w-4 text-brand" />
        </div>
      )}
      <div className="flex items-center gap-3">
        <div
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-white text-sm font-bold"
          style={{ background: aliasColor(alias.name) }}
        >
          {alias.name.charAt(0).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-foreground truncate">{alias.name}</p>
          <p className="text-[11px] text-muted-foreground/60 truncate font-mono mt-0.5">
            {alias.image_ref}
          </p>
        </div>
      </div>
    </button>
  );
}

// ─── Tab Button ─────────────────────────────────────────────

function TabButton({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-lg px-4 py-2 text-sm font-semibold transition-all",
        active
          ? "bg-brand text-primary-foreground shadow-[0_0_12px_rgba(255,107,53,0.2)]"
          : "bg-muted text-muted-foreground hover:bg-muted/80 hover:text-foreground",
      )}
    >
      {label}
    </button>
  );
}

// ─── Main Component ─────────────────────────────────────────

export function OrderWizard({ onComplete, onAdvancedMode, teamId }: OrderWizardProps) {
  const [currentStep, setCurrentStep] = useState(1);
  const [wizardState, setWizardState] = useState<WizardState>({
    services: [createDefaultServiceState()],
    activeServiceIndex: 0,
    currentStep: 1,
    target: "kitchen",
    clusterId: "",
    fastLane: true,
    selectedCombo: null,
    source: "alias",
    selectedTemplateId: null,
    selectedAliasId: null,
    resourceSize: "M",
  });

  // Step 1 state
  const [activeTab, setActiveTab] = useState<"menu" | "classics" | "repo" | "compose">("menu");
  const [aliases, setAliases] = useState<ImageAlias[]>([]);
  const [loadingAliases, setLoadingAliases] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedAlias, setSelectedAlias] = useState<ImageAlias | null>(null);
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null);
  const [importedRecipe, setImportedRecipe] = useState("");
  const [templateVars, setTemplateVars] = useState<Record<string, string>>({});
  const [showInlineCreate, setShowInlineCreate] = useState(false);
  const [newAliasName, setNewAliasName] = useState("");
  const [newAliasImage, setNewAliasImage] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState("");

  // Step 2 state
  const [appName, setAppName] = useState("");
  const selectedCombo = wizardState.selectedCombo ?? null;
  const [customizeExpanded, setCustomizeExpanded] = useState(false);

  // Step 3 state
  const [recipeSource, setRecipeSource] = useState("");
  const [editingRecipe, setEditingRecipe] = useState(false);
  const [editedRecipe, setEditedRecipe] = useState("");
  const [showRecipe, setShowRecipe] = useState(false);

  // Framework detection state
  const [frameworkResult, setFrameworkResult] = useState<DetectResult | null>(null);
  const [showFrameworkWizard, setShowFrameworkWizard] = useState(false);
  const [detectingFramework, setDetectingFramework] = useState(false);

  // Fetch aliases on mount
  useEffect(() => {
    if (!teamId) { setLoadingAliases(false); return; }
    setLoadingAliases(true);
    listAliases(teamId)
      .then((data) => setAliases(data ?? []))
      .catch(() => setAliases([]))
      .finally(() => setLoadingAliases(false));
  }, [teamId]);

  // Auto-switch to Classics for first-time users with empty menu
  useEffect(() => {
    if (!loadingAliases && aliases.length === 0 && !localStorage.getItem("mozza-onboarded")) {
      setActiveTab("classics");
    }
  }, [loadingAliases, aliases.length]);

  // ── Selection handlers ────────────────────────────────────

  const handleAliasSelect = useCallback((alias: ImageAlias) => {
    setSelectedAlias((prev) => prev?.id === alias.id ? null : alias);
    setSelectedTemplate(null);
    setImportedRecipe("");
    setAppName(alias.name);
  }, []);

  const handleTemplateSelect = useCallback((tpl: Template) => {
    setSelectedTemplate(tpl);
    setSelectedAlias(null);
    setImportedRecipe("");
    setAppName(tpl.name);
  }, []);

  const handleImportedRecipe = useCallback((recipe: string) => {
    setImportedRecipe(recipe);
    setSelectedAlias(null);
    setSelectedTemplate(null);
    setAppName("imported-app");

    // Attempt framework detection in the background.
    setDetectingFramework(true);
    detectFramework()
      .then((result) => {
        if (result.framework && result.framework !== "unknown") {
          setFrameworkResult(result);
        }
      })
      .catch(() => {
        // Detection is optional — silent failure is fine.
      })
      .finally(() => setDetectingFramework(false));
  }, []);

  const handleFrameworkContinue = useCallback(() => {
    setShowFrameworkWizard(true);
  }, []);

  const handleFrameworkSkip = useCallback(() => {
    setFrameworkResult(null);
  }, []);

  const handleFrameworkComplete = useCallback((name: string, recipe: string) => {
    setImportedRecipe(recipe);
    setAppName(name);
    setShowFrameworkWizard(false);
    setFrameworkResult(null);
    // Jump straight to review step with the generated recipe.
    setRecipeSource(recipe);
    setEditedRecipe(recipe);
    setCurrentStep(3);
  }, []);

  const handleCreateAlias = useCallback(async () => {
    if (!teamId || !newAliasName.trim() || !newAliasImage.trim()) return;
    setCreating(true);
    try {
      await createAlias(teamId, newAliasName.trim(), newAliasImage.trim());
      const refreshed = await listAliases(teamId);
      setAliases(refreshed ?? []);
      const created = refreshed?.find((a) => a.name === newAliasName.trim());
      if (created) {
        setSelectedAlias(created);
        setAppName(created.name);
      }
      setShowInlineCreate(false);
      setNewAliasName("");
      setNewAliasImage("");
    } catch {
      setCreateError("Couldn't add that app. Check the name and URL and try again.");
    } finally {
      setCreating(false);
    }
  }, [teamId, newAliasName, newAliasImage]);

  // ── Navigation ────────────────────────────────────────────

  const hasSelection = selectedAlias !== null || selectedTemplate !== null || importedRecipe !== "";

  const canNext = (() => {
    if (currentStep === 1) return hasSelection;
    if (currentStep === 2) return selectedTemplate !== null || wizardState.services.some(s => s.aliasImage);
    if (currentStep === 3) return true;
    return false;
  })();

  const goNext = useCallback(() => {
    if (currentStep === 1) {
      // Prepare service state from selection
      if (selectedAlias) {
        setWizardState((prev) => {
          const idx = prev.activeServiceIndex;
          const services = [...prev.services];
          const svc = services[idx];
          if (!svc) return prev;
          services[idx] = { ...svc, aliasName: selectedAlias.name, aliasImage: selectedAlias.image_ref, serviceName: selectedAlias.name };
          return { ...prev, services, source: "alias" as FastLaneSource, selectedAliasId: selectedAlias.id };
        });
      } else if (selectedTemplate) {
        setWizardState((prev) => ({
          ...prev, source: "template" as FastLaneSource, selectedTemplateId: selectedTemplate.id,
        }));
      } else if (importedRecipe) {
        // GitHub import: set source and populate service from recipe
        setWizardState((prev) => {
          const services = [...prev.services];
          const svc = services[0];
          if (!svc) return prev;
          services[0] = { ...svc, aliasName: appName, aliasImage: "" };
          return { ...prev, services, source: "github" as FastLaneSource };
        });
      }
      setCurrentStep(2);
    } else if (currentStep === 2) {
      // Generate recipe for review
      if (selectedTemplate && selectedTemplate.source) {
        // For templates, substitute variables into template source
        let source = selectedTemplate.source;
        for (const [key, val] of Object.entries(templateVars)) {
          source = source.replace(new RegExp(`\\{${key}\\}`, "g"), val);
        }
        setRecipeSource(source);
        setEditedRecipe(source);
      } else if (importedRecipe) {
        setRecipeSource(importedRecipe);
        setEditedRecipe(importedRecipe);
      } else {
        const source = buildRecipeSource(appName, wizardState);
        setRecipeSource(source);
        setEditedRecipe(source);
      }
      setCurrentStep(3);
    }
  }, [currentStep, selectedAlias, selectedTemplate, importedRecipe, wizardState, appName, templateVars]);

  const goBack = useCallback(() => {
    setCurrentStep((s) => Math.max(s - 1, 1));
  }, []);

  const handlePlaceOrder = useCallback(() => {
    const finalRecipe = editingRecipe ? editedRecipe : recipeSource;
    onComplete(appName, finalRecipe);
  }, [appName, recipeSource, editedRecipe, editingRecipe, onComplete]);

  // ── Step 1: Pick ──────────────────────────────────────────

  const filteredAliases = aliases.filter((a) =>
    a.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const renderStep1 = () => (
    <div className="space-y-5">
      {/* Tab bar */}
      <div className="flex gap-2">
        <TabButton label="Your menu" active={activeTab === "menu"} onClick={() => setActiveTab("menu")} />
        <TabButton label="Classics" active={activeTab === "classics"} onClick={() => setActiveTab("classics")} />
        <TabButton label="From a repo" active={activeTab === "repo"} onClick={() => setActiveTab("repo")} />
        <TabButton label="Compose" active={activeTab === "compose"} onClick={() => setActiveTab("compose")} />
      </div>

      {/* Tab content */}
      {activeTab === "menu" && (
        <div className="space-y-4">
          {/* Search */}
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search your apps..."
              className="pl-10 h-10 text-sm bg-elevated"
            />
          </div>

          {/* Alias grid */}
          {loadingAliases ? (
            <div className="grid grid-cols-2 gap-3">
              {[1, 2, 3, 4].map((i) => (
                <div key={i} className="rounded-xl border border-border/50 bg-card p-4 animate-pulse h-16" />
              ))}
            </div>
          ) : filteredAliases.length === 0 && !searchQuery ? (
            <div className="text-center py-12 space-y-4">
              <p className="text-sm text-muted-foreground">Your menu is empty — no worries!</p>
              <Button
                className="bg-brand hover:bg-brand-hover text-primary-foreground"
                onClick={() => setActiveTab("classics")}
              >
                Browse ready-to-deploy apps
              </Button>
              <div className="flex items-center justify-center gap-4">
                <button
                  type="button"
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => setShowInlineCreate(true)}
                >
                  Add your own app
                </button>
                <button
                  type="button"
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => setActiveTab("repo")}
                >
                  Import from GitHub
                </button>
              </div>
            </div>
          ) : filteredAliases.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-6">No matches for "{searchQuery}"</p>
          ) : (
            <div className="grid grid-cols-2 gap-3">
              {filteredAliases.map((alias) => (
                <AliasCard
                  key={alias.id}
                  alias={alias}
                  selected={selectedAlias?.id === alias.id}
                  onClick={() => handleAliasSelect(alias)}
                />
              ))}
            </div>
          )}

          {/* Add new alias */}
          {!showInlineCreate ? (
            <button
              type="button"
              onClick={() => setShowInlineCreate(true)}
              className="flex items-center gap-2 rounded-xl border border-dashed border-border/50 px-4 py-3 w-full text-sm text-muted-foreground hover:border-brand/25 hover:text-foreground transition-colors"
            >
              <Plus className="h-4 w-4" />
              Add a new app
            </button>
          ) : (
            <div className="rounded-xl border border-brand/30 bg-brand-muted p-4 space-y-3">
              <p className="text-sm font-medium text-foreground">Add to the menu</p>
              <Input
                value={newAliasName}
                onChange={(e) => { setNewAliasName(e.target.value); setCreateError(""); }}
                placeholder="App name (e.g. my-api)"
                className="h-9 text-sm bg-elevated"
              />
              <Input
                value={newAliasImage}
                onChange={(e) => { setNewAliasImage(e.target.value); setCreateError(""); }}
                placeholder="App package URL (ask your developer for this)"
                className="h-9 text-sm bg-elevated font-mono"
              />
              <p className="text-[11px] text-muted-foreground/50 mt-1">Your developer or CI pipeline can provide this URL</p>
              <InlineHelp>It looks like: ghcr.io/yourcompany/app:v1.0 or docker.io/nginx:latest. If you don't have one, try the Classics tab for pre-built apps.</InlineHelp>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  className="bg-brand hover:bg-brand-hover text-primary-foreground"
                  disabled={!newAliasName.trim() || !newAliasImage.trim() || creating}
                  onClick={handleCreateAlias}
                >
                  {creating ? "Adding..." : "Add"}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => { setShowInlineCreate(false); setNewAliasName(""); setNewAliasImage(""); }}
                >
                  Cancel
                </Button>
              </div>
              {createError && (
                <p className="text-xs text-red-400 mt-1">{createError}</p>
              )}
            </div>
          )}
        </div>
      )}

      {activeTab === "classics" && (
        <TemplateCatalog
          selected={selectedTemplate}
          onSelect={handleTemplateSelect}
        />
      )}

      {activeTab === "repo" && (
        showFrameworkWizard && frameworkResult ? (
          <FrameworkWizard
            detection={frameworkResult}
            appName={appName || "imported-app"}
            onComplete={handleFrameworkComplete}
            onBack={() => setShowFrameworkWizard(false)}
          />
        ) : (
          <div className="space-y-4">
            <GitHubImport onRecipeReady={handleImportedRecipe} />
            {detectingFramework && (
              <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 0" }}>
                <div style={{
                  width: 14, height: 14, borderRadius: 7,
                  border: "2px solid #ff6b35", borderTopColor: "transparent",
                  animation: "spin 0.8s linear infinite",
                }} />
                <span style={{ fontSize: 12, color: "var(--muted-foreground)" }}>
                  Detecting framework...
                </span>
              </div>
            )}
            {frameworkResult && !detectingFramework && (
              <FrameworkDetect
                result={frameworkResult}
                onContinue={handleFrameworkContinue}
                onSkip={handleFrameworkSkip}
              />
            )}
          </div>
        )
      )}

      {activeTab === "compose" && (
        <ComposeImport onRecipeReady={handleImportedRecipe} />
      )}
    </div>
  );

  // ── Step 2: Slice Builder ─────────────────────────────────

  // Legacy combo cards kept for internal state compatibility.
  const COMBO_CARDS: Array<{
    id: TraitCombo;
    icon: typeof Globe;
    label: string;
    description: string;
  }> = [
    { id: "website", icon: Globe, label: "Website", description: "A site people visit" },
    { id: "api-database", icon: Server, label: "API + Database", description: "Backend with storage" },
    { id: "background-worker", icon: Cog, label: "Background Worker", description: "Runs behind the scenes" },
    { id: "database-only", icon: DatabaseIcon, label: "Database Only", description: "Just a database" },
    { id: "custom", icon: Settings, label: "Custom", description: "Something else" },
  ];

  // Slice builder: add a new slice to the order by image name.
  const [newSliceName, setNewSliceName] = useState("");
  const [newSliceImage, setNewSliceImage] = useState("");

  // Namespace scanner state.
  const [scanQuery, setScanQuery] = useState("");
  const [scanResults, setScanResults] = useState<RegistryImage[]>([]);
  const [scanning, setScanning] = useState(false);
  const [scanError, setScanError] = useState("");
  const [showScanner, setShowScanner] = useState(false);

  const doScan = useCallback(async (ns: string) => {
    if (!ns.trim()) return;
    setScanning(true);
    setScanError("");
    try {
      const result = await scanNamespace(ns.trim());
      setScanResults(result.images);
      if (result.images.length === 0) {
        setScanError(`No images found under "${ns}"`);
      }
    } catch {
      setScanError(`Couldn't find "${ns}" on Docker Hub`);
      setScanResults([]);
    } finally {
      setScanning(false);
    }
  }, []);

  const addSlice = useCallback((name: string, image: string) => {
    if (!name.trim() || !image.trim()) return;
    setWizardState((prev) => {
      const newSvc = createDefaultServiceState();
      newSvc.serviceName = name.trim().toLowerCase().replace(/\s+/g, "-");
      newSvc.aliasName = newSvc.serviceName;
      newSvc.aliasImage = image.trim();
      // Auto-detect: if image contains "postgres" or "mysql" etc, it's a database
      const imgLower = image.toLowerCase();
      if (imgLower.includes("postgres") || imgLower.includes("mysql") || imgLower.includes("mongo") || imgLower.includes("mariadb")) {
        newSvc.port = imgLower.includes("mysql") || imgLower.includes("mariadb") ? 3306 : imgLower.includes("mongo") ? 27017 : 5432;
        newSvc.isPublic = false;
        newSvc.healthCheck = { ...newSvc.healthCheck, enabled: false };
      } else if (imgLower.includes("redis") || imgLower.includes("memcache")) {
        newSvc.port = imgLower.includes("redis") ? 6379 : 11211;
        newSvc.isPublic = false;
        newSvc.healthCheck = { ...newSvc.healthCheck, enabled: false };
      } else if (imgLower.includes("nginx") || imgLower.includes("httpd") || imgLower.includes("caddy")) {
        newSvc.port = 80;
        newSvc.isPublic = true;
      }
      const services = [...prev.services, newSvc];
      return { ...prev, services, activeServiceIndex: services.length - 1, selectedCombo: "custom" as TraitCombo };
    });
    setNewSliceName("");
    setNewSliceImage("");
  }, []);

  const renderStep2 = () => {

    // Template mode: show template variables
    if (selectedTemplate) {
      return (
        <div className="space-y-5">
          <div className="space-y-1.5">
            <label className="text-xs text-muted-foreground">App name</label>
            <Input
              value={appName}
              onChange={(e) => setAppName(e.target.value)}
              className="h-10 text-sm bg-elevated"
            />
          </div>
          {/* Target toggle */}
          <TargetToggle target={wizardState.target} onChange={(t) => setWizardState((prev) => ({ ...prev, target: t }))} />
          {selectedTemplate.variables && selectedTemplate.variables.length > 0 ? (
            <div className="space-y-3">
              <label className="text-xs text-muted-foreground block">Configure</label>
              {selectedTemplate.variables.map((v) => (
                <div key={v.key} className="space-y-1.5">
                  <label className="text-xs text-muted-foreground">{v.label || v.key}</label>
                  <Input
                    type={v.type === "password" ? "password" : "text"}
                    value={templateVars[v.key] ?? v.default ?? ""}
                    onChange={(e) => setTemplateVars((prev) => ({ ...prev, [v.key]: e.target.value }))}
                    placeholder={v.default || ""}
                    className="h-9 text-sm bg-elevated"
                  />
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">
              Ready to deploy — no extra configuration needed.
            </p>
          )}
        </div>
      );
    }

    return (
      <div className="space-y-6">
        {/* App name */}
        <div className="space-y-1.5">
          <label className="text-xs text-muted-foreground">Name your app</label>
          <Input
            value={appName}
            onChange={(e) => setAppName(e.target.value)}
            placeholder="e.g. my-web-app"
            className="h-10 text-sm bg-elevated"
          />
        </div>

        {/* Target toggle */}
        <TargetToggle target={wizardState.target} onChange={(t) => setWizardState((prev) => ({ ...prev, target: t }))} />

        {/* ── Slice list: the shopping cart ── */}
        <div className="space-y-3">
          <label className="text-xs text-muted-foreground block">
            {wizardState.services.length === 1 && !wizardState.services[0]?.aliasImage
              ? "What images do you want to run?"
              : `Your slices (${wizardState.services.length})`}
          </label>

          {/* Existing slices */}
          {wizardState.services.filter(s => s.aliasImage).map((svc, i) => (
            <div
              key={i}
              className={cn(
                "rounded-xl border px-4 py-3 transition-all",
                i === wizardState.activeServiceIndex
                  ? "border-brand/50 bg-brand-muted"
                  : "border-border/50 bg-card",
              )}
            >
              <div className="flex items-center justify-between">
                <div
                  className="flex items-center gap-3 flex-1 cursor-pointer"
                  onClick={() => setWizardState((prev) => ({ ...prev, activeServiceIndex: i }))}
                >
                  <div className={cn(
                    "flex h-8 w-8 items-center justify-center rounded-lg text-xs font-bold",
                    i === wizardState.activeServiceIndex ? "bg-brand text-white" : "bg-muted text-muted-foreground",
                  )}>
                    {i + 1}
                  </div>
                  <div>
                    <p className="text-sm font-semibold text-foreground">{svc.serviceName || svc.aliasName}</p>
                    <p className="text-[10px] text-muted-foreground font-mono">{svc.aliasImage}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  {/* Public toggle */}
                  <button
                    type="button"
                    onClick={() => {
                      setWizardState((prev) => {
                        const services = [...prev.services];
                        const current = services[i];
                        if (!current) return prev;
                        services[i] = { ...current, isPublic: !current.isPublic };
                        return { ...prev, services };
                      });
                    }}
                    className={cn(
                      "rounded-lg border px-2.5 py-1 text-[10px] font-semibold transition-all",
                      svc.isPublic
                        ? "border-brand/50 bg-brand-muted text-brand"
                        : "border-border/50 text-muted-foreground hover:border-brand/25",
                    )}
                    title={svc.isPublic ? "Reachable from the internet" : "Internal only"}
                  >
                    {svc.isPublic ? "🌐 Public" : "🔒 Internal"}
                  </button>
                  {/* Remove */}
                  <button
                    type="button"
                    onClick={() => {
                      setWizardState((prev) => {
                        const removedName = prev.services[i]?.serviceName || prev.services[i]?.aliasName || "";
                        const services = prev.services
                          .filter((_, idx) => idx !== i)
                          .map((s) => ({
                            ...s,
                            needsServices: s.needsServices.filter((n) => n !== removedName),
                          }));
                        if (services.length === 0) {
                          services.push(createDefaultServiceState());
                        }
                        return {
                          ...prev,
                          services,
                          activeServiceIndex: Math.min(prev.activeServiceIndex, services.length - 1),
                          selectedCombo: services.some(s => s.aliasImage) ? "custom" as TraitCombo : null,
                        };
                      });
                    }}
                    className="text-muted-foreground hover:text-red-400 transition-colors"
                  >
                    ×
                  </button>
                </div>
              </div>

              {/* Expanded: show dependencies when this slice is selected and there are 2+ slices */}
              {i === wizardState.activeServiceIndex && wizardState.services.filter(s => s.aliasImage).length > 1 && (
                <div className="mt-3 pt-3 border-t border-border/30">
                  <label className="text-[10px] text-muted-foreground block mb-2">Needs to start first:</label>
                  <div className="flex flex-wrap gap-1.5">
                    {wizardState.services
                      .filter((other, oi) => oi !== i && other.aliasImage)
                      .map((other) => {
                        const otherName = other.serviceName || other.aliasName;
                        const isNeeded = svc.needsServices.includes(otherName);
                        return (
                          <button
                            key={otherName}
                            type="button"
                            onClick={() => {
                              setWizardState((prev) => {
                                const services = [...prev.services];
                                const current = services[i];
                                if (!current) return prev;
                                const needsServices = isNeeded
                                  ? current.needsServices.filter((n: string) => n !== otherName)
                                  : [...current.needsServices, otherName];
                                services[i] = { ...current, needsServices };
                                return { ...prev, services };
                              });
                            }}
                            className={cn(
                              "rounded-md border px-2 py-0.5 text-[10px] font-semibold transition-all",
                              isNeeded
                                ? "border-brand/50 bg-brand-muted text-foreground"
                                : "border-border/40 text-muted-foreground/60 hover:border-brand/25",
                            )}
                          >
                            {isNeeded ? "✓ " : ""}{otherName}
                          </button>
                        );
                      })}
                  </div>
                </div>
              )}
            </div>
          ))}

          {/* Namespace scanner */}
          {showScanner ? (
            <div className="rounded-xl border border-border/50 bg-card p-4 space-y-3">
              <div className="flex items-center justify-between">
                <p className="text-xs font-medium text-foreground">Scan a Docker Hub organization</p>
                <button
                  type="button"
                  onClick={() => { setShowScanner(false); setScanResults([]); setScanError(""); }}
                  className="text-xs text-muted-foreground hover:text-foreground"
                >
                  Close
                </button>
              </div>
              <div className="flex gap-2">
                <Input
                  value={scanQuery}
                  onChange={(e) => setScanQuery(e.target.value)}
                  placeholder="e.g. hobbyfarm"
                  className="h-9 text-sm bg-elevated flex-1"
                  onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); doScan(scanQuery); } }}
                />
                <Button
                  size="sm"
                  className="bg-brand hover:bg-brand-hover text-primary-foreground h-9 px-4"
                  disabled={!scanQuery.trim() || scanning}
                  onClick={() => doScan(scanQuery)}
                >
                  {scanning ? (
                    <div className="h-4 w-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  ) : (
                    <Search className="h-4 w-4" />
                  )}
                </Button>
              </div>
              {scanError && <p className="text-xs text-red-400">{scanError}</p>}
              {scanResults.length > 0 && (
                <div className="space-y-1 max-h-64 overflow-y-auto">
                  <p className="text-[10px] text-muted-foreground mb-2">
                    Found {scanResults.length} images — click to add
                  </p>
                  {scanResults.map((img) => {
                    const alreadyAdded = wizardState.services.some(s => s.aliasImage === img.full_ref);
                    return (
                      <button
                        key={img.name}
                        type="button"
                        disabled={alreadyAdded}
                        onClick={() => addSlice(img.name, img.full_ref)}
                        className={cn(
                          "flex items-center justify-between w-full rounded-lg px-3 py-2 text-left transition-all",
                          alreadyAdded
                            ? "bg-brand-muted/50 border border-brand/30"
                            : "hover:bg-muted border border-transparent",
                        )}
                      >
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-foreground truncate">{img.name}</p>
                          {img.description && (
                            <p className="text-[10px] text-muted-foreground truncate">{img.description}</p>
                          )}
                        </div>
                        <div className="flex items-center gap-2 ml-3 shrink-0">
                          {img.last_updated && (
                            <span className="text-[9px] text-muted-foreground/50">{img.last_updated}</span>
                          )}
                          {alreadyAdded ? (
                            <Check className="h-3.5 w-3.5 text-brand" />
                          ) : (
                            <Plus className="h-3.5 w-3.5 text-muted-foreground" />
                          )}
                        </div>
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setShowScanner(true)}
              className="w-full flex items-center justify-center gap-2 rounded-xl border border-border/50 py-2.5 text-xs text-muted-foreground hover:border-brand/25 hover:text-foreground transition-all"
            >
              <Search className="h-3.5 w-3.5" />
              Scan a Docker Hub org for images
            </button>
          )}

          {/* Add slice form */}
          <div className="rounded-xl border border-dashed border-border/50 p-4 space-y-3">
            <p className="text-xs text-muted-foreground">
              {wizardState.services.some(s => s.aliasImage) ? "Or add one manually" : "Add your first slice"}
            </p>
            <div className="flex gap-2">
              <Input
                value={newSliceName}
                onChange={(e) => setNewSliceName(e.target.value)}
                placeholder="Name (e.g. api)"
                className="h-9 text-sm bg-elevated flex-1"
              />
              <Input
                value={newSliceImage}
                onChange={(e) => setNewSliceImage(e.target.value)}
                placeholder="Image (e.g. nginx:latest)"
                className="h-9 text-sm bg-elevated font-mono flex-[2]"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addSlice(newSliceName || newSliceImage.split("/").pop()?.split(":")[0] || "app", newSliceImage);
                  }
                }}
              />
              <Button
                size="sm"
                className="bg-brand hover:bg-brand-hover text-primary-foreground h-9 px-4"
                disabled={!newSliceImage.trim()}
                onClick={() => addSlice(newSliceName || newSliceImage.split("/").pop()?.split(":")[0] || "app", newSliceImage)}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-[10px] text-muted-foreground/40">
              Just enter the Docker image name. Mozza figures out ports, health checks, and storage automatically.
            </p>
          </div>
        </div>

        {/* Customize more — collapsible advanced section */}
        {wizardState.services.some(s => s.aliasImage) && (
          <div>
            <button
              type="button"
              onClick={() => setCustomizeExpanded(!customizeExpanded)}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
            >
              <span style={{ transform: customizeExpanded ? "rotate(90deg)" : "none", transition: "transform 0.15s", display: "inline-block" }}>▸</span>
              Customize more...
            </button>
            {customizeExpanded && (
              <div className="mt-4 space-y-4 rounded-xl border border-border/30 bg-card/50 p-4">
                {/* Port */}
                <div className="space-y-1.5">
                  <label className="text-xs text-muted-foreground">Port <HelpTooltip term="port" /></label>
                  <Input
                    type="number"
                    value={wizardState.services[0]?.port || ""}
                    onChange={(e) => {
                      const port = parseInt(e.target.value) || 0;
                      setWizardState((prev) => {
                        const services = [...prev.services];
                        const svc = services[0];
                        if (!svc) return prev;
                        services[0] = { ...svc, port };
                        return { ...prev, services };
                      });
                    }}
                    placeholder="8080"
                    className="h-9 text-sm bg-elevated w-32"
                  />
                  <InlineHelp label="What port should I use?">
                    Check your app's startup logs for "Listening on port XXXX". Common defaults: 3000 (Node.js), 8080 (Go/Java), 5000 (Python), 4000 (Phoenix).
                  </InlineHelp>
                </div>

                {/* Replicas */}
                <div className="space-y-1.5">
                  <label className="text-xs text-muted-foreground">How many copies? <HelpTooltip term="replica" /></label>
                  <Input
                    type="number"
                    value={wizardState.services[0]?.replicas || 1}
                    onChange={(e) => {
                      const replicas = Math.max(1, Math.min(100, parseInt(e.target.value) || 1));
                      setWizardState((prev) => {
                        const services = [...prev.services];
                        const svc = services[0];
                        if (!svc) return prev;
                        services[0] = { ...svc, replicas };
                        return { ...prev, services };
                      });
                    }}
                    min={1}
                    max={100}
                    className="h-9 text-sm bg-elevated w-32"
                  />
                  <p className="text-[10px] text-muted-foreground/40 mt-1">
                    {(wizardState.services[0]?.replicas ?? 1) === 1 ? "Single copy — fine for development" :
                     (wizardState.services[0]?.replicas ?? 1) <= 3 ? "Recommended for production — stays up if one copy fails" :
                     "High availability — handles heavy traffic"}
                  </p>
                </div>

                {/* Custom domain */}
                <div className="space-y-1.5">
                  <label className="text-xs text-muted-foreground">Custom address</label>
                  <Input
                    value={wizardState.services[0]?.domain || ""}
                    onChange={(e) => {
                      setWizardState((prev) => {
                        const services = [...prev.services];
                        const svc = services[0];
                        if (!svc) return prev;
                        services[0] = { ...svc, domain: e.target.value };
                        return { ...prev, services };
                      });
                    }}
                    placeholder="app.example.com"
                    className="h-9 text-sm bg-elevated"
                  />
                  <InlineHelp>To use your own address, enter it here. You'll need to update your DNS settings to point to your server — Mozza will show you how after deploying.</InlineHelp>
                </div>

                {/* Schedule (workers only) */}
                {(selectedCombo === "background-worker" || selectedCombo === "custom") && (
                  <div className="space-y-1.5">
                    <label className="text-xs text-muted-foreground">Schedule</label>
                    <div className="flex gap-2">
                      {[
                        { label: "Every hour", cron: "0 * * * *" },
                        { label: "Every day", cron: "0 0 * * *" },
                        { label: "Every week", cron: "0 0 * * 1" },
                      ].map((preset) => (
                        <button
                          key={preset.cron}
                          type="button"
                          onClick={() => {
                            setWizardState((prev) => {
                              const services = [...prev.services];
                              const svc = services[0];
                              if (!svc) return prev;
                              services[0] = { ...svc, schedule: preset.cron };
                              return { ...prev, services };
                            });
                          }}
                          className={cn(
                            "rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all",
                            wizardState.services[0]?.schedule === preset.cron
                              ? "border-brand/50 bg-brand-muted text-foreground"
                              : "border-border/50 text-muted-foreground hover:border-brand/25",
                          )}
                        >
                          {preset.label}
                        </button>
                      ))}
                    </div>
                    <p className="text-[10px] text-muted-foreground/40 mt-1.5">Your task will run at the scheduled time, do its work, then stop until the next run.</p>
                  </div>
                )}

                {/* Resource size (T-shirt) */}
                <div className="space-y-1.5">
                  <label className="text-xs text-muted-foreground">Resources</label>
                  <div className="flex gap-2">
                    {(["S", "M", "L", "XL"] as ResourceSize[]).map((size) => (
                      <button
                        key={size}
                        type="button"
                        onClick={() => {
                          const resources = RESOURCE_SIZE_MAP[size];
                          setWizardState((prev) => {
                            const services = [...prev.services];
                            const svc = services[0];
                            if (!svc) return prev;
                            services[0] = { ...svc, resources };
                            return { ...prev, services, resourceSize: size };
                          });
                        }}
                        className={cn(
                          "rounded-lg border px-4 py-2 text-xs font-bold transition-all",
                          wizardState.resourceSize === size
                            ? "border-brand/50 bg-brand-muted text-foreground"
                            : "border-border/50 text-muted-foreground hover:border-brand/25",
                        )}
                      >
                        {size}
                      </button>
                    ))}
                  </div>
                  <p className="text-[10px] text-muted-foreground/50">
                    {wizardState.resourceSize === "S" ? "100m CPU, 64Mi memory" :
                     wizardState.resourceSize === "M" ? "250m CPU, 128Mi memory" :
                     wizardState.resourceSize === "L" ? "500m CPU, 256Mi memory" :
                     "1000m CPU, 512Mi memory"}
                  </p>
                </div>

                {/* Health check toggle */}
                <ExtraToggle
                  label="Health check"
                  enabled={wizardState.services[0]?.healthCheck.enabled ?? true}
                  suffix={<HelpTooltip term="health-check" />}
                  onToggle={(on) => {
                    setWizardState((prev) => {
                      const services = [...prev.services];
                      const svc = services[0];
                      if (!svc) return prev;
                      services[0] = { ...svc, healthCheck: { ...svc.healthCheck, enabled: on } };
                      return { ...prev, services };
                    });
                  }}
                />
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  // ── Step 3: Review ────────────────────────────────────────

  const renderStep3 = () => {
    const svc = wizardState.services[0];
    const dbDep = svc?.dependencies.find((d) => d.type === "database" && d.enabled);
    const cacheDep = svc?.dependencies.find((d) => d.type === "cache" && d.enabled);
    const comboLabel = COMBO_CARDS.find((c) => c.id === selectedCombo)?.label ?? selectedTemplate?.name ?? "Custom";
    const serviceCount = wizardState.services.length;
    const totalLabel = serviceCount > 1 ? `${comboLabel} + ${serviceCount - 1} more` : comboLabel;

    return (
      <div className="space-y-5">
        {/* Receipt card */}
        <div className="rounded-xl border border-border/50 bg-card overflow-hidden">
          <div className="px-5 py-4 border-b border-border/30">
            <div className="flex items-center justify-between">
              <p className="text-base font-bold text-foreground">{appName || "My App"}</p>
              <Badge className="bg-brand/15 text-brand border-brand/25 text-xs">{totalLabel}</Badge>
            </div>
          </div>
          <div className="px-5 py-3 space-y-2.5">
            <ReceiptLine label="Deploying to" value={wizardState.target === "local" ? "Local" : "Kitchen"} />
            {svc?.isPublic && <ReceiptLine label="Public" value="Yes" />}
            {dbDep && <ReceiptLine label="Database" value={dbDep.engine === "postgres" ? "PostgreSQL" : dbDep.engine === "mysql" ? "MySQL" : "MongoDB"} />}
            {cacheDep && <ReceiptLine label="Cache" value="Redis" />}
          </div>
        </div>

        {/* Collapsed recipe view */}
        <div>
          <button
            type="button"
            onClick={() => setShowRecipe(!showRecipe)}
            className="text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1"
          >
            <span style={{ transform: showRecipe ? "rotate(90deg)" : "none", transition: "transform 0.15s", display: "inline-block" }}>▸</span>
            {showRecipe ? "Hide recipe" : "View recipe"}
          </button>
          {showRecipe && (
            <div className="mt-3 rounded-xl border border-border/50 bg-elevated overflow-hidden">
              <div className="flex items-center justify-between px-4 py-2 border-b border-border/50 bg-muted/30">
                <span className="text-xs font-medium text-muted-foreground">Generated recipe</span>
                <div className="flex items-center gap-2">
                  {editingRecipe && <Badge variant="outline" className="text-[10px]">Modified</Badge>}
                  <button
                    type="button"
                    className="text-xs text-muted-foreground hover:text-foreground transition-colors"
                    onClick={() => {
                      if (editingRecipe) {
                        setEditedRecipe(recipeSource);
                        setEditingRecipe(false);
                      } else {
                        setEditingRecipe(true);
                      }
                    }}
                  >
                    {editingRecipe ? "Reset" : "Edit"}
                  </button>
                </div>
              </div>
              {editingRecipe ? (
                <textarea
                  value={editedRecipe}
                  onChange={(e) => setEditedRecipe(e.target.value)}
                  className="w-full h-48 p-4 font-mono text-xs text-foreground bg-transparent resize-none focus:outline-none"
                />
              ) : (
                <pre className="p-4 font-mono text-xs text-foreground leading-relaxed whitespace-pre-wrap overflow-auto max-h-48">
                  {recipeSource}
                </pre>
              )}
            </div>
          )}
        </div>
      </div>
    );
  };

  // ── Render ────────────────────────────────────────────────

  const stepTitle = currentStep === 2 && selectedTemplate
    ? `Customize your ${selectedTemplate.name}`
    : FAST_LANE_STEPS[currentStep - 1]?.title ?? "";

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      {/* Header */}
      <div style={{ marginBottom: 20 }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 16 }}>
          <h2 style={{ fontSize: 22, fontWeight: 700, color: "#fff", margin: 0, display: "flex", alignItems: "center", gap: 10 }}>
            <MozzaLogo size={24} className="text-brand" />
            Place your order
          </h2>
          <button
            onClick={onAdvancedMode}
            style={{
              background: "none",
              border: "none",
              color: "rgba(255,255,255,0.35)",
              cursor: "pointer",
              fontSize: 12,
            }}
          >
            I know what I'm doing →
          </button>
        </div>
        <FastLaneIndicator currentStep={currentStep} />
      </div>

      {/* Step content */}
      <Card>
        <CardContent style={{ padding: 24 }}>
          <h3 style={{ fontSize: 18, fontWeight: 600, color: "#fff", margin: "0 0 20px" }}>
            {stepTitle}
          </h3>

          {currentStep === 1 && renderStep1()}
          {currentStep === 2 && renderStep2()}
          {currentStep === 3 && renderStep3()}

          {/* Navigation */}
          <div className="flex items-center justify-between pt-6 mt-6 border-t border-border/30">
            {currentStep > 1 ? (
              <Button variant="ghost" size="sm" onClick={goBack} className="text-muted-foreground hover:text-foreground">
                <ArrowLeft className="mr-2 h-4 w-4" />
                Back
              </Button>
            ) : (
              <div />
            )}
            {currentStep < 3 ? (
              <Button
                onClick={goNext}
                disabled={!canNext}
                size="sm"
                className="bg-brand hover:bg-brand-hover text-primary-foreground"
              >
                Next
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>
            ) : (
              <Button
                onClick={handlePlaceOrder}
                size="sm"
                className="bg-brand hover:bg-brand-hover text-primary-foreground shadow-[0_0_15px_rgba(255,107,53,0.2)]"
              >
                <CheckCircle className="mr-2 h-4 w-4" />
                Place Order
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

// ─── Helpers ────────────────────────────────────────────────

function ExtraToggle({
  label,
  enabled,
  onToggle,
  suffix,
}: {
  label: string;
  enabled: boolean;
  onToggle: (on: boolean) => void;
  suffix?: React.ReactNode;
}) {
  return (
    <button
      type="button"
      className="flex items-center justify-between w-full py-3 px-4 rounded-xl border border-border/50 bg-card hover:border-brand/25 transition-colors"
      onClick={() => onToggle(!enabled)}
    >
      <span className="text-sm text-foreground">{label}{suffix && <> {suffix}</>}</span>
      <div className={cn("relative w-11 h-6 rounded-full transition-colors", enabled ? "bg-brand" : "bg-muted")}>
        <div className={cn("absolute top-0.5 h-5 w-5 rounded-full bg-white transition-transform shadow-sm", enabled ? "translate-x-[22px]" : "translate-x-0.5")} />
      </div>
    </button>
  );
}

function TargetToggle({
  target,
  onChange,
}: {
  target: "local" | "kitchen";
  onChange: (t: "local" | "kitchen") => void;
}) {
  return (
    <div className="space-y-1.5">
      <label className="text-xs text-muted-foreground">Where to deploy</label>
      <div className="flex gap-2">
        {(["local", "kitchen"] as const).map((t) => (
          <button
            key={t}
            type="button"
            onClick={() => onChange(t)}
            className={cn(
              "flex-1 rounded-lg border px-3 py-2.5 text-sm font-semibold transition-all text-center",
              target === t
                ? "border-brand/50 bg-brand-muted text-foreground"
                : "border-border/50 text-muted-foreground hover:border-brand/25",
            )}
          >
            {t === "local" ? "My computer" : "Cloud server"}
          </button>
        ))}
      </div>
    </div>
  );
}

function ReceiptLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-sm text-foreground font-medium">{value}</span>
    </div>
  );
}

function buildRecipeSource(appName: string, state: WizardState): string {
  const name = appName || "app";
  const lines: string[] = [`App: ${name}`, ""];

  // Collect all database and cache deps across services for shared blocks.
  let hasDb = false;
  let dbEngine = "postgres";
  let dbVersion = "16";
  let dbStorage = "10Gi";
  let hasCache = false;

  for (const svc of state.services) {
    const dbDep = svc.dependencies.find((d) => d.type === "database" && d.enabled);
    const cacheDep = svc.dependencies.find((d) => d.type === "cache" && d.enabled);
    if (dbDep) {
      hasDb = true;
      dbEngine = dbDep.engine;
      dbVersion = dbDep.version;
      dbStorage = dbDep.storage;
    }
    if (cacheDep) hasCache = true;
  }

  // Reserved words that can't be used as slice names.
  const reserved = new Set(["app", "images", "namespace", "crds"]);

  // Generate each service block.
  for (const svc of state.services) {
    let svcName = svc.serviceName || svc.aliasName || name;
    // Avoid collision with reserved keywords.
    if (reserved.has(svcName.toLowerCase())) {
      svcName = svcName + "-svc";
    }
    const isDatabaseOnly = state.services.length === 1 && state.selectedCombo === "database-only";
    if (isDatabaseOnly) continue;

    lines.push(`${svcName}:`);
    if (svc.aliasImage) {
      lines.push(`  from image ${svc.aliasImage}`);
    }
    if (svc.isPublic && svc.port > 0) {
      lines.push(`  open to the public on port ${svc.port}`);
    } else if (svc.port > 0) {
      lines.push(`  on port ${svc.port}`);
    }
    if (svc.healthCheck.enabled && svc.healthCheck.path) {
      lines.push(`  health check ${svc.healthCheck.path}`);
    }
    if (svc.replicas > 1) {
      lines.push(`  run ${svc.replicas} copies`);
    }

    // Inter-service dependencies
    const needs: string[] = [...svc.needsServices];
    const dbDep = svc.dependencies.find((d) => d.type === "database" && d.enabled);
    const cacheDep = svc.dependencies.find((d) => d.type === "cache" && d.enabled);
    if (dbDep) needs.push(`${name}-db`);
    if (cacheDep) needs.push("cache");
    if (needs.length > 0) {
      for (const dep of needs) {
        lines.push(`  needs ${dep}`);
      }
    }

    // Env vars
    for (const ev of svc.envVars) {
      if (ev.key) lines.push(`  set ${ev.key} to "${ev.value}"`);
    }
    lines.push("");
  }

  // Database service
  if (hasDb) {
    lines.push(`${name}-db:`);
    lines.push(`  ${dbEngine} ${dbVersion}, ${dbStorage}`);
    lines.push("");
  }

  // Cache service
  if (hasCache) {
    lines.push("cache:");
    lines.push("  redis 7");
    lines.push("");
  }

  return lines.join("\n").trimEnd();
}
