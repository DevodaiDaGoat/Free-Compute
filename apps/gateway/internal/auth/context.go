package auth

import "context"

type contextKey string

const ctxKeyUser contextKey = "user"

func newContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKeyUser, user)
}
