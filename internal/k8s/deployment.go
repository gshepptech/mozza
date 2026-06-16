package k8s

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildDeployment generates a typed Kubernetes Deployment for the given slice.
func BuildDeployment(s plan.Slice, namespace string, appName string) *appsv1.Deployment {
	lbls := labels(appName, s.Name)
	replicas := int32(s.Replicas)
	if replicas < 1 {
		replicas = 1
	}

	podSpec := buildFullPodSpec(s, appName)

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: podSpec,
			},
		},
	}

	// Update strategy.
	if s.UpdateStrategy != nil {
		dep.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxSurge:       parseIntOrString(s.UpdateStrategy.MaxSurge),
				MaxUnavailable: parseIntOrString(s.UpdateStrategy.MaxUnavailable),
			},
		}
	}

	return dep
}

// buildContainer creates the container spec for a slice.
func buildContainer(s plan.Slice) corev1.Container {
	c := corev1.Container{
		Name:  s.Name,
		Image: s.Image,
	}

	// Multi-port support: use Ports[] if available, otherwise single Port.
	if len(s.Ports) > 0 {
		for _, p := range s.Ports {
			c.Ports = append(c.Ports, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: int32(p.Port),
				Protocol:      mapProtocol(p.Protocol),
			})
		}
	} else if s.Port > 0 {
		c.Ports = []corev1.ContainerPort{{ContainerPort: int32(s.Port)}}
	}

	// Plain env vars.
	if len(s.Env) > 0 {
		keys := make([]string, 0, len(s.Env))
		for k := range s.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			c.Env = append(c.Env, corev1.EnvVar{Name: k, Value: s.Env[k]})
		}
	}

	// Secret-sourced env vars.
	for _, sr := range s.Secrets {
		c.Env = append(c.Env, corev1.EnvVar{
			Name: sr.EnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: sr.SecretName},
					Key:                  sr.Key,
				},
			},
		})
	}

	// Resource limits.
	if s.Resources != nil {
		limits := corev1.ResourceList{}
		if s.Resources.CPULimit != "" {
			limits[corev1.ResourceCPU] = resource.MustParse(s.Resources.CPULimit)
		}
		if s.Resources.MemoryLimit != "" {
			limits[corev1.ResourceMemory] = resource.MustParse(s.Resources.MemoryLimit)
		}
		if len(limits) > 0 {
			c.Resources = corev1.ResourceRequirements{Limits: limits}
		}
	}

	// Expanded probes: use Probes[] if available, otherwise fall back to HealthPath.
	if len(s.Probes) > 0 {
		for _, ps := range s.Probes {
			probe := buildProbe(ps)
			switch strings.ToLower(ps.Type) {
			case "liveness":
				c.LivenessProbe = probe
			case "readiness":
				c.ReadinessProbe = probe
			case "startup":
				c.StartupProbe = probe
			}
		}
	} else if s.HealthPath != "" && s.Port > 0 {
		// Backward compat: single HealthPath generates all three probes.
		probe := &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: s.HealthPath,
					Port: intstr.FromInt32(int32(s.Port)),
				},
			},
		}
		c.LivenessProbe = probe
		c.ReadinessProbe = probe
		c.StartupProbe = probe
	}

	return c
}

// buildProbe converts a plan.ProbeSpec into a Kubernetes Probe.
// It selects the probe handler based on which field is set:
// HTTPPath → httpGet, TCPPort → tcpSocket, Command → exec.
func buildProbe(ps plan.ProbeSpec) *corev1.Probe {
	probe := &corev1.Probe{}

	switch {
	case ps.HTTPPath != "":
		port := intstr.FromInt32(int32(ps.TCPPort))
		if ps.TCPPort == 0 {
			port = intstr.FromInt32(80)
		}
		probe.ProbeHandler = corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: ps.HTTPPath,
				Port: port,
			},
		}
	case ps.TCPPort > 0:
		probe.ProbeHandler = corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(int32(ps.TCPPort)),
			},
		}
	case ps.Command != "":
		probe.ProbeHandler = corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: strings.Fields(ps.Command),
			},
		}
	}

	if ps.Interval > 0 {
		probe.PeriodSeconds = int32(ps.Interval)
	}
	if ps.Timeout > 0 {
		probe.TimeoutSeconds = int32(ps.Timeout)
	}
	if ps.Delay > 0 {
		probe.InitialDelaySeconds = int32(ps.Delay)
	}

	return probe
}

// addVolumes adds PVC-backed volumes and mounts for database and cache slices.
// Skipped for StatefulSet kinds — those use volumeClaimTemplates instead.
func addVolumes(ps *corev1.PodSpec, s plan.Slice) {
	if s.Kind == plan.SliceKindDatabase || s.Kind == plan.SliceKindStateful {
		return
	}
	if s.Database != nil && s.Database.Storage != "" {
		volName := s.Name + "-data"
		mountPath := "/var/lib/data"
		if s.Database.MountPath != "" {
			mountPath = s.Database.MountPath
		}
		ps.Volumes = append(ps.Volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: s.Name,
				},
			},
		})
		ps.Containers[0].VolumeMounts = append(ps.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: mountPath,
		})
	}

	if s.Cache != nil && s.Cache.Storage != "" {
		volName := s.Name + "-data"
		mountPath := "/var/lib/data"
		if s.Cache.MountPath != "" {
			mountPath = s.Cache.MountPath
		}
		ps.Volumes = append(ps.Volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: s.Name,
				},
			},
		})
		ps.Containers[0].VolumeMounts = append(ps.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: mountPath,
		})
	}
}

// addPullSecrets adds imagePullSecrets to the pod spec if configured.
func addPullSecrets(ps *corev1.PodSpec, s plan.Slice) {
	if s.PullSecret != "" {
		ps.ImagePullSecrets = append(ps.ImagePullSecrets, corev1.LocalObjectReference{
			Name: s.PullSecret,
		})
	}
}

// buildFullPodSpec creates a complete PodSpec with all advanced features applied:
// init containers, sidecars, lifecycle hooks, graceful shutdown, mounts,
// security context, scheduling, and service account.
func buildFullPodSpec(s plan.Slice, appName string) corev1.PodSpec {
	container := buildContainer(s)

	// Lifecycle hooks on main container.
	addLifecycle(&container, s)

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	addVolumes(&podSpec, s)
	addMounts(&podSpec, s)
	addPullSecrets(&podSpec, s)
	addInitContainers(&podSpec, s)
	addSidecars(&podSpec, s)
	addSecurityContext(&podSpec, &podSpec.Containers[0], s)
	addScheduling(&podSpec, s, appName)

	// Graceful shutdown.
	if s.GracefulShutdown > 0 {
		grace := int64(s.GracefulShutdown)
		podSpec.TerminationGracePeriodSeconds = &grace
	}

	// Service account reference.
	if s.ServiceAccount != "" {
		podSpec.ServiceAccountName = s.ServiceAccount
	} else if len(s.Permissions) > 0 {
		podSpec.ServiceAccountName = s.Name
	}

	return podSpec
}

// addInitContainers adds init containers from InitSteps to the PodSpec.
func addInitContainers(ps *corev1.PodSpec, s plan.Slice) {
	for i, step := range s.InitSteps {
		initC := corev1.Container{
			Name:  fmt.Sprintf("init-%d", i),
			Image: step.Image,
		}
		if step.Command != "" {
			initC.Command = strings.Fields(step.Command)
		}
		if len(step.Env) > 0 {
			keys := make([]string, 0, len(step.Env))
			for k := range step.Env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				initC.Env = append(initC.Env, corev1.EnvVar{
					Name: k, Value: step.Env[k],
				})
			}
		}
		ps.InitContainers = append(ps.InitContainers, initC)
	}
}

// addSidecars adds sidecar containers to the PodSpec after the main container.
func addSidecars(ps *corev1.PodSpec, s plan.Slice) {
	for _, sc := range s.Sidecars {
		c := corev1.Container{
			Name:  sc.Name,
			Image: sc.Image,
		}
		for _, p := range sc.Ports {
			c.Ports = append(c.Ports, corev1.ContainerPort{
				Name:          p.Name,
				ContainerPort: int32(p.Port),
				Protocol:      mapProtocol(p.Protocol),
			})
		}
		if len(sc.Env) > 0 {
			keys := make([]string, 0, len(sc.Env))
			for k := range sc.Env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				c.Env = append(c.Env, corev1.EnvVar{
					Name: k, Value: sc.Env[k],
				})
			}
		}
		ps.Containers = append(ps.Containers, c)
	}
}

// addLifecycle configures container lifecycle hooks from the slice's Lifecycle spec.
func addLifecycle(c *corev1.Container, s plan.Slice) {
	if s.Lifecycle == nil {
		return
	}
	lc := &corev1.Lifecycle{}
	hasHook := false

	if s.Lifecycle.PreStopCommand != "" {
		lc.PreStop = &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: strings.Fields(s.Lifecycle.PreStopCommand),
			},
		}
		hasHook = true
	} else if s.Lifecycle.PreStopWait > 0 {
		lc.PreStop = &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"sleep", fmt.Sprintf("%d", s.Lifecycle.PreStopWait)},
			},
		}
		hasHook = true
	}

	if s.Lifecycle.PostStartCommand != "" {
		lc.PostStart = &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: strings.Fields(s.Lifecycle.PostStartCommand),
			},
		}
		hasHook = true
	}

	if hasHook {
		c.Lifecycle = lc
	}
}

// addMounts adds volume and volumeMount entries from the slice's Mounts[] field.
func addMounts(ps *corev1.PodSpec, s plan.Slice) {
	for _, m := range s.Mounts {
		volName := fmt.Sprintf("mount-%s", m.Source)
		vol := corev1.Volume{Name: volName}

		switch strings.ToLower(m.Type) {
		case "pvc":
			vol.VolumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: m.Source,
					ReadOnly:  m.ReadOnly,
				},
			}
		case "configmap", "config-dir", "file":
			vol.VolumeSource = corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: m.Source},
				},
			}
		case "secret":
			vol.VolumeSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: m.Source,
				},
			}
		case "emptydir":
			vol.VolumeSource = corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}
		default:
			continue
		}

		ps.Volumes = append(ps.Volumes, vol)
		ps.Containers[0].VolumeMounts = append(ps.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: m.Target,
			ReadOnly:  m.ReadOnly,
		})
	}
}

// addSecurityContext applies pod and container security settings from the slice's Security spec.
func addSecurityContext(ps *corev1.PodSpec, c *corev1.Container, s plan.Slice) {
	if s.Security == nil {
		return
	}

	// Pod-level security context.
	podSC := &corev1.PodSecurityContext{}
	hasPodSC := false
	if s.Security.RunAsUser > 0 {
		uid := int64(s.Security.RunAsUser)
		podSC.RunAsUser = &uid
		hasPodSC = true
	}
	if s.Security.RunAsGroup > 0 {
		gid := int64(s.Security.RunAsGroup)
		podSC.RunAsGroup = &gid
		hasPodSC = true
	}
	if hasPodSC {
		ps.SecurityContext = podSC
	}

	// Container-level security context.
	csc := &corev1.SecurityContext{}
	hasCSC := false

	if s.Security.ReadOnlyRoot {
		ro := true
		csc.ReadOnlyRootFilesystem = &ro
		hasCSC = true
	}

	noEscalate := false
	csc.AllowPrivilegeEscalation = &noEscalate
	hasCSC = true

	if len(s.Security.DropCapabilities) > 0 || len(s.Security.AddCapabilities) > 0 {
		caps := &corev1.Capabilities{}
		for _, cap := range s.Security.DropCapabilities {
			caps.Drop = append(caps.Drop, corev1.Capability(cap))
		}
		for _, cap := range s.Security.AddCapabilities {
			caps.Add = append(caps.Add, corev1.Capability(cap))
		}
		csc.Capabilities = caps
		hasCSC = true
	}

	if hasCSC {
		c.SecurityContext = csc
	}
}

// addScheduling applies node affinity, pod anti-affinity, and topology spread constraints.
func addScheduling(ps *corev1.PodSpec, s plan.Slice, appName string) {
	if s.Scheduling == nil {
		return
	}

	affinity := &corev1.Affinity{}
	hasAffinity := false

	// Node affinity: required.
	if len(s.Scheduling.NodeRequirements) > 0 {
		var exprs []corev1.NodeSelectorRequirement
		for _, req := range s.Scheduling.NodeRequirements {
			exprs = append(exprs, corev1.NodeSelectorRequirement{
				Key:      req.Key,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{req.Value},
			})
		}
		if affinity.NodeAffinity == nil {
			affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{MatchExpressions: exprs},
			},
		}
		hasAffinity = true
	}

	// Node affinity: preferred.
	if len(s.Scheduling.NodePreferences) > 0 {
		var prefs []corev1.PreferredSchedulingTerm
		for _, pref := range s.Scheduling.NodePreferences {
			prefs = append(prefs, corev1.PreferredSchedulingTerm{
				Weight: 100,
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{
						{
							Key:      pref.Key,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{pref.Value},
						},
					},
				},
			})
		}
		if affinity.NodeAffinity == nil {
			affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = prefs
		hasAffinity = true
	}

	// Pod anti-affinity.
	if s.Scheduling.AntiAffinity {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								labelComponent: s.Name,
								labelApp:       appName,
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
		hasAffinity = true
	}

	if hasAffinity {
		ps.Affinity = affinity
	}

	// Topology spread constraints.
	if s.Scheduling.SpreadTopology != "" {
		maxSkew := int32(1)
		topologyKey := mapTopologyKey(s.Scheduling.SpreadTopology)
		ps.TopologySpreadConstraints = []corev1.TopologySpreadConstraint{
			{
				MaxSkew:           maxSkew,
				TopologyKey:       topologyKey,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						labelComponent: s.Name,
						labelApp:       appName,
					},
				},
			},
		}
	}
}

// mapTopologyKey maps short topology names from recipes to K8s topology keys.
func mapTopologyKey(short string) string {
	switch short {
	case "zones", "zone":
		return "topology.kubernetes.io/zone"
	case "regions", "region":
		return "topology.kubernetes.io/region"
	case "nodes", "node":
		return "kubernetes.io/hostname"
	default:
		return short
	}
}

// parseIntOrString parses a string as either an integer or a percent string
// for use with Kubernetes IntOrString fields.
func parseIntOrString(val string) *intstr.IntOrString {
	if val == "" {
		return nil
	}
	v := intstr.Parse(val)
	return &v
}
