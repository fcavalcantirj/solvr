package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// mockReferralRepo implements ReferralRepositoryInterface for testing.
type mockReferralRepo struct {
	code     string
	codeErr  error
	count    int
	countErr error
}

func (m *mockReferralRepo) GetReferralCode(ctx context.Context, userID string) (string, error) {
	return m.code, m.codeErr
}

func (m *mockReferralRepo) CountByReferrer(ctx context.Context, referrerID string) (int, error) {
	return m.count, m.countErr
}

func TestGetMyReferral_Unauthenticated(t *testing.T) {
	handler := NewReferralHandler(&mockReferralRepo{code: "ABCD1234", count: 0})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/referral", nil)
	// No auth context — unauthenticated
	w := httptest.NewRecorder()
	handler.GetMyReferral(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetMyReferral_Success_ZeroReferrals(t *testing.T) {
	handler := NewReferralHandler(&mockReferralRepo{code: "ABCD1234", count: 0})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/referral", nil)
	// Inject JWT claims into context
	claims := &auth.Claims{UserID: "user-123", Role: "user"}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetMyReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["referral_code"] != "ABCD1234" {
		t.Errorf("expected referral_code ABCD1234, got %v", resp["referral_code"])
	}
	if resp["referral_count"] != float64(0) {
		t.Errorf("expected referral_count 0, got %v", resp["referral_count"])
	}
}

func TestGetMyReferral_Success_WithReferrals(t *testing.T) {
	handler := NewReferralHandler(&mockReferralRepo{code: "XY9Z1234", count: 5})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/referral", nil)
	claims := &auth.Claims{UserID: "user-456", Role: "user"}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetMyReferral(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["referral_code"] != "XY9Z1234" {
		t.Errorf("expected referral_code XY9Z1234, got %v", resp["referral_code"])
	}
	if resp["referral_count"] != float64(5) {
		t.Errorf("expected referral_count 5, got %v", resp["referral_count"])
	}
}

func TestGetMyReferral_CodeLookupError(t *testing.T) {
	handler := NewReferralHandler(&mockReferralRepo{
		codeErr: errors.New("db error"),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/referral", nil)
	claims := &auth.Claims{UserID: "user-789", Role: "user"}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetMyReferral(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetMyReferral_CountError(t *testing.T) {
	handler := NewReferralHandler(&mockReferralRepo{
		code:     "ABCD1234",
		countErr: errors.New("count error"),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me/referral", nil)
	claims := &auth.Claims{UserID: "user-000", Role: "user"}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.GetMyReferral(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
