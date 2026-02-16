// Package services provides business logic for Solvr.
package services

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestOAuthUserServiceAdapter_FindOrCreateUser_NewUser tests adapter with new user.
func TestOAuthUserServiceAdapter_FindOrCreateUser_NewUser(t *testing.T) {
	repo := NewMockUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)
	adapter := NewOAuthUserServiceAdapter(service)

	info := &handlers.OAuthUserInfoData{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google-123",
		Email:       "test@example.com",
		DisplayName: "Test User",
		AvatarURL:   "https://example.com/avatar.jpg",
	}

	result, isNew, err := adapter.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	if !isNew {
		t.Error("FindOrCreateUser() isNew = false, want true")
	}

	if result.Email != info.Email {
		t.Errorf("FindOrCreateUser() Email = %v, want %v", result.Email, info.Email)
	}

	if result.DisplayName != info.DisplayName {
		t.Errorf("FindOrCreateUser() DisplayName = %v, want %v", result.DisplayName, info.DisplayName)
	}

	if result.AvatarURL != info.AvatarURL {
		t.Errorf("FindOrCreateUser() AvatarURL = %v, want %v", result.AvatarURL, info.AvatarURL)
	}

	if result.Role != models.UserRoleUser {
		t.Errorf("FindOrCreateUser() Role = %v, want %v", result.Role, models.UserRoleUser)
	}

	if result.ID == "" {
		t.Error("FindOrCreateUser() ID is empty")
	}
}

// TestOAuthUserServiceAdapter_FindOrCreateUser_ExistingUser tests adapter with existing user.
func TestOAuthUserServiceAdapter_FindOrCreateUser_ExistingUser(t *testing.T) {
	repo := NewMockUserRepository()

	// Create existing user
	existingUser := &models.User{
		ID:             "existing-user-id",
		Username:       "existinguser",
		DisplayName:    "Existing User",
		Email:          "existing@example.com",
		AuthProvider:   models.AuthProviderGoogle,
		AuthProviderID: "google-existing-123",
		AvatarURL:      "https://example.com/old-avatar.jpg",
		Role:           models.UserRoleUser,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.users[existingUser.ID] = existingUser
	repo.usersByProvider["google:google-existing-123"] = existingUser
	repo.usersByEmail[existingUser.Email] = existingUser

	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)
	adapter := NewOAuthUserServiceAdapter(service)

	info := &handlers.OAuthUserInfoData{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google-existing-123",
		Email:       "existing@example.com",
		DisplayName: "Updated Name",
		AvatarURL:   "https://example.com/new-avatar.jpg",
	}

	result, isNew, err := adapter.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	if isNew {
		t.Error("FindOrCreateUser() isNew = true, want false for existing user")
	}

	if result.ID != existingUser.ID {
		t.Errorf("FindOrCreateUser() ID = %v, want %v", result.ID, existingUser.ID)
	}
}

// TestOAuthUserServiceAdapter_FindOrCreateUser_LinkByEmail tests account linking by email.
func TestOAuthUserServiceAdapter_FindOrCreateUser_LinkByEmail(t *testing.T) {
	repo := NewMockUserRepository()

	// Create existing user with different provider
	existingUser := &models.User{
		ID:             "existing-user-id",
		Username:       "existinguser",
		DisplayName:    "Existing User",
		Email:          "shared@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github-123",
		Role:           models.UserRoleUser,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.users[existingUser.ID] = existingUser
	repo.usersByProvider["github:github-123"] = existingUser
	repo.usersByEmail[existingUser.Email] = existingUser

	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)
	adapter := NewOAuthUserServiceAdapter(service)

	// New Google login with same email
	info := &handlers.OAuthUserInfoData{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google-new-123",
		Email:       "shared@example.com",
		DisplayName: "Google User",
	}

	result, isNew, err := adapter.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	// Should link to existing user by email
	if isNew {
		t.Error("FindOrCreateUser() isNew = true, want false for email linking")
	}

	if result.ID != existingUser.ID {
		t.Errorf("FindOrCreateUser() ID = %v, want %v", result.ID, existingUser.ID)
	}
}

// Verify adapter implements the handler interface
var _ handlers.OAuthUserServiceInterface = (*OAuthUserServiceAdapter)(nil)
