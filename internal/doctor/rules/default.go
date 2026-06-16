package rules

import "github.com/gshepptech/mozza/internal/doctor"

// Default returns the standard set of diagnostic rules.
func Default() []doctor.Rule {
	return []doctor.Rule{
		DockerRule{},
		ImageRule{},
		PortRule{},
		K8sRBACRule{},
		PublicDatabaseWarning{},
		NoHealthCheck{},
		SingleReplicaProduction{},
		NoResourceLimits{},
		NoAutoScaleWithHighReplicas{},
		RunAsRoot{},
		NoGracefulShutdown{},
	}
}
