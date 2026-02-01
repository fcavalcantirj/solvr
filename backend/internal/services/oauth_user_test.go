// Package services provides business logic for Solvr.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockUserRepository is a mock implementation of UserRepository for testing.
type MockUserRepository struct {
	users               map[string]*models.User
	usersByProvider     map[string]*models.User // key: provider:providerID
	usersByEmail        map[string]*models.User
	createErr           error
	findByIDErr         error
	findByAuthProviderErr error
	findByEmailErr      error
}

// NewMockUserRepository creates a new MockUserRepository.
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:           make(map[string]*models.User),
		usersByProvider: make(map[string]*models.User),
		usersByEmail:    make(map[string]*models.User),
	}
}

// Create creates a new user in the mock.
func (m *MockUserRepository) Create(ctx context.Context, user *models.User) (*models.User, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	// Check for duplicate username
	for _, u := range m.users {
		if u.Username == user.Username {
			return nil, db.ErrDuplicateUsername
		}
		if u.Email == user.Email {
			return nil, db.ErrDuplicateEmail
		}
	}

	// Generate ID and timestamps
	created := &models.User{
		ID:             "generated-uuid-" + user.Username,
		Username:       user.Username,
		DisplayName:    user.DisplayName,
		Email:          user.Email,
		AuthProvider:   user.AuthProvider,
		AuthProviderID: user.AuthProviderID,
		AvatarURL:      user.AvatarURL,
		Bio:            user.Bio,
		Role:           user.Role,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.users[created.ID] = created
	m.usersByProvider[user.AuthProvider+":"+user.AuthProviderID] = created
	m.usersByEmail[user.Email] = created

	return created, nil
}

// FindByID finds a user by ID in the mock.
func (m *MockUserRepository) FindByID(ctx context.Context, id string) (*models.User, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, db.ErrNotFound
}

// FindByAuthProvider finds a user by OAuth provider in the mock.
func (m *MockUserRepository) FindByAuthProvider(ctx context.Context, provider, providerID string) (*models.User, error) {
	if m.findByAuthProviderErr != nil {
		return nil, m.findByAuthProviderErr
	}
	key := provider + ":" + providerID
	if user, ok := m.usersByProvider[key]; ok {
		return user, nil
	}
	return nil, db.ErrNotFound
}

// FindByEmail finds a user by email in the mock.
func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	if user, ok := m.usersByEmail[email]; ok {
		return user, nil
	}
	return nil, db.ErrNotFound
}

// Update updates a user in the mock.
func (m *MockUserRepository) Update(ctx context.Context, user *models.User) (*models.User, error) {
	if existing, ok := m.users[user.ID]; ok {
		existing.DisplayName = user.DisplayName
		existing.AvatarURL = user.AvatarURL
		existing.Bio = user.Bio
		existing.UpdatedAt = time.Now()
		return existing, nil
	}
	return nil, db.ErrNotFound
}

// TestOAuthUserService_FindOrCreateUser_NewUser tests creating a new user.
func TestOAuthUserService_FindOrCreateUser_NewUser(t *testing.T) {
	repo := NewMockUserRepository()
	service := NewOAuthUserService(repo)

	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  "12345",
		Email:       "newuser@example.com",
		DisplayName: "New User",
		AvatarURL:   "https://example.com/avatar.png",
	}

	user, isNew, err := service.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	if !isNew {
		t.Error("FindOrCreateUser() isNew = false, want true")
	}

	if user.Email != info.Email {
		t.Errorf("FindOrCreateUser() Email = %v, want %v", user.Email, info.Email)
	}

	if user.AuthProvider != models.AuthProviderGitHub {
		t.Errorf("FindOrCreateUser() AuthProvider = %v, want %v", user.AuthProvider, models.AuthProviderGitHub)
	}

	if user.AuthProviderID != info.ProviderID {
		t.Errorf("FindOrCreateUser() AuthProviderID = %v, want %v", user.AuthProviderID, info.ProviderID)
	}
}

// TestOAuthUserService_FindOrCreateUser_ExistingByProvider tests finding existing user by provider.
func TestOAuthUserService_FindOrCreateUser_ExistingByProvider(t *testing.T) {
	repo := NewMockUserRepository()
	service := NewOAuthUserService(repo)

	// Pre-create a user
	existingUser := &models.User{
		ID:             "existing-id",
		Username:       "existinguser",
		DisplayName:    "Existing User",
		Email:          "existing@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "existing_123",
		Role:           models.UserRoleUser,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.users[existingUser.ID] = existingUser
	repo.usersByProvider["github:existing_123"] = existingUser
	repo.usersByEmail[existingUser.Email] = existingUser

	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  "existing_123",
		Email:       "existing@example.com",
		DisplayName: "Existing User",
		AvatarURL:   "https://example.com/avatar.png",
	}

	user, isNew, err := service.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	if isNew {
		t.Error("FindOrCreateUser() isNew = true, want false")
	}

	if user.ID != existingUser.ID {
		t.Errorf("FindOrCreateUser() ID = %v, want %v", user.ID, existingUser.ID)
	}
}

// TestOAuthUserService_FindOrCreateUser_LinkByEmail tests linking account by email.
// Per SPEC.md: If email matches existing user, link accounts.
func TestOAuthUserService_FindOrCreateUser_LinkByEmail(t *testing.T) {
	repo := NewMockUserRepository()
	service := NewOAuthUserService(repo)

	// Pre-create a user with GitHub
	existingUser := &models.User{
		ID:             "existing-id",
		Username:       "githubuser",
		DisplayName:    "GitHub User",
		Email:          "shared@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "github_123",
		Role:           models.UserRoleUser,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.users[existingUser.ID] = existingUser
	repo.usersByProvider["github:github_123"] = existingUser
	repo.usersByEmail[existingUser.Email] = existingUser

	// Try to login with Google using same email
	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google_456",
		Email:       "shared@example.com", // Same email
		DisplayName: "Google User",
		AvatarURL:   "https://google.com/avatar.png",
	}

	user, isNew, err := service.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	// Should return the existing user (linked by email)
	if isNew {
		t.Error("FindOrCreateUser() isNew = true, want false (should link by email)")
	}

	if user.ID != existingUser.ID {
		t.Errorf("FindOrCreateUser() ID = %v, want %v", user.ID, existingUser.ID)
	}
}

// TestOAuthUserService_FindOrCreateUser_GeneratesUsername tests username generation.
func TestOAuthUserService_FindOrCreateUser_GeneratesUsername(t *testing.T) {
	repo := NewMockUserRepository()
	service := NewOAuthUserService(repo)

	// Info with display name that can be converted to username
	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  "user_123",
		Email:       "john.doe@example.com",
		DisplayName: "John Doe",
		AvatarURL:   "https://example.com/avatar.png",
	}

	user, _, err := service.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	// Username should be generated from display name or email
	if user.Username == "" {
		t.Error("FindOrCreateUser() Username is empty")
	}
}

// TestOAuthUserService_FindOrCreateUser_DuplicateUsername tests handling duplicate username.
func TestOAuthUserService_FindOrCreateUser_DuplicateUsername(t *testing.T) {
	repo := NewMockUserRepository()
	service := NewOAuthUserService(repo)

	// Pre-create a user with username "johndoe"
	existingUser := &models.User{
		ID:             "existing-id",
		Username:       "johndoe",
		DisplayName:    "Existing John",
		Email:          "existingjohn@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "existing_john",
		Role:           models.UserRoleUser,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.users[existingUser.ID] = existingUser
	repo.usersByProvider["github:existing_john"] = existingUser
	repo.usersByEmail[existingUser.Email] = existingUser

	// Try to create user with same username base
	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "new_john",
		Email:       "newjohn@example.com",
		DisplayName: "John Doe", // Will try to generate "johndoe"
		AvatarURL:   "https://example.com/avatar.png",
	}

	user, isNew, err := service.FindOrCreateUser(context.Background(), info)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	if !isNew {
		t.Error("FindOrCreateUser() isNew = false, want true")
	}

	// Username should be different (with suffix)
	if user.Username == "johndoe" {
		t.Error("FindOrCreateUser() should generate unique username with suffix")
	}
}

// TestOAuthUserService_FindOrCreateUser_DatabaseError tests database error handling.
func TestOAuthUserService_FindOrCreateUser_DatabaseError(t *testing.T) {
	repo := NewMockUserRepository()
	repo.findByAuthProviderErr = errors.New("database connection error")
	service := NewOAuthUserService(repo)

	info := &OAuthUserInfo{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  "12345",
		Email:       "test@example.com",
		DisplayName: "Test User",
	}

	_, _, err := service.FindOrCreateUser(context.Background(), info)
	if err == nil {
		t.Error("FindOrCreateUser() error = nil, want database error")
	}
}
