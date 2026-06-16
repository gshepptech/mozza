// field-help.ts — Per-field help text and examples for the deploy wizard.
// Separate from glossary (which is per-concept). This is per-input-field.

export interface FieldHelpEntry {
  label: string;
  hint?: string;
  help?: string;
  placeholder?: string;
  examples?: string[];
}

export const fieldHelp: Record<string, FieldHelpEntry> = {
  "image-url": {
    label: "App package URL",
    hint: "The address of your packaged app — your developer or CI pipeline can provide this",
    help: "It looks like: ghcr.io/yourcompany/app:v1.0 or docker.io/nginx:latest. If you don't have one, try the Classics tab for pre-built apps.",
    placeholder: "Paste your app's package URL",
    examples: [
      "ghcr.io/myorg/api:latest",
      "docker.io/nginx:alpine",
      "registry.example.com/app:v2.0",
    ],
  },
  port: {
    label: "Port number",
    hint: "The door number your app uses for traffic",
    help: "Your app listens on a specific port when it starts. If your app prints 'Listening on port XXXX' in its logs, that's the number. Check your app's config or Dockerfile if unsure.",
    placeholder: "3000",
    examples: [
      "3000 (Node.js, React)",
      "8080 (Go, Java, Spring)",
      "5000 (Python Flask)",
      "4000 (Elixir Phoenix)",
    ],
  },
  "storage-size": {
    label: "Database storage",
    hint: "How much disk space for your database",
    help: "Choose based on how much data you expect. You can increase it later but can't shrink it.",
    placeholder: "10",
    examples: [
      "5 GB — Small projects, prototypes",
      "10 GB — Most apps",
      "25 GB — Growing apps with lots of data",
      "50 GB — Large datasets",
    ],
  },
  "db-engine": {
    label: "Database type",
    hint: "Not sure? PostgreSQL is the safe choice",
    help: "PostgreSQL works for almost everything. MySQL is common with PHP apps. MongoDB stores data as documents (JSON-like).",
    examples: [
      "PostgreSQL — Most popular, works for everything",
      "MySQL — Common with PHP and WordPress",
      "MongoDB — For document-style data (JSON)",
    ],
  },
  "env-var": {
    label: "App settings",
    hint: "Settings your app reads when it starts — like addresses, modes, and keys",
    help: "Environment variables configure how your app behaves. DATABASE_URL tells it where the database is. NODE_ENV tells it whether it's in testing or live mode. Your app's docs will list which ones it needs.",
    examples: [
      "LOG_LEVEL=info — How detailed the logs are",
      "NODE_ENV=production — Tells the app it's live",
      "DATABASE_URL — Where to find the database (auto-generated)",
    ],
  },
  "health-path": {
    label: "Health check URL",
    hint: "The URL Mozza visits to check if your app is alive",
    help: "Most web frameworks have a built-in health endpoint. If yours doesn't, '/' (the homepage) works as a basic check.",
    placeholder: "/health",
    examples: ["/health", "/healthz", "/ready", "/ping", "/"],
  },
  "resource-size": {
    label: "Resource size",
    hint: "How much computing power your app gets",
    help: "S handles light traffic and simple apps. M is good for most apps. L handles heavy traffic. XL is for data crunching and very heavy workloads.",
    examples: [
      "S — Light traffic, simple apps",
      "M — Moderate traffic, most apps",
      "L — Heavy traffic, complex processing",
      "XL — Very heavy traffic, data crunching",
    ],
  },
  schedule: {
    label: "Run schedule",
    hint: "How often this task should run",
    help: "Choose a preset or set a custom schedule. Your task will start at the scheduled time, do its work, then stop until the next run.",
    examples: [
      "Every hour — Runs once per hour",
      "Every day at midnight — Runs once per day",
      "Every Monday at midnight — Runs once per week",
    ],
  },
  "custom-domain": {
    label: "Custom web address (optional)",
    hint: "Leave blank and we'll give you one automatically",
    help: "To use your own address (like app.yourcompany.com), enter it here. You'll need to update your DNS settings to point to your server — Mozza will show you how after deploying.",
    placeholder: "app.yourcompany.com",
  },
  replicas: {
    label: "Running copies",
    hint: "More copies = handles more traffic, stays up if one fails",
    help: "1 copy is fine for development. 2-3 copies is recommended for production — your app stays up even if one copy fails. 4+ copies handles heavy traffic.",
    examples: [
      "1 — Development, testing",
      "2-3 — Production (recommended)",
      "4+ — High traffic",
    ],
  },
};
