package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// AppError represents a structured API error.
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *AppError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// StatusCode returns the HTTP status code for this error.
func (e *AppError) StatusCode() int {
	return e.Code
}

// ToJSON serializes the error to JSON bytes.
func (e *AppError) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// NewBadRequest creates a 400 error.
func NewBadRequest(detail string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: "Bad Request", Detail: detail}
}

// NewUnauthorized creates a 401 error.
func NewUnauthorized(detail string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, Message: "Unauthorized", Detail: detail}
}

// NewForbidden creates a 403 error.
func NewForbidden(detail string) *AppError {
	return &AppError{Code: http.StatusForbidden, Message: "Forbidden", Detail: detail}
}

// NewNotFound creates a 404 error.
func NewNotFound(detail string) *AppError {
	return &AppError{Code: http.StatusNotFound, Message: "Not Found", Detail: detail}
}

// NewConflict creates a 409 error.
func NewConflict(detail string) *AppError {
	return &AppError{Code: http.StatusConflict, Message: "Conflict", Detail: detail}
}

// NewTooManyRequests creates a 429 error.
func NewTooManyRequests(detail string) *AppError {
	return &AppError{Code: http.StatusTooManyRequests, Message: "Too Many Requests", Detail: detail}
}

// NewInternalError creates a 500 error.
func NewInternalError(detail string) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: "Internal Server Error", Detail: detail}
}

// ErrorResponse is the standard JSON error envelope.
type ErrorResponse struct {
	Error AppError `json:"error"`
}

// WriteError writes a structured error response to an http.ResponseWriter.
func WriteError(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	resp := ErrorResponse{Error: *err}
	json.NewEncoder(w).Encode(resp)
}
