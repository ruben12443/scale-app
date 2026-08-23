package auth

import (
	"context"

	"scale-app/backend/services/core-api/internal/domain"
)

type contextKey int

const userContextKey contextKey = iota

// ContextWithUser returns a new context carrying the authenticated user.
func ContextWithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext returns the authenticated user attached by Middleware, if
// any.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userContextKey).(*domain.User)
	return u, ok
}
