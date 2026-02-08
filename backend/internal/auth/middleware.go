package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// contextKey is the type for context keys to avoid collisions.
type contextKey string

const (
	// ClaimsContextKey is the context key for JWT claims.
	ClaimsContextKey contextKey = "claims"

	// AgentContextKey is the context key for authenticated agent.
	AgentContextKey contextKey = "agent"

	// APIKeyIDContextKey is the context key for the API key ID (for per-key rate limiting).
	APIKeyIDContextKey contextKey = "apiKeyID"

	// APIKeyTierContextKey is the context key for the API key tier (for tiered rate limits).
	APIKeyTierContextKey contextKey = "apiKeyTier"
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

// APIKeyMiddleware creates middleware that validates API keys from Authorization header.
// API keys must start with "solvr_" prefix.
// Returns 401 if key is missing or invalid.
func APIKeyMiddleware(validator *APIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Check if it's an API key (starts with solvr_)
			if !IsAPIKey(token) {
				writeAuthError(w, NewAuthError(ErrCodeInvalidAPIKey, "invalid API key format"))
				return
			}

			agent, err := validator.ValidateAPIKey(r.Context(), token)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Add agent to context
			ctx := ContextWithAgent(r.Context(), agent)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CombinedAuthMiddleware creates middleware that tries JWT first, then API key.
// Returns 401 if both authentication methods fail.
func CombinedAuthMiddleware(jwtSecret string, apiKeyValidator *APIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Try API key first (if it has the prefix)
			if IsAPIKey(token) {
				agent, err := apiKeyValidator.ValidateAPIKey(r.Context(), token)
				if err == nil && agent != nil {
					ctx := ContextWithAgent(r.Context(), agent)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// API key validation failed, continue to try JWT
			}

			// Try JWT
			claims, err := ValidateJWT(jwtSecret, token)
			if err == nil && claims != nil {
				ctx := ContextWithClaims(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Both methods failed
			writeAuthError(w, NewAuthError(ErrCodeUnauthorized, "invalid authentication credentials"))
		})
	}
}

// ContextWithAgent adds an agent to the context.
func ContextWithAgent(ctx context.Context, agent *models.Agent) context.Context {
	return context.WithValue(ctx, AgentContextKey, agent)
}

// AgentFromContext retrieves an agent from the context.
// Returns nil if no agent is present.
func AgentFromContext(ctx context.Context) *models.Agent {
	agent, ok := ctx.Value(AgentContextKey).(*models.Agent)
	if !ok {
		return nil
	}
	return agent
}

// extractBearerToken extracts the token from the Authorization header.
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", NewAuthError(ErrCodeUnauthorized, "authorization header required")
	}

	// Must be Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", NewAuthError(ErrCodeUnauthorized, "authorization header must be Bearer token")
	}

	token := parts[1]
	if token == "" {
		return "", NewAuthError(ErrCodeUnauthorized, "token is empty")
	}

	return token, nil
}

// ContextWithAPIKeyID adds an API key ID to the context for per-key rate limiting.
func ContextWithAPIKeyID(ctx context.Context, apiKeyID string) context.Context {
	return context.WithValue(ctx, APIKeyIDContextKey, apiKeyID)
}

// APIKeyIDFromContext retrieves the API key ID from the context.
// Returns empty string if no API key ID is present.
func APIKeyIDFromContext(ctx context.Context) string {
	apiKeyID, ok := ctx.Value(APIKeyIDContextKey).(string)
	if !ok {
		return ""
	}
	return apiKeyID
}

// ContextWithAPIKeyTier adds an API key tier to the context for tiered rate limits.
func ContextWithAPIKeyTier(ctx context.Context, tier string) context.Context {
	return context.WithValue(ctx, APIKeyTierContextKey, tier)
}

// APIKeyTierFromContext retrieves the API key tier from the context.
// Returns empty string if no tier is present.
func APIKeyTierFromContext(ctx context.Context) string {
	tier, ok := ctx.Value(APIKeyTierContextKey).(string)
	if !ok {
		return ""
	}
	return tier
}

// UnifiedAuthMiddleware creates middleware that accepts all three authentication types:
// 1. User API keys (solvr_sk_...) - for humans using API programmatically
// 2. Agent API keys (solvr_...) - for AI agents
// 3. JWT tokens - for logged-in web users
// Returns 401 if all authentication methods fail.
func UnifiedAuthMiddleware(jwtSecret string, agentValidator *APIKeyValidator, userValidator *UserAPIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Try user API key first (solvr_sk_...) - more specific prefix
			if IsUserAPIKey(token) && userValidator != nil {
				user, apiKey, err := userValidator.ValidateUserAPIKey(r.Context(), token)
				if err == nil && user != nil {
					// Create claims from user info
					claims := &Claims{
						UserID: user.ID,
						Email:  user.Email,
						Role:   "user",
					}
					ctx := ContextWithClaims(r.Context(), claims)
					ctx = ContextWithAPIKeyID(ctx, apiKey.ID)
					// Update last_used_at (fire and forget)
					_ = userValidator.db.UpdateLastUsed(r.Context(), apiKey.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// User API key validation failed - don't fall through, it's a specific format
				writeAuthError(w, NewAuthError(ErrCodeInvalidAPIKey, "invalid user API key"))
				return
			} else if IsUserAPIKey(token) && userValidator == nil {
				// User API key format but no validator configured - reject
				writeAuthError(w, NewAuthError(ErrCodeInvalidAPIKey, "user API keys not supported"))
				return
			}

			// Try agent API key (solvr_... but not solvr_sk_)
			if IsAPIKey(token) {
				agent, err := agentValidator.ValidateAPIKey(r.Context(), token)
				if err == nil && agent != nil {
					ctx := ContextWithAgent(r.Context(), agent)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				// Agent API key validation failed - don't fall through, it's a specific format
				writeAuthError(w, NewAuthError(ErrCodeInvalidAPIKey, "invalid API key"))
				return
			}

			// Try JWT
			claims, err := ValidateJWT(jwtSecret, token)
			if err == nil && claims != nil {
				ctx := ContextWithClaims(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// All methods failed
			writeAuthError(w, NewAuthError(ErrCodeUnauthorized, "invalid authentication credentials"))
		})
	}
}
