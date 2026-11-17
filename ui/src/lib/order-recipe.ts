import type { WizardState } from "@/lib/interview-types";

// buildRecipeSource renders the fast-lane wizard state as a .mozza recipe.
export function buildRecipeSource(appName: string, state: WizardState): string {
  const name = appName || "app";
  const lines: string[] = [`App: ${name}`, ""];

  // Collect all database and cache deps across services for shared blocks.
  let hasDb = false;
  let dbEngine = "postgres";
  let dbVersion = "16";
  let dbStorage = "10Gi";
  let hasCache = false;

  for (const svc of state.services) {
    const dbDep = svc.dependencies.find((d) => d.type === "database" && d.enabled);
    const cacheDep = svc.dependencies.find((d) => d.type === "cache" && d.enabled);
    if (dbDep) {
      hasDb = true;
      dbEngine = dbDep.engine;
      dbVersion = dbDep.version;
      dbStorage = dbDep.storage;
    }
    if (cacheDep) hasCache = true;
  }

  // Reserved words that can't be used as slice names.
  const reserved = new Set(["app", "images", "namespace", "crds"]);

  // Generate each service block.
  for (const svc of state.services) {
    let svcName = svc.serviceName || svc.aliasName || name;
    // Avoid collision with reserved keywords.
    if (reserved.has(svcName.toLowerCase())) {
      svcName = svcName + "-svc";
    }
    const isDatabaseOnly = state.services.length === 1 && state.selectedCombo === "database-only";
    if (isDatabaseOnly) continue;

    lines.push(`${svcName}:`);
    if (svc.aliasImage) {
      lines.push(`  from image ${svc.aliasImage}`);
    }
    if (svc.isPublic && svc.port > 0) {
      lines.push(`  open to the public on port ${svc.port}`);
    } else if (svc.port > 0) {
      lines.push(`  on port ${svc.port}`);
    }
    if (svc.healthCheck.enabled && svc.healthCheck.path) {
      lines.push(`  health check ${svc.healthCheck.path}`);
    }
    if (svc.replicas > 1) {
      lines.push(`  run ${svc.replicas} copies`);
    }

    // Inter-service dependencies
    const needs: string[] = [...svc.needsServices];
    const dbDep = svc.dependencies.find((d) => d.type === "database" && d.enabled);
    const cacheDep = svc.dependencies.find((d) => d.type === "cache" && d.enabled);
    if (dbDep) needs.push(`${name}-db`);
    if (cacheDep) needs.push("cache");
    if (needs.length > 0) {
      for (const dep of needs) {
        lines.push(`  needs ${dep}`);
      }
    }

    // Env vars
    for (const ev of svc.envVars) {
      if (ev.key) lines.push(`  set ${ev.key} to "${ev.value}"`);
    }
    lines.push("");
  }

  // Database service
  if (hasDb) {
    lines.push(`${name}-db:`);
    lines.push(`  ${dbEngine} ${dbVersion}, ${dbStorage}`);
    lines.push("");
  }

  // Cache service
  if (hasCache) {
    lines.push("cache:");
    lines.push("  redis 7");
    lines.push("");
  }

  return lines.join("\n").trimEnd();
}
