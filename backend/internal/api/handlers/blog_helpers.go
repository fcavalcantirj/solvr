package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
)

// writeBlogJSON writes a JSON response for blog endpoints.
func writeBlogJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeBlogError writes an error JSON response for blog endpoints.
func writeBlogError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// validSlugRegex matches URL-friendly slugs: lowercase alphanumeric and hyphens.
var validSlugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validateSlug checks if a slug is valid for use in URLs.
func validateSlug(slug string) bool {
	if slug == "" || len(slug) > 200 {
		return false
	}
	return validSlugRegex.MatchString(slug)
}
