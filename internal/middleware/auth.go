// Package middleware provides shared HTTP middleware used across all Go services.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/DevodaiDaGoat/Free-Compute/internal/errors"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userIDKey contextKey = "user_id"
const userClaimsKey contextKey = "claims"

// Claims represents the JWT claims used across services.
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// Auth returns middleware that validates JWT Bearer tokens.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				errors.Unauthorized("missing authorization header").WriteJSON(w)
				return
			}

			tokenStr, ok := strings.CutPrefix(header, "Bearer ")
			if !ok {
				errors.Unauthorized("invalid authorization format").WriteJSON(w)
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.Unauthorized("unexpected signing method")
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				errors.Unauthorized("invalid or expired token").WriteJSON(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserID extracts the authenticated user ID from the request context.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

// UserClaims extracts the full JWT claims from the request context.
func UserClaims(ctx context.Context) *Claims {
	claims, _ := ctx.Value(userClaimsKey).(*Claims)
	return claims
}
