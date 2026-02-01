// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// AdminOnly creates middleware that requires admin role.
// Returns 401 if not authenticated, 403 if not admin.
// Per SPEC.md Part 16.4, both "admin" and "super_admin" roles have admin access.
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.ClaimsFromContext(r.Context())
		if claims == nil {
			writeAdminError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		// Check for admin or super_admin role
		if !IsAdminOrAbove(claims.Role) {
			writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "admin access required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IsAdminOrAbove checks if the given role has admin privileges.
// Per SPEC.md Part 16.4, both "admin" and "super_admin" roles have admin access.
func IsAdminOrAbove(role string) bool {
	return role == "admin" || role == "super_admin"
}

// writeAdminError writes a JSON error response with the given status code.
func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	json.NewEncoder(w).Encode(response)
}
