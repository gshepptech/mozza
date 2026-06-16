// ─── Deploy Error Intelligence ──────────────────────────────
// Maps K8s errors to human-friendly accessible messages.
// Each error answers: what happened, why, and what to do.

export interface DeployErrorInfo {
  title: string;
  message: string;
  steps: string[];
  technicalDetail: string;
}

interface ErrorPattern {
  patterns: RegExp[];
  error: DeployErrorInfo;
}

const ERROR_PATTERNS: ErrorPattern[] = [
  {
    patterns: [/ImagePullBackOff/i, /ErrImagePull/i, /image.*not found/i, /pull access denied/i],
    error: {
      title: "Can't find your app",
      message: "Mozza couldn't download the app package you specified.",
      steps: [
        "Double-check the app package URL you entered",
        "Make sure the package exists and is publicly accessible",
        "If it's a private package, check that your credentials are set up",
      ],
      technicalDetail:
        "ImagePullBackOff — the container runtime couldn't pull the specified image from the registry. Verify the image ref, tag, and pull secrets.",
    },
  },
  {
    patterns: [/Forbidden/i, /RBAC/i, /cannot create/i, /permission denied/i, /unauthorized/i],
    error: {
      title: "Permission denied",
      message: "Mozza doesn't have permission to deploy to this environment.",
      steps: [
        "Ask your system administrator to grant deployment access",
        "Share this error with them — they'll know what to do",
      ],
      technicalDetail:
        "RBAC error — the service account used by Mozza lacks the required ClusterRole/Role bindings for this namespace.",
    },
  },
  {
    patterns: [/connection refused/i, /unreachable/i, /no route to host/i, /i\/o timeout/i],
    error: {
      title: "Can't reach the kitchen",
      message:
        "We couldn't connect to your cluster. Check that it's running " +
        "and the kubeconfig is still valid.",
      steps: [
        "Make sure your cluster is running",
        "Check that your connection settings are still valid",
        "Try reconnecting from the clusters page",
      ],
      technicalDetail:
        "Connection error — the API server is unreachable. Check network connectivity, firewall rules, and kubeconfig validity.",
    },
  },
  {
    patterns: [/OOMKilled/i],
    error: {
      title: "Your app ran out of memory",
      message: "Your app tried to use more memory than was allocated.",
      steps: [
        "Try a larger resource size (L or XL in Customize)",
        "Check for memory leaks if this happens repeatedly",
      ],
      technicalDetail:
        "OOMKilled — the container exceeded its memory limit and was terminated by the kernel OOM killer.",
    },
  },
  {
    patterns: [/CrashLoopBackOff/i, /crash/i, /exit code/i],
    error: {
      title: "Your app crashed on startup",
      message:
        "Mozza started your app but it immediately stopped. This usually means there's an error in your app's code or configuration.",
      steps: [
        "Check your app's logs for errors",
        "Make sure your app runs without errors on your computer first",
        "Check that all required settings (environment variables) are set",
        "If you set a health check, make sure the URL path exists",
      ],
      technicalDetail:
        "CrashLoopBackOff — the container starts, exits with non-zero, and Kubernetes restarts it in a back-off loop.",
    },
  },
  {
    patterns: [/quota/i, /insufficient/i, /resource.*exceeded/i],
    error: {
      title: "Not enough room",
      message: "The system doesn't have enough space to run your app right now.",
      steps: [
        "Try a smaller resource size (S or M)",
        "Reduce the number of copies to 1 or 2",
        "Ask your admin if more capacity can be added",
      ],
      technicalDetail: "Resource quota exceeded or insufficient cluster resources.",
    },
  },
  {
    patterns: [/deadline exceeded/i, /timeout/i, /timed out/i, /taking too long/i],
    error: {
      title: "Taking too long",
      message: "Your app hasn't started within the expected time.",
      steps: [
        "Check your app's logs for errors",
        "Your app might need more time — this is common for large apps",
        "Try again in a few minutes",
        "If this keeps happening, try a larger resource size",
      ],
      technicalDetail: "Deployment readiness timeout. Pods are not reaching Ready state.",
    },
  },
];

/**
 * Analyze deploy log output and extract human-friendly error info.
 * Returns null if no known error pattern matched.
 */
export function analyzeDeployError(logLines: string[]): DeployErrorInfo | null {
  const fullLog = logLines.join("\n");

  for (const { patterns, error } of ERROR_PATTERNS) {
    for (const pattern of patterns) {
      if (pattern.test(fullLog)) {
        return error;
      }
    }
  }

  return null;
}

/**
 * Map deploy log step lines to pizza metaphor stage names.
 */
export function getPizzaStageLabel(stepNumber: number): string {
  switch (stepNumber) {
    case 1: return "Taking your order\u2026";
    case 2: return "Prepping ingredients\u2026";
    case 3: return "Prepping ingredients\u2026";
    case 4: return "Firing up the oven\u2026";
    case 5: return "Checking the crust\u2026";
    default: return `Step ${stepNumber}\u2026`;
  }
}

/**
 * Pizza metaphor labels for deploy progress stages.
 */
export const PIZZA_STAGE_LABELS = [
  "Taking your order\u2026",
  "Prepping ingredients\u2026",
  "Firing up the oven\u2026",
  "Checking the crust\u2026",
  "Checking the crust\u2026",
];
