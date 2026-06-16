// glossary.ts — Plain-English definitions for every technical term in the deploy wizard.
// Used by HelpTooltip and InlineHelp components.
// Rules: plain text must be understandable by a 14-year-old, under 200 chars,
// and must not reference other unexplained terms.

export interface GlossaryEntry {
  term: string;
  plain: string;
  example?: string;
}

export const glossary: Record<string, GlossaryEntry> = {
  "container-image": {
    term: "App package",
    plain: "A packaged version of your app that's ready to run — like a zip file with your app and everything it needs inside.",
    example: "ghcr.io/mycompany/my-api:v1.2.0",
  },
  port: {
    term: "Port",
    plain: "A numbered door on your app that traffic comes through. Your app picks a door number when it starts up.",
    example: "Node.js usually uses 3000. Go uses 8080.",
  },
  replica: {
    term: "Running copies",
    plain: "How many identical copies of your app run at the same time. More copies means it can handle more visitors and stays up if one fails.",
  },
  cluster: {
    term: "Cluster",
    plain: "A group of servers that work together to run your apps. You deploy your app to the cluster and it handles the rest.",
  },
  pod: {
    term: "Running instance",
    plain: "One running copy of your app on the server. You don't need to manage these directly — Mozza handles it.",
  },
  "environment-variable": {
    term: "App setting",
    plain: "A setting your app reads when it starts, like a database address or a secret key. Set them here so your app knows how to connect to things.",
    example: "DATABASE_URL, LOG_LEVEL, NODE_ENV",
  },
  secret: {
    term: "Sensitive setting",
    plain: "A setting that contains private info like passwords or API keys. Marked as sensitive so it stays hidden.",
    example: "Database passwords, API tokens, encryption keys",
  },
  "health-check": {
    term: "Health check",
    plain: "Mozza periodically visits a URL on your app to make sure it's still running. If the check fails, Mozza restarts the app automatically.",
  },
  "http-vs-tcp": {
    term: "Check type",
    plain: "Web check (HTTP) visits a URL on your app. Connection check (TCP) just verifies the port is open. Web check is better if your app has a webpage.",
  },
  "cpu-memory": {
    term: "Resources",
    plain: "How much computing power and memory your app gets. S is light, M is moderate, L is heavy, XL is very heavy.",
  },
  "auto-scaling": {
    term: "Automatic copies",
    plain: "Mozza adds more copies when your app gets busy and removes them when traffic drops, so you only use what you need.",
  },
  "storage-gi": {
    term: "Disk space",
    plain: "How much storage your database gets, measured in gigabytes (GB). 10 GB is plenty for most small apps.",
    example: "5 GB for prototypes, 10 GB for most apps, 50 GB for large datasets",
  },
  "database-url": {
    term: "Database address",
    plain: "The address your app uses to connect to the database. Mozza generates this automatically — you usually don't need to change it.",
    example: "postgres://myapp-db:5432/myapp",
  },
  "cache-redis": {
    term: "Fast storage",
    plain: "A fast temporary store that keeps frequently used data in memory so your app responds quicker. Good for sessions and counters.",
  },
  "docker-compose": {
    term: "Docker Compose",
    plain: "A file that describes how to run multiple services together on one machine. Mozza can import these.",
  },
  helm: {
    term: "Helm chart",
    plain: "A pre-made package for deploying apps to servers. Like an app installer, but for cloud apps.",
  },
  dockerfile: {
    term: "Dockerfile",
    plain: "A recipe file that tells a build system how to package your app. Your developer creates this.",
  },
  registry: {
    term: "Package registry",
    plain: "An online store where app packages are uploaded and downloaded from. Like an app store, but for server apps.",
    example: "GitHub Packages, Docker Hub",
  },
  "service-account": {
    term: "Service account",
    plain: "A special account that Mozza uses to talk to your servers. Your admin sets this up.",
  },
  rbac: {
    term: "Permissions",
    plain: "Rules that control what Mozza is allowed to do on your servers. If you see permission errors, your admin needs to update these.",
  },
  namespace: {
    term: "Environment",
    plain: "A separate space on your server for different purposes — like development, staging, or production.",
    example: "development, staging, production",
  },
  node: {
    term: "Server",
    plain: "One physical or virtual machine in your cluster. Your apps run on these servers.",
  },
  statefulset: {
    term: "Copies with own storage",
    plain: "Each copy of your app keeps its own separate data that survives restarts. Good for databases.",
  },
  replicaset: {
    term: "Identical copies",
    plain: "Multiple identical copies of your app for reliability. If one fails, others keep working. Data is shared, not per-copy.",
  },
  daemonset: {
    term: "One per server",
    plain: "Puts exactly one copy on every server in your system. Good for monitoring and logging tools.",
  },
  cronjob: {
    term: "Scheduled task",
    plain: "Runs your app on a schedule (like every hour or every night) and stops when done.",
    example: "Nightly backups, hourly data sync, weekly reports",
  },
  domain: {
    term: "Web address",
    plain: "The URL people type to visit your app, like app.yourcompany.com.",
    example: "app.yourcompany.com, api.example.org",
  },
  ingress: {
    term: "Public access",
    plain: "Makes your app accessible from the internet so people outside your system can reach it.",
  },
  "deploy-target": {
    term: "Where to run",
    plain: "'My computer' runs the app on your machine for testing. 'Cloud server' deploys it to your team's servers so others can access it.",
  },
  "workload-type": {
    term: "How to run it",
    plain: "Tells Mozza how your app should behave — as identical copies, with its own storage, on every server, or on a schedule.",
  },
};
