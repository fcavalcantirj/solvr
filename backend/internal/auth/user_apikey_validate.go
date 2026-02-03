// Package auth provides authentication functionality for Solvr.
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

const (
	// UserAPIKeyPrefix is the prefix for user API keys (sk = secret key).
	UserAPIKeyPrefix = "solvr_sk_"
)

// UserAPIKeyDB defines the interface for user API key database operations.
type UserAPIKeyDB interface {
	// GetUserByAPIKey finds a user and API key by validating the plain text key.
	// Returns nil, nil, nil if no matching key is found.
	GetUserByAPIKey(ctx context.Context, plainKey string) (*models.User, *models.UserAPIKey, error)

	// UpdateLastUsed updates the last_used_at timestamp for a key.
	UpdateLastUsed(ctx context.Context, keyID string) error
}

// UserAPIKeyValidator validates user API keys against the database.
type UserAPIKeyValidator struct {
	db UserAPIKeyDB
}

// NewUserAPIKeyValidator creates a new UserAPIKeyValidator with the given database.
func NewUserAPIKeyValidator(db UserAPIKeyDB) *UserAPIKeyValidator {
	return &UserAPIKeyValidator{db: db}
}

// ValidateUserAPIKey validates a user API key and returns the associated user and key.
// Returns an AuthError if the key is invalid, malformed, or not found.
func (v *UserAPIKeyValidator) ValidateUserAPIKey(ctx context.Context, key string) (*models.User, *models.UserAPIKey, error) {
	// Check for empty key
	if key == "" {
		return nil, nil, NewAuthError(ErrCodeInvalidAPIKey, "API key is required")
	}

	// Check for solvr_sk_ prefix (user API keys)
	if !IsUserAPIKey(key) {
		return nil, nil, NewAuthError(ErrCodeInvalidAPIKey, "invalid user API key format")
	}

	// Query database for user with matching key
	user, apiKey, err := v.db.GetUserByAPIKey(ctx, key)
	if err != nil {
		return nil, nil, NewAuthError(ErrCodeInvalidAPIKey, "failed to validate API key")
	}

	// No matching key found
	if user == nil || apiKey == nil {
		return nil, nil, NewAuthError(ErrCodeInvalidAPIKey, "invalid API key")
	}

	return user, apiKey, nil
}

// IsUserAPIKey checks if a string is a user API key (has the solvr_sk_ prefix).
func IsUserAPIKey(key string) bool {
	return strings.HasPrefix(key, UserAPIKeyPrefix)
}

// UserAPIKeyMiddleware creates middleware that validates user API keys from Authorization header.
// User API keys must start with "solvr_sk_" prefix (sk = secret key).
// Returns 401 if key is missing or invalid.
// On success, attaches Claims to context (with user info) and updates last_used_at.
func UserAPIKeyMiddleware(validator *UserAPIKeyValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := extractBearerToken(r)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Check if it's a user API key (starts with solvr_sk_)
			if !IsUserAPIKey(token) {
				writeAuthError(w, NewAuthError(ErrCodeInvalidAPIKey, "invalid user API key format"))
				return
			}

			user, apiKey, err := validator.ValidateUserAPIKey(r.Context(), token)
			if err != nil {
				writeAuthError(w, err)
				return
			}

			// Update last_used_at (fire and forget - errors are not critical)
			_ = validator.db.UpdateLastUsed(r.Context(), apiKey.ID)

			// Create claims from user info and add to context
			// This allows existing JWT-based handlers to work with API key auth
			claims := &Claims{
				UserID: user.ID,
				Email:  user.Email,
				Role:   "user", // API keys default to user role
			}

			ctx := ContextWithClaims(r.Context(), claims)
			// Add API key ID for per-key rate limiting
			ctx = ContextWithAPIKeyID(ctx, apiKey.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
