package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/template"
)

type templateListResponse struct {
	Templates []template.Template `json:"templates"`
}

type deployTemplateRequest struct {
	TeamID    string            `json:"team_id"`
	Target    string            `json:"target"`
	ClusterID string            `json:"cluster_id"`
	Namespace string            `json:"namespace"`
	Variables map[string]string `json:"variables"`
}

// handleListTemplates returns all templates, optionally filtered by category.
// GET /api/v1/templates?category=cms
func (s *Server) handleListTemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Templates == nil {
			Error(w, http.StatusServiceUnavailable, "templates not configured")
			return
		}

		category := r.URL.Query().Get("category")
		templates := s.cfg.Templates.List(category)

		JSON(w, http.StatusOK, templateListResponse{Templates: templates})
	}
}

// handleGetTemplate returns a single template by ID.
// GET /api/v1/templates/{id}
func (s *Server) handleGetTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Templates == nil {
			Error(w, http.StatusServiceUnavailable, "templates not configured")
			return
		}

		id := chi.URLParam(r, "id")
		tmpl, err := s.cfg.Templates.Get(id)
		if err != nil {
			Error(w, http.StatusNotFound, "template not found")
			return
		}

		JSON(w, http.StatusOK, tmpl)
	}
}

// handleDeployTemplate deploys a template with variable values.
// POST /api/v1/templates/{id}/deploy
func (s *Server) handleDeployTemplate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Templates == nil {
			Error(w, http.StatusServiceUnavailable, "templates not configured")
			return
		}

		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")

		tmpl, err := s.cfg.Templates.Get(id)
		if err != nil {
			Error(w, http.StatusNotFound, "template not found")
			return
		}

		var req deployTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.TeamID == "" || req.Target == "" {
			Error(w, http.StatusBadRequest, "team_id and target are required")
			return
		}
		if req.Target != "kubernetes" && req.Target != "local" {
			Error(w, http.StatusBadRequest, "target must be 'kubernetes' or 'local'")
			return
		}

		// Verify team membership.
		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		// Render the template with user-provided variables.
		rendered, err := template.RenderRecipe(*tmpl, req.Variables)
		if err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf("variable error: %v", err))
			return
		}

		// Validate the rendered recipe parses correctly.
		parser := recipe.NewParser(rendered)
		if _, err := parser.Parse(); err != nil {
			Error(w, http.StatusBadRequest, fmt.Sprintf("rendered recipe is invalid: %v", err))
			return
		}

		// Save the rendered recipe as a new recipe record.
		rec, err := s.cfg.Store.CreateRecipe(req.TeamID, tmpl.Name, rendered, "", user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to save recipe")
			return
		}

		// Validate cluster if provided.
		if req.ClusterID != "" {
			if _, err := s.cfg.Store.ClusterByID(req.ClusterID); err != nil {
				Error(w, http.StatusNotFound, "cluster not found")
				return
			}
		}

		env := "production"
		dep, err := s.cfg.Store.CreateDeployment(rec.ID, req.TeamID, req.Target, env, user.ID, req.ClusterID)
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

		// Launch async deployment.
		go s.runDeployWith(deployer, dep.ID, rendered, req.Target, env)

		JSON(w, http.StatusAccepted, toDeploymentResponse(dep))
	}
}

// deployWithSource creates a recipe record, a deployment record, and launches
// the async deploy pipeline. It is shared by template deploy and import deploy.
func (s *Server) deployWithSource(
	w http.ResponseWriter,
	source, name, teamID, target, clusterID, userID string,
) {
	// Validate the source parses correctly.
	parser := recipe.NewParser(source)
	parsedRec, err := parser.Parse()
	if err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("recipe parse failed: %v", err))
		return
	}

	// Build plan to validate.
	if _, err := plan.Build(parsedRec); err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("plan build failed: %v", err))
		return
	}

	// Save as a new recipe record.
	rec, err := s.cfg.Store.CreateRecipe(teamID, name, source, "", userID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to save recipe")
		return
	}

	// Validate cluster if provided.
	if clusterID != "" {
		if _, err := s.cfg.Store.ClusterByID(clusterID); err != nil {
			Error(w, http.StatusNotFound, "cluster not found")
			return
		}
	}

	env := "production"
	dep, err := s.cfg.Store.CreateDeployment(rec.ID, teamID, target, env, userID, clusterID)
	if err != nil {
		Error(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	// Select deployer based on target.
	deployer := s.cfg.Deployer
	if target == "local" {
		deployer = s.cfg.LocalDeployer
	}
	if deployer == nil {
		msg := fmt.Sprintf("No deployer configured for target %q.\n", target)
		_ = s.cfg.Store.UpdateDeploymentStatus(dep.ID, "failed", msg, true)
		dep.Status = "failed"
		dep.Log = msg
		JSON(w, http.StatusAccepted, toDeploymentResponse(dep))
		return
	}

	// Launch async deployment.
	go s.runDeployWith(deployer, dep.ID, source, target, env)

	JSON(w, http.StatusAccepted, toDeploymentResponse(dep))
}
