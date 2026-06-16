package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildServiceAccount generates a Kubernetes ServiceAccount for the given slice.
// Returns nil if the slice has no Permissions and no explicit ServiceAccount name.
func BuildServiceAccount(appName string, s plan.Slice, namespace string) *corev1.ServiceAccount {
	if len(s.Permissions) == 0 && s.ServiceAccount == "" {
		return nil
	}

	name := s.ServiceAccount
	if name == "" {
		name = s.Name
	}

	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels(appName, s.Name),
		},
	}
}
