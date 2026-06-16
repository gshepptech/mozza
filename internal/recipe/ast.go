package recipe

// Recipe is the root AST node representing a complete .mozza file.
// It contains the application name and all declared slices.
type Recipe struct {
	// Name is the application name from the `App: name` declaration.
	Name string

	// Namespace is the deployment namespace from the `Namespace: name` declaration.
	Namespace string

	// Aliases maps friendly names to Docker image references from the
	// `Images:` section. For example, "frontend" -> "myorg/frontend:latest".
	Aliases map[string]string

	// CRDs lists URLs to Custom Resource Definition YAML files to apply before workloads.
	CRDs []string

	// Slices holds all slice declarations in source order.
	Slices []Slice
}

// Slice represents a section block in the recipe. Each slice declares a
// deployable component with its configuration directives.
type Slice struct {
	// Name is the slice identifier from the section header.
	Name string

	// Kind classifies the slice: web, worker, database, or cache.
	// May be empty if kind should be inferred by the plan builder.
	Kind string

	// Image is the container image reference.
	Image string

	// Port is the network port the slice listens on.
	Port int

	// Public indicates whether the slice is externally accessible.
	Public bool

	// Health is the HTTP health-check endpoint path.
	Health string

	// Replicas is the desired number of running instances.
	Replicas int

	// Needs lists the names of slices this slice depends on.
	Needs []string

	// Storage is the persistent volume size (e.g. "20Gi").
	Storage string

	// Engine is the database/cache engine name (e.g. "postgres", "redis").
	Engine string

	// Version is the engine version (e.g. "16", "7").
	Version string

	// Backups is the backup policy (e.g. "daily").
	Backups string

	// Env holds environment variable key-value pairs from `set` directives.
	Env map[string]string

	// CPULimit is the CPU resource limit (e.g. "500m").
	CPULimit string

	// MemoryLimit is the memory resource limit (e.g. "256Mi").
	MemoryLimit string

	// RestartPolicy is the container restart policy (e.g. "always", "unless-stopped").
	RestartPolicy string

	// Domain is the custom domain for this slice (e.g. "api.example.com").
	Domain string

	// Secrets holds secret references from "secret KEY from NAME [key KEYNAME]" directives.
	Secrets []SecretRef

	// PullSecret is the K8s Secret name for image pull credentials.
	PullSecret string

	// Ports lists named ports exposed by the slice.
	Ports []PortSpec

	// Probes lists health, readiness, and startup probes for the slice.
	Probes []ProbeSpec

	// InitSteps lists init containers that run before the main container.
	InitSteps []InitStep

	// Sidecars lists auxiliary containers running alongside the main container.
	Sidecars []Sidecar

	// Mounts lists volume mounts for the container.
	Mounts []MountSpec

	// Lifecycle defines container lifecycle hooks (pre-stop, post-start).
	Lifecycle *LifecycleSpec

	// Permissions lists RBAC permissions required by the slice.
	Permissions []Permission

	// ServiceAccount is the Kubernetes service account name for the slice.
	ServiceAccount string

	// Scheduling controls pod scheduling preferences and constraints.
	Scheduling *SchedulingSpec

	// NetworkPolicy defines network-level access controls.
	NetworkPolicy *NetworkPolicySpec

	// AutoScale configures horizontal pod autoscaling.
	AutoScale *AutoScaleSpec

	// DisruptionBudget defines pod disruption budget constraints.
	DisruptionBudget *DisruptionBudgetSpec

	// Security defines container-level security settings.
	Security *SecuritySpec

	// UpdateStrategy configures the rolling update strategy.
	UpdateStrategy *UpdateStrategySpec

	// GracefulShutdown is the grace period in seconds before forceful termination.
	GracefulShutdown int

	// Schedule is a cron expression for scheduled (CronJob) slices.
	Schedule string

	// RunOnce indicates the slice should run as a one-shot Job.
	RunOnce bool

	// Parallelism is the number of concurrent pods for Job/task slices.
	Parallelism int

	// Retries is the number of retry attempts for failed Job/task slices.
	Retries int

	// DaemonMode indicates the slice should run on every node (DaemonSet).
	DaemonMode bool

	// OrderedStartup indicates pods should start sequentially (StatefulSet).
	OrderedStartup bool

	// PeerDiscovery enables headless service for pod-to-pod discovery (StatefulSet).
	PeerDiscovery bool

	// StatefulStorage is the per-pod persistent volume size for StatefulSet slices.
	StatefulStorage string

	// DNSName is the stable DNS name assigned to this slice.
	DNSName string

	// Line is the 1-based source line of the slice declaration,
	// retained for error reporting.
	Line int
}

// SecretRef maps an environment variable to a key within a Kubernetes Secret.
type SecretRef struct {
	// EnvVar is the environment variable name exposed to the container.
	EnvVar string

	// SecretName is the Kubernetes Secret name.
	SecretName string

	// Key is the key within the Secret. Defaults to EnvVar if not specified.
	Key string
}

// PortSpec describes a named network port exposed by a slice.
type PortSpec struct {
	// Name is an optional human-readable label for the port (e.g. "http", "grpc").
	Name string
	// Port is the numeric port number.
	Port int
	// Protocol is the transport protocol (e.g. "TCP", "UDP"). Defaults to "TCP".
	Protocol string
}

// ProbeSpec defines a health or readiness probe for a slice.
type ProbeSpec struct {
	// Type is the probe kind: "liveness", "readiness", or "startup".
	Type string
	// HTTPPath is the HTTP GET path for HTTP probes.
	HTTPPath string
	// Command is the exec command for exec probes.
	Command string
	// TCPPort is the port for TCP socket probes.
	TCPPort int
	// Interval is the probe interval in seconds.
	Interval int
	// Timeout is the probe timeout in seconds.
	Timeout int
	// Delay is the initial delay before the first probe in seconds.
	Delay int
}

// InitStep describes an init container that runs before the main container.
type InitStep struct {
	// Image is the container image for the init step.
	Image string
	// Command is the command to execute.
	Command string
	// Env holds environment variables for the init step.
	Env map[string]string
}

// Sidecar describes an auxiliary container running alongside the main container.
type Sidecar struct {
	// Name identifies the sidecar container.
	Name string
	// Image is the container image for the sidecar.
	Image string
	// Ports lists the ports exposed by the sidecar.
	Ports []PortSpec
	// Env holds environment variables for the sidecar.
	Env map[string]string
}

// MountSpec describes a volume mount for a container.
type MountSpec struct {
	// Type is the volume type (e.g. "pvc", "configmap", "secret", "emptydir").
	Type string
	// Source is the volume source name or reference.
	Source string
	// Target is the mount path inside the container.
	Target string
	// ReadOnly indicates whether the mount should be read-only.
	ReadOnly bool
}

// LifecycleSpec defines container lifecycle hooks.
type LifecycleSpec struct {
	// PreStopCommand is the command to run before the container stops.
	PreStopCommand string
	// PreStopWait is the number of seconds to wait after running the pre-stop command.
	PreStopWait int
	// PostStartCommand is the command to run after the container starts.
	PostStartCommand string
}

// Permission describes an RBAC permission grant for the slice's service account.
type Permission struct {
	// Verbs are the allowed API verbs (e.g. "get", "list", "watch").
	Verbs []string
	// Resources are the Kubernetes resource types (e.g. "pods", "services").
	Resources []string
	// Namespace restricts the permission to a specific namespace. Empty means same namespace.
	Namespace string
	// ClusterWide indicates whether the permission applies cluster-wide.
	ClusterWide bool
}

// LabelConstraint is a key-value pair used for node selection or topology constraints.
type LabelConstraint struct {
	// Key is the label key.
	Key string
	// Value is the label value.
	Value string
}

// SchedulingSpec controls pod scheduling preferences and constraints.
type SchedulingSpec struct {
	// NodePreferences are soft node-selection constraints (preferred but not required).
	NodePreferences []LabelConstraint
	// NodeRequirements are hard node-selection constraints (must match).
	NodeRequirements []LabelConstraint
	// SpreadTopology is the topology key for pod spread (e.g. "topology.kubernetes.io/zone").
	SpreadTopology string
	// AntiAffinity indicates whether pods should avoid co-location on the same node.
	AntiAffinity bool
}

// NetworkPolicySpec defines network-level access controls for a slice.
type NetworkPolicySpec struct {
	// AllowFrom lists pod selectors allowed to reach this slice.
	AllowFrom []string
	// AllowNamespace lists namespaces allowed to reach this slice.
	AllowNamespace []string
	// DenyAll blocks all ingress when true (AllowFrom/AllowNamespace act as exceptions).
	DenyAll bool
}

// AutoScaleSpec configures horizontal pod autoscaling for a slice.
type AutoScaleSpec struct {
	// MinReplicas is the minimum number of replicas.
	MinReplicas int
	// MaxReplicas is the maximum number of replicas.
	MaxReplicas int
	// CPUTarget is the target CPU utilization percentage.
	CPUTarget int
	// MemoryTarget is the target memory utilization percentage.
	MemoryTarget int
	// CustomMetric is the name of a custom metric to scale on.
	CustomMetric string
	// CustomTarget is the target value for the custom metric.
	CustomTarget int
}

// DisruptionBudgetSpec defines pod disruption budget constraints.
type DisruptionBudgetSpec struct {
	// MinAvailable is the minimum number of pods that must remain available during disruption.
	MinAvailable int
	// MaxUnavailable is the maximum number of pods that can be unavailable during disruption.
	MaxUnavailable int
}

// SecuritySpec defines container-level security settings.
type SecuritySpec struct {
	// RunAsUser is the UID to run the container process as.
	RunAsUser int
	// RunAsGroup is the GID to run the container process as.
	RunAsGroup int
	// ReadOnlyRoot makes the container's root filesystem read-only.
	ReadOnlyRoot bool
	// DropCapabilities lists Linux capabilities to drop.
	DropCapabilities []string
	// AddCapabilities lists Linux capabilities to add.
	AddCapabilities []string
}

// UpdateStrategySpec configures the rolling update strategy for a slice.
type UpdateStrategySpec struct {
	// MaxSurge is the maximum number of pods above the desired count during update (e.g. "25%", "1").
	MaxSurge string
	// MaxUnavailable is the maximum number of unavailable pods during update (e.g. "25%", "0").
	MaxUnavailable string
}
