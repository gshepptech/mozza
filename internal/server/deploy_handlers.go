package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeployer "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

type deployPreviewRequest struct {
	Source string `json:"source"`
}

type deployPreviewSlice struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Image    string `json:"image"`
	Port     int    `json:"port"`
	Replicas int    `json:"replicas"`
	Public   bool   `json:"public"`
}

type deployPreviewSummary struct {
	Services  int `json:"services"`
	Databases int `json:"databases"`
	Caches    int `json:"caches"`
	Workers   int `json:"workers"`
	Total     int `json:"total"`
}

type deployPreviewResponse struct {
	Name      string               `json:"name"`
	Slices    []deployPreviewSlice `json:"slices"`
	Summary   deployPreviewSummary `json:"summary"`
	Manifests string               `json:"manifests,omitempty"`
}

type triggerDeployRequest struct {
	RecipeID    string `json:"recipe_id"`
	TeamID      string `json:"team_id"`
	Target      string `json:"target"`
	Environment string `json:"environment"`
	ClusterID   string `json:"cluster_id"`
}

type deploymentResponse struct {
	ID           string  `json:"id"`
	RecipeID     string  `json:"recipe_id"`
	TeamID       string  `json:"team_id"`
	Target       string  `json:"target"`
	Environment  string  `json:"environment"`
	Status       string  `json:"status"`
	StartedBy    string  `json:"started_by"`
	Log          string  `json:"log"`
	OrderNumber  int     `json:"order_number"`
	Version      int     `json:"version"`
	HasPrevState bool    `json:"has_previous_state"`
	AccessURL    string  `json:"access_url,omitempty"`
	StartedAt    string  `json:"started_at"`
	FinishedAt   *string `json:"finished_at,omitempty"`
}

type deploymentsListResponse struct {
	Deployments []deploymentResponse `json:"deployments"`
}

func toDeploymentResponse(d *store.Deployment) deploymentResponse {
	resp := deploymentResponse{
		ID:           d.ID,
		RecipeID:     d.RecipeID,
		TeamID:       d.TeamID,
		Target:       d.Target,
		Environment:  d.Environment,
		Status:       d.Status,
		StartedBy:    d.StartedBy,
		Log:          d.Log,
		OrderNumber:  d.OrderNumber,
		Version:      d.Version,
		HasPrevState: d.PreviousState != "",
		AccessURL:    d.AccessURL,
		StartedAt:    d.StartedAt.Format("2006-01-02T15:04:05Z"),
	}
	if d.FinishedAt != nil {
		t := d.FinishedAt.Format("2006-01-02T15:04:05Z")
		resp.FinishedAt = &t
	}
	return resp
}

// handleDeployPreview parses a recipe and returns the plan summary without deploying.
func (s *Server) handleDeployPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req deployPreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Source == "" {
			Error(w, http.StatusBadRequest, "source is required")
			return
		}

		parser := recipe.NewParser(req.Source)
		rec, err := parser.Parse()
		if err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf("recipe parse failed: %v", err))
			return
		}

		p, err := plan.Build(rec)
		if err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf("plan build failed: %v", err))
			return
		}

		if err := plan.Validate(p); err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf("plan validation failed: %v", err))
			return
		}

		slices := make([]deployPreviewSlice, len(p.Slices))
		var summary deployPreviewSummary
		for i, sl := range p.Slices {
			slices[i] = deployPreviewSlice{
				Name:     sl.Name,
				Kind:     string(sl.Kind),
				Image:    sl.Image,
				Port:     sl.Port,
				Replicas: sl.Replicas,
				Public:   sl.Public,
			}
			switch sl.Kind {
			case plan.SliceKindDatabase:
				summary.Databases++
			case plan.SliceKindCache:
				summary.Caches++
			case plan.SliceKindWorker:
				summary.Workers++
			default:
				summary.Services++
			}
		}
		summary.Total = len(p.Slices)

		// Compile to K8s manifests for preview.
		var manifests string
		compiler := k8s.New()
		result, compileErr := compiler.Compile(r.Context(), p)
		if compileErr == nil {
			manifests = compileResultToYAML(result)
		}

		JSON(w, http.StatusOK, deployPreviewResponse{
			Name:      p.Name,
			Slices:    slices,
			Summary:   summary,
			Manifests: manifests,
		})
	}
}

// compileResultToYAML concatenates all compiled output files into a single
// YAML string separated by document markers.
func compileResultToYAML(result *compile.Result) string {
	if result == nil || len(result.Files) == 0 {
		return ""
	}
	var buf strings.Builder
	for i, f := range result.Files {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(f.Content)
	}
	return buf.String()
}

// handleTriggerDeploy starts a new deployment.
func (s *Server) handleTriggerDeploy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		var req triggerDeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.RecipeID == "" || req.TeamID == "" || req.Target == "" {
			Error(w, http.StatusBadRequest, "recipe_id, team_id, and target are required")
			return
		}
		if req.Target != "kubernetes" && req.Target != "local" {
			Error(w, http.StatusBadRequest, "target must be 'kubernetes' or 'local'")
			return
		}
		if req.Environment == "" {
			req.Environment = "production"
		}

		// Verify team membership.
		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		// Verify recipe exists and fetch source for deploy.
		rec, err := s.cfg.Store.RecipeByID(req.RecipeID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get recipe")
			return
		}

		// Validate cluster_id if provided for K8s deployments.
		if req.ClusterID != "" {
			_, err := s.cfg.Store.ClusterByID(req.ClusterID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					Error(w, http.StatusNotFound, "cluster not found")
					return
				}
				Error(w, http.StatusInternalServerError, "failed to validate cluster")
				return
			}
		}

		dep, err := s.cfg.Store.CreateDeployment(req.RecipeID, req.TeamID, req.Target, req.Environment, user.ID, req.ClusterID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to create deployment")
			return
		}

		// Select deployer based on target.
		deployer := s.cfg.Deployer
		if req.Target == "local" {
			deployer = s.cfg.LocalDeployer
		}
		if deployer == nil {
			msg := fmt.Sprintf("No deployer configured for target %q.\n", req.Target)
			_ = s.cfg.Store.UpdateDeploymentStatus(dep.ID, "failed", msg, true)
			dep.Status = "failed"
			dep.Log = msg
			JSON(w, http.StatusAccepted, toDeploymentResponse(dep))
			return
		}

		// Launch async deployment; return 202 immediately.
		go s.runDeployWith(deployer, dep.ID, rec.Source, req.Target, req.Environment)

		JSON(w, http.StatusAccepted, toDeploymentResponse(dep))
	}
}

// runDeployWith executes the deployment pipeline asynchronously using the given
// deployer. It parses the recipe source, builds and validates a plan, then
// calls the deployer. Progress is streamed via AppendDeploymentLog; final
// status is set on completion.
func (s *Server) runDeployWith(deployer deploy.Deployer, deployID, source, target, env string) {
	ctx := context.Background()
	start := time.Now()

	logMsg := func(msg string) {
		if err := s.cfg.Store.AppendDeploymentLog(deployID, msg+"\n"); err != nil {
			slog.Error("failed to append deploy log", "deploy_id", deployID, "err", err)
		}
	}
	fail := func(msg string) {
		elapsed := time.Since(start).Truncate(time.Millisecond)
		logMsg(fmt.Sprintf("\u2717 Deploy failed: %s (after %s)", msg, elapsed))
		RecordDeployment(target, "failed")
		if err := s.cfg.Store.UpdateDeploymentStatus(deployID, "failed", "", true); err != nil {
			slog.Error("failed to update deploy status", "deploy_id", deployID, "err", err)
		}
	}

	logMsg("Step 1/5: Parsing recipe...")
	parser := recipe.NewParser(source)
	rec, err := parser.Parse()
	if err != nil {
		fail(fmt.Sprintf("recipe parse failed: %v", err))
		return
	}

	p, err := plan.Build(rec)
	if err != nil {
		fail(fmt.Sprintf("plan build failed: %v", err))
		return
	}
	logMsg(fmt.Sprintf("Step 2/5: Building plan (%d services)...", len(p.Slices)))

	logMsg("Step 3/5: Validating plan...")
	if err := plan.Validate(p); err != nil {
		fail(fmt.Sprintf("plan validation failed: %v", err))
		return
	}

	namespace := env
	if p.Namespace != "" {
		namespace = p.Namespace
	}

	// Snapshot current namespace state before applying (for rollback).
	if k8d, ok := deployer.(*k8sdeployer.Deployer); ok {
		snapshot, snapErr := k8d.SnapshotNamespace(ctx, nil, namespace)
		if snapErr != nil {
			slog.Warn("failed to snapshot namespace for rollback", "namespace", namespace, "err", snapErr)
		} else if snapshot != "" {
			if err := s.cfg.Store.SetDeploymentPreviousState(deployID, snapshot); err != nil {
				slog.Warn("failed to store previous state", "deploy_id", deployID, "err", err)
			}
		}
	}

	logMsg(fmt.Sprintf("Step 4/5: Applying to kubernetes (%s)...", namespace))
	opts := deploy.DeployOptions{
		RecipeContent: source,
		TriggeredBy:   "ui",
	}

	result, err := deployer.Deploy(ctx, p, opts)
	if err != nil {
		fail(fmt.Sprintf("deploy failed: %v", err))
		return
	}

	logMsg("Step 5/5: Waiting for pods to be healthy...")

	// If the deployer is a K8s deployer, run the health wait and detect access URL.
	status := "succeeded"
	if k8d, ok := deployer.(*k8sdeployer.Deployer); ok {
		if err := k8d.WaitForHealthy(ctx, namespace, logMsg); err != nil {
			slog.Warn("health wait timed out, marking partial", "deploy_id", deployID, "err", err)
			status = "partial"
		}

		// Detect and store the access URL.
		if accessURL := k8d.DetectAccessURL(ctx, namespace); accessURL != "" {
			if err := s.cfg.Store.SetDeploymentAccessURL(deployID, accessURL); err != nil {
				slog.Error("failed to set access URL", "deploy_id", deployID, "err", err)
			} else {
				logMsg(fmt.Sprintf("App accessible at: %s", accessURL))
			}
		}
	}

	if result.Status == deploy.StatusFailed {
		status = "failed"
	} else if result.Status == deploy.StatusRolledBack {
		status = "rolled_back"
	}

	elapsed := time.Since(start).Truncate(time.Millisecond)
	if status == "succeeded" {
		logMsg(fmt.Sprintf("\u2713 Deploy complete in %s.", elapsed))
	} else if status == "partial" {
		logMsg(fmt.Sprintf("\u2713 Deploy applied in %s (pods not yet healthy).", elapsed))
	} else {
		logMsg(fmt.Sprintf("\u2717 Deploy failed: status %s (after %s)", status, elapsed))
	}

	RecordDeployment(target, status)

	if err := s.cfg.Store.UpdateDeploymentStatus(deployID, status, "", true); err != nil {
		slog.Error("failed to update deploy status", "deploy_id", deployID, "err", err)
	}
}

// handleListDeployments returns deployments for a team.
func (s *Server) handleListDeployments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.URL.Query().Get("team_id")
		if teamID == "" {
			Error(w, http.StatusBadRequest, "team_id query parameter required")
			return
		}

		user := UserFromContext(r.Context())
		isMember, err := s.isTeamMember(teamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := 50
		if limitStr != "" {
			if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}

		deployments, err := s.cfg.Store.DeploymentsForTeam(teamID, limit)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list deployments")
			return
		}

		resp := deploymentsListResponse{Deployments: make([]deploymentResponse, len(deployments))}
		for i, d := range deployments {
			resp.Deployments[i] = toDeploymentResponse(&d)
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleGetDeployment returns a single deployment.
func (s *Server) handleGetDeployment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")
		dep, err := s.cfg.Store.DeploymentByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "deployment not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get deployment")
			return
		}
		isMember, err := s.isTeamMember(dep.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}
		JSON(w, http.StatusOK, toDeploymentResponse(dep))
	}
}

// handleDeleteDeployment removes a deployment record.
func (s *Server) handleDeleteDeployment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")
		dep, err := s.cfg.Store.DeploymentByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "deployment not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get deployment")
			return
		}
		isMember, err := s.isTeamMember(dep.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}
		if err := s.cfg.Store.DeleteDeployment(id); err != nil {
			Error(w, http.StatusInternalServerError, "failed to delete deployment")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// handleRollbackDeploy rolls back a deployment to its previous state.
func (s *Server) handleRollbackDeploy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")
		dep, err := s.cfg.Store.DeploymentByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "deployment not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get deployment")
			return
		}

		isMember, err := s.isTeamMember(dep.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		// Only allow rollback for finished deployments.
		if dep.Status != "succeeded" && dep.Status != "failed" && dep.Status != "partial" {
			Error(w, http.StatusBadRequest, "can only roll back finished deployments")
			return
		}

		// First deploy (version=1) has no previous state to restore.
		if dep.PreviousState == "" {
			Error(w, http.StatusBadRequest, "no previous version to roll back to")
			return
		}

		// Perform the rollback via the K8s deployer.
		k8d, ok := s.cfg.Deployer.(*k8sdeployer.Deployer)
		if !ok || k8d == nil {
			Error(w, http.StatusBadRequest, "rollback only supported for kubernetes deployments")
			return
		}

		if err := k8d.RollbackSnapshot(r.Context(), dep.PreviousState, dep.Environment); err != nil {
			slog.Error("rollback failed", "deployment_id", id, "error", err)
			Error(w, http.StatusInternalServerError, fmt.Sprintf("rollback failed: %v", err))
			return
		}

		// Update deployment status to rolled_back.
		if err := s.cfg.Store.UpdateDeploymentStatus(dep.ID, "rolled_back", "", true); err != nil {
			slog.Error("failed to update deployment status after rollback", "error", err)
		}

		// Re-fetch to return updated state.
		dep, _ = s.cfg.Store.DeploymentByID(id)
		JSON(w, http.StatusOK, toDeploymentResponse(dep))
	}
}

// handleDeploymentLogs streams deployment logs via SSE.
func (s *Server) handleDeploymentLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")
		deploy, err := s.cfg.Store.DeploymentByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "deployment not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get deployment")
			return
		}
		isMember, err := s.isTeamMember(deploy.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		// Set SSE headers.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			Error(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		// Send existing log as initial event.
		fmt.Fprintf(w, "data: %s\n\n", deploy.Log)
		flusher.Flush()

		// If deployment is finished, send done event and close.
		if deploy.FinishedAt != nil {
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", deploy.Status)
			flusher.Flush()
			return
		}

		// For running deployments, poll for log updates every 2 seconds.
		lastLen := len(deploy.Log)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				updated, err := s.cfg.Store.DeploymentByID(id)
				if err != nil {
					return
				}
				if len(updated.Log) > lastLen {
					newData := updated.Log[lastLen:]
					fmt.Fprintf(w, "data: %s\n\n", newData)
					flusher.Flush()
					lastLen = len(updated.Log)
				}
				if updated.FinishedAt != nil {
					fmt.Fprintf(w, "event: done\ndata: %s\n\n", updated.Status)
					flusher.Flush()
					return
				}
			}
		}
	}
}
