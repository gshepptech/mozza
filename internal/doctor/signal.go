package doctor

import "context"

// Signal holds environment data collected for rule evaluation.
// Rules inspect these fields to diagnose issues with the runtime environment.
type Signal struct {
	// DockerReachable indicates if the Docker daemon is accessible.
	DockerReachable bool
	// DockerError holds the error if Docker is not reachable.
	DockerError error
	// AvailableImages lists images present in the local Docker cache.
	AvailableImages []string
	// UsedPorts lists TCP ports currently in use on the host.
	UsedPorts []int
	// K8sReachable indicates if a Kubernetes cluster is accessible.
	K8sReachable bool
	// K8sError holds the error if the cluster is not reachable.
	K8sError error
	// K8sPermissions maps "resource/verb" to whether it's allowed.
	K8sPermissions map[string]bool
}

// SignalCollector gathers environment signals for rule evaluation.
// Implementations may query Docker, scan ports, or return fixed test data.
type SignalCollector interface {
	// Collect gathers environment signals. The context allows callers to set
	// timeouts on potentially slow operations like Docker health checks.
	Collect(ctx context.Context) (*Signal, error)
}
