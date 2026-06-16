package k8s

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildPDB generates a PodDisruptionBudget for the given slice.
// Returns nil if no DisruptionBudget is configured.
func BuildPDB(appName string, s plan.Slice, namespace string) *policyv1.PodDisruptionBudget {
	if s.DisruptionBudget == nil {
		return nil
	}

	lbls := labels(appName, s.Name)

	pdb := &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "policy/v1",
			Kind:       "PodDisruptionBudget",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: lbls,
			},
		},
	}

	if s.DisruptionBudget.MinAvailable > 0 {
		minAvail := intstr.FromInt32(int32(s.DisruptionBudget.MinAvailable))
		pdb.Spec.MinAvailable = &minAvail
	} else if s.DisruptionBudget.MaxUnavailable > 0 {
		maxUnavail := intstr.FromInt32(int32(s.DisruptionBudget.MaxUnavailable))
		pdb.Spec.MaxUnavailable = &maxUnavail
	}

	return pdb
}
