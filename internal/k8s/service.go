package k8s

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildService generates a typed Kubernetes ClusterIP Service for the given slice.
// When the slice has multiple named Ports, a multi-port Service is generated.
// When only the single Port field is set, backward-compatible single-port behavior applies.
func BuildService(s plan.Slice, namespace string, appName string) *corev1.Service {
	lbls := labels(appName, s.Name)

	return &corev1.Service{
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
			Type:     corev1.ServiceTypeClusterIP,
			Selector: lbls,
			Ports:    buildServicePorts(s),
		},
	}
}

// buildServicePorts generates ServicePort entries from the slice's Ports or Port field.
func buildServicePorts(s plan.Slice) []corev1.ServicePort {
	if len(s.Ports) > 0 {
		ports := make([]corev1.ServicePort, 0, len(s.Ports))
		for _, p := range s.Ports {
			sp := corev1.ServicePort{
				Name:       p.Name,
				Port:       int32(p.Port),
				TargetPort: intstr.FromInt32(int32(p.Port)),
				Protocol:   mapProtocol(p.Protocol),
			}
			ports = append(ports, sp)
		}
		return ports
	}

	// Backward compat: single port from Port field.
	if s.Port > 0 {
		return []corev1.ServicePort{
			{
				Port:       int32(s.Port),
				TargetPort: intstr.FromInt32(int32(s.Port)),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	}

	return nil
}

// mapProtocol converts a protocol string to the K8s Protocol type.
// Defaults to TCP for empty or unrecognized values.
func mapProtocol(proto string) corev1.Protocol {
	switch strings.ToLower(proto) {
	case "udp":
		return corev1.ProtocolUDP
	case "sctp":
		return corev1.ProtocolSCTP
	default:
		return corev1.ProtocolTCP
	}
}
