package deploy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gshepptech/mozza/internal/deploy"
	k8s "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/deploy/local"
)

// Compile-time interface compliance checks.
var (
	_ deploy.Deployer = (*k8s.Deployer)(nil)
	_ deploy.Deployer = (*local.Deployer)(nil)
)

func TestDeployStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status deploy.DeployStatus
		want   string
	}{
		{name: "in_progress", status: deploy.StatusInProgress, want: "in_progress"},
		{name: "success", status: deploy.StatusSuccess, want: "success"},
		{name: "failed", status: deploy.StatusFailed, want: "failed"},
		{name: "rolled_back", status: deploy.StatusRolledBack, want: "rolled_back"},
		{name: "partial", status: deploy.StatusPartial, want: "partial"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, deploy.DeployStatus(tt.want), tt.status)
		})
	}
}

func TestDeployResultFields(t *testing.T) {
	t.Parallel()

	result := deploy.DeployResult{
		DeployID: "deploy-123",
		Resources: []deploy.DeployedResource{
			{Kind: "Deployment", Name: "web", Namespace: "default", Status: "applied"},
			{Kind: "Service", Name: "web-svc", Namespace: "default", Status: "applied"},
		},
		Duration: 5 * time.Second,
		Status:   deploy.StatusSuccess,
	}

	assert.Equal(t, "deploy-123", result.DeployID)
	assert.Len(t, result.Resources, 2)
	assert.Equal(t, deploy.StatusSuccess, result.Status)
	assert.Equal(t, 5*time.Second, result.Duration)
}

func TestDeployedResourceFields(t *testing.T) {
	t.Parallel()

	r := deploy.DeployedResource{
		Kind:      "Deployment",
		Name:      "api",
		Namespace: "production",
		Status:    "applied",
	}

	assert.Equal(t, "Deployment", r.Kind)
	assert.Equal(t, "api", r.Name)
	assert.Equal(t, "production", r.Namespace)
	assert.Equal(t, "applied", r.Status)
}

func TestDeployOptionsFields(t *testing.T) {
	t.Parallel()

	opts := deploy.DeployOptions{
		Timeout:       30 * time.Second,
		Context:       "minikube",
		Export:        true,
		RecipeContent: "app: test",
		TriggeredBy:   "cli",
		RollbackOf:    "deploy-100",
	}

	assert.Equal(t, 30*time.Second, opts.Timeout)
	assert.Equal(t, "minikube", opts.Context)
	assert.True(t, opts.Export)
	assert.Equal(t, "app: test", opts.RecipeContent)
	assert.Equal(t, "cli", opts.TriggeredBy)
	assert.Equal(t, "deploy-100", opts.RollbackOf)
}
