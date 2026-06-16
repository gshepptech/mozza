import type { WizardState, ServiceInterviewState, DependencyConfig } from "./interview-types";

// ─── Public API ─────────────────────────────────────────────

/**
 * Generate a valid .mozza recipe from the wizard state.
 * Produces the same DSL format as the existing GuidedWizard.
 */
export function generateOrderRecipe(state: WizardState): string {
  const firstService = state.services[0];
  const appName = firstService?.aliasName || "myapp";
  const lines: string[] = [`App: ${appName}`, ""];

  // Collect all unique images for the Images section
  const images = new Map<string, string>();
  for (const svc of state.services) {
    if (svc.aliasName && svc.aliasImage) {
      images.set(svc.aliasName, svc.aliasImage);
    }
  }

  if (images.size > 0) {
    lines.push("Images:");
    for (const [alias, image] of images) {
      lines.push(`  ${alias}: ${image}`);
    }
    lines.push("");
  }

  // Generate each service slice
  for (const svc of state.services) {
    generateServiceSlice(svc, state, lines);
  }

  // Generate dependency slices (DB, cache)
  generateDependencySlices(state, lines);

  return lines.join("\n").trimEnd();
}

// ─── Service slice generation ───────────────────────────────

function generateServiceSlice(
  svc: ServiceInterviewState,
  state: WizardState,
  lines: string[],
): void {
  const name = svc.aliasName || "service";
  lines.push(`${name}:`);

  // Image
  if (svc.aliasImage) {
    lines.push(`  from image ${svc.aliasName || svc.aliasImage}`);
  }

  // Replicas
  if (svc.replicas > 1) {
    lines.push(`  run ${svc.replicas} copies`);
  }

  // Schedule (CronJob)
  if (svc.workloadType === "cronjob" && svc.schedule) {
    lines.push(`  run every "${svc.schedule}"`);
  }

  // Port and public access
  if (svc.port > 0) {
    if (svc.isPublic && state.target === "kitchen") {
      lines.push(`  open to the public on port ${svc.port}`);
    } else {
      lines.push(`  on port ${svc.port}`);
    }
  }

  // Domain
  if (svc.domain && svc.isPublic && state.target === "kitchen") {
    lines.push(`  domain ${svc.domain}`);
  }

  // Dependencies (needs)
  const depNames = getDependencyNames(svc);
  if (depNames.length > 0) {
    lines.push(`  needs ${depNames.join(" and ")}`);
  }

  // Environment variables
  for (const env of svc.envVars) {
    if (env.key && env.value) {
      lines.push(`  set ${env.key} to "${env.value}"`);
    }
  }

  // Health check (K8s only)
  if (state.target === "kitchen" && svc.healthCheck.enabled) {
    if (svc.healthCheck.type === "http" && svc.healthCheck.path) {
      lines.push(`  health check ${svc.healthCheck.path}`);
    }
  }

  // Resource limits (parser accepts: "limit cpu to VALUE" / "limit memory to VALUE")
  if (svc.resources.cpuLimit) {
    lines.push(`  limit cpu to ${svc.resources.cpuLimit}`);
  }
  if (svc.resources.memoryLimit) {
    lines.push(`  limit memory to ${svc.resources.memoryLimit}`);
  }

  // Storage (for stateful workloads: "each copy needs its own storage of SIZE")
  if (svc.workloadType === "statefulset") {
    lines.push("  each copy needs its own storage of 10Gi");
  }

  lines.push("");
}

// ─── Dependency slice generation ────────────────────────────

function generateDependencySlices(state: WizardState, lines: string[]): void {
  const addedDeps = new Set<string>();

  for (const svc of state.services) {
    for (const dep of svc.dependencies) {
      if (!dep.enabled) continue;
      const depKey = `${dep.type}-${dep.engine}`;
      if (addedDeps.has(depKey)) continue;
      addedDeps.add(depKey);

      if (dep.type === "database") {
        generateDbSlice(dep, svc.aliasName, lines);
      } else if (dep.type === "cache") {
        generateCacheSlice(lines);
      }
    }
  }
}

function generateDbSlice(dep: DependencyConfig, appName: string, lines: string[]): void {
  const sliceName = `${appName || "app"}-db`;
  lines.push(`${sliceName}:`);

  // Use parser's database shorthand: "engine version, storage[, daily backups]"
  const parts = [dep.engine || "postgres"];
  if (dep.version) parts[0] += ` ${dep.version}`;
  if (dep.storage) parts.push(dep.storage);
  lines.push(`  ${parts.join(", ")}`);

  lines.push("");
}

function generateCacheSlice(lines: string[]): void {
  lines.push("cache:");
  lines.push("  redis 7");
  lines.push("");
}

// ─── Helpers ────────────────────────────────────────────────

function getDependencyNames(svc: ServiceInterviewState): string[] {
  const names: string[] = [];
  for (const dep of svc.dependencies) {
    if (!dep.enabled) continue;
    if (dep.type === "database") {
      names.push(`${svc.aliasName || "app"}-db`);
    } else if (dep.type === "cache") {
      names.push("cache");
    }
  }
  return names;
}
