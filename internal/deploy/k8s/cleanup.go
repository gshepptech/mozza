package k8s

import (
	"context"
	"fmt"
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

// DetectRemovedSlices compares the current recipe's slices against the previous
// deploy's slices and returns the names of slices that were removed.
func DetectRemovedSlices(s *store.Store, appName string, currentSlices []plan.Slice) ([]string, error) {
	prev, err := s.LatestSuccessfulDeploy(appName)
	if err != nil {
		// No previous deploy — nothing to clean up.
		return nil, nil
	}

	if prev.RecipeContent == "" {
		return nil, nil
	}

	// Parse the previous recipe to get its slice names.
	prevRecipe, err := recipe.NewParser(prev.RecipeContent).Parse()
	if err != nil {
		return nil, nil // Can't parse old recipe — skip cleanup.
	}

	prevNames := make(map[string]bool, len(prevRecipe.Slices))
	for _, s := range prevRecipe.Slices {
		prevNames[s.Name] = true
	}

	currentNames := make(map[string]bool, len(currentSlices))
	for _, s := range currentSlices {
		currentNames[s.Name] = true
	}

	var removed []string
	for name := range prevNames {
		if !currentNames[name] {
			removed = append(removed, name)
		}
	}

	return removed, nil
}

// CleanupRemovedSlices deletes K8s resources for slices that were removed from the recipe.
func CleanupRemovedSlices(ctx context.Context, cs kubernetes.Interface, namespace, appName string, sliceNames []string) {
	for _, name := range sliceNames {
		labelSelector := fmt.Sprintf("app.kubernetes.io/managed-by=mozza,app.kubernetes.io/name=%s,app.kubernetes.io/component=%s", appName, name)
		delOpts := metav1.DeleteOptions{}

		// Delete in reverse dependency order: Ingress → Service → Deployment → PVC.
		ingresses, err := cs.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil {
			for _, ing := range ingresses.Items {
				if err := cs.NetworkingV1().Ingresses(namespace).Delete(ctx, ing.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
					slog.Error("cleanup: delete ingress", "name", ing.Name, "error", err)
				}
			}
		}

		services, err := cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil {
			for _, svc := range services.Items {
				if err := cs.CoreV1().Services(namespace).Delete(ctx, svc.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
					slog.Error("cleanup: delete service", "name", svc.Name, "error", err)
				}
			}
		}

		deps, err := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err == nil {
			for _, dep := range deps.Items {
				if err := cs.AppsV1().Deployments(namespace).Delete(ctx, dep.Name, delOpts); err != nil && !apierrors.IsNotFound(err) {
					slog.Error("cleanup: delete deployment", "name", dep.Name, "error", err)
				}
			}
		}

		slog.Info("cleaned up removed slice", "slice", name, "namespace", namespace)
	}
}
