package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// UnsubscribeRepo sets the unsubscribed_at timestamp for a user by email.
type UnsubscribeRepo interface {
	UnsubscribeByEmail(ctx context.Context, email string) error
}

// UnsubscribeHandler handles email unsubscribe requests.
type UnsubscribeHandler struct {
	repo      UnsubscribeRepo
	hmacKey   string
}

// NewUnsubscribeHandler creates a new UnsubscribeHandler.
func NewUnsubscribeHandler(repo UnsubscribeRepo, hmacKey string) *UnsubscribeHandler {
	return &UnsubscribeHandler{repo: repo, hmacKey: hmacKey}
}

// GenerateUnsubscribeToken creates an HMAC-SHA256 token for the given email.
// Used by the broadcast handler to generate per-recipient unsubscribe URLs.
func GenerateUnsubscribeToken(email, hmacKey string) string {
	mac := hmac.New(sha256.New, []byte(hmacKey))
	mac.Write([]byte(email))
	return hex.EncodeToString(mac.Sum(nil))
}

// Unsubscribe handles GET /v1/email/unsubscribe?email=X&token=Y
// Validates the HMAC token and sets email_unsubscribed_at on the user.
// No auth required — the HMAC token proves the user received the email.
func (h *UnsubscribeHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	token := r.URL.Query().Get("token")

	if email == "" || token == "" {
		writeAdminJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "MISSING_PARAMS",
			"message": "email and token query parameters are required",
		})
		return
	}

	// Validate HMAC token
	expected := GenerateUnsubscribeToken(email, h.hmacKey)
	if !hmac.Equal([]byte(token), []byte(expected)) {
		writeAdminJSON(w, http.StatusForbidden, map[string]string{
			"error":   "INVALID_TOKEN",
			"message": "invalid unsubscribe token",
		})
		return
	}

	// Set email_unsubscribed_at
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.repo.UnsubscribeByEmail(ctx, email); err != nil {
		writeAdminJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "INTERNAL_ERROR",
			"message": "failed to process unsubscribe",
		})
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]string{
		"status":  "unsubscribed",
		"message": "You have been unsubscribed from Solvr emails.",
	})
}
