// Package plan provides the intermediate representation for Mozza applications.
// The AppPlan is the central data structure that recipe parsing produces and
// compilers consume.
package plan

// SliceKind categorizes the type of a deployment slice.
type SliceKind string

// Slice kind constants.
const (
	// SliceKindWeb represents a web-facing service slice.
	SliceKindWeb SliceKind = "web"
	// SliceKindWorker represents a background worker slice.
	SliceKindWorker SliceKind = "worker"
	// SliceKindDatabase represents a database slice.
	SliceKindDatabase SliceKind = "database"
	// SliceKindCache represents a cache slice.
	SliceKindCache SliceKind = "cache"
	// SliceKindAPI represents an API service slice.
	SliceKindAPI SliceKind = "api"
	// SliceKindTask represents a one-shot or batch task slice.
	SliceKindTask SliceKind = "task"
	// SliceKindScheduled represents a cron-scheduled slice.
	SliceKindScheduled SliceKind = "scheduled"
	// SliceKindStateful represents a stateful (StatefulSet) slice.
	SliceKindStateful SliceKind = "stateful"
	// SliceKindGateway represents an API gateway or ingress slice.
	SliceKindGateway SliceKind = "gateway"
	// SliceKindDaemon represents a daemon (DaemonSet) slice that runs on every node.
	SliceKindDaemon SliceKind = "daemon"
)

// AppPlan is the intermediate representation of a Mozza application.
// It is produced by parsing a .mozza recipe and consumed by compilers
// to generate deployment artifacts.
type AppPlan struct {
	// Name is the application name from the recipe.
	Name string
	// Namespace is the deployment namespace (e.g. "production", "staging").
	Namespace string
	// Slices are the deployment units (services, databases, caches).
	Slices []Slice
	// Ingredients are the resolved dependencies between slices.
	Ingredients []Ingredient
	// CRDs lists URLs to Custom Resource Definition YAML files to apply before workloads.
	CRDs []string
}

// Slice represents a single deployment unit within an application.
type Slice struct {
	// Name identifies the slice within the application.
	Name string
	// Kind categorizes the slice (web, worker, database, cache).
	Kind SliceKind
	// Image is the container image reference.
	Image string
	// Port is the container port to expose (0 means no port).
	Port int
	// Public indicates whether the slice should be externally accessible.
	Public bool
	// Replicas is the desired number of instances.
	Replicas int
	// HealthPath is the HTTP health check endpoint path.
	HealthPath string
	// Needs lists the names of slices this slice depends on.
	Needs []string
	// Env holds environment variable key-value pairs.
	Env map[string]string
	// Resources holds CPU and memory resource limits.
	Resources *ResourceSpec
	// RestartPolicy is the container restart policy (e.g. "always", "unless-stopped").
	RestartPolicy string
	// Domain is the custom domain for routing (e.g. "api.example.com").
	Domain string
	// Database holds database-specific configuration (nil for non-database slices).
	Database *DatabaseSpec
	// Cache holds cache-specific configuration (nil for non-cache slices).
	Cache *CacheSpec
	// Secrets holds secret references for environment variables sourced from K8s Secrets.
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
}

// SecretRef maps an environment variable to a key within a Kubernetes Secret.
type SecretRef struct {
	// EnvVar is the environment variable name exposed to the container.
	EnvVar string
	// SecretName is the Kubernetes Secret name.
	SecretName string
	// Key is the key within the Secret.
	Key string
}

// ResourceSpec holds CPU and memory resource limits for a slice.
type ResourceSpec struct {
	// CPULimit is the CPU resource limit (e.g., "500m", "1").
	CPULimit string
	// MemoryLimit is the memory resource limit (e.g., "256Mi", "1Gi").
	MemoryLimit string
}

// DatabaseSpec holds database-specific configuration.
type DatabaseSpec struct {
	// Storage is the persistent volume size (e.g., "10Gi").
	Storage string
	// BackupPolicy is the backup schedule (e.g., "daily").
	BackupPolicy string
	// MountPath is the engine-specific data directory path.
	MountPath string
}

// CacheSpec holds cache-specific configuration.
type CacheSpec struct {
	// Storage is the optional persistent volume size for the cache.
	Storage string
	// MountPath is the engine-specific data directory path.
	MountPath string
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

// Ingredient represents a dependency relationship between two slices.
type Ingredient struct {
	// From is the name of the dependent slice.
	From string
	// To is the name of the dependency slice.
	To string
}

// SliceByName returns the slice with the given name, or nil if not found.
func (p *AppPlan) SliceByName(name string) *Slice {
	for i := range p.Slices {
		if p.Slices[i].Name == name {
			return &p.Slices[i]
		}
	}
	return nil
}

// PublicSlices returns all slices marked as public.
func (p *AppPlan) PublicSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Public {
			result = append(result, s)
		}
	}
	return result
}

// DatabaseSlices returns all slices of kind database.
func (p *AppPlan) DatabaseSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindDatabase {
			result = append(result, s)
		}
	}
	return result
}

// CacheSlices returns all slices of kind cache.
func (p *AppPlan) CacheSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindCache {
			result = append(result, s)
		}
	}
	return result
}

// APISlices returns all slices of kind API.
func (p *AppPlan) APISlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindAPI {
			result = append(result, s)
		}
	}
	return result
}

// TaskSlices returns all slices of kind task.
func (p *AppPlan) TaskSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindTask {
			result = append(result, s)
		}
	}
	return result
}

// ScheduledSlices returns all slices of kind scheduled.
func (p *AppPlan) ScheduledSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindScheduled {
			result = append(result, s)
		}
	}
	return result
}

// StatefulSlices returns all slices of kind stateful.
func (p *AppPlan) StatefulSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindStateful {
			result = append(result, s)
		}
	}
	return result
}

// DaemonSlices returns all slices of kind daemon.
func (p *AppPlan) DaemonSlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindDaemon {
			result = append(result, s)
		}
	}
	return result
}

// GatewaySlices returns all slices of kind gateway.
func (p *AppPlan) GatewaySlices() []Slice {
	var result []Slice
	for _, s := range p.Slices {
		if s.Kind == SliceKindGateway {
			result = append(result, s)
		}
	}
	return result
}
