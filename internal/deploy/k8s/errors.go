package k8s

import (
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// HumanError translates a K8s API error into a human-friendly message.
// If the error is not a recognized K8s error, it returns the original message.
func HumanError(err error, context string) string {
	if err == nil {
		return ""
	}

	// Check for K8s API status errors.
	if ok := apierrors.IsNotFound(err); ok {
		return fmt.Sprintf("Resource not found: %s. Check that it exists in the cluster.", context)
	}

	if apierrors.IsForbidden(err) {
		return fmt.Sprintf("Mozza doesn't have permission to %s. Run: mozza doctor", context)
	}

	if apierrors.IsConflict(err) {
		return fmt.Sprintf("Conflict while updating %s. Another tool may have modified it. Try again.", context)
	}

	if apierrors.IsAlreadyExists(err) {
		return fmt.Sprintf("Resource %s already exists. Mozza will update it.", context)
	}

	if apierrors.IsServiceUnavailable(err) {
		return fmt.Sprintf("Kubernetes API is unavailable. Check your cluster connection.")
	}

	if apierrors.IsUnauthorized(err) {
		return "Not authorized to access the Kubernetes cluster. Check your kubeconfig credentials."
	}

	if apierrors.IsInvalid(err) {
		var statusErr *apierrors.StatusError
		if errors.As(err, &statusErr) {
			return fmt.Sprintf("Invalid resource configuration for %s: %s", context, statusErr.Status().Message)
		}
		return fmt.Sprintf("Invalid resource configuration for %s. Check your recipe.", context)
	}

	// Check for common pod status strings in error messages.
	msg := err.Error()
	return translatePodError(msg, context)
}

// translatePodError maps common K8s pod error patterns to human-friendly messages.
func translatePodError(msg, context string) string {
	switch {
	case strings.Contains(msg, "ImagePullBackOff") || strings.Contains(msg, "ErrImagePull"):
		return fmt.Sprintf("Your app %q failed to start: the container image could not be found. Check that the image exists in your registry.", context)

	case strings.Contains(msg, "CrashLoopBackOff"):
		return fmt.Sprintf("Your app %q keeps crashing on startup. Check your app's logs with: mozza logs %s", context, context)

	case strings.Contains(msg, "OOMKilled"):
		return fmt.Sprintf("Your app %q ran out of memory. Increase the memory limit in your recipe.", context)

	case strings.Contains(msg, "CreateContainerConfigError"):
		return fmt.Sprintf("Your app %q has a configuration error (missing secret or config). Check your recipe's secret references.", context)

	case strings.Contains(msg, "Insufficient cpu") || strings.Contains(msg, "Insufficient memory"):
		return fmt.Sprintf("Not enough cluster resources to schedule %q. Try reducing resource limits or adding nodes.", context)

	case strings.Contains(msg, "context deadline exceeded"):
		return fmt.Sprintf("Operation timed out for %q. The cluster may be under heavy load. Try increasing --timeout.", context)

	default:
		return msg
	}
}
