package k8s

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildJob generates a typed Kubernetes Job for the given task slice.
// It configures backoff limit, parallelism, and completions from slice fields.
func BuildJob(s plan.Slice, namespace string, appName string) *batchv1.Job {
	lbls := labels(appName, s.Name)

	podSpec := buildFullPodSpec(s, appName)
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	backoffLimit := int32(s.Retries)
	if backoffLimit < 0 {
		backoffLimit = 6
	}

	parallelism := int32(s.Parallelism)
	if parallelism < 1 {
		parallelism = 1
	}

	completions := parallelism

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Parallelism:  &parallelism,
			Completions:  &completions,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: podSpec,
			},
		},
	}
}
