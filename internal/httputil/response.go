// Package httputil provides shared HTTP response helpers used across all Go services.
package httputil

import (
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// OK writes a 200 JSON response.
func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, data)
}

// Created writes a 201 JSON response.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, data)
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// MessageResponse is a simple {message: ...} envelope.
type MessageResponse struct {
	Message string `json:"message"`
}

// OKMessage writes a 200 JSON response with a message field.
func OKMessage(w http.ResponseWriter, msg string) {
	OK(w, MessageResponse{Message: msg})
}

// PaginatedResponse wraps list data with pagination metadata.
type PaginatedResponse struct {
	Data       any `json:"data"`
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// Paginated writes a paginated JSON response.
func Paginated(w http.ResponseWriter, data any, page, perPage, total int) {
	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}
	OK(w, PaginatedResponse{
		Data:       data,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	})
}

// DecodeJSON decodes a JSON request body into dst, returning an error message
// on failure suitable for client display.
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return ErrEmptyBody
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

type constError string

func (e constError) Error() string { return string(e) }

const ErrEmptyBody = constError("request body is empty")
