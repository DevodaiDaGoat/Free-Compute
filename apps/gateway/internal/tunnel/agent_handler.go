package tunnel

import (
	"net/http"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
)

func (s *Server) handleAgentTunnel(w http.ResponseWriter, r *http.Request) {
	routeID, _ := routeIDFromPath("/agent/", r.URL.Path)
	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}
	if user := auth.UserFromContext(r); user != nil {
		if !s.incrementUserConns(user.ID) {
			s.writeConnLimitReached(w, route.ID)
			return
		}
		defer s.decrementUserConns(user.ID)
	}
	if !route.UsesAgentTunnel() {
		http.Error(w, "route is not configured for agent tunneling", http.StatusBadRequest)
		return
	}
	if r.Method != http.MethodConnect {
		http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		s.logger.Printf("agent hijack route=%s error=%v", route.ID, err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("HTTP/1.1 200 Tunnel Ready\r\n\r\n")); err != nil {
		s.logger.Printf("agent ready route=%s error=%v", route.ID, err)
		return
	}

	done, err := s.agents.add(r.Context(), route.ID, conn)
	if err != nil {
		s.logger.Printf("agent add route=%s error=%v", route.ID, err)
		return
	}

	s.logger.Printf("agent tunnel route=%s ready", route.ID)
	select {
	case <-done:
	case <-r.Context().Done():
	}
}
