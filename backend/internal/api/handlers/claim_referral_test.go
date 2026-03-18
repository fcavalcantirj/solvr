package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

func newClaimTestHandler(refRepo *mockReferralRepoForAuth) *AuthHandlers {
	return NewAuthHandlers(
		&OAuthConfig{JWTSecret: "test-secret", JWTExpiry: "15m", RefreshExpiry: "168h"},
		newMockUserRepoForAuth(),
		newMockAuthMethodRepoStub(),
		refRepo,
	)
}

func claimRequest(ref string, userID string) (*http.Request, *httptest.ResponseRecorder) {
	body, _ := json.Marshal(map[string]string{"ref": ref})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/claim-referral", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		claims := &auth.Claims{UserID: userID, Role: "user"}
		ctx := auth.ContextWithClaims(req.Context(), claims)
		req = req.WithContext(ctx)
	}
	return req, httptest.NewRecorder()
}

func TestClaimReferral_Unauthenticated(t *testing.T) {
	handler := newClaimTestHandler(&mockReferralRepoForAuth{})

	body, _ := json.Marshal(map[string]string{"ref": "ABC12345"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/claim-referral", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No claims in context
	w := httptest.NewRecorder()

	handler.ClaimReferral(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestClaimReferral_EmptyRef(t *testing.T) {
	mockRefRepo := &mockReferralRepoForAuth{}
	handler := newClaimTestHandler(mockRefRepo)

	req, w := claimRequest("", "user-123")
	handler.ClaimReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %q", resp["status"])
	}
	if mockRefRepo.createCalled {
		t.Error("CreateReferral should not be called for empty ref")
	}
}

func TestClaimReferral_ValidRef(t *testing.T) {
	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			if code == "ABC12345" {
				return "referrer-id", nil
			}
			return "", db.ErrNotFound
		},
	}
	handler := newClaimTestHandler(mockRefRepo)

	req, w := claimRequest("ABC12345", "new-user-id")
	handler.ClaimReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "claimed" {
		t.Errorf("expected status 'claimed', got %q", resp["status"])
	}
	if !mockRefRepo.createCalled {
		t.Error("expected CreateReferral to be called")
	}
	if mockRefRepo.lastReferrerID != "referrer-id" {
		t.Errorf("expected referrer 'referrer-id', got %q", mockRefRepo.lastReferrerID)
	}
	if mockRefRepo.lastReferredID != "new-user-id" {
		t.Errorf("expected referred 'new-user-id', got %q", mockRefRepo.lastReferredID)
	}
}

func TestClaimReferral_UnknownCode(t *testing.T) {
	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			return "", db.ErrNotFound
		},
	}
	handler := newClaimTestHandler(mockRefRepo)

	req, w := claimRequest("BADCODE1", "user-123")
	handler.ClaimReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %q", resp["status"])
	}
	if mockRefRepo.createCalled {
		t.Error("CreateReferral should not be called for unknown code")
	}
}

func TestClaimReferral_SelfReferral(t *testing.T) {
	mockRefRepo := &mockReferralRepoForAuth{
		findByCodeFn: func(ctx context.Context, code string) (string, error) {
			return "same-user-id", nil // Returns the same user
		},
	}
	handler := newClaimTestHandler(mockRefRepo)

	req, w := claimRequest("MYCODE12", "same-user-id")
	handler.ClaimReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %q", resp["status"])
	}
	if mockRefRepo.createCalled {
		t.Error("CreateReferral should not be called for self-referral")
	}
}

func TestClaimReferral_NilReferralRepo(t *testing.T) {
	handler := NewAuthHandlers(
		&OAuthConfig{JWTSecret: "test-secret", JWTExpiry: "15m", RefreshExpiry: "168h"},
		newMockUserRepoForAuth(),
		newMockAuthMethodRepoStub(),
		nil, // no referral repo
	)

	req, w := claimRequest("ABC12345", "user-123")
	handler.ClaimReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "skipped" {
		t.Errorf("expected status 'skipped', got %q", resp["status"])
	}
}
