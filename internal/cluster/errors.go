// Package cluster provides Kubernetes cluster management: health monitoring,
// resource caching, error classification, and graceful degradation.
package cluster

import (
	"errors"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Error codes returned in JSON error responses for frontend matching.
const (
	CodeUnreachable   = "CLUSTER_UNREACHABLE"
	CodeUnauthorized  = "CLUSTER_UNAUTHORIZED"
	CodeTimeout       = "CLUSTER_TIMEOUT"
	CodeInternalError = "INTERNAL_ERROR"
)

// ClassifiedError holds a K8s error mapped to an HTTP status and error code.
type ClassifiedError struct {
	Status  int    // HTTP status code
	Code    string // machine-readable error code
	Message string // human-readable message
}

// ClassifyError maps a K8s client error to an HTTP status code, error code,
// and human-readable message. Returns nil if err is nil.
func ClassifyError(err error) *ClassifiedError {
	if err == nil {
		return nil
	}

	if ce := classifyAPIStatusError(err); ce != nil {
		return ce
	}
	if ce := classifyConnectionError(err.Error()); ce != nil {
		return ce
	}

	return &ClassifiedError{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternalError,
		Message: "An unexpected error occurred while communicating with the cluster.",
	}
}

// classifyAPIStatusError checks for typed K8s API status errors.
func classifyAPIStatusError(err error) *ClassifiedError {
	var statusErr *apierrors.StatusError
	if !errors.As(err, &statusErr) {
		return nil
	}
	switch {
	case apierrors.IsUnauthorized(statusErr):
		return &ClassifiedError{
			Status:  http.StatusForbidden,
			Code:    CodeUnauthorized,
			Message: "Cluster authentication failed. Check your kubeconfig credentials.",
		}
	case apierrors.IsForbidden(statusErr):
		return &ClassifiedError{
			Status:  http.StatusForbidden,
			Code:    CodeUnauthorized,
			Message: "Insufficient permissions. Your service account may lack the required RBAC roles.",
		}
	case apierrors.IsNotFound(statusErr):
		return &ClassifiedError{
			Status:  http.StatusNotFound,
			Code:    CodeInternalError,
			Message: "Resource not found in the cluster.",
		}
	case apierrors.IsServerTimeout(statusErr), apierrors.IsTimeout(statusErr):
		return &ClassifiedError{
			Status:  http.StatusGatewayTimeout,
			Code:    CodeTimeout,
			Message: "Cluster API did not respond in time. The cluster may be overloaded.",
		}
	default:
		return nil
	}
}

// classifyConnectionError checks error message text for connection-level issues.
func classifyConnectionError(msg string) *ClassifiedError {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "no route to host"),
		strings.Contains(lower, "network is unreachable"):
		return &ClassifiedError{
			Status:  http.StatusServiceUnavailable,
			Code:    CodeUnreachable,
			Message: "Cannot reach the cluster. Check that it is running and the kubeconfig is valid.",
		}
	case strings.Contains(lower, "deadline exceeded"),
		strings.Contains(lower, "context deadline"),
		strings.Contains(lower, "timeout"):
		return &ClassifiedError{
			Status:  http.StatusGatewayTimeout,
			Code:    CodeTimeout,
			Message: "Cluster API timed out. The cluster may be overloaded or unreachable.",
		}
	case strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "forbidden"):
		return &ClassifiedError{
			Status:  http.StatusForbidden,
			Code:    CodeUnauthorized,
			Message: "Cluster credentials are invalid or expired.",
		}
	case strings.Contains(lower, "kubeconfig"),
		strings.Contains(lower, "kubeclient"):
		return &ClassifiedError{
			Status:  http.StatusServiceUnavailable,
			Code:    CodeUnreachable,
			Message: "No valid kubeconfig found. Register a cluster first.",
		}
	default:
		return nil
	}
}
