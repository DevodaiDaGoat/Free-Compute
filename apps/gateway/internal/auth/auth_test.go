package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateTokenRejectsExpiredTokens(t *testing.T) {
	mgr := NewAuthManager(nil)

	user, tokens, err := mgr.Register("test@example.com", "password123", "Test")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := mgr.SetRole(user.ID, "user"); err != nil {
		t.Fatalf("set role: %v", err)
	}

	mgr.mu.Lock()
	expiredTime := time.Now().Add(-1 * time.Hour)
	mgr.tokenExpiry[tokens.AccessToken] = expiredTime
	mgr.mu.Unlock()

	_, err = mgr.ValidateToken(tokens.AccessToken)
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestValidateTokenAcceptsNonExpiredTokens(t *testing.T) {
	mgr := NewAuthManager(nil)

	_, tokens, err := mgr.Register("test2@example.com", "password123", "Test2")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	user, err := mgr.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}
	if user.Email != "test2@example.com" {
		t.Fatalf("unexpected user email")
	}
}

func TestRequireAuthRejectsQueryStringToken(t *testing.T) {
	mgr := NewAuthManager(nil)
	next := func(w http.ResponseWriter, r *http.Request) {}

	handler := RequireAuth(mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test?token=abc", &body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for query-string token, got %d", rr.Code)
	}
}

func TestRequireAuthAcceptsBearerHeader(t *testing.T) {
	mgr := NewAuthManager(nil)
	_, tokens, err := mgr.Register("hdr@example.com", "password123", "Header")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		user := UserFromContext(r)
		if user == nil || user.Email != "hdr@example.com" {
			t.Errorf("expected user in context")
		}
	}

	handler := RequireAuth(mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test", &body)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid bearer token, got %d: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatalf("next handler was not called")
	}
}

func TestAuthMiddlewareIgnoresQueryStringToken(t *testing.T) {
	mgr := NewAuthManager(nil)
	_, tokens, err := mgr.Register("middle@example.com", "password123", "Middleware")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	next := func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		if user != nil {
			t.Fatalf("expected no user in context for query-string token")
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}

	handler := AuthMiddleware(mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test?token="+tokens.AccessToken, &body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareAcceptsBearerHeader(t *testing.T) {
	mgr := NewAuthManager(nil)
	_, tokens, err := mgr.Register("middle2@example.com", "password123", "Middleware2")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	next := func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r)
		if user == nil {
			t.Fatalf("expected user in context")
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
	}

	handler := AuthMiddleware(mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test", &body)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCleanupRemovesOnlyExpiredTokens(t *testing.T) {
	mgr := NewAuthManager(nil)

	user1, tokens1, err := mgr.Register("keep@example.com", "password123", "Keep")
	if err != nil {
		t.Fatalf("register keep: %v", err)
	}

	_, tokens2, err := mgr.Register("remove@example.com", "password123", "Remove")
	if err != nil {
		t.Fatalf("register remove: %v", err)
	}

	if err := mgr.SetRole(user1.ID, "user"); err != nil {
		t.Fatalf("set role: %v", err)
	}

	mgr.mu.Lock()
	mgr.tokenExpiry[tokens2.AccessToken] = time.Now().Add(-1 * time.Hour)
	mgr.tokenExpiry[tokens2.RefreshToken] = time.Now().Add(-1 * time.Hour)
	mgr.mu.Unlock()

	mgr.cleanup()

	_, err = mgr.ValidateToken(tokens1.AccessToken)
	if err == ErrInvalidToken {
		t.Fatalf("valid token was incorrectly cleaned up")
	}

	_, err = mgr.ValidateToken(tokens2.AccessToken)
	if err != ErrInvalidToken {
		t.Fatalf("expired token was not cleaned up")
	}

	_, err = mgr.ValidateToken(tokens2.RefreshToken)
	if err != ErrInvalidToken {
		t.Fatalf("expired refresh token was not cleaned up")
	}
}

func TestRequireRoleRejectsQueryStringToken(t *testing.T) {
	mgr := NewAuthManager(nil)
	next := func(w http.ResponseWriter, r *http.Request) {}

	handler := RequireRole(1, mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test?token=abc", &body)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for query-string token, got %d", rr.Code)
	}
}

func TestRequireRoleAcceptsBearerHeader(t *testing.T) {
	mgr := NewAuthManager(nil)
	user, tokens, err := mgr.Register("role@example.com", "password123", "Role")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := mgr.SetRole(user.ID, "moderator"); err != nil {
		t.Fatalf("set role: %v", err)
	}

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		user := UserFromContext(r)
		if user == nil || user.Role != "moderator" {
			t.Errorf("expected moderator user in context, got role=%s", user.Role)
		}
	}

	handler := RequireRole(1, mgr, next)

	var body bytes.Buffer
	req := httptest.NewRequest(http.MethodGet, "/test", &body)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatalf("next handler was not called")
	}
}

func TestTokenPairJSONContainsExpiresAt(t *testing.T) {
	mgr := NewAuthManager(nil)

	_, tokens, err := mgr.Register("json@example.com", "password123", "JSON")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	mgr.mu.RLock()
	_, hasAccessExp := mgr.tokenExpiry[tokens.AccessToken]
	_, hasRefreshExp := mgr.tokenExpiry[tokens.RefreshToken]
	mgr.mu.RUnlock()

	if !hasAccessExp {
		t.Fatalf("access token expiry not tracked")
	}
	if !hasRefreshExp {
		t.Fatalf("refresh token expiry not tracked")
	}

	body, err := json.Marshal(tokens)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(body, []byte("expiresAt")) {
		t.Fatalf("token pair JSON missing expiresAt: %s", string(body))
	}
}
