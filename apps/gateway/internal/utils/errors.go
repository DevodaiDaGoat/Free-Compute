package utils

import (
	"encoding/json"
	"net/http"
)

// ErrorCode is a machine-readable identifier for an error condition.
type ErrorCode string

const (
	CodeBadRequest       ErrorCode = "BAD_REQUEST"
	CodeUnauthorized     ErrorCode = "UNAUTHORIZED"
	CodeForbidden        ErrorCode = "FORBIDDEN"
	CodeNotFound         ErrorCode = "NOT_FOUND"
	CodeConflict         ErrorCode = "CONFLICT"
	CodeTooManyRequests  ErrorCode = "TOO_MANY_REQUESTS"
	CodeInternalError    ErrorCode = "INTERNAL_ERROR"
	CodeNotImplemented   ErrorCode = "NOT_IMPLEMENTED"
	CodeValidationFailed ErrorCode = "VALIDATION_FAILED"
)

// ErrorBody describes the error portion of an API envelope.
type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// Envelope is the consistent response shape used across all API endpoints.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
}

// WriteJSON writes a successful JSON envelope with the given status and data.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	writeEnvelope(w, status, Envelope{Success: true, Data: data})
}

// WriteError writes an error JSON envelope with the given status and code.
func WriteError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	writeEnvelope(w, status, Envelope{
		Success: false,
		Error:   &ErrorBody{Code: code, Message: message},
	})
}

func writeEnvelope(w http.ResponseWriter, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Encoding a plain struct here does not fail in practice; ignore the error.
	_ = json.NewEncoder(w).Encode(env)
}
