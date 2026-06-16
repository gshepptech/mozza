package k8s

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildHPA generates a HorizontalPodAutoscaler for the given slice.
// Returns nil if no AutoScale is configured.
func BuildHPA(appName string, s plan.Slice, namespace string) *autoscalingv2.HorizontalPodAutoscaler {
	if s.AutoScale == nil {
		return nil
	}

	// Determine the target workload kind.
	targetKind := "Deployment"
	if isStatefulKind(s.Kind) {
		targetKind = "StatefulSet"
	}

	minReplicas := int32(s.AutoScale.MinReplicas)
	if minReplicas < 1 {
		minReplicas = 1
	}
	maxReplicas := int32(s.AutoScale.MaxReplicas)
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "autoscaling/v2",
			Kind:       "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    labels(appName, s.Name),
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       targetKind,
				Name:       s.Name,
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
		},
	}

	var metrics []autoscalingv2.MetricSpec

	if s.AutoScale.CPUTarget > 0 {
		cpuTarget := int32(s.AutoScale.CPUTarget)
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &cpuTarget,
				},
			},
		})
	}

	if s.AutoScale.MemoryTarget > 0 {
		memTarget := int32(s.AutoScale.MemoryTarget)
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &memTarget,
				},
			},
		})
	}

	if len(metrics) > 0 {
		hpa.Spec.Metrics = metrics
	}

	return hpa
}
