package tunnel

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) authorize(route *Route, w http.ResponseWriter, r *http.Request) bool {
	if route != nil && !route.AuthRequired(s.cfg.TunnelToken) {
		return true
	}
	if s.cfg.TunnelToken == "" {
		return true
	}

	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = r.Header.Get("X-FreeCompute-Tunnel-Token")
	}

	if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.TunnelToken)) == 1 {
		return true
	}

	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return false
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
