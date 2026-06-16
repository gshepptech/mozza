package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/recipe"
	"github.com/gshepptech/mozza/internal/store"
)

type createRecipeRequest struct {
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Canvas string `json:"canvas"`
}

type updateRecipeRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Canvas string `json:"canvas"`
}

type recipeResponse struct {
	ID        string `json:"id"`
	TeamID    string `json:"team_id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Canvas    string `json:"canvas"`
	CreatedBy string `json:"created_by"`
}

type recipesListResponse struct {
	Recipes []recipeResponse `json:"recipes"`
}

func toRecipeResponse(r *store.Recipe) recipeResponse {
	return recipeResponse{
		ID: r.ID, TeamID: r.TeamID, Name: r.Name,
		Source: r.Source, Canvas: r.Canvas, CreatedBy: r.CreatedBy,
	}
}

// handleCreateRecipe creates a new recipe.
func (s *Server) handleCreateRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		var req createRecipeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.TeamID == "" {
			Error(w, http.StatusBadRequest, "name and team_id are required")
			return
		}
		if req.Canvas == "" {
			req.Canvas = "{}"
		}

		// Verify team membership.
		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		recipe, err := s.cfg.Store.CreateRecipe(req.TeamID, req.Name, req.Source, req.Canvas, user.ID)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				Error(w, http.StatusConflict, "recipe name already exists in team")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to create recipe")
			return
		}

		JSON(w, http.StatusCreated, toRecipeResponse(recipe))
	}
}

// handleGetRecipe returns a recipe by ID.
func (s *Server) handleGetRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")
		recipe, err := s.cfg.Store.RecipeByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get recipe")
			return
		}
		isMember, err := s.isTeamMember(recipe.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}
		JSON(w, http.StatusOK, toRecipeResponse(recipe))
	}
}

// handleListRecipes returns all recipes for a team.
func (s *Server) handleListRecipes() http.HandlerFunc {
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

		recipes, err := s.cfg.Store.RecipesForTeam(teamID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list recipes")
			return
		}

		resp := recipesListResponse{Recipes: make([]recipeResponse, len(recipes))}
		for i, rec := range recipes {
			resp.Recipes[i] = toRecipeResponse(&rec)
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleUpdateRecipe updates a recipe.
func (s *Server) handleUpdateRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")

		// Verify team membership before allowing mutation.
		existing, err := s.cfg.Store.RecipeByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get recipe")
			return
		}
		isMember, err := s.isTeamMember(existing.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		var req updateRecipeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := s.cfg.Store.UpdateRecipe(id, req.Name, req.Source, req.Canvas); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to update recipe")
			return
		}

		recipe, _ := s.cfg.Store.RecipeByID(id)
		JSON(w, http.StatusOK, toRecipeResponse(recipe))
	}
}

// handleDeleteRecipe deletes a recipe.
func (s *Server) handleDeleteRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		id := chi.URLParam(r, "id")

		// Verify team membership before allowing deletion.
		existing, err := s.cfg.Store.RecipeByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get recipe")
			return
		}
		isMember, err := s.isTeamMember(existing.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		if err := s.cfg.Store.DeleteRecipe(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "recipe not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to delete recipe")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// handleValidateRecipe validates recipe source text.
func (s *Server) handleValidateRecipe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Source string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Use the existing recipe parser to validate.
		errs := s.validateRecipeSource(req.Source)
		if len(errs) > 0 {
			JSON(w, http.StatusOK, map[string]any{"valid": false, "errors": errs})
			return
		}
		JSON(w, http.StatusOK, map[string]any{"valid": true, "errors": []string{}})
	}
}

// validateRecipeSource parses recipe source and returns any errors.
func (s *Server) validateRecipeSource(source string) []string {
	if strings.TrimSpace(source) == "" {
		return []string{"recipe source is empty"}
	}
	p := recipe.NewParser(source)
	if _, err := p.Parse(); err != nil {
		return []string{err.Error()}
	}
	return nil
}
