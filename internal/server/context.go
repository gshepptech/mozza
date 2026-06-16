package server

import (
	"context"
	"net/http"

	"github.com/gshepptech/mozza/internal/store"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	sessionContextKey contextKey = "session"
)

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userContextKey).(*store.User)
	return u
}

// SessionFromContext returns the session from the request context.
func SessionFromContext(ctx context.Context) *store.Session {
	s, _ := ctx.Value(sessionContextKey).(*store.Session)
	return s
}

// withUser stores the user in the request context.
func withUser(r *http.Request, u *store.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, u)
	return r.WithContext(ctx)
}

// withSession stores the session in the request context.
func withSession(r *http.Request, s *store.Session) *http.Request {
	ctx := context.WithValue(r.Context(), sessionContextKey, s)
	return r.WithContext(ctx)
}
