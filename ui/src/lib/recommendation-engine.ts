import type { Trait, WorkloadType, Recommendation } from "./interview-types";

// ─── Inference Rules ────────────────────────────────────────

interface InferenceRule {
  match: (traits: Trait[]) => boolean;
  workloadType: WorkloadType;
  confidence: "high" | "medium";
  reasoning: string;
  explanation: string;
}

const RULES: InferenceRule[] = [
  {
    // All three traits → StatefulSet (data trumps)
    match: (t) => t.includes("web-facing") && t.includes("stateful") && t.includes("worker"),
    workloadType: "statefulset",
    confidence: "high",
    reasoning:
      "Your app is web-facing, stores data, and runs background tasks. " +
      "Since it manages its own data, each copy needs dedicated storage.",
    explanation: "Like assigning a dedicated chef to each station",
  },
  {
    // Stateful + web-facing → StatefulSet
    match: (t) => t.includes("stateful") && t.includes("web-facing"),
    workloadType: "statefulset",
    confidence: "high",
    reasoning:
      "Your app serves web traffic and stores data. " +
      "Each copy needs its own data that survives restarts to keep data safe.",
    explanation: "Like assigning a dedicated chef to each station",
  },
  {
    // Stateful + worker → StatefulSet
    match: (t) => t.includes("stateful") && t.includes("worker"),
    workloadType: "statefulset",
    confidence: "high",
    reasoning:
      "Your app processes tasks in the background and stores data. " +
      "Each worker needs its own dedicated data space.",
    explanation: "Like assigning a dedicated chef to each station",
  },
  {
    // Stateful only → StatefulSet
    match: (t) => t.includes("stateful") && t.length === 1,
    workloadType: "statefulset",
    confidence: "high",
    reasoning:
      "Your app manages data, so each copy needs its own data that survives restarts.",
    explanation: "Like assigning a dedicated chef to each station",
  },
  {
    // Web-facing only → ReplicaSet
    match: (t) => t.includes("web-facing") && !t.includes("stateful"),
    workloadType: "replicaset",
    confidence: "high",
    reasoning:
      "Your app serves web traffic without managing its own data. " +
      "Identical copies give you the best availability and scaling.",
    explanation: "Like having multiple ovens running the same recipe",
  },
  {
    // Worker only → ReplicaSet (long-running default)
    match: (t) => t.includes("worker") && t.length === 1,
    workloadType: "replicaset",
    confidence: "medium",
    reasoning:
      "Your app runs background tasks. For long-running workers, " +
      "identical copies handle the workload. If it runs on a schedule, " +
      "switch to Scheduled below.",
    explanation: "Like having multiple ovens running the same recipe",
  },
];

// ─── Public API ─────────────────────────────────────────────

/**
 * Given a set of traits, return the recommended workload type
 * with reasoning and pizza-metaphor explanation.
 */
export function getRecommendation(traits: Trait[]): Recommendation {
  for (const rule of RULES) {
    if (rule.match(traits)) {
      return {
        workloadType: rule.workloadType,
        confidence: rule.confidence,
        reasoning: rule.reasoning,
        explanation: rule.explanation,
      };
    }
  }

  // Fallback — shouldn't happen if at least one trait is selected
  return {
    workloadType: "replicaset",
    confidence: "medium",
    reasoning: "We recommend identical copies as a safe default.",
    explanation: "Like having multiple ovens running the same recipe",
  };
}

/**
 * Human-readable label for a workload type.
 */
export function workloadLabel(wt: WorkloadType): string {
  switch (wt) {
    case "replicaset": return "Identical copies";
    case "statefulset": return "Copies with own storage";
    case "daemonset": return "One per server";
    case "cronjob": return "Runs on a timer";
  }
}

/**
 * Short subtitle for each workload card.
 */
export function workloadSubtitle(wt: WorkloadType): string {
  switch (wt) {
    case "replicaset": return "Multiple copies for reliability. If one fails, others keep working.";
    case "statefulset": return "Each copy remembers its own data. Good for databases.";
    case "daemonset": return "Puts one copy on every server in your system. Good for monitoring.";
    case "cronjob": return "Runs at specific times, then stops until the next run.";
  }
}

/**
 * Pizza analogy for each workload type.
 */
export function workloadAnalogy(wt: WorkloadType): string {
  switch (wt) {
    case "replicaset": return "Like having multiple ovens running the same recipe";
    case "statefulset": return "Like assigning a dedicated chef to each station";
    case "daemonset": return "Like putting a menu in every window";
    case "cronjob": return "Like a timer that fires up the oven on a schedule";
  }
}
