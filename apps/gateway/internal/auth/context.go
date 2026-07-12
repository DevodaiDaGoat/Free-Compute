package auth

import "context"

type contextKey string

const ctxKeyUser contextKey = "user"

func newContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser, user)
}

// WithUser returns a new context with the given user attached. Exported so
// packages that authenticate outside of RequireAuth (e.g. tunnel token +
// user hybrid auth) can still stash the authenticated user for downstream
// handlers.
func WithUser(ctx context.Context, user *User) context.Context {
	return newContextWithUser(ctx, user)
}
