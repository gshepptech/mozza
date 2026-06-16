// Package local implements the deploy.Deployer interface using Docker Compose.
// It generates a docker-compose.yml from the app plan and executes it locally.
package local

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gshepptech/mozza/internal/deploy"
	localcompose "github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

// Deployer deploys applications locally via Docker Compose.
type Deployer struct {
	store      *store.Store
	projectDir string
}

// New creates a local Deployer that writes compose files to projectDir.
func New(s *store.Store, projectDir string) *Deployer {
	return &Deployer{store: s, projectDir: projectDir}
}

// Deploy generates a docker-compose.yml from the plan and runs docker compose up.
func (d *Deployer) Deploy(ctx context.Context, p *plan.AppPlan, opts deploy.DeployOptions) (*deploy.DeployResult, error) {
	start := time.Now()

	// Create a deploy record.
	dep, err := d.store.CreateDeploy(p.Name, "local", "", "", opts.RecipeContent, opts.TriggeredBy)
	if err != nil {
		return nil, fmt.Errorf("local deploy: create record: %w", err)
	}

	// Build and write docker-compose.yml.
	composeDir := filepath.Join(d.projectDir, p.Name)
	if err := os.MkdirAll(composeDir, 0o755); err != nil {
		d.failDeploy(dep.ID, start, err)
		return nil, fmt.Errorf("local deploy: mkdir: %w", err)
	}

	cf, err := localcompose.BuildComposeFile(p)
	if err != nil {
		d.failDeploy(dep.ID, start, err)
		return nil, fmt.Errorf("local deploy: build compose: %w", err)
	}

	yamlBytes, err := localcompose.MarshalComposeFile(cf)
	if err != nil {
		d.failDeploy(dep.ID, start, err)
		return nil, fmt.Errorf("local deploy: marshal compose: %w", err)
	}

	composePath := filepath.Join(composeDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, yamlBytes, 0o644); err != nil {
		d.failDeploy(dep.ID, start, err)
		return nil, fmt.Errorf("local deploy: write compose: %w", err)
	}

	// Run docker compose up.
	runner, err := localcompose.NewRunner(composeDir)
	if err != nil {
		d.failDeploy(dep.ID, start, err)
		return nil, fmt.Errorf("local deploy: %w", err)
	}

	var buf bytes.Buffer
	if err := runner.Up(ctx, &buf); err != nil {
		d.failDeploy(dep.ID, start, fmt.Errorf("%s: %w", buf.String(), err))
		return nil, fmt.Errorf("local deploy: compose up: %w", err)
	}

	duration := time.Since(start)
	_ = d.store.CompleteDeploy(dep.ID, "success", "", duration.Milliseconds())

	return &deploy.DeployResult{
		DeployID:  dep.ID,
		Resources: sliceResources(p.Slices),
		Duration:  duration,
		Status:    deploy.StatusSuccess,
	}, nil
}

// sliceResources converts plan slices into deployed resource entries.
func sliceResources(slices []plan.Slice) []deploy.DeployedResource {
	resources := make([]deploy.DeployedResource, 0, len(slices))
	for _, s := range slices {
		resources = append(resources, deploy.DeployedResource{
			Kind:   "Service",
			Name:   s.Name,
			Status: "applied",
		})
	}
	return resources
}

// Rollback restores the previous local deployment by re-deploying the stored recipe.
func (d *Deployer) Rollback(ctx context.Context, appName string) error {
	prev, err := d.store.PreviousSuccessfulDeploy(appName)
	if err != nil {
		return fmt.Errorf("local rollback: no previous deployment to roll back to: %w", err)
	}

	if prev.RecipeContent == "" {
		return fmt.Errorf("local rollback: previous deploy %s has no stored recipe content", prev.ID)
	}

	// Parse the stored recipe.
	r, err := recipe.NewParser(prev.RecipeContent).Parse()
	if err != nil {
		return fmt.Errorf("local rollback: parse stored recipe: %w", err)
	}

	p, err := plan.Build(r)
	if err != nil {
		return fmt.Errorf("local rollback: build plan: %w", err)
	}

	// Tear down current deployment first.
	composeDir := filepath.Join(d.projectDir, appName)
	runner, runnerErr := localcompose.NewRunner(composeDir)
	if runnerErr == nil {
		var buf bytes.Buffer
		if downErr := runner.Down(ctx, &buf); downErr != nil {
			slog.Warn("local rollback: down failed, continuing", "error", downErr)
		}
	}

	// Re-deploy with previous recipe.
	result, err := d.Deploy(ctx, p, deploy.DeployOptions{
		RecipeContent: prev.RecipeContent,
		TriggeredBy:   "rollback",
	})
	if err != nil {
		return fmt.Errorf("local rollback: redeploy: %w", err)
	}

	// Mark as rollback.
	current, _ := d.store.LatestSuccessfulDeploy(appName)
	if current != nil {
		_ = d.store.SetRollbackOf(result.DeployID, current.ID)
	}

	slog.Info("local rollback complete", "app", appName, "deploy_id", result.DeployID)
	return nil
}

// Status returns the status of locally deployed services.
func (d *Deployer) Status(_ context.Context, appName string) (*deploy.AppStatus, error) {
	return &deploy.AppStatus{
		AppName: appName,
		Context: "local",
	}, nil
}

// Logs streams logs from docker compose.
func (d *Deployer) Logs(ctx context.Context, appName string, _ deploy.LogOptions) (io.ReadCloser, error) {
	composeDir := filepath.Join(d.projectDir, appName)
	runner, err := localcompose.NewRunner(composeDir)
	if err != nil {
		return nil, fmt.Errorf("local logs: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		err := runner.Logs(ctx, pw, 100)
		pw.CloseWithError(err)
	}()
	return pr, nil
}

// Down tears down local compose services.
func (d *Deployer) Down(ctx context.Context, appName string, _ deploy.DownOptions) error {
	composeDir := filepath.Join(d.projectDir, appName)
	runner, err := localcompose.NewRunner(composeDir)
	if err != nil {
		return fmt.Errorf("local down: %w", err)
	}

	var buf bytes.Buffer
	if err := runner.Down(ctx, &buf); err != nil {
		return fmt.Errorf("local down: %w", err)
	}
	return nil
}

func (d *Deployer) failDeploy(deployID string, start time.Time, err error) {
	duration := time.Since(start)
	_ = d.store.CompleteDeploy(deployID, "failed", err.Error(), duration.Milliseconds())
}
