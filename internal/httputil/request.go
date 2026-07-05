package httputil

import (
	"net/http"
	"strconv"
)

// QueryInt reads an integer query parameter, returning fallback on absence or parse error.
func QueryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// QueryString reads a string query parameter, returning fallback on absence.
func QueryString(r *http.Request, key, fallback string) string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	return v
}

// PaginationParams extracts page and per_page from query params with defaults.
func PaginationParams(r *http.Request) (page, perPage int) {
	page = QueryInt(r, "page", 1)
	perPage = QueryInt(r, "per_page", 20)

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}
