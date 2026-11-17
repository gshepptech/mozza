import type { Dispatch, SetStateAction } from "react";
import { Badge } from "@/components/ui/badge";
import type { Template } from "@/api/types";
import type { TraitCombo, WizardState } from "@/lib/interview-types";

const COMBO_LABELS: Record<TraitCombo, string> = {
  website: "Website",
  "api-database": "API + Database",
  "background-worker": "Background Worker",
  "database-only": "Database Only",
  custom: "Custom",
};

interface OrderReviewStepProps {
  wizardState: WizardState;
  appName: string;
  selectedTemplate: Template | null;
  recipeSource: string;
  editedRecipe: string;
  setEditedRecipe: Dispatch<SetStateAction<string>>;
  editingRecipe: boolean;
  setEditingRecipe: Dispatch<SetStateAction<boolean>>;
  showRecipe: boolean;
  setShowRecipe: Dispatch<SetStateAction<boolean>>;
}

// The final step: a receipt of what will be deployed and the generated recipe,
// which the user can view and edit before placing the order.
export function OrderReviewStep({
  wizardState,
  appName,
  selectedTemplate,
  recipeSource,
  editedRecipe,
  setEditedRecipe,
  editingRecipe,
  setEditingRecipe,
  showRecipe,
  setShowRecipe,
}: OrderReviewStepProps) {
  const selectedCombo = wizardState.selectedCombo ?? null;
  const svc = wizardState.services[0];
  const dbDep = svc?.dependencies.find((d) => d.type === "database" && d.enabled);
  const cacheDep = svc?.dependencies.find((d) => d.type === "cache" && d.enabled);
  const comboLabel = (selectedCombo ? COMBO_LABELS[selectedCombo] : undefined) ?? selectedTemplate?.name ?? "Custom";
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
}

function ReceiptLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between py-1.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-sm text-foreground font-medium">{value}</span>
    </div>
  );
}
