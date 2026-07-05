package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/DevodaiDaGoat/Free-Compute/apps/gateway/internal/utils"
)

// AuthService defines the behavior the auth handler depends on. Concrete
// implementations (e.g. a remote auth-service client) are injected at wiring time.
type AuthService interface {
	Register(email, password string) (userID string, err error)
	Login(email, password string) (token string, err error)
	Verify(token string) (bool, error)
	Logout(token string) error
}

// AuthHandler exposes authentication endpoints.
type AuthHandler struct {
	svc AuthService
}

// NewAuthHandler constructs an AuthHandler with the given service dependency.
func NewAuthHandler(svc AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Routes returns a chi router group mounting the auth endpoints.
func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/verify", h.verify)
	r.Post("/logout", h.logout)
	return r
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusCreated, map[string]any{
		"user_id": "usr_placeholder",
		"message": "registration accepted",
	})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"token":      "jwt.placeholder.token",
		"expires_in": 3600,
	})
}

func (h *AuthHandler) verify(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"valid": true,
	})
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "logged out",
	})
}
