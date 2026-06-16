import { CheckCircle, AlertTriangle, HelpCircle, Code2 } from "lucide-react";
import type { DetectResult } from "@/api/types";

interface FrameworkDetectProps {
  result: DetectResult;
  onContinue: () => void;
  onSkip: () => void;
}

const FRAMEWORK_LABELS: Record<string, { name: string; color: string }> = {
  nextjs: { name: "Next.js", color: "#000000" },
  react: { name: "React", color: "#61DAFB" },
  vue: { name: "Vue.js", color: "#4FC08D" },
  django: { name: "Django", color: "#092E20" },
  flask: { name: "Flask", color: "#000000" },
  rails: { name: "Ruby on Rails", color: "#CC0000" },
  laravel: { name: "Laravel", color: "#FF2D20" },
  express: { name: "Express.js", color: "#000000" },
  go: { name: "Go", color: "#00ADD8" },
  fastapi: { name: "FastAPI", color: "#009688" },
  spring: { name: "Spring Boot", color: "#6DB33F" },
};

const CONFIDENCE_CONFIG: Record<string, {
  icon: typeof CheckCircle;
  label: string;
  borderColor: string;
  bgColor: string;
  textColor: string;
}> = {
  high: {
    icon: CheckCircle,
    label: "High confidence",
    borderColor: "rgba(34, 197, 94, 0.3)",
    bgColor: "rgba(34, 197, 94, 0.08)",
    textColor: "rgb(34, 197, 94)",
  },
  medium: {
    icon: AlertTriangle,
    label: "Medium confidence",
    borderColor: "rgba(234, 179, 8, 0.3)",
    bgColor: "rgba(234, 179, 8, 0.08)",
    textColor: "rgb(234, 179, 8)",
  },
  low: {
    icon: HelpCircle,
    label: "Low confidence",
    borderColor: "rgba(148, 163, 184, 0.3)",
    bgColor: "rgba(148, 163, 184, 0.08)",
    textColor: "rgb(148, 163, 184)",
  },
};

export default function FrameworkDetect({
  result,
  onContinue,
  onSkip,
}: FrameworkDetectProps) {
  const fw = FRAMEWORK_LABELS[result.framework] || {
    name: result.framework,
    color: "#ff6b35",
  };
  const defaultConf = {
    icon: HelpCircle,
    label: "Low confidence",
    borderColor: "rgba(148, 163, 184, 0.3)",
    bgColor: "rgba(148, 163, 184, 0.08)",
    textColor: "rgb(148, 163, 184)",
  };
  const conf = CONFIDENCE_CONFIG[result.confidence] ?? defaultConf;
  const ConfIcon = conf.icon;

  if (result.framework === "unknown") {
    return (
      <div
        style={{
          border: "1px solid rgba(148, 163, 184, 0.2)",
          borderRadius: 16,
          padding: 24,
          background: "rgba(148, 163, 184, 0.04)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 12 }}>
          <div
            style={{
              width: 40,
              height: 40,
              borderRadius: 12,
              background: "rgba(148, 163, 184, 0.1)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <HelpCircle style={{ width: 20, height: 20, color: "rgb(148, 163, 184)" }} />
          </div>
          <div>
            <p style={{ fontSize: 14, fontWeight: 600, color: "var(--foreground)", margin: 0 }}>
              Could not detect a framework
            </p>
            <p style={{ fontSize: 12, color: "var(--muted-foreground)", margin: "2px 0 0" }}>
              No worries — you can set everything up manually
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={onSkip}
          style={{
            width: "100%",
            padding: "10px 16px",
            borderRadius: 10,
            border: "1px solid rgba(255, 107, 53, 0.3)",
            background: "rgba(255, 107, 53, 0.08)",
            color: "#ff6b35",
            fontSize: 13,
            fontWeight: 600,
            cursor: "pointer",
            transition: "all 0.15s",
          }}
        >
          Continue with manual setup
        </button>
      </div>
    );
  }

  return (
    <div
      style={{
        border: `1px solid ${conf.borderColor}`,
        borderRadius: 16,
        padding: 24,
        background: conf.bgColor,
        transition: "all 0.2s",
      }}
    >
      <div style={{ display: "flex", alignItems: "flex-start", gap: 14, marginBottom: 16 }}>
        <div
          style={{
            width: 48,
            height: 48,
            borderRadius: 14,
            background: "rgba(255, 107, 53, 0.1)",
            border: "1px solid rgba(255, 107, 53, 0.15)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Code2 style={{ width: 24, height: 24, color: "#ff6b35" }} />
        </div>
        <div style={{ flex: 1 }}>
          <p
            style={{
              fontSize: 16,
              fontWeight: 700,
              color: "var(--foreground)",
              margin: 0,
              lineHeight: 1.3,
            }}
          >
            We detected a{" "}
            <span style={{ color: "#ff6b35" }}>{fw.name}</span>{" "}
            app
          </p>
          <div
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 5,
              marginTop: 6,
              padding: "3px 8px",
              borderRadius: 6,
              background: conf.bgColor,
              border: `1px solid ${conf.borderColor}`,
            }}
          >
            <ConfIcon style={{ width: 12, height: 12, color: conf.textColor }} />
            <span style={{ fontSize: 11, fontWeight: 600, color: conf.textColor }}>
              {conf.label}
            </span>
          </div>
        </div>
      </div>

      {/* Detection details */}
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "1fr 1fr",
          gap: 8,
          marginBottom: 16,
          padding: 12,
          borderRadius: 10,
          background: "rgba(0, 0, 0, 0.15)",
          border: "1px solid rgba(255, 255, 255, 0.04)",
        }}
      >
        {result.language && (
          <DetailItem label="Language" value={result.language} />
        )}
        {result.port > 0 && (
          <DetailItem label="Port" value={String(result.port)} />
        )}
        {result.base_image && (
          <DetailItem label="Base image" value={result.base_image} />
        )}
        {result.health_path && (
          <DetailItem label="Health check" value={result.health_path} />
        )}
      </div>

      <div style={{ display: "flex", gap: 8 }}>
        <button
          type="button"
          onClick={onContinue}
          style={{
            flex: 1,
            padding: "10px 16px",
            borderRadius: 10,
            border: "none",
            background: "#ff6b35",
            color: "#fff",
            fontSize: 13,
            fontWeight: 600,
            cursor: "pointer",
            transition: "background 0.15s",
          }}
          onMouseEnter={(e) => { (e.target as HTMLButtonElement).style.background = "#e55a2b"; }}
          onMouseLeave={(e) => { (e.target as HTMLButtonElement).style.background = "#ff6b35"; }}
        >
          Continue with {fw.name}
        </button>
        <button
          type="button"
          onClick={onSkip}
          style={{
            padding: "10px 16px",
            borderRadius: 10,
            border: "1px solid rgba(255, 255, 255, 0.1)",
            background: "transparent",
            color: "var(--muted-foreground)",
            fontSize: 13,
            fontWeight: 500,
            cursor: "pointer",
            transition: "all 0.15s",
          }}
        >
          Manual setup
        </button>
      </div>
    </div>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p style={{ fontSize: 10, color: "var(--muted-foreground)", margin: 0, textTransform: "uppercase", letterSpacing: "0.05em" }}>
        {label}
      </p>
      <p style={{ fontSize: 13, color: "var(--foreground)", margin: "2px 0 0", fontFamily: "monospace" }}>
        {value}
      </p>
    </div>
  );
}
