package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// ReferralRepositoryInterface defines operations for referral data.
type ReferralRepositoryInterface interface {
	// GetReferralCode returns the referral code for a given user.
	GetReferralCode(ctx context.Context, userID string) (string, error)
	// CountByReferrer returns the number of users referred by the given user.
	CountByReferrer(ctx context.Context, referrerID string) (int, error)
}

// ReferralResponse is the response for GET /v1/users/me/referral.
type ReferralResponse struct {
	ReferralCode  string `json:"referral_code"`
	ReferralCount int    `json:"referral_count"`
}

// ReferralHandler handles referral endpoints.
type ReferralHandler struct {
	repo ReferralRepositoryInterface
}

// NewReferralHandler creates a new ReferralHandler.
func NewReferralHandler(repo ReferralRepositoryInterface) *ReferralHandler {
	return &ReferralHandler{repo: repo}
}

// GetMyReferral handles GET /v1/users/me/referral.
// Returns the authenticated user's referral code and referral count.
// Requires JWT authentication (human users only).
func (h *ReferralHandler) GetMyReferral(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "UNAUTHORIZED",
				"message": "authentication required",
			},
		})
		return
	}

	code, err := h.repo.GetReferralCode(r.Context(), claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "failed to get referral code",
			},
		})
		return
	}

	count, err := h.repo.CountByReferrer(r.Context(), claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "failed to get referral count",
			},
		})
		return
	}

	resp := ReferralResponse{
		ReferralCode:  code,
		ReferralCount: count,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
