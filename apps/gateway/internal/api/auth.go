package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// Register proxies registration to the auth service.
// Input validation is performed before forwarding.
func Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Input validation
	if !isValidEmail(req.Email) {
		respondError(w, http.StatusBadRequest, "invalid email format")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 128 {
		respondError(w, http.StatusBadRequest, "password too long")
		return
	}

	// TODO: Proxy to auth service
	log.Info().Str("email", req.Email).Msg("registration request")
	respondJSON(w, http.StatusCreated, map[string]string{"message": "registration successful, check email for verification"})
}

// Login proxies login to the auth service.
// SECURITY: Returns consistent error message to prevent account enumeration.
func Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		// SECURITY: Same error message whether email exists or not
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// TODO: Proxy to auth service
	respondJSON(w, http.StatusOK, map[string]string{"message": "login endpoint placeholder"})
}

// Verify handles email verification.
func Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token required")
		return
	}

	// TODO: Proxy to auth service
	respondJSON(w, http.StatusOK, map[string]string{"message": "verification endpoint placeholder"})
}

// Logout invalidates the user's session/token.
func Logout(w http.ResponseWriter, r *http.Request) {
	// TODO: Add token to revocation list in Redis
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// --- Helpers ---

func decodeJSON(r *http.Request, v interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // SECURITY: Reject unexpected fields
	return decoder.Decode(v)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func isValidEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	atIdx := -1
	for i, c := range email {
		if c == '@' {
			if atIdx != -1 {
				return false // multiple @
			}
			atIdx = i
		}
	}
	if atIdx < 1 || atIdx >= len(email)-1 {
		return false
	}
	// Check domain has at least one dot after @
	domain := email[atIdx+1:]
	hasDot := false
	for _, c := range domain {
		if c == '.' {
			hasDot = true
		}
	}
	return hasDot
}
