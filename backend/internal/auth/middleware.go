package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const (
	// ClaimsContextKey is the context key for JWT claims.
	ClaimsContextKey contextKey = "claims"

	// AgentContextKey is the context key for authenticated agent.
	AgentContextKey contextKey = "agent"
)

// JWTMiddleware creates middleware that validates JWT tokens from Authorization header.
// Returns 401 if token is missing or invalid.
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractAndValidateJWT(secret, r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Add claims to context
			ctx := ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWTMiddleware creates middleware that attempts to validate JWT tokens.
// If valid, adds claims to context. If invalid or missing, continues without claims.
func OptionalJWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractAndValidateJWT(secret, r)
			if err == nil && claims != nil {
				ctx := ContextWithClaims(r.Context(), claims)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole creates middleware that checks if the authenticated user has the required role.
// Admin role can access all routes. Returns 401 if not authenticated, 403 if wrong role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				writeAuthError(w, NewAuthError(ErrCodeUnauthorized, "authentication required"))
				return
			}

			// Admin can access everything
			if claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}

			// Check if user has required role
			if claims.Role != role {
				writeForbiddenError(w, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ContextWithClaims adds claims to the context.
func ContextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, ClaimsContextKey, claims)
}

// ClaimsFromContext retrieves claims from the context.
// Returns nil if no claims are present.
func ClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(ClaimsContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// extractAndValidateJWT extracts the JWT from Authorization header and validates it.
func extractAndValidateJWT(secret string, r *http.Request) (*Claims, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, NewAuthError(ErrCodeUnauthorized, "authorization header required")
	}

	// Must be Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, NewAuthError(ErrCodeUnauthorized, "authorization header must be Bearer token")
	}

	token := parts[1]
	if token == "" {
		return nil, NewAuthError(ErrCodeUnauthorized, "token is empty")
	}

	return ValidateJWT(secret, token)
}

// writeAuthError writes an authentication error response as JSON.
func writeAuthError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	authErr, ok := err.(*AuthError)
	if !ok {
		authErr = NewAuthError(ErrCodeUnauthorized, err.Error())
	}

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    authErr.Code,
			"message": authErr.Message,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// writeForbiddenError writes a 403 Forbidden error response as JSON.
func writeForbiddenError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "FORBIDDEN",
			"message": message,
		},
	}

	json.NewEncoder(w).Encode(response)
}
