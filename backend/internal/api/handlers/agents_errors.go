package handlers

import (
	"encoding/json"
	"net/http"
)

// writeAgentError writes an error response.
func writeAgentError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// writeAgentUnauthorized writes a 401 Unauthorized error.
func writeAgentUnauthorized(w http.ResponseWriter, message string) {
	writeAgentError(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// writeAgentValidationError writes a 400 Validation Error.
func writeAgentValidationError(w http.ResponseWriter, message string) {
	writeAgentError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
}
