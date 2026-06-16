package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/deploy"
	k8sdeploy "github.com/gshepptech/mozza/internal/deploy/k8s"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
)

type deployResponse struct {
	DeployID string `json:"deploy_id"`
	Status   string `json:"status"`
	AppName  string `json:"app_name"`
}

type deployStatusResponse struct {
	DeployID string `json:"deploy_id"`
	Status   string `json:"status"`
	AppName  string `json:"app_name"`
	Duration int64  `json:"duration_ms"`
	Error    string `json:"error,omitempty"`
}

// handleDeploy accepts raw recipe text and starts an async deploy.
// POST /api/v1/deploy
// Content-Type: text/plain
// Returns 202 with deploy_id.
func (s *Server) handleDeploy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			Error(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		recipeContent := string(body)
		if recipeContent == "" {
			Error(w, http.StatusBadRequest, "recipe content is required")
			return
		}

		// Parse recipe.
		rec, err := recipe.NewParser(recipeContent).Parse()
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid recipe: "+err.Error())
			return
		}

		// Build and validate plan.
		p, err := plan.Build(rec)
		if err != nil {
			Error(w, http.StatusBadRequest, "plan build failed: "+err.Error())
			return
		}
		if err := plan.Validate(p); err != nil {
			Error(w, http.StatusBadRequest, "validation failed: "+err.Error())
			return
		}

		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		deployer := k8sdeploy.New(s.cfg.Store)

		// The deployer creates its own deploy record internally.
		// Start deploy async with background context (request context cancels on return).
		go func() {
			result, err := deployer.Deploy(context.Background(), p, deploy.DeployOptions{
				RecipeContent: recipeContent,
				TriggeredBy:   "ui",
			})
			if err != nil {
				slog.Error("async deploy failed", "app", p.Name, "error", err)
				return
			}
			slog.Info("async deploy complete", "deploy_id", result.DeployID, "status", result.Status)
		}()

		JSON(w, http.StatusAccepted, deployResponse{
			DeployID: "",
			Status:   "in_progress",
			AppName:  p.Name,
		})
	}
}

// handleDeployStatus returns the current status of a deploy.
// GET /api/v1/deploys/{id}
func (s *Server) handleDeployStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		rec, err := s.cfg.Store.DeployByID(id)
		if err != nil {
			Error(w, http.StatusNotFound, "deploy not found")
			return
		}

		JSON(w, http.StatusOK, deployStatusResponse{
			DeployID: rec.ID,
			Status:   rec.Status,
			AppName:  rec.AppName,
			Duration: rec.DurationMs,
			Error:    rec.ErrorMessage,
		})
	}
}
