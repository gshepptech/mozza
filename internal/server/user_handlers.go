package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/store"
)

type usersListResponse struct {
	Users []userResponse `json:"users"`
}

// handleListUsers returns all users.
func (s *Server) handleListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := s.cfg.Store.ListUsers()
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list users")
			return
		}

		resp := usersListResponse{Users: make([]userResponse, len(users))}
		for i, u := range users {
			resp.Users[i] = userResponse{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role}
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleGetUser returns a single user by ID.
func (s *Server) handleGetUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		u, err := s.cfg.Store.UserByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "user not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get user")
			return
		}
		JSON(w, http.StatusOK, toUserResponse(u))
	}
}
