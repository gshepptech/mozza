import { useEffect, useRef } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Globe } from "lucide-react";
import { HelpTooltip } from "@/components/ui/HelpTooltip";
import { InlineHelp } from "@/components/ui/InlineHelp";
import type { Trait, WizardTarget } from "@/lib/interview-types";

interface StepNetworkingProps {
  traits: Trait[];
  target: WizardTarget;
  port: number;
  isPublic: boolean;
  domain: string;
  onUpdate: (updates: { port?: number; isPublic?: boolean; domain?: string }) => void;
}

function smartDefaultPort(traits: Trait[]): number {
  if (traits.includes("web-facing")) return 3000;
  if (traits.includes("stateful")) return 5432;
  return 8080;
}

export default function StepNetworking({
  traits,
  target,
  port,
  isPublic,
  domain,
  onUpdate,
}: StepNetworkingProps) {
  const initialized = useRef(false);

  // Set smart default port on first mount
  useEffect(() => {
    if (!initialized.current) {
      initialized.current = true;
      const defaultPort = smartDefaultPort(traits);
      const defaultPublic = traits.includes("web-facing");
      if (port === 8080 || port === 0) {
        onUpdate({ port: defaultPort, isPublic: defaultPublic });
      }
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const isLocal = target === "local";

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      {/* Port input */}
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        <Label
          htmlFor="port"
          style={{ color: "rgba(255,255,255,0.7)", fontSize: 13 }}
        >
          Port <HelpTooltip term="port" />
        </Label>
        <Input
          id="port"
          type="number"
          min={1}
          max={65535}
          value={port}
          onChange={(e) => {
            const val = parseInt(e.target.value, 10);
            if (!isNaN(val)) onUpdate({ port: val });
          }}
          style={{
            maxWidth: 160,
            background: "rgba(255,255,255,0.05)",
            border: "1px solid rgba(255,255,255,0.15)",
            borderRadius: 6,
            color: "#fff",
          }}
        />
        <span style={{ color: "rgba(255,255,255,0.4)", fontSize: 12 }}>
          The port number your app uses — check its startup logs for 'Listening on port XXXX'
        </span>
        <InlineHelp label="What port should I use?">
          Check your app's startup logs for "Listening on port XXXX". Common defaults: 3000 (Node.js), 8080 (Go/Java), 5000 (Python), 4000 (Phoenix).
        </InlineHelp>
      </div>

      {/* Public toggle and domain — only for kitchen target */}
      {!isLocal && (
        <>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <span style={{ color: "rgba(255,255,255,0.7)", fontSize: 13 }}>
              Open the front door?
            </span>
            <button
              type="button"
              onClick={() => onUpdate({ isPublic: !isPublic })}
              style={{
                display: "flex",
                alignItems: "center",
                gap: 10,
                padding: "10px 16px",
                background: isPublic
                  ? "rgba(255,107,53,0.1)"
                  : "rgba(255,255,255,0.03)",
                border: `1px solid ${isPublic ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
                borderRadius: 10,
                cursor: "pointer",
                transition: "border-color 0.15s ease",
                maxWidth: 320,
              }}
            >
              <Globe
                size={20}
                color={isPublic ? "#ff6b35" : "rgba(255,255,255,0.4)"}
              />
              <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-start" }}>
                <span
                  style={{
                    color: isPublic ? "#fff" : "rgba(255,255,255,0.6)",
                    fontSize: 14,
                    fontWeight: 500,
                  }}
                >
                  {isPublic ? "Public" : "Private"}
                </span>
                <span style={{ color: "rgba(255,255,255,0.4)", fontSize: 12 }}>
                  {isPublic
                    ? "Accessible from the internet"
                    : "Only reachable by other apps in your system"}
                </span>
              </div>

              {/* Toggle indicator */}
              <div
                style={{
                  marginLeft: "auto",
                  width: 40,
                  height: 22,
                  borderRadius: 11,
                  background: isPublic ? "#ff6b35" : "rgba(255,255,255,0.15)",
                  position: "relative",
                  transition: "background 0.15s ease",
                }}
              >
                <div
                  style={{
                    width: 16,
                    height: 16,
                    borderRadius: "50%",
                    background: "#fff",
                    position: "absolute",
                    top: 3,
                    left: isPublic ? 21 : 3,
                    transition: "left 0.15s ease",
                  }}
                />
              </div>
            </button>
          </div>

          {/* Domain field — only when public + kitchen */}
          {isPublic && target === "kitchen" && (
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <Label
                htmlFor="domain"
                style={{ color: "rgba(255,255,255,0.7)", fontSize: 13 }}
              >
                Custom address? (optional)
              </Label>
              <Input
                id="domain"
                type="text"
                placeholder="e.g. api.myapp.com"
                value={domain}
                onChange={(e) => onUpdate({ domain: e.target.value })}
                style={{
                  maxWidth: 320,
                  background: "rgba(255,255,255,0.05)",
                  border: "1px solid rgba(255,255,255,0.15)",
                  borderRadius: 6,
                  color: "#fff",
                }}
              />
              <span style={{ color: "rgba(255,255,255,0.4)", fontSize: 12 }}>
                We'll generate one for you if left blank
              </span>
              <InlineHelp>Leave blank for an auto-generated address. To use your own (like app.yourcompany.com), you'll need to update your DNS settings after deploying.</InlineHelp>
            </div>
          )}
        </>
      )}

      {isLocal && (
        <span style={{ color: "rgba(255,255,255,0.4)", fontSize: 13 }}>
          Local deployments are accessible on localhost:{port}. No public
          access or domain needed.
        </span>
      )}
    </div>
  );
}
