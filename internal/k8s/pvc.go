package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildPVC generates a typed Kubernetes PersistentVolumeClaim for a slice
// that requires persistent storage. Returns nil if no storage is configured.
func BuildPVC(s plan.Slice, namespace string, appName string) *corev1.PersistentVolumeClaim {
	var storageSize string

	if s.Database != nil && s.Database.Storage != "" {
		storageSize = s.Database.Storage
	} else if s.Cache != nil && s.Cache.Storage != "" {
		storageSize = s.Cache.Storage
	}

	if storageSize == "" {
		return nil
	}

	lbls := labels(appName, s.Name)

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}
}
