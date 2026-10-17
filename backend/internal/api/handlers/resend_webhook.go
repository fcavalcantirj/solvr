package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// POST /v1/webhooks/resend (SPEC.md 16.6): Resend's delivery events, signed the Svix way
// (https://docs.svix.com/receiving/verifying-payloads/how-manual,
// https://resend.com/docs/webhooks/verify-webhooks-requests).

const (
	// resendWebhookMaxBody bounds a webhook body; Resend's email events are a few hundred bytes.
	resendWebhookMaxBody = 64 << 10
	// svixTolerance is how far a request's svix-timestamp may be from now, either way.
	svixTolerance = 5 * time.Minute
	// resendHardBounce is the bounce type Resend documents as the hard bounce
	// (https://resend.com/docs/dashboard/emails/email-bounces); Transient and Undetermined are not.
	resendHardBounce = "Permanent"
)

// resendMessageStatuses are the events that set a message's status, each to the event's name.
var resendMessageStatuses = map[string]string{
	"email.sent":             "sent",
	"email.delivered":        "delivered",
	"email.delivery_delayed": "delivery_delayed",
	"email.bounced":          "bounced",
	"email.complained":       "complained",
	"email.failed":           "failed",
	"email.suppressed":       "suppressed",
}

// EmailEventRepo stores a provider event once, with its effect, in one transaction. stored is
// false when the event's svix-id was already stored; then nothing changes.
type EmailEventRepo interface {
	ApplyEvent(ctx context.Context, ev *models.EmailEvent, effect models.EmailEventEffect) (stored bool, err error)
}

// ResendWebhookHandler serves POST /v1/webhooks/resend.
type ResendWebhookHandler struct {
	events EmailEventRepo
	now    func() time.Time
	log    *slog.Logger
	key    []byte // the signing key; nil refuses every request
}

// NewResendWebhookHandler builds the handler from RESEND_WEBHOOK_SECRET. Without a usable secret
// it refuses every request with 503 and logs why: nothing unverified is ever taken in.
func NewResendWebhookHandler(events EmailEventRepo, secret string) *ResendWebhookHandler {
	return newResendWebhookHandler(events, secret, slog.Default())
}

func newResendWebhookHandler(events EmailEventRepo, secret string, log *slog.Logger) *ResendWebhookHandler {
	h := &ResendWebhookHandler{events: events, now: time.Now, log: log}
	if key, ok := parseWebhookSecret(secret); ok {
		h.key = key
	} else {
		log.Error("the Resend webhook refuses every event: RESEND_WEBHOOK_SECRET is not set to a signing secret (whsec_...)")
	}
	return h
}

// parseWebhookSecret returns the signing key of a Svix secret: the base64 after "whsec_".
func parseWebhookSecret(secret string) ([]byte, bool) {
	secret = strings.TrimSpace(secret)
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) == 0 {
		return nil, false
	}
	return key, true
}

// validSvixSignature reports whether any v1 entry of the svix-signature header is the
// HMAC-SHA256 of "<id>.<timestamp>.<body>", compared in constant time.
func validSvixSignature(key []byte, id, timestamp string, body []byte, header string) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(body)
	expected := []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	for _, entry := range strings.Fields(header) {
		version, sig, ok := strings.Cut(entry, ",")
		if ok && version == "v1" && hmac.Equal([]byte(sig), expected) {
			return true
		}
	}
	return false
}

// resendEvent is the part of a Resend event Solvr reads.
type resendEvent struct {
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Data      struct {
		Bounce *struct {
			Type string `json:"type"`
		} `json:"bounce"`
		EmailID string   `json:"email_id"`
		To      []string `json:"to"`
	} `json:"data"`
}

// resendEventEffect is Solvr's rule for an event: a message status for the seven delivery
// events, a hard bounce suppresses its addresses, and a complaint suppresses and unsubscribes.
func resendEventEffect(ev *resendEvent) models.EmailEventEffect {
	effect := models.EmailEventEffect{Status: resendMessageStatuses[ev.Type]}
	switch {
	case ev.Type == "email.bounced" && ev.Data.Bounce != nil && ev.Data.Bounce.Type == resendHardBounce:
		effect.Suppress = models.EmailSuppressedHardBounce
	case ev.Type == "email.complained":
		effect.Suppress = models.EmailSuppressedComplaint
		effect.Unsubscribe = true
	}
	if effect.Suppress != "" {
		for _, to := range ev.Data.To {
			if addr, err := mail.ParseAddress(to); err == nil {
				to = addr.Address
			}
			if to = strings.ToLower(strings.TrimSpace(to)); to != "" {
				effect.Recipients = append(effect.Recipients, to)
			}
		}
	}
	return effect
}

// Receive handles POST /v1/webhooks/resend. It answers 503 without a signing secret, 401 for a
// request that is unsigned, wrongly signed or more than five minutes off, 400 for a verified body
// that is not an event, and 200 once the event is stored, or was already (duplicate true). A
// storage error answers 500, so Resend retries.
func (h *ResendWebhookHandler) Receive(w http.ResponseWriter, r *http.Request) {
	if h.key == nil {
		writeAdminError(w, http.StatusServiceUnavailable, "WEBHOOK_NOT_CONFIGURED", "the Resend webhook is not configured")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, resendWebhookMaxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeAdminError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "the webhook body is too large")
			return
		}
		writeAdminError(w, http.StatusBadRequest, "INVALID_BODY", "the webhook body could not be read")
		return
	}

	id, timestamp, signature := r.Header.Get("svix-id"), r.Header.Get("svix-timestamp"), r.Header.Get("svix-signature")
	if id == "" || timestamp == "" || signature == "" {
		writeAdminError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "the svix-id, svix-timestamp and svix-signature headers are required")
		return
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "svix-timestamp is not a number of seconds")
		return
	}
	sentAt := time.Unix(seconds, 0)
	if d := h.now().Sub(sentAt); d > svixTolerance || d < -svixTolerance {
		writeAdminError(w, http.StatusUnauthorized, "TIMESTAMP_OUT_OF_RANGE", "svix-timestamp is more than five minutes from now")
		return
	}
	if !validSvixSignature(h.key, id, timestamp, body, signature) {
		writeAdminError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "no signature matches")
		return
	}

	var ev resendEvent
	if err = json.Unmarshal(body, &ev); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_JSON", "the webhook body is not a JSON event")
		return
	}
	if ev.Type == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_EVENT", "the event has no type")
		return
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, ev.CreatedAt)
	if err != nil {
		occurredAt = sentAt.UTC()
	}
	stored := &models.EmailEvent{SvixID: id, ProviderMessageID: ev.Data.EmailID, EventType: ev.Type, OccurredAt: occurredAt}
	if ev.Data.Bounce != nil {
		stored.BounceType = ev.Data.Bounce.Type
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	isNew, err := h.events.ApplyEvent(ctx, stored, resendEventEffect(&ev))
	if err != nil {
		h.log.Error("resend webhook event not stored", "svix_id", id, "type", ev.Type, "error", err)
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "the event could not be stored")
		return
	}
	writeAdminJSON(w, http.StatusOK, map[string]bool{"received": true, "duplicate": !isNew})
}
