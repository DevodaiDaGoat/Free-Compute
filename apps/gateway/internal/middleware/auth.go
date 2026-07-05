package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	RoleKey   contextKey = "role"
)

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role"`
}

// Authenticate validates the JWT from the Authorization header.
// Tokens must use RS256; HS256 and "none" are rejected.
func Authenticate(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			tokenStr := parts[1]

			token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
				// SECURITY: Reject non-RS256 algorithms to prevent algorithm confusion attacks
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				// Load public key for verification
				pubKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKey))
				if err != nil {
					return nil, err
				}
				return pubKey, nil
			})

			if err != nil || !token.Valid {
				log.Debug().Err(err).Msg("invalid token")
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(*Claims)
			if !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			// Inject user context
			ctx := context.WithValue(r.Context(), UserIDKey, claims.Subject)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole rejects requests from users without the specified role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole, _ := r.Context().Value(RoleKey).(string)
			if userRole != role {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID extracts the user ID from the request context.
func GetUserID(r *http.Request) string {
	id, _ := r.Context().Value(UserIDKey).(string)
	return id
}
