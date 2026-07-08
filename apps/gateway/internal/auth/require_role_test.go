package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRole(t *testing.T) {
	mgr := NewAuthManager(nil)

	userTokens := make(map[string]string)

	usernames := []string{"alice", "bob", "charlie"}
	roles := []string{"user", "moderator", "admin"}

	for i, username := range usernames {
		user, tokens, err := mgr.Register(username, "password123", username)
		if err != nil {
			t.Fatalf("register %s: %v", username, err)
		}
		if err := mgr.SetRole(user.ID, roles[i]); err != nil {
			t.Fatalf("set role %s: %v", username, err)
		}
		userTokens[roles[i]] = tokens.AccessToken
	}

	next := func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		if user == nil {
			t.Errorf("expected user in context")
			http.Error(w, `{"error":"no user"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"role": user.Role})
	}

	handler := RequireRole(1, mgr, next)

	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantRole   string
	}{
		{"user forbidden", userTokens["user"], http.StatusForbidden, ""},
		{"moderator allowed", userTokens["moderator"], http.StatusOK, "moderator"},
		{"admin allowed", userTokens["admin"], http.StatusOK, "admin"},
		{"unauthenticated forbidden", "", http.StatusUnauthorized, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			req := httptest.NewRequest(http.MethodGet, "/test", &body)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantRole != "" {
				var resp map[string]string
				if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
					t.Fatalf("unmarshal response: %v", err)
				}
				if resp["role"] != tt.wantRole {
					t.Errorf("got role %q, want %q", resp["role"], tt.wantRole)
				}
			}
		})
	}
}
