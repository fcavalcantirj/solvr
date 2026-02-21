package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// writePostsJSON writes a JSON response.
func writePostsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// float32SliceToVectorString converts a float32 slice to PostgreSQL vector literal format.
// Example: [0.1, 0.2, 0.3] -> "[0.1,0.2,0.3]"
func float32SliceToVectorString(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	s := "["
	for i, f := range v {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%g", f)
	}
	s += "]"
	return s
}

// writePostsError writes an error JSON response.
func writePostsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
