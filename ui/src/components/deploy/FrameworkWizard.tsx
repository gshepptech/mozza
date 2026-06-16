import { useState, useCallback } from "react";
import { ArrowLeft, ArrowRight, CheckCircle, Copy, Check, Settings, Code2 } from "lucide-react";
import type { DetectResult } from "@/api/types";
import * as api from "@/api/client";

// ─── Types ───────────────────────────────────────────────────

interface FrameworkQuestion {
  id: string;
  label: string;
  explanation: string;
  type: "select" | "toggle" | "text";
  options?: { value: string; label: string; description?: string }[];
  defaultValue: string;
}

interface FrameworkWizardProps {
  detection: DetectResult;
  appName: string;
  onComplete: (recipeName: string, recipeSource: string) => void;
  onBack: () => void;
}

// ─── Framework question flows ────────────────────────────────

function getQuestionsForFramework(detection: DetectResult): FrameworkQuestion[] {
  const fw = detection.framework;
  const details = detection.details || {};

  switch (fw) {
    case "nextjs":
      return [
        {
          id: "database",
          label: "Does your app need a database?",
          explanation: "We'll set up the database and connect it to your app automatically.",
          type: "select",
          options: [
            { value: "none", label: "No database", description: "My app doesn't need one" },
            { value: "postgres", label: "PostgreSQL", description: "Most popular — great for everything" },
            { value: "mysql", label: "MySQL", description: "Classic relational database" },
            { value: "mongodb", label: "MongoDB", description: "Document store for flexible schemas" },
          ],
          defaultValue: "none",
        },
        {
          id: "cache",
          label: "Want a Redis cache for faster performance?",
          explanation: "Redis stores frequently accessed data in memory so your app responds faster.",
          type: "toggle",
          defaultValue: "no",
        },
      ];

    case "django":
      return [
        {
          id: "database",
          label: details.has_postgres ? "Your app uses PostgreSQL. Set it up automatically?" : "Which database does your app use?",
          explanation: details.has_postgres
            ? "We found PostgreSQL in your requirements. We'll provision it and configure the connection."
            : "Django needs a database. We'll set it up and connect it for you.",
          type: "select",
          options: [
            { value: "postgres", label: "PostgreSQL", description: details.has_postgres ? "Detected in your project" : "Recommended for Django" },
            { value: "mysql", label: "MySQL", description: "Also well-supported by Django" },
          ],
          defaultValue: "postgres",
        },
        {
          id: "worker",
          label: details.has_celery ? "Your app has Celery. Want a background worker?" : "Need background task processing?",
          explanation: details.has_celery
            ? "We found Celery in your requirements. A worker process will handle async tasks."
            : "A background worker runs tasks like sending emails or processing uploads.",
          type: "toggle",
          defaultValue: details.has_celery ? "yes" : "no",
        },
        {
          id: "cache",
          label: "Add Redis for caching and task queues?",
          explanation: "Redis powers both caching and Celery task queues. Recommended if using background workers.",
          type: "toggle",
          defaultValue: details.has_celery ? "yes" : "no",
        },
      ];

    case "flask":
    case "fastapi":
      return [
        {
          id: "database",
          label: "Does your app need a database?",
          explanation: "We'll provision the database and set the connection URL in your environment.",
          type: "select",
          options: [
            { value: "none", label: "No database" },
            { value: "postgres", label: "PostgreSQL", description: "Recommended" },
            { value: "mysql", label: "MySQL" },
            { value: "mongodb", label: "MongoDB" },
          ],
          defaultValue: details.has_sqlalchemy ? "postgres" : "none",
        },
        {
          id: "cache",
          label: "Want a Redis cache?",
          explanation: "Useful for session storage, rate limiting, and caching API responses.",
          type: "toggle",
          defaultValue: "no",
        },
      ];

    case "rails":
      return [
        {
          id: "database",
          label: "Which database does your Rails app use?",
          explanation: "Rails needs a database. We'll set it up with the right adapter.",
          type: "select",
          options: [
            { value: "postgres", label: "PostgreSQL", description: "Default for production Rails" },
            { value: "mysql", label: "MySQL" },
          ],
          defaultValue: "postgres",
        },
        {
          id: "worker",
          label: details.has_sidekiq ? "Your app has Sidekiq. Want a background worker?" : "Need background job processing?",
          explanation: details.has_sidekiq
            ? "We found Sidekiq in your Gemfile. A dedicated worker will process your background jobs."
            : "A background worker handles async tasks like emails and file processing.",
          type: "toggle",
          defaultValue: details.has_sidekiq ? "yes" : "no",
        },
        {
          id: "cache",
          label: "Add Redis?",
          explanation: details.has_sidekiq
            ? "Sidekiq requires Redis. We'll set it up for both job queues and caching."
            : "Redis is used for caching, sessions, and job queues.",
          type: "toggle",
          defaultValue: details.has_sidekiq ? "yes" : "no",
        },
      ];

    case "laravel":
      return [
        {
          id: "database",
          label: "Which database?",
          explanation: "Laravel supports several databases out of the box.",
          type: "select",
          options: [
            { value: "postgres", label: "PostgreSQL" },
            { value: "mysql", label: "MySQL", description: "Traditional Laravel choice" },
          ],
          defaultValue: "mysql",
        },
        {
          id: "octane",
          label: "Using Laravel Octane for better performance?",
          explanation: "Octane serves your app with Swoole or RoadRunner for much faster response times. If you're not sure, leave this off.",
          type: "toggle",
          defaultValue: "no",
        },
        {
          id: "cache",
          label: "Add Redis for queues and caching?",
          explanation: "Laravel uses Redis for queue workers, cache, and real-time broadcasting.",
          type: "toggle",
          defaultValue: "no",
        },
      ];

    case "go":
      return [
        {
          id: "static_binary",
          label: "Static binary or needs CGO?",
          explanation: "A static binary is smaller and more portable. CGO is needed for some C libraries like SQLite.",
          type: "select",
          options: [
            { value: "static", label: "Static binary", description: "Smaller image, no C dependencies" },
            { value: "cgo", label: "Needs CGO", description: "Required for SQLite, some crypto libs" },
          ],
          defaultValue: "static",
        },
        {
          id: "database",
          label: "Does your app need a database?",
          explanation: "We'll provision it and pass the connection string as an environment variable.",
          type: "select",
          options: [
            { value: "none", label: "No database" },
            { value: "postgres", label: "PostgreSQL" },
            { value: "mysql", label: "MySQL" },
          ],
          defaultValue: "none",
        },
        {
          id: "cache",
          label: "Want a Redis cache?",
          explanation: "Useful for session storage and caching.",
          type: "toggle",
          defaultValue: "no",
        },
      ];

    default:
      return [
        {
          id: "database",
          label: "Does your app need a database?",
          explanation: "We'll set up the database alongside your app.",
          type: "select",
          options: [
            { value: "none", label: "No database" },
            { value: "postgres", label: "PostgreSQL" },
            { value: "mysql", label: "MySQL" },
            { value: "mongodb", label: "MongoDB" },
          ],
          defaultValue: "none",
        },
        {
          id: "cache",
          label: "Want a Redis cache?",
          explanation: "Speeds up your app by storing data in memory.",
          type: "toggle",
          defaultValue: "no",
        },
      ];
  }
}

// ─── Sub-components ──────────────────────────────────────────

function SelectQuestion({
  question,
  value,
  onChange,
}: {
  question: FrameworkQuestion;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {question.options?.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => onChange(opt.value)}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            padding: "12px 14px",
            borderRadius: 12,
            border: `1px solid ${value === opt.value ? "rgba(255, 107, 53, 0.4)" : "rgba(255, 255, 255, 0.08)"}`,
            background: value === opt.value ? "rgba(255, 107, 53, 0.06)" : "rgba(255, 255, 255, 0.02)",
            cursor: "pointer",
            textAlign: "left",
            transition: "all 0.15s",
          }}
        >
          <div
            style={{
              width: 18,
              height: 18,
              borderRadius: 9,
              border: `2px solid ${value === opt.value ? "#ff6b35" : "rgba(255, 255, 255, 0.15)"}`,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
              transition: "all 0.15s",
            }}
          >
            {value === opt.value && (
              <div style={{ width: 8, height: 8, borderRadius: 4, background: "#ff6b35" }} />
            )}
          </div>
          <div style={{ flex: 1 }}>
            <p style={{ fontSize: 13, fontWeight: 600, color: "var(--foreground)", margin: 0 }}>
              {opt.label}
            </p>
            {opt.description && (
              <p style={{ fontSize: 11, color: "var(--muted-foreground)", margin: "2px 0 0" }}>
                {opt.description}
              </p>
            )}
          </div>
        </button>
      ))}
    </div>
  );
}

function ToggleQuestion({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const isOn = value === "yes";
  return (
    <button
      type="button"
      onClick={() => onChange(isOn ? "no" : "yes")}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "12px 14px",
        borderRadius: 12,
        border: `1px solid ${isOn ? "rgba(255, 107, 53, 0.4)" : "rgba(255, 255, 255, 0.08)"}`,
        background: isOn ? "rgba(255, 107, 53, 0.06)" : "rgba(255, 255, 255, 0.02)",
        cursor: "pointer",
        width: "100%",
        textAlign: "left",
        transition: "all 0.15s",
      }}
    >
      <div
        style={{
          width: 40,
          height: 22,
          borderRadius: 11,
          background: isOn ? "#ff6b35" : "rgba(255, 255, 255, 0.1)",
          position: "relative",
          transition: "background 0.2s",
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: 16,
            height: 16,
            borderRadius: 8,
            background: "#fff",
            position: "absolute",
            top: 3,
            left: isOn ? 21 : 3,
            transition: "left 0.2s",
            boxShadow: "0 1px 3px rgba(0,0,0,0.2)",
          }}
        />
      </div>
      <span style={{ fontSize: 13, fontWeight: 500, color: isOn ? "var(--foreground)" : "var(--muted-foreground)" }}>
        {isOn ? "Yes" : "No"}
      </span>
    </button>
  );
}

function RecipePreview({
  recipe,
  dockerfile,
  onEditRecipe,
}: {
  recipe: string;
  dockerfile: string;
  onEditRecipe: (s: string) => void;
}) {
  const [activeTab, setActiveTab] = useState<"recipe" | "dockerfile">("recipe");
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState(recipe);
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    const text = activeTab === "recipe" ? recipe : dockerfile;
    navigator.clipboard.writeText(text).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (editing) {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        <textarea
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          style={{
            width: "100%",
            height: 240,
            borderRadius: 12,
            border: "1px solid rgba(255, 255, 255, 0.08)",
            background: "rgba(0, 0, 0, 0.2)",
            padding: 14,
            fontFamily: "monospace",
            fontSize: 12,
            color: "var(--foreground)",
            resize: "none",
            outline: "none",
          }}
        />
        <div style={{ display: "flex", gap: 8 }}>
          <button
            type="button"
            onClick={() => { onEditRecipe(editValue); setEditing(false); }}
            style={{
              padding: "8px 14px",
              borderRadius: 8,
              border: "none",
              background: "#ff6b35",
              color: "#fff",
              fontSize: 12,
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Save changes
          </button>
          <button
            type="button"
            onClick={() => { setEditValue(recipe); setEditing(false); }}
            style={{
              padding: "8px 14px",
              borderRadius: 8,
              border: "1px solid rgba(255, 255, 255, 0.1)",
              background: "transparent",
              color: "var(--muted-foreground)",
              fontSize: 12,
              fontWeight: 500,
              cursor: "pointer",
            }}
          >
            Cancel
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      style={{
        borderRadius: 12,
        border: "1px solid rgba(255, 255, 255, 0.06)",
        overflow: "hidden",
        background: "rgba(0, 0, 0, 0.15)",
      }}
    >
      {/* Tab bar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          borderBottom: "1px solid rgba(255, 255, 255, 0.06)",
          padding: "0 12px",
        }}
      >
        <div style={{ display: "flex", gap: 0 }}>
          {(["recipe", "dockerfile"] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              style={{
                padding: "8px 14px",
                fontSize: 12,
                fontWeight: activeTab === tab ? 600 : 400,
                color: activeTab === tab ? "#ff6b35" : "var(--muted-foreground)",
                background: "none",
                border: "none",
                borderBottom: activeTab === tab ? "2px solid #ff6b35" : "2px solid transparent",
                cursor: "pointer",
                transition: "all 0.15s",
              }}
            >
              {tab === "recipe" ? "Recipe" : "Dockerfile"}
            </button>
          ))}
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <button
            type="button"
            onClick={handleCopy}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 4,
              padding: "4px 8px",
              background: "none",
              border: "none",
              color: "var(--muted-foreground)",
              fontSize: 11,
              cursor: "pointer",
            }}
          >
            {copied ? <Check style={{ width: 12, height: 12 }} /> : <Copy style={{ width: 12, height: 12 }} />}
            {copied ? "Copied" : "Copy"}
          </button>
          {activeTab === "recipe" && (
            <button
              type="button"
              onClick={() => { setEditValue(recipe); setEditing(true); }}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 4,
                padding: "4px 8px",
                background: "none",
                border: "none",
                color: "var(--muted-foreground)",
                fontSize: 11,
                cursor: "pointer",
              }}
            >
              <Settings style={{ width: 12, height: 12 }} />
              Edit
            </button>
          )}
        </div>
      </div>

      {/* Code */}
      <pre
        style={{
          padding: 14,
          fontFamily: "monospace",
          fontSize: 12,
          color: "var(--foreground)",
          lineHeight: 1.6,
          whiteSpace: "pre-wrap",
          overflowY: "auto",
          maxHeight: 260,
          margin: 0,
        }}
      >
        {activeTab === "recipe" ? recipe : dockerfile}
      </pre>
    </div>
  );
}

// ─── Main FrameworkWizard ────────────────────────────────────

export default function FrameworkWizard({
  detection,
  appName,
  onComplete,
  onBack,
}: FrameworkWizardProps) {
  const questions = getQuestionsForFramework(detection);
  const totalSteps = questions.length + 1; // questions + preview

  const [currentStep, setCurrentStep] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string>>(() => {
    const defaults: Record<string, string> = {};
    for (const q of questions) {
      defaults[q.id] = q.defaultValue;
    }
    return defaults;
  });
  const [recipe, setRecipe] = useState("");
  const [dockerfile, setDockerfile] = useState("");
  const [generating, setGenerating] = useState(false);

  const isPreviewStep = currentStep === questions.length;
  const currentQuestion = questions[currentStep] || null;

  const generatePreview = useCallback(async () => {
    setGenerating(true);
    try {
      const result = await api.generateFromDetection({
        framework: detection.framework,
        language: detection.language,
        app_name: appName,
        port: detection.port,
        user_choices: answers,
      });
      setRecipe(result.recipe);
      setDockerfile(result.dockerfile);
    } catch {
      // Fallback: build a basic recipe client-side.
      const lines = [`App: ${appName}`, "", `# ${detection.framework}`, "", "App:"];
      lines.push(`  from image ${appName}:latest`);
      lines.push(`  open to the public on port ${detection.port}`);
      lines.push("  health check /health");
      lines.push("  run 1 copy");
      setRecipe(lines.join("\n"));
      setDockerfile(detection.dockerfile || "# Dockerfile not generated");
    } finally {
      setGenerating(false);
    }
  }, [detection, appName, answers]);

  const handleNext = useCallback(async () => {
    if (currentStep === questions.length - 1) {
      // Moving to preview — generate recipe.
      await generatePreview();
      setCurrentStep((s) => s + 1);
    } else if (isPreviewStep) {
      onComplete(appName, recipe);
    } else {
      setCurrentStep((s) => s + 1);
    }
  }, [currentStep, questions.length, isPreviewStep, generatePreview, onComplete, appName, recipe]);

  const handleBack = useCallback(() => {
    if (currentStep === 0) {
      onBack();
    } else {
      setCurrentStep((s) => s - 1);
    }
  }, [currentStep, onBack]);

  const fwLabel = detection.framework.charAt(0).toUpperCase() + detection.framework.slice(1);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: 10,
            background: "rgba(255, 107, 53, 0.1)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Code2 style={{ width: 18, height: 18, color: "#ff6b35" }} />
        </div>
        <div>
          <p style={{ fontSize: 14, fontWeight: 700, color: "var(--foreground)", margin: 0 }}>
            Setting up {fwLabel}
          </p>
          <p style={{ fontSize: 11, color: "var(--muted-foreground)", margin: "1px 0 0" }}>
            {isPreviewStep
              ? "Review your configuration"
              : `Question ${currentStep + 1} of ${questions.length}`}
          </p>
        </div>
      </div>

      {/* Step indicator */}
      <div style={{ display: "flex", gap: 4 }}>
        {Array.from({ length: totalSteps }, (_, i) => (
          <div
            key={i}
            style={{
              flex: 1,
              height: 3,
              borderRadius: 2,
              background: i <= currentStep ? "#ff6b35" : "rgba(255, 255, 255, 0.08)",
              transition: "background 0.2s",
            }}
          />
        ))}
      </div>

      {/* Content */}
      {!isPreviewStep && currentQuestion && (
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <div>
            <p style={{ fontSize: 15, fontWeight: 600, color: "var(--foreground)", margin: 0 }}>
              {currentQuestion.label}
            </p>
            <p style={{ fontSize: 12, color: "var(--muted-foreground)", margin: "6px 0 0", lineHeight: 1.5 }}>
              {currentQuestion.explanation}
            </p>
          </div>

          {currentQuestion.type === "select" && (
            <SelectQuestion
              question={currentQuestion}
              value={answers[currentQuestion.id] || ""}
              onChange={(v) => setAnswers((a) => ({ ...a, [currentQuestion.id]: v }))}
            />
          )}
          {currentQuestion.type === "toggle" && (
            <ToggleQuestion
              value={answers[currentQuestion.id] || "no"}
              onChange={(v) => setAnswers((a) => ({ ...a, [currentQuestion.id]: v }))}
            />
          )}
        </div>
      )}

      {isPreviewStep && (
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {generating ? (
            <div
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                padding: 40,
                color: "var(--muted-foreground)",
                fontSize: 13,
              }}
            >
              Generating your configuration...
            </div>
          ) : (
            <RecipePreview
              recipe={recipe}
              dockerfile={dockerfile}
              onEditRecipe={setRecipe}
            />
          )}
        </div>
      )}

      {/* Navigation */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          paddingTop: 12,
          borderTop: "1px solid rgba(255, 255, 255, 0.06)",
        }}
      >
        <button
          type="button"
          onClick={handleBack}
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            padding: "8px 12px",
            borderRadius: 8,
            border: "none",
            background: "transparent",
            color: "var(--muted-foreground)",
            fontSize: 13,
            cursor: "pointer",
          }}
        >
          <ArrowLeft style={{ width: 14, height: 14 }} />
          Back
        </button>

        {isPreviewStep ? (
          <button
            type="button"
            onClick={handleNext}
            disabled={generating || !recipe}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              padding: "8px 16px",
              borderRadius: 8,
              border: "none",
              background: generating || !recipe ? "rgba(255, 107, 53, 0.3)" : "#ff6b35",
              color: "#fff",
              fontSize: 13,
              fontWeight: 600,
              cursor: generating || !recipe ? "not-allowed" : "pointer",
              boxShadow: "0 0 15px rgba(255, 107, 53, 0.2)",
              transition: "all 0.15s",
            }}
          >
            <CheckCircle style={{ width: 14, height: 14 }} />
            Looks good — deploy!
          </button>
        ) : (
          <button
            type="button"
            onClick={handleNext}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              padding: "8px 16px",
              borderRadius: 8,
              border: "none",
              background: "#ff6b35",
              color: "#fff",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
              transition: "all 0.15s",
            }}
          >
            Next
            <ArrowRight style={{ width: 14, height: 14 }} />
          </button>
        )}
      </div>
    </div>
  );
}
