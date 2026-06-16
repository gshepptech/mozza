package k8s

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gshepptech/mozza/internal/plan"
)

// BuildCronJob generates a typed Kubernetes CronJob for the given scheduled slice.
// The schedule is taken from the slice's Schedule field. The concurrency policy
// defaults to Forbid to prevent overlapping runs.
func BuildCronJob(s plan.Slice, namespace string, appName string) *batchv1.CronJob {
	lbls := labels(appName, s.Name)

	podSpec := buildFullPodSpec(s, appName)
	podSpec.RestartPolicy = corev1.RestartPolicyNever

	backoffLimit := int32(s.Retries)
	if backoffLimit < 0 {
		backoffLimit = 6
	}

	successHistory := int32(3)
	failedHistory := int32(1)

	return &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "CronJob",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   s.Schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: &successHistory,
			FailedJobsHistoryLimit:     &failedHistory,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lbls,
				},
				Spec: batchv1.JobSpec{
					BackoffLimit: &backoffLimit,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: lbls,
						},
						Spec: podSpec,
					},
				},
			},
		},
	}
}
