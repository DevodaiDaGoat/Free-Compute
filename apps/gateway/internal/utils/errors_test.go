package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  AppError
		want string
	}{
		{
			"with detail",
			AppError{Code: 400, Message: "Bad Request", Detail: "missing field"},
			"[400] Bad Request: missing field",
		},
		{
			"without detail",
			AppError{Code: 500, Message: "Internal Server Error"},
			"[500] Internal Server Error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppError_StatusCode(t *testing.T) {
	err := NewNotFound("user 123")
	if err.StatusCode() != 404 {
		t.Errorf("StatusCode() = %d, want 404", err.StatusCode())
	}
}

func TestAppError_ToJSON(t *testing.T) {
	err := NewBadRequest("invalid email")
	data, jsonErr := err.ToJSON()
	if jsonErr != nil {
		t.Fatalf("ToJSON() error: %v", jsonErr)
	}

	var parsed AppError
	if e := json.Unmarshal(data, &parsed); e != nil {
		t.Fatalf("Unmarshal error: %v", e)
	}
	if parsed.Code != 400 {
		t.Errorf("Code = %d, want 400", parsed.Code)
	}
	if parsed.Detail != "invalid email" {
		t.Errorf("Detail = %q, want 'invalid email'", parsed.Detail)
	}
}

func TestNewBadRequest(t *testing.T) {
	err := NewBadRequest("test")
	if err.Code != http.StatusBadRequest || err.Message != "Bad Request" || err.Detail != "test" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("expired token")
	if err.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want 401", err.Code)
	}
}

func TestNewForbidden(t *testing.T) {
	err := NewForbidden("no access")
	if err.Code != http.StatusForbidden {
		t.Errorf("Code = %d, want 403", err.Code)
	}
}

func TestNewNotFound(t *testing.T) {
	err := NewNotFound("vm-123")
	if err.Code != http.StatusNotFound || err.Detail != "vm-123" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestNewConflict(t *testing.T) {
	err := NewConflict("already exists")
	if err.Code != http.StatusConflict {
		t.Errorf("Code = %d, want 409", err.Code)
	}
}

func TestNewTooManyRequests(t *testing.T) {
	err := NewTooManyRequests("rate limit exceeded")
	if err.Code != http.StatusTooManyRequests {
		t.Errorf("Code = %d, want 429", err.Code)
	}
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError("db connection failed")
	if err.Code != http.StatusInternalServerError {
		t.Errorf("Code = %d, want 500", err.Code)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := NewNotFound("resource xyz")

	WriteError(w, appErr)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Error.Code != 404 {
		t.Errorf("response code = %d, want 404", resp.Error.Code)
	}
	if resp.Error.Detail != "resource xyz" {
		t.Errorf("response detail = %q, want 'resource xyz'", resp.Error.Detail)
	}
}

func TestWriteError_InternalError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := NewInternalError("something broke")

	WriteError(w, appErr)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestAppError_JSONRoundTrip(t *testing.T) {
	original := &AppError{Code: 422, Message: "Unprocessable", Detail: "bad input"}
	data, _ := original.ToJSON()

	var decoded AppError
	json.Unmarshal(data, &decoded)

	if decoded.Code != original.Code || decoded.Message != original.Message || decoded.Detail != original.Detail {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestAppError_EmptyDetail_OmittedInJSON(t *testing.T) {
	err := &AppError{Code: 500, Message: "Internal Server Error"}
	data, _ := err.ToJSON()
	if strings.Contains(string(data), "detail") {
		t.Error("empty detail should be omitted from JSON")
	}
}
