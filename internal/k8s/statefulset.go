package k8s

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildStatefulSet generates a typed Kubernetes StatefulSet for the given slice.
// It creates a StatefulSet with volumeClaimTemplates for persistent per-pod storage
// and a headless service name matching the slice name.
func BuildStatefulSet(s plan.Slice, namespace string, appName string) *appsv1.StatefulSet {
	lbls := labels(appName, s.Name)
	replicas := int32(s.Replicas)
	if replicas < 1 {
		replicas = 1
	}

	podSpec := buildFullPodSpec(s, appName)

	// Pod management policy: OrderedReady for ordered startup, Parallel otherwise.
	podMgmt := appsv1.ParallelPodManagement
	if s.OrderedStartup {
		podMgmt = appsv1.OrderedReadyPodManagement
	}

	ss := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName:         s.Name,
			Replicas:            &replicas,
			PodManagementPolicy: podMgmt,
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

	// Add volumeClaimTemplates for persistent storage.
	storageSize := s.StatefulStorage
	mountName := "data"
	if storageSize == "" && s.Database != nil && s.Database.Storage != "" {
		storageSize = s.Database.Storage
	}
	if storageSize != "" {
		// Mount path for the volume — use database-specific path if available.
		mountPath := "/data"
		if s.Database != nil && s.Database.MountPath != "" {
			mountPath = s.Database.MountPath
		}

		ss.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: mountName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(storageSize),
						},
					},
				},
			},
		}

		// Add volume mount to the first container if not already present.
		if len(podSpec.Containers) > 0 {
			hasMountPath := false
			for _, vm := range podSpec.Containers[0].VolumeMounts {
				if vm.MountPath == mountPath {
					hasMountPath = true
					break
				}
			}
			if !hasMountPath {
				podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts,
					corev1.VolumeMount{Name: mountName, MountPath: mountPath})
			}
			ss.Spec.Template.Spec = podSpec
		}
	}

	// Inject default env vars for well-known database images.
	if s.Kind == plan.SliceKindDatabase && len(ss.Spec.Template.Spec.Containers) > 0 {
		injectDatabaseEnv(&ss.Spec.Template.Spec.Containers[0], s)
	}

	// Update strategy.
	if s.UpdateStrategy != nil {
		ss.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: appsv1.RollingUpdateStatefulSetStrategyType,
		}
	}

	return ss
}

// injectDatabaseEnv adds required env vars for well-known database images
// if they're not already set in the slice's env map.
func injectDatabaseEnv(c *corev1.Container, s plan.Slice) {
	has := func(key string) bool {
		for _, e := range c.Env {
			if e.Name == key {
				return true
			}
		}
		return false
	}
	add := func(key, val string) {
		if !has(key) {
			c.Env = append(c.Env, corev1.EnvVar{Name: key, Value: val})
		}
	}

	img := s.Image
	switch {
	case strings.Contains(img, "postgres"):
		add("POSTGRES_PASSWORD", "mozza")
		add("POSTGRES_DB", s.Name)
	case strings.Contains(img, "mysql") || strings.Contains(img, "mariadb"):
		add("MYSQL_ROOT_PASSWORD", "mozza")
		add("MYSQL_DATABASE", s.Name)
	case strings.Contains(img, "mongo"):
		add("MONGO_INITDB_ROOT_USERNAME", "mozza")
		add("MONGO_INITDB_ROOT_PASSWORD", "mozza")
	}
}
