package k8s

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildConfigMaps generates ConfigMap objects for MountSpec entries with
// Type "file", "config-dir", or "configmap". Returns nil if no matching mounts exist.
func BuildConfigMaps(appName string, s plan.Slice, namespace string) []*corev1.ConfigMap {
	seen := make(map[string]bool)
	var cms []*corev1.ConfigMap

	for _, m := range s.Mounts {
		t := strings.ToLower(m.Type)
		if t != "file" && t != "config-dir" && t != "configmap" {
			continue
		}
		if seen[m.Source] {
			continue
		}
		seen[m.Source] = true

		cms = append(cms, &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      m.Source,
				Namespace: namespace,
				Labels:    labels(appName, s.Name),
			},
			Data: map[string]string{
				m.Source: "",
			},
		})
	}

	return cms
}
