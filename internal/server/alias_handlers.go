package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/store"
)

type createAliasRequest struct {
	TeamID   string `json:"team_id"`
	Name     string `json:"name"`
	ImageRef string `json:"image_ref"`
}

type aliasResponse struct {
	ID       string `json:"id"`
	TeamID   string `json:"team_id"`
	Name     string `json:"name"`
	ImageRef string `json:"image_ref"`
}

type aliasListResponse struct {
	Aliases []aliasResponse `json:"aliases"`
}

// handleCreateAlias creates a new image alias for a team.
func (s *Server) handleCreateAlias() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		var req createAliasRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.ImageRef == "" || req.TeamID == "" {
			Error(w, http.StatusBadRequest, "name, image_ref, and team_id are required")
			return
		}

		// Verify team membership.
		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		if err := s.cfg.Store.CreateAlias(req.TeamID, req.Name, req.ImageRef); err != nil {
			if errors.Is(err, store.ErrConflict) {
				Error(w, http.StatusConflict, "alias already exists for this team")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to create alias")
			return
		}

		JSON(w, http.StatusCreated, map[string]string{"status": "created"})
	}
}

// handleListAliases returns all image aliases for a team.
func (s *Server) handleListAliases() http.HandlerFunc {
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

		aliases, err := s.cfg.Store.ListAliases(teamID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list aliases")
			return
		}

		resp := aliasListResponse{Aliases: make([]aliasResponse, len(aliases))}
		for i, a := range aliases {
			resp.Aliases[i] = aliasResponse{
				ID: a.ID, TeamID: a.TeamID,
				Name: a.AliasName, ImageRef: a.ImageRef,
			}
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleDeleteAlias deletes an image alias by name.
func (s *Server) handleDeleteAlias() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
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

		if err := s.cfg.Store.DeleteAlias(teamID, name); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "alias not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to delete alias")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
