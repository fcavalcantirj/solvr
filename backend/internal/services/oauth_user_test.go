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
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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

	// Verify auth_method was created
	methods, _ := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if len(methods) != 1 {
		t.Fatalf("Expected 1 auth method, got %d", len(methods))
	}

	if methods[0].AuthProvider != models.AuthProviderGitHub {
		t.Errorf("Auth method provider = %v, want %v", methods[0].AuthProvider, models.AuthProviderGitHub)
	}

	if methods[0].AuthProviderID != info.ProviderID {
		t.Errorf("Auth method provider ID = %v, want %v", methods[0].AuthProviderID, info.ProviderID)
	}
}

// TestOAuthUserService_FindOrCreateUser_ExistingByProvider tests finding existing user by provider.
func TestOAuthUserService_FindOrCreateUser_ExistingByProvider(t *testing.T) {
	repo := NewMockUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(repo, authMethodRepo)

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

// MockAuthMethodRepository is a mock implementation of AuthMethodRepository for testing.
type MockAuthMethodRepository struct {
	methods    map[string]*models.AuthMethod // key: id
	methodsByUser map[string][]*models.AuthMethod // key: user_id
	methodsByProvider map[string]*models.AuthMethod // key: provider:providerID
	createErr  error
}

// NewMockAuthMethodRepository creates a new MockAuthMethodRepository.
func NewMockAuthMethodRepository() *MockAuthMethodRepository {
	return &MockAuthMethodRepository{
		methods:           make(map[string]*models.AuthMethod),
		methodsByUser:     make(map[string][]*models.AuthMethod),
		methodsByProvider: make(map[string]*models.AuthMethod),
	}
}

// Create creates a new auth method in the mock.
func (m *MockAuthMethodRepository) Create(ctx context.Context, method *models.AuthMethod) (*models.AuthMethod, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	// Check for duplicate provider per user
	if existing, ok := m.methodsByUser[method.UserID]; ok {
		for _, e := range existing {
			if e.AuthProvider == method.AuthProvider {
				return nil, errors.New("user already has this auth provider")
			}
			// Check OAuth ID uniqueness (only for OAuth providers)
			if method.AuthProviderID != "" && e.AuthProviderID == method.AuthProviderID {
				return nil, errors.New("oauth provider ID already in use")
			}
		}
	}

	// Generate ID and timestamp
	created := &models.AuthMethod{
		ID:             "auth-method-" + method.UserID + "-" + method.AuthProvider,
		UserID:         method.UserID,
		AuthProvider:   method.AuthProvider,
		AuthProviderID: method.AuthProviderID,
		PasswordHash:   method.PasswordHash,
		CreatedAt:      time.Now(),
		LastUsedAt:     time.Now(),
	}

	m.methods[created.ID] = created
	m.methodsByUser[method.UserID] = append(m.methodsByUser[method.UserID], created)
	if method.AuthProviderID != "" {
		key := method.AuthProvider + ":" + method.AuthProviderID
		m.methodsByProvider[key] = created
	}

	return created, nil
}

// FindByUserID returns all auth methods for a user.
func (m *MockAuthMethodRepository) FindByUserID(ctx context.Context, userID string) ([]*models.AuthMethod, error) {
	if methods, ok := m.methodsByUser[userID]; ok {
		return methods, nil
	}
	return []*models.AuthMethod{}, nil
}

// UpdateLastUsed updates the last used timestamp.
func (m *MockAuthMethodRepository) UpdateLastUsed(ctx context.Context, methodID string) error {
	if method, ok := m.methods[methodID]; ok {
		method.LastUsedAt = time.Now()
		return nil
	}
	return db.ErrNotFound
}

// TestOAuthUserService_AccountLinking_CreatesAuthMethod tests that OAuth linking creates auth_method.
// This is the CRITICAL BUG: when a user with email/password logs in with OAuth (same email),
// the system finds the user by email but never creates the auth_method record for OAuth.
func TestOAuthUserService_AccountLinking_CreatesAuthMethod(t *testing.T) {
	userRepo := NewMockUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(userRepo, authMethodRepo)

	// Step 1: Create a user with email/password
	emailUser := &models.User{
		Username:    "testuser",
		DisplayName: "Test User",
		Email:       "user@example.com",
		Role:        models.UserRoleUser,
	}
	createdUser, err := userRepo.Create(context.Background(), emailUser)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create email auth method
	emailMethod := &models.AuthMethod{
		UserID:       createdUser.ID,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: "hashed_password",
	}
	_, err = authMethodRepo.Create(context.Background(), emailMethod)
	if err != nil {
		t.Fatalf("Failed to create email auth method: %v", err)
	}

	// Verify user has only email auth method
	methods, _ := authMethodRepo.FindByUserID(context.Background(), createdUser.ID)
	if len(methods) != 1 {
		t.Fatalf("Expected 1 auth method, got %d", len(methods))
	}
	if methods[0].AuthProvider != models.AuthProviderEmail {
		t.Errorf("Expected email provider, got %s", methods[0].AuthProvider)
	}

	// Step 2: User logs in with Google OAuth (same email)
	googleInfo := &OAuthUserInfo{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google-12345",
		Email:       "user@example.com", // Same email!
		DisplayName: "Test User",
		AvatarURL:   "https://google.com/avatar.png",
	}

	returnedUser, isNew, err := service.FindOrCreateUser(context.Background(), googleInfo)
	if err != nil {
		t.Fatalf("FindOrCreateUser() error = %v", err)
	}

	// Should return existing user (not create new one)
	if isNew {
		t.Error("FindOrCreateUser() isNew = true, want false (should link to existing account)")
	}

	if returnedUser.ID != createdUser.ID {
		t.Errorf("FindOrCreateUser() returned different user: got %s, want %s", returnedUser.ID, createdUser.ID)
	}

	// Step 3: CRITICAL - Verify Google auth_method was created
	methods, _ = authMethodRepo.FindByUserID(context.Background(), createdUser.ID)
	if len(methods) != 2 {
		t.Errorf("Expected 2 auth methods (email + google), got %d", len(methods))
		for i, m := range methods {
			t.Logf("  method[%d]: %s", i, m.AuthProvider)
		}
		t.Fatal("BUG: OAuth linking did not create auth_method record!")
	}

	// Verify both providers exist
	hasEmail := false
	hasGoogle := false
	for _, m := range methods {
		if m.AuthProvider == models.AuthProviderEmail {
			hasEmail = true
		}
		if m.AuthProvider == models.AuthProviderGoogle {
			hasGoogle = true
			if m.AuthProviderID != googleInfo.ProviderID {
				t.Errorf("Google auth_method has wrong provider ID: got %s, want %s", m.AuthProviderID, googleInfo.ProviderID)
			}
		}
	}

	if !hasEmail {
		t.Error("Missing email auth_method after OAuth linking")
	}
	if !hasGoogle {
		t.Error("Missing Google auth_method after OAuth linking - THIS IS THE BUG!")
	}
}

// TestOAuthUserService_AccountLinking_MultipleProviders tests linking all three auth methods.
func TestOAuthUserService_AccountLinking_MultipleProviders(t *testing.T) {
	userRepo := NewMockUserRepository()
	authMethodRepo := NewMockAuthMethodRepository()
	service := NewOAuthUserService(userRepo, authMethodRepo)

	// Step 1: User registers with email/password
	emailUser := &models.User{
		Username:    "multiuser",
		DisplayName: "Multi User",
		Email:       "multi@example.com",
		Role:        models.UserRoleUser,
	}
	user, err := userRepo.Create(context.Background(), emailUser)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	emailMethod := &models.AuthMethod{
		UserID:       user.ID,
		AuthProvider: models.AuthProviderEmail,
		PasswordHash: "hashed",
	}
	_, err = authMethodRepo.Create(context.Background(), emailMethod)
	if err != nil {
		t.Fatalf("Failed to create email auth: %v", err)
	}

	// Step 2: User logs in with Google (should link)
	googleInfo := &OAuthUserInfo{
		Provider:    models.AuthProviderGoogle,
		ProviderID:  "google-999",
		Email:       "multi@example.com",
		DisplayName: "Multi User",
	}
	_, isNew, err := service.FindOrCreateUser(context.Background(), googleInfo)
	if err != nil {
		t.Fatalf("Google login error: %v", err)
	}
	if isNew {
		t.Error("Google login should link to existing user, not create new")
	}

	// Step 3: User logs in with GitHub (should link)
	githubInfo := &OAuthUserInfo{
		Provider:    models.AuthProviderGitHub,
		ProviderID:  "github-888",
		Email:       "multi@example.com",
		DisplayName: "Multi User",
	}
	_, isNew, err = service.FindOrCreateUser(context.Background(), githubInfo)
	if err != nil {
		t.Fatalf("GitHub login error: %v", err)
	}
	if isNew {
		t.Error("GitHub login should link to existing user, not create new")
	}

	// Step 4: Verify all three auth methods exist
	methods, _ := authMethodRepo.FindByUserID(context.Background(), user.ID)
	if len(methods) != 3 {
		t.Fatalf("Expected 3 auth methods (email, google, github), got %d", len(methods))
	}

	providers := make(map[string]bool)
	for _, m := range methods {
		providers[m.AuthProvider] = true
	}

	if !providers[models.AuthProviderEmail] {
		t.Error("Missing email auth method")
	}
	if !providers[models.AuthProviderGoogle] {
		t.Error("Missing Google auth method")
	}
	if !providers[models.AuthProviderGitHub] {
		t.Error("Missing GitHub auth method")
	}
}
