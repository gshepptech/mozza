package k8s

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gshepptech/mozza/internal/plan"
)

// waitForReady blocks until all Deployments in the plan are Available.
func waitForReady(ctx context.Context, cs kubernetes.Interface, namespace string, p *plan.AppPlan) error {
	// Collect deployment names.
	var deployNames []string
	for _, s := range p.Slices {
		deployNames = append(deployNames, s.Name)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("waitForReady: %w", ctx.Err())
		case <-ticker.C:
			allReady := true
			for _, name := range deployNames {
				dep, err := cs.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						allReady = false
						continue
					}
					return fmt.Errorf("waitForReady: get %q: %w", name, err)
				}

				if !isDeploymentAvailable(dep) {
					allReady = false
				}
			}

			if allReady {
				return nil
			}
		}
	}
}

// isDeploymentAvailable checks if a Deployment has the Available condition True.
func isDeploymentAvailable(dep *appsv1.Deployment) bool {
	for _, c := range dep.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// WaitForHealthy polls K8s workloads in the given namespace until all are
// ready or the 5-minute timeout expires. logFn receives progress messages.
func (d *Deployer) WaitForHealthy(ctx context.Context, namespace string, logFn func(string)) error {
	const healthTimeout = 5 * time.Minute
	const healthPoll = 5 * time.Second

	waitCtx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()

	// Connect to cluster using default context.
	clientset, _, err := newClientset("")
	if err != nil {
		return fmt.Errorf("WaitForHealthy: %w", err)
	}

	labelSelector := "app.kubernetes.io/managed-by=mozza"

	ticker := time.NewTicker(healthPoll)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("WaitForHealthy: timed out after %s waiting for pods in %q", healthTimeout, namespace)
		case <-ticker.C:
			ready, total, details, err := d.checkWorkloadsReady(waitCtx, clientset, namespace, labelSelector)
			if err != nil {
				return fmt.Errorf("WaitForHealthy: %w", err)
			}
			if logFn != nil {
				logFn(fmt.Sprintf("Waiting for pods... (%d/%d services ready)", ready, total))
			}
			if total > 0 && ready == total {
				return nil
			}
			_ = details // used for error context below
		}
	}
}

// checkWorkloadsReady counts ready vs total workloads in a namespace.
func (d *Deployer) checkWorkloadsReady(ctx context.Context, cs kubernetes.Interface, namespace, labelSelector string) (ready, total int, details []string, err error) {
	// Check Deployments.
	deps, err := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, dep := range deps.Items {
		total++
		if dep.Status.ReadyReplicas >= *dep.Spec.Replicas && *dep.Spec.Replicas > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("Deployment/%s: %d/%d ready", dep.Name, dep.Status.ReadyReplicas, *dep.Spec.Replicas))
		}
	}

	// Check StatefulSets.
	stss, err := cs.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list statefulsets: %w", err)
	}
	for _, sts := range stss.Items {
		total++
		if sts.Status.ReadyReplicas >= *sts.Spec.Replicas && *sts.Spec.Replicas > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("StatefulSet/%s: %d/%d ready", sts.Name, sts.Status.ReadyReplicas, *sts.Spec.Replicas))
		}
	}

	// Check DaemonSets.
	dss, err := cs.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return 0, 0, nil, fmt.Errorf("list daemonsets: %w", err)
	}
	for _, ds := range dss.Items {
		total++
		if ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
			ready++
		} else {
			details = append(details, fmt.Sprintf("DaemonSet/%s: %d/%d ready", ds.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled))
		}
	}

	return ready, total, details, nil
}
