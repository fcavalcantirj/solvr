package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// POST /v1/webhooks/resend (SPEC.md 16.6) takes Resend's events, signed the Svix way: the signed
// content is "<svix-id>.<svix-timestamp>.<raw body>", HMAC-SHA256 keyed with the base64 part of
// RESEND_WEBHOOK_SECRET (after "whsec_"); any "v1,<signature>" entry of the space-separated
// svix-signature header may match, compared in constant time, and a timestamp more than five
// minutes from now is refused. Without a usable secret it refuses every request (503). Each event
// is stored once by its svix-id; a Permanent bounce suppresses the address and a complaint
// suppresses and unsubscribes it.
// Svix's scheme: https://docs.svix.com/receiving/verifying-payloads/how-manual
// Resend's page on it: https://resend.com/docs/webhooks/verify-webhooks-requests

// fakeEmailEventRepo stores each svix-id once, as the email_events table does.
type fakeEmailEventRepo struct {
	seen    map[string]bool
	err     error
	events  []models.EmailEvent
	effects []models.EmailEventEffect
	calls   int
}

func (f *fakeEmailEventRepo) ApplyEvent(ctx context.Context, ev *models.EmailEvent, effect models.EmailEventEffect) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[ev.SvixID] {
		return false, nil
	}
	f.seen[ev.SvixID] = true
	f.events = append(f.events, *ev)
	f.effects = append(f.effects, effect)
	return true, nil
}

var webhookTestKey = []byte("solvr-webhook-test-key-24b")

// webhookTestSecret is a signing secret in Resend's form for webhookTestKey.
func webhookTestSecret() string {
	return "whsec_" + base64.StdEncoding.EncodeToString(webhookTestKey)
}

// webhookTestNow is the clock of the handler under test.
var webhookTestNow = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

func svixSign(key []byte, id, timestamp, body string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "." + body))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func newTestWebhook(repo EmailEventRepo, secret string) *ResendWebhookHandler {
	h := NewResendWebhookHandler(repo, secret)
	h.now = func() time.Time { return webhookTestNow }
	return h
}

type webhookDelivery struct {
	id, timestamp, signature, body string
}

// signedDelivery is the request Resend sends for body, signed with the test key now.
func signedDelivery(id, body string) webhookDelivery {
	ts := strconv.FormatInt(webhookTestNow.Unix(), 10)
	return webhookDelivery{id: id, timestamp: ts, signature: svixSign(webhookTestKey, id, ts, body), body: body}
}

func deliver(h *ResendWebhookHandler, d webhookDelivery) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/webhooks/resend", strings.NewReader(d.body))
	req.Header.Set("Content-Type", "application/json")
	if d.id != "" {
		req.Header.Set("svix-id", d.id)
	}
	if d.timestamp != "" {
		req.Header.Set("svix-timestamp", d.timestamp)
	}
	if d.signature != "" {
		req.Header.Set("svix-signature", d.signature)
	}
	rr := httptest.NewRecorder()
	h.Receive(rr, req)
	return rr
}

func webhookErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rr.Body.String(), err)
	}
	return body.Error.Code
}

// withBounceType is the documented bounce example with only bounce.type changed.
func withBounceType(bounceType string) string {
	return strings.Replace(resendBouncedExample, `"type": "Permanent"`, `"type": "`+bounceType+`"`, 1)
}

// Svix's own published example (the manual-verification page above) verifies, so the signed
// content, the key decoding and the header format follow Svix's scheme and not only this
// package's own signing helper. The example's secret, as that page prints it, is written below
// in two parts, so that a secret scanner does not take Svix's public example for a live secret.
func TestSvix_VerifiesTheDocumentedExample(t *testing.T) {
	key, ok := parseWebhookSecret("whsec_" + "plJ3nmyCDGBKInavdOK15jsl")
	if !ok {
		t.Fatal("Svix's example secret did not parse")
	}
	const (
		payload   = `{"event_type":"ping","data":{"success":true}}`
		msgID     = "msg_loFOjxBNrRLzqYUf"
		timestamp = "1731705121"
		signature = "v1,rAvfW3dJ/X/qxhsaXPOyyCGmRKsaKWcsNccKXlIktD0="
	)
	if !validSvixSignature(key, msgID, timestamp, []byte(payload), signature) {
		t.Error("Svix's documented example signature did not verify")
	}
	if validSvixSignature(key, msgID, timestamp, []byte(payload+" "), signature) {
		t.Error("a changed body verified")
	}
	if validSvixSignature(key, msgID, "1731705122", []byte(payload), signature) {
		t.Error("a changed timestamp verified")
	}
}

func TestResendWebhook_StoresAVerifiedEventOnce(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	d := signedDelivery("msg_bounce_1", resendBouncedExample)

	rr := deliver(h, d)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"duplicate":false`) {
		t.Errorf("body = %s, want duplicate false", rr.Body.String())
	}
	if len(repo.events) != 1 {
		t.Fatalf("stored %d events, want 1", len(repo.events))
	}
	ev := repo.events[0]
	if ev.SvixID != "msg_bounce_1" || ev.EventType != "email.bounced" || ev.BounceType != "Permanent" ||
		ev.ProviderMessageID != "56761188-7520-42d8-8898-ff6fc54ce618" {
		t.Errorf("stored %+v", ev)
	}
	if want := time.Date(2026, 11, 22, 23, 41, 12, 126000000, time.UTC); !ev.OccurredAt.Equal(want) {
		t.Errorf("occurred_at = %s, want the event's created_at %s", ev.OccurredAt, want)
	}

	// Resend sends the same svix-id again when it retries: answered, and acted on once.
	rr = deliver(h, d)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"duplicate":true`) {
		t.Errorf("replay: %d %s, want 200 duplicate true", rr.Code, rr.Body.String())
	}
	if len(repo.events) != 1 || len(repo.effects) != 1 {
		t.Errorf("a replay was acted on again: %d events, %d effects", len(repo.events), len(repo.effects))
	}
}

func TestResendWebhook_RefusesAWrongSignature(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())

	other := signedDelivery("msg_wrong_1", resendBouncedExample)
	other.signature = svixSign([]byte("another-key-another-key!!"), other.id, other.timestamp, other.body)
	tampered := signedDelivery("msg_wrong_2", resendBouncedExample)
	tampered.body = strings.Replace(tampered.body, "Permanent", "Transient", 1)
	unsigned := signedDelivery("msg_wrong_3", `{}`)
	unsigned.signature = ""

	for name, d := range map[string]webhookDelivery{"another key": other, "a changed body": tampered, "no signature": unsigned} {
		rr := deliver(h, d)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rr.Code)
			continue
		}
		if code := webhookErrorCode(t, rr); code != "INVALID_SIGNATURE" {
			t.Errorf("%s: code = %s, want INVALID_SIGNATURE", name, code)
		}
	}
	if repo.calls != 0 {
		t.Errorf("an unverified request reached the repository %d times", repo.calls)
	}
}

func TestResendWebhook_RefusesMissingHeaders(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	for name, mutate := range map[string]func(*webhookDelivery){
		"no svix-id":        func(d *webhookDelivery) { d.id = "" },
		"no svix-timestamp": func(d *webhookDelivery) { d.timestamp = "" },
		"a timestamp that is not a number": func(d *webhookDelivery) {
			d.timestamp = "soon"
			d.signature = svixSign(webhookTestKey, d.id, d.timestamp, d.body)
		},
	} {
		d := signedDelivery("msg_headers", resendSentExample)
		mutate(&d)
		if rr := deliver(h, d); rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rr.Code)
		}
	}
	if repo.calls != 0 {
		t.Errorf("the repository was reached %d times", repo.calls)
	}
}

func TestResendWebhook_RefusesATimestampMoreThanFiveMinutesOff(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	at := func(offset time.Duration, id string) webhookDelivery {
		ts := strconv.FormatInt(webhookTestNow.Add(offset).Unix(), 10)
		return webhookDelivery{id: id, timestamp: ts, signature: svixSign(webhookTestKey, id, ts, resendSentExample), body: resendSentExample}
	}

	for _, offset := range []time.Duration{-301 * time.Second, 301 * time.Second} {
		rr := deliver(h, at(offset, "msg_stale_"+offset.String()))
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s off: status = %d, want 401", offset, rr.Code)
			continue
		}
		if code := webhookErrorCode(t, rr); code != "TIMESTAMP_OUT_OF_RANGE" {
			t.Errorf("%s off: code = %s, want TIMESTAMP_OUT_OF_RANGE", offset, code)
		}
	}
	if repo.calls != 0 {
		t.Errorf("a stale request reached the repository %d times", repo.calls)
	}
	for _, offset := range []time.Duration{-299 * time.Second, 299 * time.Second} {
		if rr := deliver(h, at(offset, "msg_fresh_"+offset.String())); rr.Code != http.StatusOK {
			t.Errorf("%s off: status = %d, want 200", offset, rr.Code)
		}
	}
}

func TestResendWebhook_AcceptsAnyMatchingV1SignatureInTheList(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	d := signedDelivery("msg_list_1", resendDeliveredExample)
	good := strings.TrimPrefix(d.signature, "v1,")

	d.signature = "v1,bm9ldHUjKzFob2VudXRob2VodWUzMjRvdWVvdW9ldQo= v1," + good
	if rr := deliver(h, d); rr.Code != http.StatusOK {
		t.Errorf("the second of two v1 signatures matches: status = %d, want 200", rr.Code)
	}

	v2 := signedDelivery("msg_list_2", resendDeliveredExample)
	v2.signature = "v2," + strings.TrimPrefix(v2.signature, "v1,")
	if rr := deliver(h, v2); rr.Code != http.StatusUnauthorized {
		t.Errorf("only a v2 entry: status = %d, want 401", rr.Code)
	}
}

func TestResendWebhook_APermanentBounceSuppressesTheAddress(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())

	if rr := deliver(h, signedDelivery("msg_hard", resendBouncedExample)); rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	got := repo.effects[0]
	if got.Status != "bounced" || got.Suppress != models.EmailSuppressedHardBounce || got.Unsubscribe {
		t.Errorf("effect = %+v, want status bounced, suppressed hard_bounce, not unsubscribed", got)
	}
	if len(got.Recipients) != 1 || got.Recipients[0] != "delivered@resend.dev" {
		t.Errorf("recipients = %v, want [delivered@resend.dev]", got.Recipients)
	}
}

func TestResendWebhook_ASoftOrUndeterminedBounceSuppressesNoOne(t *testing.T) {
	for _, bounceType := range []string{"Transient", "Undetermined"} {
		repo := &fakeEmailEventRepo{}
		h := newTestWebhook(repo, webhookTestSecret())
		if rr := deliver(h, signedDelivery("msg_soft_"+bounceType, withBounceType(bounceType))); rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", bounceType, rr.Code)
		}
		got := repo.effects[0]
		if got.Status != "bounced" || got.Suppress != "" || got.Unsubscribe {
			t.Errorf("%s: effect = %+v, want status bounced and no one suppressed", bounceType, got)
		}
		if repo.events[0].BounceType != bounceType {
			t.Errorf("%s: stored bounce type %q", bounceType, repo.events[0].BounceType)
		}
	}
}

func TestResendWebhook_AComplaintSuppressesAndUnsubscribes(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())

	if rr := deliver(h, signedDelivery("msg_spam", resendComplainedExample)); rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	got := repo.effects[0]
	if got.Status != "complained" || got.Suppress != models.EmailSuppressedComplaint || !got.Unsubscribe {
		t.Errorf("effect = %+v, want status complained, suppressed complaint, unsubscribed", got)
	}
	if len(got.Recipients) != 1 || got.Recipients[0] != "delivered@resend.dev" {
		t.Errorf("recipients = %v", got.Recipients)
	}
}

func TestResendWebhook_EventsMoveTheMessageStatus(t *testing.T) {
	for body, want := range map[string]string{
		resendSentExample:            "sent",
		resendDeliveredExample:       "delivered",
		resendDeliveryDelayedExample: "delivery_delayed",
		resendFailedExample:          "failed",
		resendSuppressedExample:      "suppressed",
	} {
		repo := &fakeEmailEventRepo{}
		h := newTestWebhook(repo, webhookTestSecret())
		if rr := deliver(h, signedDelivery("msg_status_"+want, body)); rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", want, rr.Code)
		}
		got := repo.effects[0]
		if got.Status != want || got.Suppress != "" || got.Unsubscribe {
			t.Errorf("%s: effect = %+v, want only the status %s", want, got, want)
		}
	}
}

// An event Solvr does not act on (an open, here) is stored and answered, and changes nothing.
func TestResendWebhook_StoresAnEventItDoesNotActOn(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())

	if rr := deliver(h, signedDelivery("msg_open", resendOpenedExample)); rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	if len(repo.events) != 1 || repo.events[0].EventType != "email.opened" {
		t.Fatalf("stored %+v", repo.events)
	}
	if got := repo.effects[0]; got.Status != "" || got.Suppress != "" || got.Unsubscribe {
		t.Errorf("effect = %+v, want none", got)
	}
}

func TestResendWebhook_ReadsTheAddressOutOfADisplayName(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	body := strings.Replace(resendBouncedExample, `["delivered@resend.dev"]`, `["Alice <Alice@Example.com>"]`, 1)

	if rr := deliver(h, signedDelivery("msg_named", body)); rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got := repo.effects[0].Recipients; len(got) != 1 || got[0] != "alice@example.com" {
		t.Errorf("recipients = %v, want [alice@example.com]", got)
	}
}

// Fail closed: with no usable secret nothing is verified, so nothing is taken in, whatever the
// request carries, and the configuration error is logged when the handler is built.
func TestResendWebhook_FailsClosedWithoutASecret(t *testing.T) {
	for name, secret := range map[string]string{
		"no secret":            "",
		"a secret not base64":  "whsec_not*base64!",
		"an empty whsec_":      "whsec_",
		"blank":                "   ",
		"a secret of the form": "whsec_====",
	} {
		repo := &fakeEmailEventRepo{}
		var logs bytes.Buffer
		h := newResendWebhookHandler(repo, secret, slog.New(slog.NewTextHandler(&logs, nil)))
		h.now = func() time.Time { return webhookTestNow }

		signed := signedDelivery("msg_closed", resendBouncedExample)
		unsigned := webhookDelivery{body: `{}`}
		for _, d := range []webhookDelivery{signed, unsigned} {
			rr := deliver(h, d)
			if rr.Code != http.StatusServiceUnavailable {
				t.Errorf("%s: status = %d, want 503", name, rr.Code)
				continue
			}
			if code := webhookErrorCode(t, rr); code != "WEBHOOK_NOT_CONFIGURED" {
				t.Errorf("%s: code = %s, want WEBHOOK_NOT_CONFIGURED", name, code)
			}
		}
		if repo.calls != 0 {
			t.Errorf("%s: the repository was reached %d times", name, repo.calls)
		}
		if !strings.Contains(logs.String(), "RESEND_WEBHOOK_SECRET") {
			t.Errorf("%s: no configuration error was logged: %q", name, logs.String())
		}
	}
}

func TestResendWebhook_RefusesABodyThatIsNotAnEvent(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	for name, body := range map[string]string{
		"not JSON":     `not json`,
		"no type":      `{"created_at":"2026-10-17T12:00:00Z","data":{}}`,
		"a JSON array": `[]`,
	} {
		rr := deliver(h, signedDelivery("msg_bad_"+name, body))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, rr.Code)
		}
	}
	if repo.calls != 0 {
		t.Errorf("the repository was reached %d times", repo.calls)
	}
}

func TestResendWebhook_RefusesAnOversizedBody(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())
	body := `{"type":"email.sent","data":{"subject":"` + strings.Repeat("x", resendWebhookMaxBody) + `"}}`

	if rr := deliver(h, signedDelivery("msg_big", body)); rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
	if repo.calls != 0 {
		t.Errorf("the repository was reached %d times", repo.calls)
	}
}

// An event with no created_at is dated by its verified svix-timestamp.
func TestResendWebhook_DatesAnEventWithoutCreatedAtByItsTimestamp(t *testing.T) {
	repo := &fakeEmailEventRepo{}
	h := newTestWebhook(repo, webhookTestSecret())

	if rr := deliver(h, signedDelivery("msg_undated", `{"type":"email.sent","data":{"email_id":"e1"}}`)); rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got := repo.events[0].OccurredAt; !got.Equal(webhookTestNow) {
		t.Errorf("occurred_at = %s, want %s", got, webhookTestNow)
	}
}

// A database error answers 500, so Resend retries the event later.
func TestResendWebhook_ARepositoryErrorAsksForARetry(t *testing.T) {
	repo := &fakeEmailEventRepo{err: errors.New("db down")}
	h := newTestWebhook(repo, webhookTestSecret())

	rr := deliver(h, signedDelivery("msg_retry", resendSentExample))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "db down") {
		t.Errorf("internal error text leaked: %s", rr.Body.String())
	}
}
