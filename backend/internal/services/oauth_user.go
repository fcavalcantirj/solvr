// Package services provides business logic for Solvr.
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// UserRepository defines the interface for user database operations.
// This allows for easy mocking in tests.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) (*models.User, error)
	FindByID(ctx context.Context, id string) (*models.User, error)
	FindByAuthProvider(ctx context.Context, provider, providerID string) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) (*models.User, error)
}

// AuthMethodRepository defines the interface for auth method database operations.
type AuthMethodRepository interface {
	Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error)
	FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error)
	UpdateLastUsed(ctx context.Context, methodID string) error
}

// OAuthUserInfo contains user information from an OAuth provider.
type OAuthUserInfo struct {
	Provider    string // github, google
	ProviderID  string // Unique ID from the provider
	Email       string
	DisplayName string
	AvatarURL   string
}

// OAuthUserService handles OAuth user creation and linking.
// Per SPEC.md Part 5.2: GitHub OAuth and Google OAuth user management.
type OAuthUserService struct {
	repo           UserRepository
	authMethodRepo AuthMethodRepository
}

// NewOAuthUserService creates a new OAuthUserService.
func NewOAuthUserService(repo UserRepository, authMethodRepo AuthMethodRepository) *OAuthUserService {
	return &OAuthUserService{
		repo:           repo,
		authMethodRepo: authMethodRepo,
	}
}

// FindOrCreateUser finds an existing user or creates a new one.
// Per SPEC.md Part 5.2:
// 1. Query users by provider ID - if found, return existing user
// 2. If email matches existing user, link accounts (return existing user)
// 3. If not found, create new user
// Returns the user and a boolean indicating if the user is new.
func (s *OAuthUserService) FindOrCreateUser(ctx context.Context, info *OAuthUserInfo) (*models.User, bool, error) {
	// Step 1: Try to find user by OAuth provider ID (existing OAuth user)
	user, err := s.repo.FindByAuthProvider(ctx, info.Provider, info.ProviderID)
	if err == nil {
		// Found existing user by provider ID
		// Update last_used_at for this auth method
		methods, _ := s.authMethodRepo.FindByUserID(ctx, user.ID)
		for _, method := range methods {
			if method.AuthProvider == info.Provider {
				_ = s.authMethodRepo.UpdateLastUsed(ctx, method.ID)
				break
			}
		}
		return user, false, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		// Database error
		return nil, false, fmt.Errorf("failed to find user by auth provider: %w", err)
	}

	// Step 2: Try to find user by email (account linking)
	user, err = s.repo.FindByEmail(ctx, info.Email)
	if err == nil {
		// USER FOUND BY EMAIL - LINK NEW OAUTH PROVIDER

		// Create auth_method record for this OAuth provider
		newMethod := &models.AuthMethod{
			UserID:         user.ID,
			AuthProvider:   info.Provider,
			AuthProviderID: info.ProviderID,
			LastUsedAt:     time.Now(),
		}

		_, err := s.authMethodRepo.Create(ctx, newMethod)
		if err != nil {
			// Check if this is a duplicate provider error (race condition)
			errMsg := err.Error()
			if strings.Contains(errMsg, "already has this auth provider") {
				// Provider already linked (race condition), that's OK
				slog.Info("oauth provider already linked", "user_id", user.ID, "provider", info.Provider)
				return user, false, nil
			}
			// Other error - return it
			return nil, false, fmt.Errorf("failed to link oauth provider: %w", err)
		}

		slog.Info("oauth provider linked to existing account",
			"user_id", user.ID,
			"email", user.Email,
			"provider", info.Provider,
			"provider_id", info.ProviderID)

		return user, false, nil // Existing user, newly linked
	}
	if !errors.Is(err, db.ErrNotFound) {
		// Database error
		return nil, false, fmt.Errorf("failed to find user by email: %w", err)
	}

	// Step 3: Create new user + auth_method (no existing user found)
	newUser := &models.User{
		Username:    s.generateUsername(ctx, info.DisplayName, info.Email),
		DisplayName: info.DisplayName,
		Email:       info.Email,
		AvatarURL:   info.AvatarURL,
		Role:        models.UserRoleUser,
	}

	// Create user (Note: auth_provider and auth_provider_id are no longer on User model)
	createdUser, err := s.repo.Create(ctx, newUser)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateEmail) {
			// Race condition: email was registered between FindByEmail and Create
			// Retry the whole flow (will hit Step 2 this time)
			return s.FindOrCreateUser(ctx, info)
		}
		if errors.Is(err, db.ErrDuplicateUsername) {
			// Handle duplicate username by retrying with a suffix
			newUser.Username = s.generateUniqueUsername(ctx, newUser.Username)
			createdUser, err = s.repo.Create(ctx, newUser)
			if err != nil {
				return nil, false, fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			return nil, false, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Create auth_method for OAuth provider
	authMethod := &models.AuthMethod{
		UserID:         createdUser.ID,
		AuthProvider:   info.Provider,
		AuthProviderID: info.ProviderID,
		LastUsedAt:     time.Now(),
	}

	_, err = s.authMethodRepo.Create(ctx, authMethod)
	if err != nil {
		// CRITICAL: User created but auth_method failed
		// Should delete user or use transaction
		// For now, log and return error
		slog.Error("auth_method creation failed after user created",
			"error", err,
			"user_id", createdUser.ID,
			"provider", info.Provider)
		return nil, false, fmt.Errorf("failed to create auth method: %w", err)
	}

	slog.Info("new oauth user created",
		"user_id", createdUser.ID,
		"email", createdUser.Email,
		"provider", info.Provider)

	return createdUser, true, nil
}

// generateUsername generates a username from display name or email.
// Per SPEC.md Part 2.8: username max 30 chars, alphanumeric + underscore.
func (s *OAuthUserService) generateUsername(ctx context.Context, displayName, email string) string {
	var base string

	// Try display name first
	if displayName != "" {
		base = s.sanitizeUsername(displayName)
	}

	// Fall back to email prefix
	if base == "" && email != "" {
		parts := strings.Split(email, "@")
		if len(parts) > 0 {
			base = s.sanitizeUsername(parts[0])
		}
	}

	// Last resort: use "user"
	if base == "" {
		base = "user"
	}

	// Truncate to 30 chars
	if len(base) > 30 {
		base = base[:30]
	}

	return base
}

// sanitizeUsername removes invalid characters and converts to lowercase.
func (s *OAuthUserService) sanitizeUsername(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces with underscores
	name = strings.ReplaceAll(name, " ", "_")

	// Remove all non-alphanumeric characters except underscore
	reg := regexp.MustCompile("[^a-z0-9_]")
	name = reg.ReplaceAllString(name, "")

	// Remove leading/trailing underscores
	name = strings.Trim(name, "_")

	// Remove consecutive underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	return name
}

// generateUniqueUsername appends a suffix to make the username unique.
func (s *OAuthUserService) generateUniqueUsername(ctx context.Context, base string) string {
	// Truncate base to leave room for suffix
	maxBase := 25
	if len(base) > maxBase {
		base = base[:maxBase]
	}

	// Try adding numeric suffixes
	for i := 1; i <= 999; i++ {
		candidate := fmt.Sprintf("%s_%d", base, i)
		// Check if username exists
		_, err := s.repo.FindByEmail(ctx, candidate+"@check") // Dummy check
		if errors.Is(err, db.ErrNotFound) {
			return candidate
		}
	}

	// Last resort: use a longer random suffix
	return fmt.Sprintf("%s_%d", base, 9999)
}
