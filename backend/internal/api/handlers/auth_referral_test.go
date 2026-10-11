package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// mockReferralRepoForAuth is a mock implementation of ReferralRepositoryForAuth.
type mockReferralRepoForAuth struct {
	findByCodeFn   func(ctx context.Context, code string) (string, error)
	createFn       func(ctx context.Context, referrerID, referredID string) error
	findCalled     bool
	createCalled   bool
	lastReferrerID string
	lastReferredID string
}

func (m *mockReferralRepoForAuth) FindUserIDByReferralCode(ctx context.Context, code string) (string, error) {
	m.findCalled = true
	if m.findByCodeFn != nil {
		return m.findByCodeFn(ctx, code)
	}
	return "", db.ErrNotFound
}

func (m *mockReferralRepoForAuth) CreateReferral(ctx context.Context, referrerID, referredID string) error {
	m.createCalled = true
	m.lastReferrerID = referrerID
	m.lastReferredID = referredID
	if m.createFn != nil {
		return m.createFn(ctx, referrerID, referredID)
	}
	return nil
}

// TestRegister_WithValidRef tests registration with a valid ref field creates a referral.
func TestRegister_WithValidRef(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()

	referrerID := "referrer-user-id"
	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			if code == "ABC12345" {
				return referrerID, nil
			}
			return "", db.ErrNotFound
		},
	}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, mockRefRepo)

	reqBody := RegisterRequest{
		Email:       "referred@example.com",
		Password:    "securepass123",
		Username:    "referreduser",
		DisplayName: "Referred User",
		Ref:         "ABC12345",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	if !mockRefRepo.createCalled {
		t.Error("expected CreateReferral to be called, but it was not")
	}

	if mockRefRepo.lastReferrerID != referrerID {
		t.Errorf("expected referrer ID %q, got %q", referrerID, mockRefRepo.lastReferrerID)
	}
}

// TestRegister_WithUnknownRef tests registration with unknown ref ignores it and returns 201.
func TestRegister_WithUnknownRef(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	mockRefRepo := &mockReferralRepoForAuth{
		// findByCodeFn returns ErrNotFound for any code (default behavior)
	}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, mockRefRepo)

	reqBody := RegisterRequest{
		Email:       "newuser2@example.com",
		Password:    "securepass123",
		Username:    "newuser2",
		DisplayName: "New User 2",
		Ref:         "UNKNOWN1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 with unknown ref, got %d. Body: %s", w.Code, w.Body.String())
	}

	if mockRefRepo.createCalled {
		t.Error("expected CreateReferral NOT to be called for unknown ref")
	}
}

// TestRegister_WithEmptyRef tests registration with empty ref does not invoke referral logic.
func TestRegister_WithEmptyRef(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()
	mockRefRepo := &mockReferralRepoForAuth{}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, mockRefRepo)

	reqBody := RegisterRequest{
		Email:       "newuser3@example.com",
		Password:    "securepass123",
		Username:    "newuser3",
		DisplayName: "New User 3",
		Ref:         "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 with empty ref, got %d. Body: %s", w.Code, w.Body.String())
	}

	if mockRefRepo.findCalled {
		t.Error("expected FindUserIDByReferralCode NOT to be called for empty ref")
	}

	if mockRefRepo.createCalled {
		t.Error("expected CreateReferral NOT to be called for empty ref")
	}
}

// TestRegister_SelfReferral tests that a user cannot refer themselves (no referral created).
func TestRegister_SelfReferral(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()

	// FindUserIDByReferralCode returns the same ID as the user being created
	// In the mock, Create sets ID to "mock-user-id"
	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			return "mock-user-id", nil // same as the newly created user
		},
	}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, mockRefRepo)

	reqBody := RegisterRequest{
		Email:       "selfref@example.com",
		Password:    "securepass123",
		Username:    "selfrefuser",
		DisplayName: "Self Ref User",
		Ref:         "OWNCODE1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 for self-referral, got %d. Body: %s", w.Code, w.Body.String())
	}

	if mockRefRepo.createCalled {
		t.Error("expected CreateReferral NOT to be called for self-referral")
	}
}

// TestRegister_ReferralCreateFailure tests that a referral creation failure does not block registration.
func TestRegister_ReferralCreateFailure(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}
	mockAuthMethodRepo := newMockAuthMethodRepoStub()

	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			return "referrer-id-456", nil
		},
		createFn: func(ctx context.Context, referrerID, referredID string) error {
			return errors.New("database error")
		},
	}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, mockRefRepo)

	reqBody := RegisterRequest{
		Email:       "failref@example.com",
		Password:    "securepass123",
		Username:    "failrefuser",
		DisplayName: "Fail Ref User",
		Ref:         "VALIDREF1",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	// Registration must succeed even if referral creation fails
	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201 even when referral creation fails, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestRegister_AuthMethodFailure_RollsBackUser tests that if auth_method creation fails,
// the user is deleted (compensating transaction).
func TestRegister_AuthMethodFailure_RollsBackUser(t *testing.T) {
	mockRepo := newMockUserRepoForAuth()
	config := &OAuthConfig{
		JWTSecret:     "test-secret",
		JWTExpiry:     "15m",
		RefreshExpiry: "168h",
	}

	// Mock auth method repo that always fails
	mockAuthMethodRepo := &mockAuthMethodRepoStub{
		methods:   make(map[string][]*models.AuthMethod),
		createErr: errors.New("database constraint violation"),
	}

	handler := NewAuthHandlers(config, mockRepo, mockAuthMethodRepo, nil)

	reqBody := RegisterRequest{
		Email:       "rollback@example.com",
		Password:    "securepass123",
		Username:    "rollbackuser",
		DisplayName: "Rollback User",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	// Verify registration failed
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify error response indicates registration failed
	var errResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	errorObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error object not found in response")
	}
	code := errorObj["code"].(string)
	if code != "REGISTRATION_FAILED" {
		t.Errorf("expected error code REGISTRATION_FAILED, got %s", code)
	}

	// CRITICAL VERIFICATION: User should have been deleted (rollback)
	_, exists := mockRepo.users["rollback@example.com"]
	if exists {
		t.Error("User should have been rolled back after auth_method creation failed!")
	}

	// Verify user is not in username map either
	_, exists = mockRepo.usersByUsername["rollbackuser"]
	if exists {
		t.Error("User should have been rolled back from username map after auth_method creation failed!")
	}
}
