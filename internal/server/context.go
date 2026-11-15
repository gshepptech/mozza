package server

import (
	"context"
	"net/http"

	"github.com/gshepptech/mozza/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext returns the authenticated user from the request context.
func UserFromContext(ctx context.Context) *store.User {
	u, _ := ctx.Value(userContextKey).(*store.User)
	return u
}

// withUser stores the user in the request context.
func withUser(r *http.Request, u *store.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, u)
	return r.WithContext(ctx)
}
