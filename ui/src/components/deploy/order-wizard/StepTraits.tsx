import { Globe, Database, Cog, Check } from "lucide-react";
import type { Trait } from "@/lib/interview-types";

interface StepTraitsProps {
  traits: Trait[];
  onUpdate: (traits: Trait[]) => void;
}

const TRAIT_CARDS: Array<{
  trait: Trait;
  icon: typeof Globe;
  label: string;
}> = [
  { trait: "web-facing", icon: Globe, label: "Do people visit this in a browser?" },
  { trait: "stateful", icon: Database, label: "Does it store or manage data?" },
  { trait: "worker", icon: Cog, label: "Does it run tasks in the background?" },
];

export default function StepTraits({ traits, onUpdate }: StepTraitsProps) {
  const toggle = (t: Trait) => {
    if (traits.includes(t)) {
      onUpdate(traits.filter((v) => v !== t));
    } else {
      onUpdate([...traits, t]);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <p style={{ color: "rgba(255,255,255,0.5)", fontSize: 14, margin: 0 }}>
        Select everything that applies. This helps us pick the right setup.
      </p>

      <div style={{ display: "flex", gap: 12 }}>
        {TRAIT_CARDS.map(({ trait, icon: Icon, label }) => {
          const selected = traits.includes(trait);
          return (
            <button
              key={trait}
              type="button"
              onClick={() => toggle(trait)}
              style={{
                flex: 1,
                display: "flex",
                flexDirection: "column",
                alignItems: "center",
                gap: 12,
                padding: 20,
                background: "rgba(255,255,255,0.03)",
                border: `1px solid ${selected ? "#ff6b35" : "rgba(255,255,255,0.1)"}`,
                borderRadius: 10,
                cursor: "pointer",
                position: "relative",
                transition: "border-color 0.15s ease",
              }}
            >
              {selected && (
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
              <Icon
                size={28}
                color={selected ? "#ff6b35" : "rgba(255,255,255,0.5)"}
              />
              <span
                style={{
                  color: selected ? "#fff" : "rgba(255,255,255,0.7)",
                  fontSize: 14,
                  textAlign: "center",
                  lineHeight: 1.4,
                }}
              >
                {label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
