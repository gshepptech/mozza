import { useState } from "react";
import type { Template } from "../../api/types";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { Eye, EyeOff, Settings } from "lucide-react";

interface TemplateConfigProps {
  template: Template;
  values: Record<string, string>;
  onChange: (values: Record<string, string>) => void;
}

export default function TemplateConfig({ template, values, onChange }: TemplateConfigProps) {
  const [revealedPasswords, setRevealedPasswords] = useState<Set<string>>(new Set());

  const togglePassword = (key: string) => {
    setRevealedPasswords((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const setValue = (key: string, val: string) => {
    onChange({ ...values, [key]: val });
  };

  const missingRequired = template.variables
    .filter((v) => v.required)
    .filter((v) => {
      const val = values[v.key] ?? v.default;
      return !val || val.trim() === "";
    });

  return (
    <div className="space-y-6">
      {/* Template header */}
      <div className="flex items-start gap-4">
        <span className="text-3xl leading-none" role="img" aria-label={template.name}>
          {template.icon}
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-base font-bold text-foreground">{template.name}</p>
          <p className="text-xs text-muted-foreground mt-0.5">{template.description}</p>
          <div className="flex items-center gap-2 mt-2">
            <Badge variant="outline" className="text-[10px] px-1.5 py-0">
              {template.category}
            </Badge>
            {template.est_resources && (
              <Badge variant="outline" className="text-[10px] px-1.5 py-0 border-yellow-400/30 text-yellow-400/80">
                {template.est_resources}
              </Badge>
            )}
          </div>
        </div>
      </div>

      {/* Variable form */}
      {template.variables.length === 0 ? (
        <div className="flex flex-col items-center py-10 text-center">
          <Settings className="h-6 w-6 text-muted-foreground/50 mb-3" />
          <p className="text-sm text-muted-foreground">
            No configuration needed — this template is ready to deploy.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            Configuration
          </p>
          {template.variables.map((v) => {
            const currentVal = values[v.key] ?? "";

            if (v.type === "boolean") {
              const checked = (currentVal || v.default) === "true";
              return (
                <div key={v.key} className="flex items-center gap-3">
                  <button
                    type="button"
                    className={cn(
                      "flex h-5 w-5 shrink-0 items-center justify-center rounded border-2 transition-colors",
                      checked
                        ? "border-brand bg-brand"
                        : "border-muted-foreground/30 bg-transparent",
                    )}
                    onClick={() => setValue(v.key, checked ? "false" : "true")}
                  >
                    {checked && (
                      <svg className="h-3 w-3 text-primary-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                      </svg>
                    )}
                  </button>
                  <div>
                    <Label className="text-xs text-foreground font-medium">{v.label}</Label>
                    {v.description && (
                      <p className="text-[10px] text-muted-foreground mt-0.5">{v.description}</p>
                    )}
                  </div>
                </div>
              );
            }

            if (v.type === "select" && v.options) {
              return (
                <div key={v.key} className="space-y-2">
                  <Label className="text-xs text-muted-foreground font-medium flex items-center gap-1">
                    {v.label}
                    {v.required && <span className="text-red-400">*</span>}
                  </Label>
                  {v.description && (
                    <p className="text-[10px] text-muted-foreground -mt-1">{v.description}</p>
                  )}
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-1.5">
                    {v.options.map((opt) => {
                      const active = (currentVal || v.default) === opt;
                      return (
                        <button
                          key={opt}
                          type="button"
                          className={cn(
                            "rounded-lg border px-3 py-2 text-xs font-semibold transition-all text-left",
                            active
                              ? "border-brand/50 bg-brand-muted text-foreground"
                              : "border-border/50 bg-elevated text-muted-foreground hover:border-brand/30",
                          )}
                          onClick={() => setValue(v.key, opt)}
                        >
                          {opt}
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            }

            // string, password, number
            const isPassword = v.type === "password";
            const revealed = revealedPasswords.has(v.key);

            return (
              <div key={v.key} className="space-y-2">
                <Label className="text-xs text-muted-foreground font-medium flex items-center gap-1">
                  {v.label}
                  {v.required && <span className="text-red-400">*</span>}
                </Label>
                {v.description && (
                  <p className="text-[10px] text-muted-foreground -mt-1">{v.description}</p>
                )}
                <div className="relative">
                  <Input
                    type={isPassword && !revealed ? "password" : v.type === "number" ? "number" : "text"}
                    value={currentVal}
                    onChange={(e) => setValue(v.key, e.target.value)}
                    placeholder={v.default || `Enter ${v.label.toLowerCase()}`}
                    className={cn(
                      "h-10 text-sm bg-elevated",
                      isPassword && "pr-10",
                      v.type === "number" || isPassword ? "font-mono" : "",
                    )}
                  />
                  {isPassword && (
                    <button
                      type="button"
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground/50 hover:text-muted-foreground transition-colors"
                      onClick={() => togglePassword(v.key)}
                    >
                      {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  )}
                </div>
              </div>
            );
          })}

          {/* Validation hint */}
          {missingRequired.length > 0 && (
            <p className="text-[11px] text-red-400/80 mt-2">
              {missingRequired.length} required field{missingRequired.length > 1 ? "s" : ""} remaining
            </p>
          )}
        </div>
      )}
    </div>
  );
}
