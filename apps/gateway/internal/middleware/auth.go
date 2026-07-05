package middleware

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

type contextKey string

// UserIDContextKey is the context key under which the authenticated user ID is stored.
const UserIDContextKey contextKey = "userID"

// jwtClaims is a minimal representation of the JWT payload the gateway cares about.
type jwtClaims struct {
	Subject string `json:"sub"`
}

// Auth returns middleware that extracts and validates a bearer JWT from the
// Authorization header, storing the resolved user ID in the request context.
//
// This is a skeleton implementation: it validates the token's structural shape
// and decodes the claims, but does not yet verify the cryptographic signature
// against the configured secret.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				utils.WriteError(w, http.StatusUnauthorized, utils.CodeUnauthorized, "missing Authorization header")
				return
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				utils.WriteError(w, http.StatusUnauthorized, utils.CodeUnauthorized, "malformed Authorization header")
				return
			}

			claims, err := parseJWT(parts[1])
			if err != nil || claims.Subject == "" {
				utils.WriteError(w, http.StatusUnauthorized, utils.CodeUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDContextKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext returns the authenticated user ID stored in the context, if any.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	return id, ok
}

// parseJWT decodes a JWT and returns its claims. It validates the three-segment
// structure and base64url-decodes the payload. Signature verification is a
// placeholder left for a future implementation.
func parseJWT(token string) (*jwtClaims, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return nil, errInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return nil, err
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

var errInvalidToken = &authError{"invalid token structure"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
