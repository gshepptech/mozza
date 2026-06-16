package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/store"
)

// validSlug matches DNS-compatible slugs: lowercase alphanumeric with hyphens.
var validSlug = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type createTeamRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type teamResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	CreatedBy string `json:"created_by"`
}

type teamsListResponse struct {
	Teams []teamResponse `json:"teams"`
}

type teamMemberResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Email  string `json:"email,omitempty"`
	Name   string `json:"name,omitempty"`
}

type teamMembersResponse struct {
	Members []teamMemberResponse `json:"members"`
}

type addMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// handleCreateTeam creates a new team.
func (s *Server) handleCreateTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		var req createTeamRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.Slug == "" {
			Error(w, http.StatusBadRequest, "name and slug are required")
			return
		}
		if !validSlug.MatchString(req.Slug) {
			Error(w, http.StatusBadRequest, "slug must be lowercase alphanumeric with hyphens (e.g. my-team)")
			return
		}

		team, err := s.cfg.Store.CreateTeam(req.Name, req.Slug, user.ID)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				Error(w, http.StatusConflict, "team name or slug already exists")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to create team")
			return
		}

		JSON(w, http.StatusCreated, teamResponse{
			ID: team.ID, Name: team.Name, Slug: team.Slug, CreatedBy: team.CreatedBy,
		})
	}
}

// handleListTeams returns teams for the authenticated user.
func (s *Server) handleListTeams() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		teams, err := s.cfg.Store.TeamsForUser(user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list teams")
			return
		}

		resp := teamsListResponse{Teams: make([]teamResponse, len(teams))}
		for i, t := range teams {
			resp.Teams[i] = teamResponse{ID: t.ID, Name: t.Name, Slug: t.Slug, CreatedBy: t.CreatedBy}
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleGetTeam returns a single team by ID.
func (s *Server) handleGetTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		user := UserFromContext(r.Context())

		isMember, err := s.isTeamMember(id, user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to check membership")
			return
		}
		if !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		team, err := s.cfg.Store.TeamByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "team not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get team")
			return
		}
		JSON(w, http.StatusOK, teamResponse{
			ID: team.ID, Name: team.Name, Slug: team.Slug, CreatedBy: team.CreatedBy,
		})
	}
}

// handleDeleteTeam deletes a team. Only team owners can delete.
func (s *Server) handleDeleteTeam() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		user := UserFromContext(r.Context())

		// Verify the user is a member of the team.
		isMember, err := s.isTeamMember(id, user.ID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to check membership")
			return
		}
		if !isMember {
			Error(w, http.StatusForbidden, "not a member of this team")
			return
		}

		// Verify the user is the team owner (created_by).
		team, err := s.cfg.Store.TeamByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "team not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get team")
			return
		}
		if team.CreatedBy != user.ID {
			Error(w, http.StatusForbidden, "only the team owner can delete a team")
			return
		}

		if err := s.cfg.Store.DeleteTeam(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "team not found")
				return
			}
			// FK violation means team still has dependent records.
			errMsg := err.Error()
			if strings.Contains(errMsg, "FOREIGN KEY") || strings.Contains(errMsg, "foreign key") ||
				strings.Contains(errMsg, "violates foreign key") {
				Error(w, http.StatusConflict, "cannot delete team: it has recipes or deployments — delete them first")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to delete team")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// handleListTeamMembers returns members of a team.
func (s *Server) handleListTeamMembers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		members, err := s.cfg.Store.TeamMembers(teamID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list members")
			return
		}

		resp := teamMembersResponse{Members: make([]teamMemberResponse, len(members))}
		for i, m := range members {
			resp.Members[i] = teamMemberResponse{UserID: m.UserID, Role: m.Role}
			// Optionally enrich with user info.
			if u, err := s.cfg.Store.UserByID(m.UserID); err == nil {
				resp.Members[i].Email = u.Email
				resp.Members[i].Name = u.Name
			}
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleAddTeamMember adds a user to a team by email.
func (s *Server) handleAddTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		var req addMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		u, err := s.cfg.Store.UserByEmail(req.Email)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "user not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to find user")
			return
		}

		role := req.Role
		if role == "" {
			role = "member"
		}

		if err := s.cfg.Store.AddTeamMember(teamID, u.ID, role); err != nil {
			if errors.Is(err, store.ErrConflict) {
				Error(w, http.StatusConflict, "user already a member")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to add member")
			return
		}

		JSON(w, http.StatusCreated, map[string]string{"status": "added"})
	}
}

// handleRemoveTeamMember removes a user from a team.
func (s *Server) handleRemoveTeamMember() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := chi.URLParam(r, "id")
		userID := chi.URLParam(r, "userID")

		if err := s.cfg.Store.RemoveTeamMember(teamID, userID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "member not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to remove member")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "removed"})
	}
}
