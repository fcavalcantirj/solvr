package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The email-delivery routes as the router serves them (SPEC.md 16.6, Part 26 email-delivery):
// RFC 8058's one-click POST (405 before this release), the GET that only redirects, the Resend
// webhook that fails closed without its secret, and the notice that refuses without its sender.

func emailDeliveryRouter(t *testing.T) (http.Handler, *db.Pool) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return NewRouter(pool, nil, nil), pool
}

func routerJWTSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "test-jwt-secret-32-chars-long!!" // the router's own fallback
}

func serve(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, target, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRouter_EmailUnsubscribe_OneClickPostAndARedirectingGet(t *testing.T) {
	t.Setenv("RESEND_WEBHOOK_SECRET", "")
	router, pool := emailDeliveryRouter(t)
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	email := "unsub+" + suffix + "@router-test.solvr.dev"
	user, err := db.NewUserRepository(pool).Create(ctx, &models.User{Username: "unsub_" + suffix, DisplayName: "Unsub",
		Email: email, AuthProvider: models.AuthProviderGitHub, AuthProviderID: "unsub_" + suffix, Role: models.UserRoleUser})
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID) }) //nolint:errcheck
	query := "?email=" + url.QueryEscape(email) + "&token=" + handlers.GenerateUnsubscribeToken(email, routerJWTSecret())
	unsubscribed := func() bool {
		var at *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT email_unsubscribed_at FROM users WHERE id = $1`, user.ID).Scan(&at))
		return at != nil
	}

	get := serve(router, http.MethodGet, "/v1/email/unsubscribe"+query, "", nil)
	assert.Equal(t, http.StatusSeeOther, get.Code, get.Body.String())
	assert.True(t, strings.HasPrefix(get.Header().Get("Location"), "https://solvr.dev/email/unsubscribe?email="))
	assert.False(t, unsubscribed(), "GET never unsubscribes")

	post := serve(router, http.MethodPost, "/v1/email/unsubscribe"+query, "List-Unsubscribe=One-Click",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	assert.Equal(t, http.StatusOK, post.Code, post.Body.String())
	assert.JSONEq(t, `{"status":"unsubscribed","message":"You have been unsubscribed from Solvr emails."}`, post.Body.String())
	assert.True(t, unsubscribed())

	bad := serve(router, http.MethodPost, "/v1/email/unsubscribe?email="+url.QueryEscape(email)+"&token=bad",
		"List-Unsubscribe=One-Click", map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	assert.Equal(t, http.StatusForbidden, bad.Code)
}

func TestRouter_ResendWebhook_FailsClosedWithoutItsSecret(t *testing.T) {
	t.Setenv("RESEND_WEBHOOK_SECRET", "")
	router, _ := emailDeliveryRouter(t)

	rr := serve(router, http.MethodPost, "/v1/webhooks/resend", `{}`, map[string]string{"Content-Type": "application/json"})
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "WEBHOOK_NOT_CONFIGURED")
}

func TestRouter_ResendWebhook_TakesASignedEventAndRefusesAnUnsignedOne(t *testing.T) {
	key := []byte("router-webhook-test-key!")
	t.Setenv("RESEND_WEBHOOK_SECRET", "whsec_"+base64.StdEncoding.EncodeToString(key))
	router, pool := emailDeliveryRouter(t)
	ctx := context.Background()

	unsigned := serve(router, http.MethodPost, "/v1/webhooks/resend", `{}`, map[string]string{"Content-Type": "application/json"})
	assert.Equal(t, http.StatusUnauthorized, unsigned.Code, unsigned.Body.String())

	id := "msg_router_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM email_events WHERE svix_id = $1`, id) }) //nolint:errcheck
	body := `{"type":"email.delivered","created_at":"2026-10-17T12:00:00.000Z","data":{"email_id":"re-router-test","to":["nobody@router-test.solvr.dev"]}}`
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "." + body))
	signed := serve(router, http.MethodPost, "/v1/webhooks/resend", body, map[string]string{
		"Content-Type": "application/json", "svix-id": id, "svix-timestamp": ts,
		"svix-signature": "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	})
	assert.Equal(t, http.StatusOK, signed.Code, signed.Body.String())
	var eventType string
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_type FROM email_events WHERE svix_id = $1`, id).Scan(&eventType))
	assert.Equal(t, "email.delivered", eventType)
}

func TestRouter_EmailNotice_RefusesWithoutItsSender(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	t.Setenv("ADMIN_API_KEY", "router-notice-admin-key")
	router, _ := emailDeliveryRouter(t)

	rr := serve(router, http.MethodPost, "/admin/email/notice",
		`{"to":"a@b.c","subject":"S","body_html":"<p>x</p>","dry_run":true}`,
		map[string]string{"Content-Type": "application/json", "X-Admin-API-Key": "router-notice-admin-key"})
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "TRANSACTIONAL_SENDER_NOT_CONFIGURED")
}
