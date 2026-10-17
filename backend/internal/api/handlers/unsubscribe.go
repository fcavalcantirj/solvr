package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The signed links of Solvr's emails (SPEC.md 16.6). Every link carries the address and
// HMAC-SHA256(address, JWT_SECRET), so every link already sent keeps working:
//
//	POST /v1/email/unsubscribe?email=&token=  RFC 8058 one-click: any form body, no credential,
//	                                          idempotent, 200; a bad token is 403
//	GET  /v1/email/unsubscribe?email=&token=  never writes: 303 to the solvr.dev page, which
//	                                          shows the masked address and POSTs on a click
const (
	unsubscribePageBase     = "https://solvr.dev/email/unsubscribe"
	unsubscribeOneClickBase = "https://api.solvr.dev/v1/email/unsubscribe"
)

// UnsubscribeRepo sets the unsubscribed_at timestamp for a user by email.
type UnsubscribeRepo interface {
	UnsubscribeByEmail(ctx context.Context, email string) error
}

// UnsubscribeHandler handles email unsubscribe requests.
type UnsubscribeHandler struct {
	repo    UnsubscribeRepo
	hmacKey string
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

// unsubscribePageURL is the page an email's footer links: the address and token encoded, and
// the address masked for the page to show as it is.
func unsubscribePageURL(email, token string) string {
	return unsubscribePageBase + "?email=" + url.QueryEscape(email) + "&masked=" + url.QueryEscape(maskEmail(email)) +
		"&token=" + url.QueryEscape(token)
}

// oneClickUnsubscribeURL is the List-Unsubscribe URL a mail client POSTs to (RFC 8058).
func oneClickUnsubscribeURL(email, token string) string {
	return unsubscribeOneClickBase + "?email=" + url.QueryEscape(email) + "&token=" + url.QueryEscape(token)
}

// maskEmail keeps the first character of the local part and the domain: a***@example.com.
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return "***"
	}
	if at == 0 {
		return "***" + email[at:]
	}
	return email[:1] + "***" + email[at:]
}

// Unsubscribe handles POST /v1/email/unsubscribe?email=X&token=Y, RFC 8058's one-click
// unsubscribe: the body (List-Unsubscribe=One-Click, or any form body) is not read. It sets
// email_unsubscribed_at; a second call changes nothing and answers the same.
// No auth required: the HMAC token proves the user received the email.
func (h *UnsubscribeHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	email, token, ok := unsubscribeParams(w, r)
	if !ok {
		return
	}
	email, ok = h.signedAddress(email, token)
	if !ok {
		writeAdminError(w, http.StatusForbidden, "INVALID_TOKEN", "invalid unsubscribe token")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.repo.UnsubscribeByEmail(ctx, email); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to process unsubscribe")
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]string{
		"status":  "unsubscribed",
		"message": "You have been unsubscribed from Solvr emails.",
	})
}

// ShowPage handles GET /v1/email/unsubscribe?email=X&token=Y, the link of the emails sent before
// the footer linked the page: it answers 303 to the page and writes nothing, so a link scanner
// or a prefetch unsubscribes no one. The token is checked when the page POSTs.
func (h *UnsubscribeHandler) ShowPage(w http.ResponseWriter, r *http.Request) {
	email, token, ok := unsubscribeParams(w, r)
	if !ok {
		return
	}
	w.Header().Del("Content-Type")
	http.Redirect(w, r, unsubscribePageURL(email, token), http.StatusSeeOther)
}

func unsubscribeParams(w http.ResponseWriter, r *http.Request) (email, token string, ok bool) {
	email = r.URL.Query().Get("email")
	token = r.URL.Query().Get("token")
	if email == "" || token == "" {
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "email and token query parameters are required")
		return "", "", false
	}
	return email, token, true
}

// signedAddress returns the address the token was made for. Links sent before the address was
// encoded carry a '+' of the address raw, which a query reads as a space: that form is accepted
// only when its token matches, so it rescues those links and nothing else.
func (h *UnsubscribeHandler) signedAddress(email, token string) (string, bool) {
	if h.validToken(email, token) {
		return email, true
	}
	if strings.Contains(email, " ") {
		if plus := strings.ReplaceAll(email, " ", "+"); h.validToken(plus, token) {
			return plus, true
		}
	}
	return "", false
}

func (h *UnsubscribeHandler) validToken(email, token string) bool {
	return hmac.Equal([]byte(token), []byte(GenerateUnsubscribeToken(email, h.hmacKey)))
}
