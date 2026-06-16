# Full Workload Taxonomy — Comprehensive Spec

> **Status:** Draft
> **Date:** 2026-03-16
> **Author:** gshepptech
> **Scope:** Recipe language, AST, plan builder, K8s compiler, local compiler, wizard UI, recipe builder, CLI, API

---

## 1. Problem Statement

Mozza's recipe language currently supports exactly 4 workload kinds: `web`, `worker`, `database`, `cache`. This covers the "web app + database" pattern but fails for real production microservice architectures.

### What breaks today

Take HobbyFarm/Gargantua — a real-world K8s application with ~15 microservices:

| Component | What it is | Why Mozza can't express it |
|---|---|---|
| `gargantua` | API server (gRPC + HTTP dual-port) | No multi-port support |
| `ui` | Frontend SPA | Works (web kind) |
| `shell-service` | WebSocket proxy to user shells | No WebSocket/TCP protocol concept |
| `scenario-service` | Watches K8s CRDs, serves gRPC | No operator/controller concept |
| `environment-service` | Manages K8s namespaces via API | No RBAC/ServiceAccount concept |
| `session-service` | gRPC + HTTP, needs headless service | No headless service, no multi-port |
| `terraform-controller` | Runs Terraform on events | No job/task runner concept |
| `progress-service` | StatefulSet with persistent identity | No StatefulSet concept |
| CRD installation | Must happen before controllers start | No init/setup concept |
| RBAC setup | ServiceAccounts, Roles, Bindings | No RBAC concept |
| ConfigMaps | Shared config mounted as files | Only env vars, no file mounts |

A non-engineer trying to deploy Gargantua through Mozza's wizard would hit a wall at step 1.

### What we want

A single `.mozza` file that can express Gargantua's full architecture in plain English, and a wizard that can guide someone through configuring it — without ever saying "StatefulSet", "DaemonSet", or "ClusterRoleBinding."

---

## 2. Design Principles

1. **Plain English surface, full K8s power underneath.** The recipe never uses K8s jargon. The compiler maps plain-English directives to the right K8s resources.
2. **Progressive disclosure.** Simple apps stay simple. A basic web app + database recipe looks identical to today. Advanced features are opt-in directives.
3. **Compile to both targets.** Every directive must have a Docker Compose equivalent OR gracefully degrade with a clear warning. K8s-only concepts (RBAC, CRDs) emit warnings when targeting local.
4. **The wizard exposes what the recipe supports.** No gap between what you can write in a `.mozza` file and what the wizard can configure.
5. **Kind inference stays smart.** Users shouldn't have to specify `kind: statefulset`. If they say "keep data between restarts" + "each copy needs its own identity", Mozza infers StatefulSet.

---

## 3. Workload Taxonomy

### 3.1 Slice Kinds (expanded from 4 → 10)

| Kind | What it means | K8s resource | Compose equivalent | Inference rules |
|---|---|---|---|---|
| `web` | Serves HTTP/HTTPS traffic, externally accessible | Deployment + Service + Ingress | service with ports | Has port + public, or explicit |
| `api` | Internal HTTP/gRPC service, not public-facing | Deployment + Service | service with ports | Has port, not public, name contains "api" |
| `worker` | Long-running background process | Deployment | service (no ports) | Image + no port, or name contains worker/job/processor |
| `task` | Run-to-completion batch job | Job | service with `restart: "no"` | Explicit "run once" or "run to completion" |
| `scheduled` | Runs on a cron schedule | CronJob | N/A (warn: use external cron) | Has "every" or "at" schedule directive |
| `database` | Stateful data store | StatefulSet + Service + PVC | service with volume | Engine shorthand (postgres/mysql/mongo) |
| `cache` | In-memory data store | Deployment + Service (or StatefulSet if persistent) | service | Engine shorthand (redis/memcached) |
| `stateful` | Ordered, sticky-identity workload | StatefulSet + Headless Service + PVCs | service with volume | "each copy needs its own storage" or "ordered startup" |
| `gateway` | Reverse proxy / API gateway / ingress controller | Deployment + Service (LoadBalancer) | service with ports | Name contains gateway/proxy/ingress, or explicit |
| `daemon` | One copy per node (log collectors, monitoring agents) | DaemonSet | service with `deploy.mode: global` | "run on every node" or explicit |

### 3.2 Kind inference rules (priority order)

The plan builder infers kind using this priority list. First match wins.

```
Rule  1: Explicit kind directive                     → use it
Rule  2: Engine = postgres/mysql/mongo               → database
Rule  3: Engine = redis/memcached                    → cache
Rule  4: Has schedule directive ("every", "at")      → scheduled
Rule  5: Has "run once" / "run to completion"        → task
Rule  6: Has "run on every node"                     → daemon
Rule  7: Has "each copy needs its own storage"       → stateful
Rule  8: Has "open to the public" + port             → web
Rule  9: Has port + name contains api/gateway/proxy  → api (or gateway)
Rule 10: Has port                                    → api
Rule 11: Name contains worker/job/processor/cron     → worker
Rule 12: Image + no port                             → worker
```

---

## 4. Recipe Language Expansion

### 4.1 Current directives (preserved, no breaking changes)

```
App: <name>                              # Application name
Namespace: <name>                        # Deployment namespace
Images:                                  # Image alias section
  <alias>: <image:tag>

<SliceName>:                             # Slice declaration
  from image <image:tag>                 # Container image
  from <alias>                           # Image from alias
  open to the public on port <N>         # Public + port
  on port <N>                            # Internal port
  health check <path>                    # HTTP health check path
  run <N> copies                         # Replica count
  needs <slice> [and <slice>...]         # Dependencies
  set <KEY> to "<value>"                 # Environment variable
  limit cpu to "<value>"                 # CPU resource limit
  limit memory to "<value>"              # Memory resource limit
  restart <policy>                       # Restart policy
  domain "<domain>"                      # Custom domain
  secret <KEY> from <secret> [key <k>]   # K8s secret reference
  pull secret <name>                     # Image pull secret
  <engine> <version>[, <storage>[, <backup>]] # Engine shorthand
```

### 4.2 New directives

#### 4.2.1 Multi-port support

```
# Multiple named ports on the same slice
on port 8080 as http
on port 9090 as grpc
on port 3000 as metrics

# Protocol specification (default: http)
on port 8080 as http using tcp
on port 9090 as grpc using http2
on port 6379 as data using tcp
```

**AST change:** `Port int` → `Ports []PortSpec`
```go
type PortSpec struct {
    Name     string // "http", "grpc", "metrics", "" for unnamed
    Port     int
    Protocol string // "tcp", "http", "http2", "udp" — default "tcp"
}
```

**Backward compatibility:** `on port 8080` still works, creates a single unnamed port. `open to the public on port 8080` creates a single unnamed public port.

#### 4.2.2 Schedule directive (CronJob)

```
# Cron-style schedule in plain English
run every day at 3am
run every hour
run every 15 minutes
run every monday at 6am
run on the 1st of every month

# Or explicit cron expression for power users
schedule "0 3 * * *"
```

**AST change:** Add `Schedule string` to Slice. The parser converts plain English to cron expressions.

**Plain English → cron mapping:**

| Plain English | Cron |
|---|---|
| `every day at 3am` | `0 3 * * *` |
| `every hour` | `0 * * * *` |
| `every 15 minutes` | `*/15 * * * *` |
| `every monday at 6am` | `0 6 * * 1` |
| `on the 1st of every month` | `0 0 1 * *` |

**K8s:** CronJob resource.
**Compose:** Warning — "Scheduled tasks require an external scheduler. This service will run once on `docker compose up`." Consider generating a `crontab` sidecar or documenting the limitation.

#### 4.2.3 Run-once / task directive (Job)

```
# Run to completion, then stop
run once
run to completion

# With parallelism
run once with 3 parallel
run once, retry up to 5 times
```

**AST change:** Add `RunOnce bool`, `Parallelism int`, `Retries int` to Slice.

**K8s:** Job resource with `backoffLimit` and `parallelism`.
**Compose:** Service with `restart: "no"`. After `docker compose up`, the container runs and exits.

#### 4.2.4 Daemon directive (DaemonSet)

```
# Run one copy on every node
run on every node

# With node selection
run on every node labeled "monitoring=true"
run on every node except control-plane
```

**AST change:** Add `DaemonMode bool`, `NodeSelector map[string]string`, `Tolerations []string` to Slice.

**K8s:** DaemonSet resource with `nodeSelector` and `tolerations`.
**Compose:** `deploy.mode: global` (Swarm mode). For plain Compose: warning — "DaemonSet behavior requires Docker Swarm. Running as a single instance."

#### 4.2.5 Stateful identity directive (StatefulSet)

```
# Each copy gets its own persistent storage
each copy needs its own storage of 10Gi

# Ordered startup (wait for copy N before starting N+1)
start copies in order

# Headless service (for peer discovery)
allow copies to find each other
```

**AST change:** Add `StatefulStorage string`, `OrderedStartup bool`, `PeerDiscovery bool` to Slice.

**K8s:** StatefulSet + headless Service + volumeClaimTemplates.
**Compose:** Named volumes per service instance (limited — can't truly replicate StatefulSet identity in Compose). Warning about limitations.

#### 4.2.6 Init steps (init containers)

```
# Run setup steps before the main service starts
before starting:
  run image flyway/flyway:latest with "migrate"
  run image busybox with "chmod 700 /data"
```

**AST change:** Add `InitSteps []InitStep` to Slice.
```go
type InitStep struct {
    Image   string
    Command string
    Env     map[string]string
}
```

**K8s:** `initContainers` in the Pod spec.
**Compose:** `depends_on` with a separate init service and `condition: service_completed_successfully`.

#### 4.2.7 Sidecar containers

```
# Run alongside the main container
with sidecar envoy from envoyproxy/envoy:v1.28 on port 15001
with sidecar log-shipper from fluent/fluent-bit:latest
```

**AST change:** Add `Sidecars []Sidecar` to Slice.
```go
type Sidecar struct {
    Name  string
    Image string
    Port  int
    Env   map[string]string
}
```

**K8s:** Additional containers in the Pod spec (native sidecar with `restartPolicy: Always` in K8s 1.28+).
**Compose:** Additional service with `network_mode: "service:<main>"` and shared volumes.

#### 4.2.8 File mounts (ConfigMaps, Secrets as files)

```
# Mount a config file
mount file "nginx.conf" at /etc/nginx/nginx.conf
mount file "app-config.yaml" at /config/app.yaml

# Mount a secret as a file
mount secret "tls-cert" at /certs/tls.crt
mount secret "tls-key" at /certs/tls.key

# Mount a directory
mount config "templates/" at /templates/
```

**AST change:** Add `Mounts []MountSpec` to Slice.
```go
type MountSpec struct {
    Type     string // "file", "secret", "config-dir"
    Source   string // file path, secret name, or directory
    Target   string // mount path in container
    ReadOnly bool
}
```

**K8s:** ConfigMap volumes, Secret volumes, projected volumes.
**Compose:** `volumes` with bind mounts. Secrets via `secrets` top-level key.

#### 4.2.9 Service accounts and permissions

```
# Request specific cluster permissions
needs permission to read pods
needs permission to read and write deployments
needs permission to read secrets in namespace "kube-system"
needs cluster-wide permission to manage custom-resources

# Use a specific service account
use account "gargantua-admin"
```

**AST change:** Add `Permissions []Permission`, `ServiceAccount string` to Slice.
```go
type Permission struct {
    Verbs      []string // "read", "write", "manage" (= all verbs)
    Resources  []string // "pods", "deployments", "custom-resources", etc.
    Namespace  string   // "" = same namespace, specific = cross-namespace
    ClusterWide bool    // true = ClusterRole, false = Role
}
```

**Plain English → RBAC mapping:**

| Plain English | K8s verbs |
|---|---|
| `read` | `get`, `list`, `watch` |
| `write` | `create`, `update`, `patch` |
| `delete` | `delete` |
| `manage` | `get`, `list`, `watch`, `create`, `update`, `patch`, `delete` |

**K8s:** ServiceAccount + Role/ClusterRole + RoleBinding/ClusterRoleBinding.
**Compose:** Warning — "Permissions are a Kubernetes concept. Skipped for local Docker Compose target."

#### 4.2.10 Probes (expanded)

```
# Current (preserved)
health check /healthz

# New: explicit probe types
readiness check /ready
liveness check /healthz
startup check /startup

# Non-HTTP probes
readiness check by running "pg_isready"
liveness check on tcp port 6379

# Timing configuration
health check /healthz every 10s, timeout 5s, wait 30s before starting
```

**AST change:** Replace `Health string` with `Probes []ProbeSpec`.
```go
type ProbeSpec struct {
    Type     string // "readiness", "liveness", "startup"
    HTTPPath string // "/healthz" (HTTP probe)
    Command  string // "pg_isready" (exec probe)
    TCPPort  int    // 6379 (TCP probe)
    Interval int    // seconds between checks
    Timeout  int    // seconds before timeout
    Delay    int    // initial delay seconds
}
```

**Backward compatibility:** `health check /healthz` creates both a readiness and liveness probe with the same path (current behavior).

**K8s:** `readinessProbe`, `livenessProbe`, `startupProbe` in container spec.
**Compose:** `healthcheck` (only supports one health check, maps to liveness).

#### 4.2.11 Lifecycle hooks

```
# Graceful shutdown
before stopping, wait 30 seconds
before stopping, run "nginx -s quit"

# Startup hook
after starting, run "warmup.sh"
```

**AST change:** Add `Lifecycle *LifecycleSpec` to Slice.
```go
type LifecycleSpec struct {
    PreStopCommand string
    PreStopWait    int // terminationGracePeriodSeconds
    PostStartCommand string
}
```

**K8s:** `lifecycle.preStop`, `lifecycle.postStart`, `terminationGracePeriodSeconds`.
**Compose:** `stop_grace_period`, `stop_signal`. No postStart equivalent.

#### 4.2.12 Scheduling constraints

```
# Node selection
prefer nodes labeled "tier=compute"
require nodes labeled "gpu=true"
avoid nodes labeled "spot=true"

# Pod spreading
spread copies across nodes
spread copies across zones
never run two copies on the same node
```

**AST change:** Add `Scheduling *SchedulingSpec` to Slice.
```go
type SchedulingSpec struct {
    NodePreferences []LabelConstraint // soft: preferredDuringSchedulingIgnoredDuringExecution
    NodeRequirements []LabelConstraint // hard: requiredDuringSchedulingIgnoredDuringExecution
    NodeAvoidances  []LabelConstraint // anti-affinity
    SpreadTopology  string            // "nodes", "zones"
    AntiAffinity    bool              // never co-locate
}

type LabelConstraint struct {
    Key   string
    Value string
}
```

**K8s:** `nodeSelector`, `nodeAffinity`, `podAntiAffinity`, `topologySpreadConstraints`.
**Compose:** `deploy.placement.constraints` (Swarm). Warning for plain Compose.

#### 4.2.13 Explicit kind override

```
# For cases where inference isn't enough
kind: gateway
kind: daemon
kind: stateful
```

**AST change:** The `Kind` field already exists. Expand `validKinds` map to include all 10 kinds.

#### 4.2.14 Networking enhancements

```
# Internal DNS name override
reachable as "auth-service"

# Network policy: restrict who can talk to this slice
only accept traffic from api and frontend
block all traffic except from namespace "monitoring"
```

**AST change:** Add `DNSName string`, `NetworkPolicy *NetworkPolicySpec` to Slice.
```go
type NetworkPolicySpec struct {
    AllowFrom      []string // slice names
    AllowNamespace []string // namespace names
    DenyAll        bool
}
```

**K8s:** NetworkPolicy resource.
**Compose:** Network aliases. Network policies not supported (warning).

#### 4.2.15 Auto-wiring (dependency injection)

```
# When slice A needs slice B, auto-inject connection info
Api:
  from image myapp/api:latest
  needs db
  # Mozza auto-injects: DATABASE_URL=postgres://db:5432/app (if db is postgres)
```

**Behavior:** When a slice `needs` a database/cache slice, Mozza auto-generates the standard connection environment variable. The user can override with explicit `set` directives.

**Auto-wire rules:**

| Dependency kind | Injected env var | Value pattern |
|---|---|---|
| postgres | `DATABASE_URL` | `postgres://<slice>:5432/<app-name>` |
| mysql | `DATABASE_URL` | `mysql://<slice>:3306/<app-name>` |
| mongo | `MONGO_URL` | `mongodb://<slice>:27017/<app-name>` |
| redis | `REDIS_URL` | `redis://<slice>:6379` |
| memcached | `MEMCACHED_URL` | `<slice>:11211` |
| any service with port | `<SLICE_NAME>_URL` | `http://<slice>:<port>` |

#### 4.2.16 Auto-scaling (HPA)

```
# Scale based on CPU usage
scale between 2 and 10 copies based on cpu 80%

# Scale based on memory
scale between 1 and 5 copies based on memory 70%

# Scale based on custom metric
scale between 2 and 20 copies based on requests-per-second 1000
```

**AST change:** Add `AutoScale *AutoScaleSpec` to Slice.
```go
type AutoScaleSpec struct {
    MinReplicas    int
    MaxReplicas    int
    CPUTarget      int    // percentage (e.g., 80)
    MemoryTarget   int    // percentage (e.g., 70)
    CustomMetric   string // metric name
    CustomTarget   int    // target value
}
```

**K8s:** HorizontalPodAutoscaler resource.
**Compose:** Warning — "Auto-scaling is Kubernetes-only. Running with the minimum replica count."

**Interaction with `run N copies`:** If both are set, `Replicas` becomes the initial count. HPA overrides it dynamically. If only `scale between` is set, `MinReplicas` is the initial count.

#### 4.2.17 Disruption budget

```
# Ensure availability during updates and maintenance
keep at least 2 copies running during updates
allow at most 1 copy down during updates
```

**AST change:** Add `DisruptionBudget *DisruptionBudgetSpec` to Slice.
```go
type DisruptionBudgetSpec struct {
    MinAvailable   int // "keep at least N"
    MaxUnavailable int // "allow at most N down"
}
```

**K8s:** PodDisruptionBudget resource.
**Compose:** Warning — "Disruption budgets are Kubernetes-only. Skipped for local target."

#### 4.2.18 Security context

```
# Run as a specific user (non-root)
run as user 1000
run as group 1000

# Drop all capabilities and add specific ones
drop all capabilities
add capability NET_BIND_SERVICE

# Read-only root filesystem
read-only filesystem
```

**AST change:** Add `Security *SecuritySpec` to Slice.
```go
type SecuritySpec struct {
    RunAsUser       int
    RunAsGroup      int
    ReadOnlyRoot    bool
    DropCapabilities []string // ["ALL"]
    AddCapabilities  []string // ["NET_BIND_SERVICE"]
}
```

**K8s:** `securityContext` on both Pod and Container level.
**Compose:** `user:`, `read_only:`, `cap_drop:`, `cap_add:` in service config.

#### 4.2.19 Rolling update strategy

```
# Control how updates roll out
update one at a time
update 25% at a time, keep 75% running
update with at most 1 extra copy
```

**AST change:** Add `UpdateStrategy *UpdateStrategySpec` to Slice.
```go
type UpdateStrategySpec struct {
    MaxSurge       string // "1" or "25%"
    MaxUnavailable string // "0" or "25%"
}
```

**K8s:** `strategy.rollingUpdate` on Deployment, `updateStrategy` on StatefulSet/DaemonSet.
**Compose:** `deploy.update_config` (Swarm only). Warning for standalone Compose.

#### 4.2.20 Graceful shutdown

```
# Allow time for in-flight requests to complete
graceful shutdown 60s
graceful shutdown 5m
```

**AST change:** Add `GracefulShutdown int` (seconds) to Slice. Works alongside lifecycle hooks from 4.2.11.

**K8s:** `terminationGracePeriodSeconds` on Pod spec.
**Compose:** `stop_grace_period` on service.

---

## 5. AST Changes

### 5.1 Recipe AST (internal/recipe/ast.go)

```go
type Recipe struct {
    Name      string
    Namespace string
    Aliases   map[string]string
    Slices    []Slice
}

type Slice struct {
    Name string
    Line int // source line for errors

    // --- Identity ---
    Kind   string   // explicit kind override (optional)
    Image  string
    Engine string   // postgres, redis, etc.
    Version string

    // --- Scaling ---
    Replicas      int
    DaemonMode    bool    // run on every node
    RunOnce       bool    // run to completion (Job)
    Parallelism   int     // Job parallelism
    Retries       int     // Job backoffLimit
    Schedule      string  // cron expression (CronJob)
    OrderedStartup bool   // StatefulSet ordered pod management

    // --- Networking ---
    Ports        []PortSpec
    Public       bool
    Domain       string
    DNSName      string
    PeerDiscovery bool   // headless service for StatefulSet

    // --- Storage ---
    Storage        string  // PVC size for database/cache
    StatefulStorage string // per-replica PVC size for StatefulSet
    Backups        string
    Mounts         []MountSpec

    // --- Configuration ---
    Env            map[string]string
    Secrets        []SecretRef
    PullSecret     string

    // --- Resources ---
    CPULimit       string
    MemoryLimit    string
    RestartPolicy  string

    // --- Lifecycle ---
    Probes         []ProbeSpec
    InitSteps      []InitStep
    Sidecars       []Sidecar
    Lifecycle      *LifecycleSpec

    // --- Dependencies ---
    Needs          []string

    // --- Permissions ---
    Permissions    []Permission
    ServiceAccount string

    // --- Scheduling ---
    Scheduling     *SchedulingSpec

    // --- Network Policy ---
    NetworkPolicy  *NetworkPolicySpec

    // --- Node Selection ---
    NodeSelector   map[string]string
    Tolerations    []string

    // --- Auto-scaling ---
    AutoScale      *AutoScaleSpec

    // --- Disruption Budget ---
    DisruptionBudget *DisruptionBudgetSpec

    // --- Security ---
    Security       *SecuritySpec

    // --- Update Strategy ---
    UpdateStrategy *UpdateStrategySpec

    // --- Graceful Shutdown ---
    GracefulShutdown int // seconds
}

type AutoScaleSpec struct {
    MinReplicas    int
    MaxReplicas    int
    CPUTarget      int    // percentage
    MemoryTarget   int    // percentage
    CustomMetric   string
    CustomTarget   int
}

type DisruptionBudgetSpec struct {
    MinAvailable   int
    MaxUnavailable int
}

type SecuritySpec struct {
    RunAsUser        int
    RunAsGroup       int
    ReadOnlyRoot     bool
    DropCapabilities []string
    AddCapabilities  []string
}

type UpdateStrategySpec struct {
    MaxSurge       string
    MaxUnavailable string
}

type PortSpec struct {
    Name     string
    Port     int
    Protocol string // "tcp", "http2", "udp"
}

type ProbeSpec struct {
    Type     string // "readiness", "liveness", "startup"
    HTTPPath string
    Command  string
    TCPPort  int
    Interval int
    Timeout  int
    Delay    int
}

type InitStep struct {
    Image   string
    Command string
    Env     map[string]string
}

type Sidecar struct {
    Name  string
    Image string
    Ports []PortSpec
    Env   map[string]string
}

type MountSpec struct {
    Type     string // "file", "secret", "config-dir"
    Source   string
    Target   string
    ReadOnly bool
}

type LifecycleSpec struct {
    PreStopCommand   string
    PreStopWait      int
    PostStartCommand string
}

type Permission struct {
    Verbs       []string
    Resources   []string
    Namespace   string
    ClusterWide bool
}

type SchedulingSpec struct {
    NodePreferences  []LabelConstraint
    NodeRequirements []LabelConstraint
    SpreadTopology   string
    AntiAffinity     bool
}

type LabelConstraint struct {
    Key   string
    Value string
}

type NetworkPolicySpec struct {
    AllowFrom      []string
    AllowNamespace []string
    DenyAll        bool
}

type SecretRef struct {
    EnvVar     string
    SecretName string
    Key        string
}
```

### 5.2 Backward compatibility

The old single-field forms are syntactic sugar:

| Old syntax | Maps to |
|---|---|
| `Port int` | `Ports: [{Port: N}]` |
| `Health string` | `Probes: [{Type: "readiness", HTTPPath: path}, {Type: "liveness", HTTPPath: path}]` |
| `Storage string` on database kind | `Database.Storage` (unchanged) |
| `Public bool` | `Public` flag + first port gets Ingress |

The plan builder handles this mapping so existing recipes produce identical plans.

---

## 6. Plan Builder Changes (internal/plan/)

### 6.1 Plan SliceKind expansion

```go
const (
    SliceKindWeb       SliceKind = "web"
    SliceKindAPI       SliceKind = "api"
    SliceKindWorker    SliceKind = "worker"
    SliceKindTask      SliceKind = "task"      // Job
    SliceKindScheduled SliceKind = "scheduled"  // CronJob
    SliceKindDatabase  SliceKind = "database"   // StatefulSet
    SliceKindCache     SliceKind = "cache"
    SliceKindStateful  SliceKind = "stateful"   // StatefulSet (non-database)
    SliceKindGateway   SliceKind = "gateway"
    SliceKindDaemon    SliceKind = "daemon"     // DaemonSet
)
```

### 6.2 Plan Slice expansion

The `plan.Slice` struct mirrors the recipe AST additions. Every new recipe field has a corresponding plan field. The `build.go` conversion logic maps recipe → plan with the extended inference rules from Section 3.2.

### 6.3 Auto-wiring in plan builder

When building ingredients, if slice A `needs` slice B and B is a database/cache, the plan builder auto-injects connection env vars into A's `Env` map (unless already set). See Section 4.2.15 for the auto-wire rules.

### 6.4 Validation expansion

New validation rules:

| Rule | Error |
|---|---|
| `scheduled` kind must have `Schedule` | "scheduled slice %q requires a schedule (e.g., 'run every day at 3am')" |
| `task` kind must have `RunOnce` | "task slice %q requires 'run once' or 'run to completion'" |
| `daemon` kind must not have `Replicas > 0` | "daemon slice %q runs on every node — replicas is not applicable" |
| `daemon` kind must not have `AutoScale` | "daemon slice %q runs on every node — auto-scaling is not applicable" |
| `stateful` kind must have `StatefulStorage` or `OrderedStartup` or `PeerDiscovery` | "stateful slice %q needs at least one stateful feature" |
| Multi-port names must be unique within a slice | "slice %q has duplicate port name %q" |
| Init step images must not be empty | "slice %q init step[%d]: image must not be empty" |
| Sidecar names must be unique within a slice | "slice %q has duplicate sidecar name %q" |
| Schedule must be valid cron expression | "slice %q: invalid schedule %q" |
| Permission resources must be known K8s resources | "slice %q: unknown permission resource %q" |
| Mount target paths must be absolute | "slice %q mount %q: target must be an absolute path" |
| NetworkPolicy AllowFrom must reference existing slices | "slice %q network policy: unknown source %q" |
| AutoScale min must be > 0 | "slice %q: auto-scale min replicas must be > 0" |
| AutoScale max must be >= min | "slice %q: auto-scale max must be >= min" |
| AutoScale target must be 1-100 | "slice %q: auto-scale CPU/memory target must be 1-100%%" |
| DisruptionBudget only on kinds with replicas | "slice %q: disruption budget requires a workload with replicas" |
| DisruptionBudget min < replicas | "slice %q: disruption budget min (%d) must be < replicas (%d)" |
| SecuritySpec RunAsUser must be >= 0 | "slice %q: run-as user must be >= 0" |
| GracefulShutdown must be > 0 | "slice %q: graceful shutdown must be > 0 seconds" |

---

## 7. K8s Compiler Changes (internal/k8s/)

### 7.1 New resource generators

| Kind | K8s resources generated |
|---|---|
| `web` | Deployment + Service (ClusterIP) + Ingress |
| `api` | Deployment + Service (ClusterIP) |
| `worker` | Deployment (no Service) |
| `task` | Job |
| `scheduled` | CronJob |
| `database` | StatefulSet + Service (ClusterIP) + PVC |
| `cache` | Deployment + Service (ClusterIP) [or StatefulSet if persistent] |
| `stateful` | StatefulSet + Headless Service + VolumeClaimTemplates |
| `gateway` | Deployment + Service (LoadBalancer or NodePort) |
| `daemon` | DaemonSet + Service (ClusterIP, optional) |

### 7.2 New files needed

```
internal/k8s/statefulset.go      — StatefulSet resource builder
internal/k8s/job.go              — Job resource builder
internal/k8s/cronjob.go          — CronJob resource builder
internal/k8s/daemonset.go        — DaemonSet resource builder
internal/k8s/configmap.go        — ConfigMap from file mounts
internal/k8s/secret_volume.go    — Secret volume mount builder
internal/k8s/serviceaccount.go   — ServiceAccount resource builder
internal/k8s/rbac.go             — Role + RoleBinding + ClusterRole + ClusterRoleBinding
internal/k8s/networkpolicy.go    — NetworkPolicy resource builder
internal/k8s/headless_service.go — Headless Service for StatefulSets
internal/k8s/hpa.go              — HorizontalPodAutoscaler resource builder
internal/k8s/pdb.go              — PodDisruptionBudget resource builder
```

### 7.2.1 HPA generation

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: <slice-name>
  labels:
    app.kubernetes.io/name: <slice-name>
    app.kubernetes.io/part-of: <app-name>
    app.kubernetes.io/managed-by: mozza
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment  # or StatefulSet
    name: <slice-name>
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
```

### 7.2.2 PodDisruptionBudget generation

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: <slice-name>
spec:
  minAvailable: 2   # or maxUnavailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: <slice-name>
```

### 7.2.3 SecurityContext generation

Applied at both Pod and Container level:

```yaml
spec:
  securityContext:          # Pod level
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
  containers:
    - name: main
      securityContext:      # Container level
        readOnlyRootFilesystem: true
        allowPrivilegeEscalation: false
        capabilities:
          drop: ["ALL"]
          add: ["NET_BIND_SERVICE"]
```

### 7.3 Multi-port Service generation

Current: single port per Service.
New: Service with multiple named ports.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: session-service
spec:
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: grpc
      port: 9090
      targetPort: 9090
    - name: metrics
      port: 3000
      targetPort: 3000
```

### 7.4 Init containers

```yaml
spec:
  initContainers:
    - name: migrate
      image: flyway/flyway:latest
      command: ["migrate"]
    - name: permissions
      image: busybox
      command: ["sh", "-c", "chmod 700 /data"]
  containers:
    - name: main
      ...
```

### 7.5 Sidecar containers

```yaml
spec:
  containers:
    - name: main
      image: myapp/api:latest
    - name: envoy
      image: envoyproxy/envoy:v1.28
      ports:
        - containerPort: 15001
      restartPolicy: Always  # K8s 1.28+ native sidecar
```

### 7.6 RBAC generation

For a slice with `needs permission to read pods`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: <slice-name>
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: <slice-name>
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: <slice-name>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: <slice-name>
subjects:
  - kind: ServiceAccount
    name: <slice-name>
```

### 7.7 StatefulSet with volumeClaimTemplates

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: <slice-name>
spec:
  serviceName: <slice-name>  # headless service
  podManagementPolicy: OrderedReady  # if orderedStartup
  replicas: 3
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### 7.8 NetworkPolicy generation

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: <slice-name>
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: <slice-name>
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: <allowed-slice>
  policyTypes:
    - Ingress
```

---

## 8. Local Compiler Changes (internal/local/)

### 8.1 Docker Compose mapping

| Kind | Compose output |
|---|---|
| `web` | service with `ports: ["<host>:<container>"]` |
| `api` | service with `expose: [<port>]` |
| `worker` | service (no ports) |
| `task` | service with `restart: "no"` |
| `scheduled` | **Warning**: "CronJob not supported in Docker Compose. Service will start once. Use `crontab` externally." Output service with `restart: "no"` + comment with schedule |
| `database` | service with volume + engine image |
| `cache` | service with optional volume |
| `stateful` | service with named volumes. **Warning**: "StatefulSet identity not replicable in Compose. Running as regular service." |
| `gateway` | service with `ports` (host-mapped) |
| `daemon` | service with `deploy.mode: global` if Swarm, otherwise **warning** |

### 8.2 Init containers → depends_on

```yaml
services:
  api-migrate:
    image: flyway/flyway:latest
    command: migrate
    restart: "no"
    depends_on:
      db:
        condition: service_healthy

  api:
    image: myapp/api:latest
    depends_on:
      api-migrate:
        condition: service_completed_successfully
      db:
        condition: service_healthy
```

### 8.3 Sidecar containers → separate services

```yaml
services:
  api:
    image: myapp/api:latest
    networks:
      - default

  api-envoy:
    image: envoyproxy/envoy:v1.28
    network_mode: "service:api"
    depends_on:
      - api
```

### 8.4 File mounts → bind mounts

```yaml
services:
  api:
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./templates/:/templates/:ro
```

### 8.5 K8s-only features → warnings

The local compiler emits warnings (not errors) for features that have no Compose equivalent:

- RBAC/Permissions → "Permissions are Kubernetes-only. Skipped for local target."
- NetworkPolicy → "Network policies are Kubernetes-only. All services can communicate locally."
- Node scheduling → "Node scheduling is Kubernetes-only. Skipped for local target."
- DaemonSet mode → "DaemonSet mode requires Docker Swarm. Running as single instance."
- Headless services → "Headless services are Kubernetes-only. Using standard DNS resolution."
- HPA auto-scaling → "Auto-scaling is Kubernetes-only. Running with minimum replica count."
- PodDisruptionBudget → "Disruption budgets are Kubernetes-only. Skipped for local target."
- SecurityContext (partial) → `user:`, `read_only:`, `cap_drop:`, `cap_add:` map directly. `runAsGroup`/`fsGroup` have no Compose equivalent.
- Rolling update strategy → `deploy.update_config` (Swarm only). Warning for standalone Compose.
- Graceful shutdown → `stop_grace_period` maps directly.

Warnings are returned in `compile.Result.Warnings []string`.

---

## 9. Parser Changes (internal/recipe/)

### 9.1 New tokens

```go
// Multi-port
TokenAs       // "as"
TokenUsing    // "using"

// Schedule
TokenEvery    // "every"
TokenAt       // "at"
TokenSchedule // "schedule"

// Run modifiers
TokenOnce        // "once"
TokenCompletion  // "completion"
TokenParallel    // "parallel"
TokenRetry       // "retry"

// Daemon
TokenNode    // "node"
TokenLabeled // "labeled"
TokenExcept  // "except"

// Stateful
TokenEach    // "each"
TokenOwn     // "own"
TokenOrdered // "ordered" / "in order"
TokenFind    // "find"

// Init
TokenBefore  // "before"
TokenStarting // "starting"

// Sidecar
TokenWith    // "with" (already exists for "with sidecar")
TokenSidecar // "sidecar"

// Mounts
TokenMount   // "mount"
TokenFile    // "file"
TokenConfig  // "config"

// Permissions
TokenPermission // "permission"
TokenRead    // "read"
TokenWrite   // "write"  (context-dependent: also used in file mounts)
TokenManage  // "manage"
TokenAccount // "account"

// Probes
TokenReadiness // "readiness"
TokenLiveness  // "liveness"
TokenStartup   // "startup"
TokenRunning   // "running"

// Lifecycle
TokenStopping // "stopping"
TokenWait     // "wait"
TokenSeconds  // "seconds"

// Scheduling
TokenPrefer   // "prefer"
TokenRequire  // "require"
TokenAvoid    // "avoid"
TokenSpread   // "spread"
TokenAcross   // "across"
TokenZones    // "zones"
TokenNever    // "never"

// Network
TokenAccept   // "accept"
TokenTraffic  // "traffic"
TokenBlock    // "block"
TokenReachable // "reachable"

// Kind override
TokenKind     // "kind:"

// Auto-scaling
TokenScale    // "scale"
TokenBetween  // "between"
TokenBased    // "based"

// Disruption
TokenKeep     // "keep"
TokenDuring   // "during"
TokenUpdates  // "updates"
TokenAllow    // "allow"
TokenDown     // "down"

// Security
TokenUser     // "user" (context: "run as user")
TokenGroup    // "group"
TokenDrop     // "drop"
TokenCapability // "capability" / "capabilities"
TokenReadOnly // "read-only"
TokenFilesystem // "filesystem"

// Update strategy
TokenUpdate   // "update"

// Graceful shutdown
TokenGraceful // "graceful"
TokenShutdown // "shutdown"
```

### 9.2 Parser directive patterns

Each new directive is recognized by a regex or keyword pattern at the start of an indented line within a slice block.

```
# Multi-port
/^on port (\d+) as (\w+)(?:\s+using\s+(\w+))?$/

# Schedule
/^run every (.+)$/
/^run on the (.+)$/
/^schedule "(.+)"$/

# Run once
/^run once$/
/^run to completion$/
/^run once with (\d+) parallel$/
/^run once, retry up to (\d+) times$/

# Daemon
/^run on every node$/
/^run on every node labeled "(.+)"$/
/^run on every node except (.+)$/

# Stateful
/^each copy needs its own storage of (.+)$/
/^start copies in order$/
/^allow copies to find each other$/

# Init
/^before starting:$/  → enters init block, reads indented "run image ..." lines

# Sidecar
/^with sidecar (\w+) from (\S+)(?:\s+on port (\d+))?$/

# Mounts
/^mount file "(.+)" at (.+)$/
/^mount secret "(.+)" at (.+)$/
/^mount config "(.+)" at (.+)$/

# Permissions
/^needs permission to (read|write|manage|delete) (.+)$/
/^needs cluster-wide permission to (read|write|manage|delete) (.+)$/
/^use account "(.+)"$/

# Probes
/^(readiness|liveness|startup) check (.+)$/
/^(readiness|liveness|startup) check by running "(.+)"$/
/^(readiness|liveness|startup) check on tcp port (\d+)$/
/^health check (.+) every (\d+)s(?:, timeout (\d+)s)?(?:, wait (\d+)s before starting)?$/

# Lifecycle
/^before stopping, wait (\d+) seconds$/
/^before stopping, run "(.+)"$/
/^after starting, run "(.+)"$/

# Scheduling
/^prefer nodes labeled "(.+)"$/
/^require nodes labeled "(.+)"$/
/^avoid nodes labeled "(.+)"$/
/^spread copies across (nodes|zones)$/
/^never run two copies on the same node$/

# Network policy
/^only accept traffic from (.+)$/
/^block all traffic except from namespace "(.+)"$/

# DNS
/^reachable as "(.+)"$/

# Kind override
/^kind:\s*(.+)$/

# Auto-scaling
/^scale between (\d+) and (\d+) copies based on cpu (\d+)%$/
/^scale between (\d+) and (\d+) copies based on memory (\d+)%$/
/^scale between (\d+) and (\d+) copies based on (\S+) (\d+)$/

# Disruption budget
/^keep at least (\d+) cop(?:y|ies) running during updates$/
/^allow at most (\d+) cop(?:y|ies) down during updates$/

# Security context
/^run as user (\d+)$/
/^run as group (\d+)$/
/^drop all capabilities$/
/^add capability (\w+)$/
/^read-only filesystem$/

# Update strategy
/^update one at a time$/
/^update (\d+)% at a time(?:, keep (\d+)% running)?$/
/^update with at most (\d+) extra cop(?:y|ies)$/

# Graceful shutdown
/^graceful shutdown (\d+)(s|m)$/
```

---

## 10. Wizard UI Changes

### 10.1 Step 1: Recipe Selection (expanded block types)

Current: 4 block types (Web, Worker, Database, Cache).
New: 10 block types organized in categories.

```typescript
const BLOCK_CATEGORIES = [
  {
    label: "Services",
    types: [
      { kind: "web", label: "Web Service", icon: Globe, description: "Serves web traffic, accessible from the internet" },
      { kind: "api", label: "API / Backend", icon: Server, description: "Internal service, handles business logic" },
      { kind: "gateway", label: "Gateway / Proxy", icon: Shield, description: "Reverse proxy, API gateway, load balancer" },
    ],
  },
  {
    label: "Background Work",
    types: [
      { kind: "worker", label: "Worker", icon: Cog, description: "Long-running background process" },
      { kind: "task", label: "One-time Task", icon: PlayCircle, description: "Runs once and stops (migration, seed, etc.)" },
      { kind: "scheduled", label: "Scheduled Job", icon: Clock, description: "Runs on a schedule (daily, hourly, etc.)" },
    ],
  },
  {
    label: "Data",
    types: [
      { kind: "database", label: "Database", icon: DatabaseIcon, description: "PostgreSQL, MySQL, MongoDB, etc." },
      { kind: "cache", label: "Cache", icon: Zap, description: "Redis, Memcached, etc." },
      { kind: "stateful", label: "Stateful Service", icon: HardDrive, description: "Needs persistent identity per copy (Kafka, ZooKeeper, etc.)" },
    ],
  },
  {
    label: "Infrastructure",
    types: [
      { kind: "daemon", label: "Node Agent", icon: Cpu, description: "Runs one copy on every node (monitoring, logging)" },
    ],
  },
];
```

### 10.2 Step 2: Configure Services (expanded)

Current: image, replicas, port per service.
New: depends on the kind.

| Kind | Configurable fields |
|---|---|
| web | image, replicas, ports (multi), domain, health check |
| api | image, replicas, ports (multi), health check |
| worker | image, replicas |
| task | image, parallelism, retries |
| scheduled | image, schedule (with plain-English picker), parallelism |
| database | engine picker (postgres/mysql/mongo), version, storage size, backup policy |
| cache | engine picker (redis/memcached), version, optional storage |
| stateful | image, replicas, per-copy storage size, ordered startup toggle |
| gateway | image, replicas, ports |
| daemon | image, node selector |

#### Schedule picker (for `scheduled` kind)

```
┌─────────────────────────────────────────┐
│  When should this run?                  │
│                                         │
│  ○ Every hour                           │
│  ○ Every day at  [3] : [00]  [AM ▾]    │
│  ○ Every week on [Monday ▾] at [6 AM]  │
│  ○ Every month on the [1st ▾]          │
│  ○ Custom: [*/15 * * * *]              │
│                                         │
│  Preview: "Runs every day at 3:00 AM"   │
└─────────────────────────────────────────┘
```

#### Database engine picker

```
┌─────────────────────────────────────────┐
│  Pick your database                     │
│                                         │
│  [PostgreSQL]  [MySQL]  [MongoDB]       │
│                                         │
│  Version: [16 ▾]                        │
│  Storage: [10] Gi                       │
│  Backups: ○ None  ● Daily  ○ Hourly    │
│                                         │
│  Auto-configured:                       │
│  • Image: postgres:16-alpine            │
│  • Port: 5432                           │
│  • Mount: /var/lib/postgresql/data      │
└─────────────────────────────────────────┘
```

### 10.3 Step 3: Environment Variables (enhanced)

Current: manual key-value pairs with auto-fill suggestions.
New: add auto-wiring display.

When service A is configured to `need` service B (a database), show:

```
┌─────────────────────────────────────────┐
│  Auto-configured (from dependencies)     │
│                                         │
│  DATABASE_URL = postgres://db:5432/app  │  ← auto-wired, editable
│  REDIS_URL = redis://cache:6379         │  ← auto-wired, editable
│                                         │
│  Custom Variables                       │
│  [KEY]  [value]  [🔒]  [×]             │
│  [+ Add Variable]                       │
└─────────────────────────────────────────┘
```

### 10.4 Step 4: Networking (enhanced)

Current: public/private toggle + domain per service.
New: add dependency wiring + multi-port config.

#### Dependencies sub-step

```
┌─────────────────────────────────────────┐
│  How do your services connect?           │
│                                         │
│  api ──needs──> db                      │
│  api ──needs──> cache                   │
│  frontend ──needs──> api                │
│  worker ──needs──> db                   │
│                                         │
│  [+ Add Connection]                     │
│                                         │
│  Drag to connect, or click to add:      │
│  From: [api ▾]  To: [db ▾]  [Add]      │
└─────────────────────────────────────────┘
```

#### Multi-port config (for services with multiple ports)

```
┌─────────────────────────────────────────┐
│  session-service ports                   │
│                                         │
│  Port 8080  Name: [http  ]  Protocol: [HTTP  ▾]  │
│  Port 9090  Name: [grpc  ]  Protocol: [gRPC  ▾]  │
│  [+ Add Port]                           │
└─────────────────────────────────────────┘
```

### 10.5 New Step: Advanced (optional, collapsed by default)

For power users. Collapsed accordion sections:

```
┌─────────────────────────────────────────┐
│  ▶ Init Steps                           │
│  ▶ Sidecars                             │
│  ▶ File Mounts                          │
│  ▶ Permissions                          │
│  ▶ Health Checks                        │
│  ▶ Lifecycle Hooks                      │
│  ▶ Scheduling                           │
│  ▶ Network Policies                     │
└─────────────────────────────────────────┘
```

Each section expands to show the relevant configuration UI. This keeps the wizard clean for simple apps while making power available for complex ones.

### 10.6 Step 5: Target Selection (unchanged)

Local (Docker Compose) vs. Kubernetes with cluster/namespace picker. No changes needed.

### 10.7 Step 6: Review & Deploy (enhanced)

Current: recipe preview + deploy button.
New: add visual architecture diagram + warnings.

```
┌─────────────────────────────────────────┐
│  Architecture                           │
│                                         │
│  ┌──────┐     ┌─────┐     ┌────┐       │
│  │ web  │────→│ api │────→│ db │       │
│  └──────┘     └──┬──┘     └────┘       │
│                  │                      │
│                  └────→┌───────┐        │
│                        │ cache │        │
│                        └───────┘        │
│                                         │
│  Warnings                               │
│  ⚠ CronJob not supported in Docker     │
│    Compose. Will run once on start.     │
│                                         │
│  Recipe Preview                         │
│  ┌─────────────────────────────────┐    │
│  │ App: my-app                     │    │
│  │ ...                             │    │
│  └─────────────────────────────────┘    │
│                                         │
│  [Deploy]                               │
└─────────────────────────────────────────┘
```

---

## 11. Recipe Builder Changes (Canvas)

### 11.1 Block palette expansion

The drag-and-drop canvas block palette expands from 4 to 10 block types, matching the wizard's categories.

### 11.2 Property panel expansion

The right-side property panel for each block gains all the kind-specific fields from Section 10.2.

### 11.3 Connection types

Current: connections are unlabeled (just "needs").
New: connections show the dependency type and auto-wired env vars.

---

## 12. CLI Changes

### 12.1 `mozza validate` (enhanced)

The validate command already exists. With the expanded recipe language, it validates all new directives and provides helpful error messages for the new syntax.

### 12.2 `mozza doctor` (enhanced rules)

New doctor rules:

| Rule | What it checks |
|---|---|
| `ScheduleWithoutConcurrency` | CronJob without concurrency policy (default: allow, warn if long-running) |
| `PublicDatabaseWarning` | Database/cache marked as public (security risk) |
| `NoHealthCheck` | Web/API service without a health check |
| `SingleReplicaProduction` | Production namespace with replicas=1 on web/api |
| `NoResourceLimits` | Service without CPU/memory limits |
| `InitContainerNoTimeout` | Init step without timeout guidance |
| `DaemonWithReplicas` | DaemonSet with explicit replicas (conflict) |
| `StatefulWithoutStorage` | StatefulSet without per-copy storage |
| `PermissionsWithoutRBAC` | Permissions requested but targeting local (warning) |
| `PublicWithoutTLS` | Public web service without domain/TLS |
| `NoAutoScaleWithHighReplicas` | Static replicas >= 5 without auto-scaling (suggest HPA) |
| `NoDisruptionBudget` | Production namespace with replicas >= 3 but no disruption budget |
| `RunAsRoot` | Service without `run as user` (defaults to root — security risk) |
| `NoGracefulShutdown` | Web/API with replicas > 1 but no graceful shutdown (risks dropped connections) |
| `ReadOnlyFSMissing` | Service with no file writes but no read-only filesystem (security hardening) |
| `NoUpdateStrategy` | Production namespace with replicas > 1 but no update strategy |

---

## 13. API Changes (internal/server/)

### 13.1 Plan response expansion

The `/api/v1/plan` endpoint returns the expanded plan structure. New fields are added to the JSON response.

### 13.2 Deploy endpoint expansion

The `/api/v1/deploy` endpoint accepts the expanded wizard state. The `ServiceConfig` type gains all new fields.

### 13.3 New endpoint: `/api/v1/recipe/validate`

Already exists. Validate now checks all new directives.

---

## 14. Example: HobbyFarm/Gargantua as a .mozza file

```mozza
# HobbyFarm — Kubernetes-based interactive learning platform

App: hobbyfarm
Namespace: hobbyfarm-system

Images:
  gargantua: hobbyfarm/gargantua:v3.0.0
  ui: hobbyfarm/ui:v3.0.0
  shell: hobbyfarm/shell:v3.0.0

# ─── Public-facing ───────────────────────────

Ui:
  from ui
  open to the public on port 8080
  health check /healthz
  run 2 copies
  needs gargantua

# ─── API Server ──────────────────────────────

Gargantua:
  from gargantua
  on port 8080 as http
  on port 9090 as grpc
  readiness check /ready
  liveness check /healthz
  run 3 copies
  scale between 3 and 10 copies based on cpu 75%
  keep at least 2 copies running during updates
  needs db
  needs shell-service
  needs permission to manage custom-resources
  needs permission to read and write pods
  needs permission to read and write namespaces
  spread copies across nodes
  graceful shutdown 30s
  run as user 1000

# ─── Shell Service ───────────────────────────

Shell-Service:
  from shell
  on port 8080 as http
  on port 2222 as ssh using tcp
  run 2 copies
  needs permission to manage pods
  needs permission to read namespaces

# ─── Controllers ─────────────────────────────

Scenario-Controller:
  from gargantua
  run 1 copy
  needs permission to manage custom-resources
  needs permission to read pods
  set CONTROLLER_MODE to "scenario"

Environment-Controller:
  from gargantua
  run 1 copy
  needs permission to manage namespaces
  needs permission to manage custom-resources
  needs permission to read and write pods
  set CONTROLLER_MODE to "environment"

Session-Controller:
  from gargantua
  run 1 copy
  needs permission to manage custom-resources
  needs permission to manage pods
  set CONTROLLER_MODE to "session"

# ─── Database ────────────────────────────────

Db:
  postgres 16, 50Gi, daily backups

# ─── Scheduled Maintenance ───────────────────

Session-Cleanup:
  from gargantua
  run every day at 2am
  set CONTROLLER_MODE to "cleanup"
  needs db
  needs permission to manage custom-resources

# ─── Database Migration ──────────────────────

Db-Migrate:
  from gargantua
  run once
  set CONTROLLER_MODE to "migrate"
  needs db
```

That's 10 services, RBAC, multi-port, gRPC, CronJob, Job, and all in plain English. No K8s jargon.

---

## 15. Migration / Backward Compatibility

### 15.1 Recipe language

**Zero breaking changes.** All existing `.mozza` files parse and compile identically. New features are purely additive.

### 15.2 Plan structure

The plan gains new fields. Existing fields are unchanged. Old plans (from before this change) are valid — new fields default to zero values.

### 15.3 Compiler output

K8s compiler: existing manifests are generated identically. New resource types are only emitted when new features are used.

Local compiler: existing docker-compose.yml output is identical. New services (init containers as separate services, sidecars) are only emitted when used.

### 15.4 Wizard UI

The wizard gains new block types and configuration panels. The existing 4 types behave identically. Users see more options but the existing flow is unchanged.

---

## 16. Implementation Order

### Phase 1: Core language + plan (no UI)
1. Expand recipe AST with all new fields (PortSpec, ProbeSpec, InitStep, Sidecar, MountSpec, Permission, SchedulingSpec, NetworkPolicySpec, AutoScaleSpec, DisruptionBudgetSpec, SecuritySpec, UpdateStrategySpec, LifecycleSpec)
2. Add new lexer tokens (~30 new token types, see Section 9.1)
3. Add new parser directives — multi-port, schedule, run-once, daemon, stateful, init blocks, sidecar, mounts, permissions, expanded probes, lifecycle hooks, scheduling, network policy, auto-scaling, disruption budget, security context, update strategy, graceful shutdown
4. Expand plan builder with 10 kinds (web, api, worker, task, scheduled, database, cache, stateful, gateway, daemon)
5. Expand kind inference rules (12-rule priority chain, see Section 3.2)
6. Auto-wiring in plan builder (dependency injection of DATABASE_URL, REDIS_URL, etc.)
7. Expand plan validation with all new rules (~20 new validation checks, see Section 6.4)
8. Plain English → cron expression parser for schedule directives
9. Backward compatibility: ensure `Port int` → `Ports []PortSpec`, `Health string` → `Probes []ProbeSpec` mapping
10. Tests for all new parser directives (each directive needs parse + round-trip tests)
11. Tests for all new plan builder paths (kind inference, auto-wiring, validation)
12. Tests for backward compatibility (existing .mozza files produce identical plans)

### Phase 2: K8s compiler
1. Refactor compiler dispatch — route by SliceKind instead of always emitting Deployment
2. StatefulSet generator (with volumeClaimTemplates, podManagementPolicy)
3. Job generator (with backoffLimit, parallelism, completions)
4. CronJob generator (with schedule, concurrencyPolicy, successfulJobsHistoryLimit)
5. DaemonSet generator (with updateStrategy, nodeSelector, tolerations)
6. Multi-port Service generator (named ports, protocol support)
7. Headless Service generator (for StatefulSets — clusterIP: None)
8. Init containers in Pod spec (initContainers array)
9. Sidecar containers in Pod spec (additional containers with restartPolicy: Always)
10. ConfigMap generator (from mount file directives)
11. Secret volume mount builder
12. RBAC generator — ServiceAccount + Role/ClusterRole + RoleBinding/ClusterRoleBinding
13. NetworkPolicy generator (ingress rules from AllowFrom)
14. HPA generator (autoscaling/v2 with CPU/memory/custom metrics)
15. PodDisruptionBudget generator (policy/v1)
16. SecurityContext generation (Pod + Container level)
17. Scheduling constraints — nodeAffinity, podAntiAffinity, topologySpreadConstraints
18. Expanded probe generation — readiness vs liveness vs startup, HTTP/TCP/exec/gRPC types, timing params
19. Lifecycle hooks — preStop, postStart, terminationGracePeriodSeconds
20. Rolling update strategy — maxSurge, maxUnavailable on Deployment/StatefulSet/DaemonSet
21. Tests for every new K8s resource type (YAML output validation)
22. Tests for resource ordering (PVCs before StatefulSets, RBAC before Deployments, etc.)

### Phase 3: Local compiler
1. Add `compile.Result.Warnings []string` field
2. Multi-port Compose services (multiple port entries)
3. Init containers → separate services with `depends_on: condition: service_completed_successfully`
4. Sidecar containers → separate services with `network_mode: "service:<main>"`
5. File mount → bind mount mapping (relative to .mozza file)
6. Security context → `user:`, `read_only:`, `cap_drop:`, `cap_add:`
7. Graceful shutdown → `stop_grace_period`
8. Job/CronJob degradation (restart: "no" + warning comments)
9. DaemonSet degradation (single instance + warning)
10. StatefulSet degradation (named volumes + warning about identity)
11. K8s-only feature warnings (RBAC, NetworkPolicy, HPA, PDB, scheduling, headless)
12. Tests for all Compose output including warnings

### Phase 4: Wizard UI
1. Expanded block palette (10 types in 4 categories: Services, Background Work, Data, Infrastructure)
2. Kind-specific configuration panels (fields vary by kind — see Section 10.2)
3. Schedule picker component (plain-English presets + custom cron input + preview)
4. Database engine picker component (engine, version, storage, backup selector)
5. Dependency wiring UI (visual "from → to" connections with add/remove)
6. Multi-port configuration (add/name/protocol per port)
7. Auto-wiring display in env vars step (show auto-injected DATABASE_URL etc., editable)
8. Advanced section — collapsible accordion with: init steps, sidecars, file mounts, permissions, health checks (readiness/liveness/startup), lifecycle hooks, scheduling constraints, network policies, auto-scaling, disruption budget, security context, update strategy, graceful shutdown
9. Architecture diagram in review step (visual service graph with connections)
10. Compile warnings display in review step (K8s-only features when targeting local)
11. Update `ServiceConfig` TypeScript interface with all new fields
12. Update `parseRecipeServices` to handle new directives
13. Update `generateRecipePreview` to emit new directives

### Phase 5: Recipe Builder (Canvas)
1. Expanded block palette in Canvas (10 types matching wizard categories)
2. Kind-specific property panels in PropertyPanel.tsx
3. Connection type labels (show dependency type and auto-wired env vars)
4. Update `generateSource` to emit all new directives
5. Update `CanvasBlock` type with new fields

### Phase 6: Doctor rules + CLI
1. 16 new doctor rules (see Section 12.2 — ScheduleWithoutConcurrency through NoUpdateStrategy)
2. Enhanced validate output with new directive validation
3. CLI help text updates for new recipe syntax
4. Example recipes in `examples/` directory showcasing new features:
   - `examples/microservices.mozza` (multi-service with gRPC, RBAC, auto-scaling)
   - `examples/batch-pipeline.mozza` (jobs, cron jobs, init containers)
   - `examples/stateful-cluster.mozza` (StatefulSet, peer discovery, per-copy storage)
   - `examples/hobbyfarm.mozza` (full Gargantua architecture)

---

## 17. Open Questions

1. **CRD installation:** Should Mozza handle CRD installation (e.g., HobbyFarm's custom resources)? This is a one-time setup step, not a workload. Options: (a) a `setup:` section in the recipe, (b) a separate `mozza install-crds` command, (c) out of scope — user installs CRDs themselves.

2. **Helm chart import:** Should Mozza be able to read a Helm chart and generate a `.mozza` file? This would help onboard existing projects.

3. **Multi-container pods vs. separate services:** The sidecar feature puts containers in the same pod. Should there be a way to group non-sidecar containers in the same pod? (Probably not — this is a K8s power-user pattern that breaks the "plain English" principle.)

4. **Config file management:** The `mount file` directive references local files. Where do these files live? Options: (a) relative to the `.mozza` file, (b) in a `config/` directory convention, (c) stored in the Mozza database (for wizard-created recipes).

5. **Secret management:** The wizard collects secret values. Where are they stored? Options: (a) K8s Secrets (created by Mozza before deploy), (b) external secret manager reference, (c) encrypted in Mozza's database.

6. **Concurrency policy for CronJobs:** Should the recipe support `allow overlapping` / `skip if already running` / `replace if already running`? Defaults to "skip if already running" (safest).

7. **Resource defaults:** Should Mozza set default CPU/memory limits if the user doesn't specify? This prevents runaway containers but might surprise users.

8. **Ingress class:** Should the recipe support specifying which ingress controller to use (nginx, traefik, etc.)? Or should this be a cluster-level configuration in Mozza?

9. **RBAC auto-generation vs. explicit:** The research agent suggests RBAC should be auto-generated from `needs` relationships rather than user-specified. E.g., if a slice `needs permission to manage pods`, Mozza auto-creates the ServiceAccount + Role + RoleBinding. The current spec has explicit `needs permission to...` directives — should these be the ONLY way, or should Mozza also auto-infer some permissions (e.g., a slice that `needs db` gets read permission to the db service)?

10. **DaemonSet scope:** The research agent suggests DaemonSets are more of a cluster-operator concern than an app-developer concern. Should Mozza support them at all in the recipe, or should they be managed separately? Use cases like log collectors and monitoring agents are infrastructure, not app workloads.

11. **Resource requests vs. limits:** Currently only `limit cpu/memory` is supported. Should we add `request cpu/memory` (K8s requests = guaranteed minimum)? Plain English: "this needs at least 256MB RAM" vs. "this can use at most 1GB RAM". Compose maps requests to `deploy.resources.reservations`.

12. **TLS/cert-manager integration:** Should `domain "api.example.com"` auto-generate TLS annotations for cert-manager? Or is TLS configuration out of scope for the recipe? Options: (a) `domain "api.example.com" with tls` auto-adds cert-manager annotations, (b) TLS is a cluster-level configuration, (c) explicit `tls from cert-manager` directive.

13. **Ingress path routing:** Current K8s compiler generates `/{sliceName}` paths. Should the recipe support explicit path routing? E.g., `route /api/* to this` or `route shop.example.com to this`. Multiple slices could share a domain with different paths.

14. **Compose profiles for optional services:** Should the local compiler use Docker Compose profiles to make init/task services opt-in? E.g., `docker compose up` starts the app, `docker compose run --rm migrate` runs the migration task.

15. **Monitoring/observability directives:** Should the recipe support `expose metrics on port 9090 at /metrics` for Prometheus scraping? This is common enough in production to warrant a first-class directive rather than just a named port.
