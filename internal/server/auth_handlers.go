package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/store"
)

const (
	sessionCookieName = "mozza_session"
	cookieMaxAge      = 7 * 24 * 60 * 60 // 7 days in seconds
)

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func toUserResponse(u *store.User) userResponse {
	return userResponse{
		ID:    u.ID,
		Email: u.Email,
		Name:  u.Name,
		Role:  u.Role,
	}
}

// handleRegister creates a new user account and sets a session cookie.
func (s *Server) handleRegister() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if s.cfg.Auth == nil {
			Error(w, http.StatusServiceUnavailable, "auth not configured")
			return
		}

		user, sess, err := s.cfg.Auth.Register(req.Email, req.Name, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, auth.ErrInvalidEmail):
				Error(w, http.StatusBadRequest, "invalid email address")
			case errors.Is(err, auth.ErrWeakPassword):
				Error(w, http.StatusBadRequest, "password must be at least 8 characters")
			default:
				if containsStr(err.Error(), "already registered") {
					Error(w, http.StatusConflict, "email already registered")
					return
				}
				Error(w, http.StatusInternalServerError, "registration failed")
			}
			return
		}

		SessionCreated()
		setSessionCookie(w, r, sess)
		JSON(w, http.StatusCreated, toUserResponse(user))
	}
}

// handleLogin authenticates a user and sets a session cookie.
func (s *Server) handleLogin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if s.cfg.Auth == nil {
			Error(w, http.StatusServiceUnavailable, "auth not configured")
			return
		}

		user, sess, err := s.cfg.Auth.Login(req.Email, req.Password)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				Error(w, http.StatusUnauthorized, "invalid email or password")
				return
			}
			Error(w, http.StatusInternalServerError, "login failed")
			return
		}

		SessionCreated()
		setSessionCookie(w, r, sess)
		JSON(w, http.StatusOK, toUserResponse(user))
	}
}

// handleLogout clears the session cookie and invalidates the session.
func (s *Server) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && s.cfg.Auth != nil {
			_ = s.cfg.Auth.Logout(cookie.Value)
			SessionDestroyed()
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})

		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// handleMe returns the currently authenticated user.
func (s *Server) handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			Error(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		JSON(w, http.StatusOK, toUserResponse(user))
	}
}

// handleUpdateMe updates the authenticated user's profile.
func (s *Server) handleUpdateMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			Error(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		name := req.Name
		if name == "" {
			name = user.Name
		}

		if err := s.cfg.Store.UpdateUser(user.ID, name, user.Role); err != nil {
			Error(w, http.StatusInternalServerError, "failed to update profile")
			return
		}

		user.Name = name
		JSON(w, http.StatusOK, toUserResponse(user))
	}
}

// validRoles is the set of allowed RBAC roles.
var validRoles = map[string]bool{
	"viewer":   true,
	"deployer": true,
	"admin":    true,
}

// handleUpdateRole allows an admin to change another user's role.
func (s *Server) handleUpdateRole() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin := UserFromContext(r.Context())
		if admin == nil {
			Error(w, http.StatusUnauthorized, "not authenticated")
			return
		}

		targetID := chi.URLParam(r, "id")

		var req struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if !validRoles[req.Role] {
			Error(w, http.StatusBadRequest, "invalid role: must be viewer, deployer, or admin")
			return
		}

		// Prevent the last admin from demoting themselves.
		if targetID == admin.ID && req.Role != "admin" {
			users, listErr := s.cfg.Store.ListUsers()
			if listErr == nil {
				adminCount := 0
				for _, u := range users {
					if u.Role == "admin" {
						adminCount++
					}
				}
				if adminCount <= 1 {
					Error(w, http.StatusConflict, "cannot demote the last admin")
					return
				}
			}
		}

		target, err := s.cfg.Store.UserByID(targetID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "user not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get user")
			return
		}

		oldRole := target.Role
		if err := s.cfg.Store.UpdateUser(target.ID, target.Name, req.Role); err != nil {
			Error(w, http.StatusInternalServerError, "failed to update role")
			return
		}

		slog.Info("role_changed",
			"user_id", target.ID,
			"from", oldRole,
			"to", req.Role,
			"by", admin.ID,
		)

		target.Role = req.Role
		JSON(w, http.StatusOK, toUserResponse(target))
	}
}

// noAuthUser is the synthetic admin user injected when --no-auth is set.
var noAuthUser = &store.User{
	ID:    "no-auth-admin",
	Email: "admin@localhost",
	Name:  "Local Admin",
	Role:  "admin",
}

// sessionAuth is middleware that validates the session cookie and injects user into context.
// When NoAuth is enabled, it injects a synthetic admin user instead.
func (s *Server) sessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.NoAuth {
			r = withUser(r, noAuthUser)
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			Error(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if s.cfg.Auth == nil {
			Error(w, http.StatusServiceUnavailable, "auth not configured")
			return
		}

		user, sess, err := s.cfg.Auth.ValidateSession(cookie.Value)
		if err != nil {
			http.SetCookie(w, &http.Cookie{
				Name:   sessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			Error(w, http.StatusUnauthorized, "session expired")
			return
		}

		r = withUser(r, user)
		r = withSession(r, sess)
		next.ServeHTTP(w, r)
	})
}

// isTeamMember checks team membership. When NoAuth is enabled, it always
// returns true so that the synthetic admin can operate on any team.
func (s *Server) isTeamMember(teamID, userID string) (bool, error) {
	if s.cfg.NoAuth {
		return true, nil
	}
	return s.cfg.Store.IsTeamMember(teamID, userID)
}

// ensureNoAuthUser creates the synthetic no-auth admin user in the database
// if it doesn't already exist, so that foreign key constraints are satisfied.
func (s *Server) ensureNoAuthUser() {
	_, err := s.cfg.Store.UserByID(noAuthUser.ID)
	if err == nil {
		return // already exists
	}
	// Insert directly — CreateUser hashes a password which we don't need.
	_, err = s.cfg.Store.EnsureUser(noAuthUser.ID, noAuthUser.Email, noAuthUser.Name, noAuthUser.Role)
	if err != nil {
		slog.Warn("failed to create no-auth user in database", "error", err)
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, sess *store.Session) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sess.ID,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
