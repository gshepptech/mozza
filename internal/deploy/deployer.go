// Package deploy defines the Deployer interface for deploying applications
// to different targets (Kubernetes, local Docker, etc.).
package deploy

import (
	"context"
	"io"
	"time"

	"github.com/gshepptech/mozza/internal/plan"
)

// DeployStatus represents the outcome of a deploy operation.
type DeployStatus string

// Deploy status constants.
const (
	StatusInProgress DeployStatus = "in_progress"
	StatusSuccess    DeployStatus = "success"
	StatusFailed     DeployStatus = "failed"
	StatusRolledBack DeployStatus = "rolled_back"
	StatusPartial    DeployStatus = "partial"
)

// DeployOptions configures a deploy operation.
type DeployOptions struct {
	// Timeout is the maximum time to wait for readiness (default 5m).
	Timeout time.Duration
	// Context is the kubeconfig context override.
	Context string
	// Export writes manifests to disk without applying.
	Export bool
	// RecipeContent is the raw .mozza file content for audit trail storage.
	RecipeContent string
	// TriggeredBy indicates who triggered the deploy ("cli", "ui", or "rollback").
	TriggeredBy string
	// RollbackOf is the deploy ID being rolled back (set during rollback operations).
	RollbackOf string
}

// DeployResult holds the outcome of a deploy operation.
type DeployResult struct {
	// DeployID is the unique identifier for this deploy.
	DeployID string
	// Resources lists all resources created/updated during the deploy.
	Resources []DeployedResource
	// Duration is how long the deploy took.
	Duration time.Duration
	// Status is the final outcome.
	Status DeployStatus
}

// DeployedResource represents a single K8s resource created during a deploy.
type DeployedResource struct {
	// Kind is the resource type (Deployment, Service, etc.).
	Kind string
	// Name is the resource name.
	Name string
	// Namespace is the resource namespace.
	Namespace string
	// Status is the resource outcome (applied, failed, rolled-back).
	Status string
}

// SliceStatus represents the health of a deployed slice.
type SliceStatus struct {
	Name       string
	Status     string // running, degraded, down, pending
	Ready      int
	Desired    int
	Restarts   int
	Age        time.Duration
	Image      string
	Port       int
	Public     bool
	HealthPath string
}

// AppStatus represents the overall health of a deployed application.
type AppStatus struct {
	AppName   string
	Namespace string
	Context   string
	Slices    []SliceStatus
}

// LogOptions configures log streaming.
type LogOptions struct {
	// SliceName filters to a specific slice (empty = all slices).
	SliceName string
	// Follow enables live streaming (default true).
	Follow bool
	// Since shows logs since this duration ago.
	Since time.Duration
}

// DownOptions configures teardown behavior.
type DownOptions struct {
	// DeletePVCs controls whether PVCs are deleted.
	DeletePVCs bool
}

// Deployer defines the interface for deploying applications.
type Deployer interface {
	// Deploy applies the plan to the target environment.
	Deploy(ctx context.Context, p *plan.AppPlan, opts DeployOptions) (*DeployResult, error)

	// Rollback reverts an application to its previous successful deploy.
	Rollback(ctx context.Context, appName string) error

	// Status returns the current health of a deployed application.
	Status(ctx context.Context, appName string) (*AppStatus, error)

	// Logs streams pod logs for an application.
	Logs(ctx context.Context, appName string, opts LogOptions) (io.ReadCloser, error)

	// Down tears down all resources for an application.
	Down(ctx context.Context, appName string, opts DownOptions) error
}
