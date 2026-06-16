package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/marketplace"
	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/template"
)

type deployMarketplaceRequest struct {
	TeamID    string            `json:"team_id"`
	Target    string            `json:"target"`
	ClusterID string            `json:"cluster_id"`
	Variables map[string]string `json:"variables"`
}

// handleListMarketplaceRecipes returns marketplace recipes with search and pagination.
// GET /api/v1/marketplace/recipes?query=word&category=cms&tags=blog,php&page=1&per_page=20
func (s *Server) handleListMarketplaceRecipes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Marketplace == nil {
			Error(w, http.StatusServiceUnavailable, "marketplace not configured")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

		var tags []string
		if t := r.URL.Query().Get("tags"); t != "" {
			tags = strings.Split(t, ",")
		}

		params := marketplace.ListParams{
			Query:    r.URL.Query().Get("query"),
			Category: r.URL.Query().Get("category"),
			Tags:     tags,
			Page:     page,
			PerPage:  perPage,
		}

		result, err := s.cfg.Marketplace.Search(r.Context(), params)
		if err != nil {
			Error(w, http.StatusInternalServerError, "search failed")
			return
		}

		JSON(w, http.StatusOK, result)
	}
}

// handleGetMarketplaceRecipe returns a single marketplace recipe by name.
// GET /api/v1/marketplace/recipes/{name}
func (s *Server) handleGetMarketplaceRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Marketplace == nil {
			Error(w, http.StatusServiceUnavailable, "marketplace not configured")
			return
		}

		name := chi.URLParam(r, "name")
		tmpl, err := s.cfg.Marketplace.Get(r.Context(), name)
		if err != nil {
			Error(w, http.StatusNotFound, "recipe not found")
			return
		}

		JSON(w, http.StatusOK, tmpl)
	}
}

// handleDeployMarketplaceRecipe deploys a marketplace recipe.
// POST /api/v1/marketplace/recipes/{name}/deploy
func (s *Server) handleDeployMarketplaceRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Marketplace == nil {
			Error(w, http.StatusServiceUnavailable, "marketplace not configured")
			return
		}

		user := UserFromContext(r.Context())
		name := chi.URLParam(r, "name")

		tmpl, err := s.cfg.Marketplace.Get(r.Context(), name)
		if err != nil {
			Error(w, http.StatusNotFound, "recipe not found")
			return
		}

		var req deployMarketplaceRequest
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

		s.deployWithSource(w, rendered, tmpl.Name, req.TeamID, req.Target, req.ClusterID, user.ID)
	}
}
