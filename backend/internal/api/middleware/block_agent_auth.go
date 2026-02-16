package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// BlockAgentAPIKeys is middleware that prevents AI agents from accessing human registration endpoints.
// It checks for agent API keys (format: "Bearer solvr_*") in the Authorization header.
// If an agent API key is detected, it returns 403 FORBIDDEN with a helpful error message.
//
// This prevents the security vulnerability where agents could register as human users,
// bypassing the intended separation between human and agent entities.
//
// Usage: Wrap human registration/auth endpoints with this middleware.
// Example: r.With(BlockAgentAPIKeys).Post("/auth/register", handler.Register)
func BlockAgentAPIKeys(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		// No auth header or empty - allow request
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if it's a Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			// Not a Bearer token (e.g., Basic auth) - allow request
			next.ServeHTTP(w, r)
			return
		}

		// Extract token after "Bearer "
		token := strings.TrimSpace(authHeader[7:])

		// Check if token starts with "solvr_" (case-insensitive)
		if strings.HasPrefix(strings.ToLower(token), "solvr_") {
			// This is an agent API key - block the request
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)

			response := map[string]interface{}{
				"code":    "FORBIDDEN",
				"message": "Agents cannot register as humans. Use POST /v1/agents/register instead.",
				"details": "AI agents must use the agent registration endpoint, not human authentication endpoints.",
			}

			json.NewEncoder(w).Encode(response)
			return
		}

		// Not an agent API key - allow request to proceed
		next.ServeHTTP(w, r)
	})
}
