package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// NoticeRecipientRepo finds the one user a notice goes to: a live account with that address.
// suppressed reports an address Solvr no longer mails (a hard bounce or a complaint).
type NoticeRecipientRepo interface {
	FindNoticeRecipient(ctx context.Context, email string) (recipient *models.EmailRecipient, suppressed bool, err error)
}

// noticeRequest is the JSON body for POST /admin/email/notice.
type noticeRequest struct {
	To       string `json:"to"`
	Subject  string `json:"subject"`
	BodyHTML string `json:"body_html"`
	BodyText string `json:"body_text"`
	DryRun   bool   `json:"dry_run"`
}

// SendNotice handles POST /admin/email/notice (SPEC.md 16.6): one transactional email to one
// user, such as an account notice. It goes from FROM_EMAIL_TRANSACTIONAL with the body as given:
// no branded marketing footer and no List-Unsubscribe headers, since it is not a mailing. A user
// who unsubscribed from broadcasts still gets it; an address Solvr no longer mails does not.
// It is recorded in email_messages as kind transactional. Without the transactional sender it
// refuses (503) with a logged error, and never falls back to the broadcast sender.
// Requires X-Admin-API-Key. With dry_run=true it resolves the recipient and sends nothing.
func (h *AdminHandler) SendNotice(w http.ResponseWriter, r *http.Request) {
	if !h.checkAdminAuth(w, r) {
		return
	}
	if h.noticeSender == nil {
		h.logger().Error("transactional email refused: the notice sender is not configured "+
			"(it needs RESEND_API_KEY and FROM_EMAIL_TRANSACTIONAL, a bare address)", "route", "POST /admin/email/notice")
		writeAdminError(w, http.StatusServiceUnavailable, "TRANSACTIONAL_SENDER_NOT_CONFIGURED",
			"transactional email is not configured: FROM_EMAIL_TRANSACTIONAL is not set")
		return
	}

	var req noticeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}
	if !IsBareAddress(req.To) {
		writeAdminError(w, http.StatusBadRequest, "INVALID_RECIPIENT", "to must be one email address")
		return
	}
	if strings.TrimSpace(req.Subject) == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELD", "subject is required")
		return
	}
	if strings.TrimSpace(req.BodyHTML) == "" {
		writeAdminError(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELD", "body_html is required")
		return
	}
	if h.noticeRecipients == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "REPO_NOT_CONFIGURED", "the user repository is not configured")
		return
	}

	recipient, suppressed, err := h.noticeRecipients.FindNoticeRecipient(r.Context(), req.To)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to find the recipient")
		return
	}
	if recipient == nil {
		writeAdminError(w, http.StatusNotFound, "RECIPIENT_NOT_FOUND", "no live user has this email")
		return
	}
	if suppressed {
		writeAdminError(w, http.StatusConflict, "RECIPIENT_SUPPRESSED",
			"Solvr no longer mails this address: it hard-bounced or its owner reported spam")
		return
	}

	if req.DryRun {
		writeAdminJSON(w, http.StatusOK, map[string]interface{}{
			"would_send":       1,
			"kind":             models.EmailKindTransactional,
			"recipient_id":     recipient.ID,
			"from":             senderAddress(h.noticeSender),
			"footer":           false,
			"list_unsubscribe": false,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	msg := &models.EmailMessage{Kind: models.EmailKindTransactional, UserID: recipient.ID, Subject: req.Subject,
		Status: models.EmailStatusSent}
	providerID, err := h.noticeSender.Send(ctx, recipient.Email, req.Subject, req.BodyHTML, req.BodyText)
	if err != nil {
		msg.Status = models.EmailStatusFailed
		h.logger().Error("email send failed", "kind", msg.Kind, "user_id", recipient.ID, "error", redactAddress(err, recipient.Email))
		h.recordMessage(ctx, msg)
		writeAdminError(w, http.StatusBadGateway, "EMAIL_SEND_FAILED", "the email provider refused the notice")
		return
	}
	msg.ProviderMessageID = providerID
	h.recordMessage(ctx, msg)

	writeAdminJSON(w, http.StatusOK, map[string]interface{}{
		"sent":                true,
		"kind":                models.EmailKindTransactional,
		"recipient_id":        recipient.ID,
		"provider_message_id": providerID,
	})
}

// IsBareAddress reports one address with no display name and no list: "a@b.c", never
// "Name <a@b.c>" or "a@b.c, d@e.f". A notice's recipient and FROM_EMAIL_TRANSACTIONAL are both.
func IsBareAddress(s string) bool {
	if strings.TrimSpace(s) != s || s == "" {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Name == "" && addr.Address == s
}

// senderAddress is the address a sender sends from, when it says.
func senderAddress(sender EmailSender) string {
	if s, ok := sender.(interface{ FromAddress() string }); ok {
		return s.FromAddress()
	}
	return ""
}
