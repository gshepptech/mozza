package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildDaemonSet generates a typed Kubernetes DaemonSet for the given daemon slice.
// DaemonSets run one pod per node and have no replicas field. The update strategy
// is RollingUpdate. Node selection is applied from Scheduling.NodeRequirements.
func BuildDaemonSet(s plan.Slice, namespace string, appName string) *appsv1.DaemonSet {
	lbls := labels(appName, s.Name)

	podSpec := buildFullPodSpec(s, appName)

	// For DaemonSets, also apply a simple nodeSelector for backward compatibility.
	if s.Scheduling != nil && len(s.Scheduling.NodeRequirements) > 0 {
		nodeSelector := make(map[string]string, len(s.Scheduling.NodeRequirements))
		for _, req := range s.Scheduling.NodeRequirements {
			nodeSelector[req.Key] = req.Value
		}
		podSpec.NodeSelector = nodeSelector
	}

	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: podSpec,
			},
		},
	}
}
