package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gshepptech/mozza/internal/importer"
)

// composeParseRequest is the request body for POST /api/v1/import/compose.
type composeParseRequest struct {
	Content string `json:"content"`
}

// composeServiceInfo represents a detected service for the UI breakdown.
type composeServiceInfo struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Engine  string            `json:"engine,omitempty"`
	Version string            `json:"version,omitempty"`
	Port    int               `json:"port,omitempty"`
	Public  bool              `json:"public,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// composeParseResponse is returned by POST /api/v1/import/compose.
type composeParseResponse struct {
	Services []composeServiceInfo `json:"services"`
	Recipe   string               `json:"recipe"`
	Warnings []importer.Warning   `json:"warnings"`
}

// composeDeployRequest is the request body for POST /api/v1/import/compose/deploy.
type composeDeployRequest struct {
	Content     string            `json:"content"`
	Adjustments map[string]string `json:"adjustments"`
	TeamID      string            `json:"team_id"`
	Target      string            `json:"target"`
	ClusterID   string            `json:"cluster_id,omitempty"`
}

// handleComposeImport parses a Docker Compose file and returns services + recipe.
// POST /api/v1/import/compose
func (s *Server) handleComposeImport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req composeParseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Content == "" {
			Error(w, http.StatusBadRequest, "content is required")
			return
		}
		if len(req.Content) > 1<<20 {
			Error(w, http.StatusBadRequest, "compose file too large (max 1MB)")
			return
		}

		ast, warnings, err := importer.ComposeToRecipeAST([]byte(req.Content))
		if err != nil {
			Error(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("failed to parse compose file: %v", err))
			return
		}

		recipeText, err := importer.ComposeToRecipe(req.Content)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("failed to generate recipe: %v", err))
			return
		}

		services := make([]composeServiceInfo, 0, len(ast.Slices))
		for _, slice := range ast.Slices {
			svc := composeServiceInfo{
				Name:    slice.Name,
				Image:   slice.Image,
				Engine:  slice.Engine,
				Version: slice.Version,
				Port:    slice.Port,
				Public:  slice.Public,
				Env:     slice.Env,
			}
			services = append(services, svc)
		}

		JSON(w, http.StatusOK, composeParseResponse{
			Services: services,
			Recipe:   recipeText,
			Warnings: warnings,
		})
	}
}

// handleComposeImportDeploy parses compose content and deploys the result.
// POST /api/v1/import/compose/deploy
func (s *Server) handleComposeImportDeploy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())

		var req composeDeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Content == "" || req.TeamID == "" || req.Target == "" {
			Error(w, http.StatusBadRequest,
				"content, team_id, and target are required")
			return
		}
		if req.Target != "kubernetes" && req.Target != "local" {
			Error(w, http.StatusBadRequest,
				"target must be 'kubernetes' or 'local'")
			return
		}

		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		recipeText, err := importer.ComposeToRecipe(req.Content)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity,
				fmt.Sprintf("failed to parse compose file: %v", err))
			return
		}

		s.deployWithSource(w, recipeText, "compose-import",
			req.TeamID, req.Target, req.ClusterID, user.ID)
	}
}
