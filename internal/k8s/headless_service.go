package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildHeadlessService generates a typed Kubernetes headless Service (clusterIP: None)
// for StatefulSet slices. This enables stable DNS names for pod-to-pod discovery.
func BuildHeadlessService(s plan.Slice, namespace string, appName string) *corev1.Service {
	lbls := labels(appName, s.Name)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Selector:  lbls,
		},
	}

	// Add ports from Ports slice or single Port.
	svc.Spec.Ports = buildServicePorts(s)

	return svc
}
