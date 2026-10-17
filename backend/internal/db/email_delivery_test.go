package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Email delivery (SPEC.md 16.6): one email_messages row per message sent, each provider event
// stored once in email_events (by its svix-id) and acted on once, and the addresses Solvr no
// longer mails (users.email_suppressed_at) after a hard bounce or a complaint.

func newDeliveryTestUser(ctx context.Context, t *testing.T, pool *Pool, email string) *models.User {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	u, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: "mail_" + suffix, DisplayName: "Mail Test", Email: email,
		AuthProvider: models.AuthProviderGitHub, AuthProviderID: "mail_" + suffix, Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	deliveryCleanup(t, pool, `DELETE FROM users WHERE id = $1`, u.ID)
	return u
}

// deliveryCleanup removes what a test wrote when it ends; a failed clean-up fails nothing.
func deliveryCleanup(t *testing.T, pool *Pool, sql string, args ...any) {
	t.Helper()
	t.Cleanup(func() { pool.Exec(context.Background(), sql, args...) }) //nolint:errcheck
}

func deliveryTestAddress(name string) string {
	return name + "+" + strings.ReplaceAll(uuid.NewString(), "-", "")[:10] + "@delivery-test.solvr.dev"
}

type suppression struct {
	at           *time.Time
	reason       *string
	unsubscribed *time.Time
}

func readSuppression(ctx context.Context, t *testing.T, pool *Pool, userID string) suppression {
	t.Helper()
	var s suppression
	require.NoError(t, pool.QueryRow(ctx, `SELECT email_suppressed_at, email_suppressed_reason, email_unsubscribed_at
		FROM users WHERE id = $1`, userID).Scan(&s.at, &s.reason, &s.unsubscribed))
	return s
}

func newEventID() string { return "msg_test_" + strings.ReplaceAll(uuid.NewString(), "-", "") }

func TestEmailDelivery_RecordsEachMessage(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	user := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("record"))
	broadcast, err := NewEmailBroadcastRepository(pool).CreateLog(ctx, &models.EmailBroadcast{
		Subject: "delivery test " + user.ID, BodyHTML: "<p>x</p>", TotalRecipients: 1, Status: "sending"})
	require.NoError(t, err)
	deliveryCleanup(t, pool, `DELETE FROM email_broadcast_logs WHERE id = $1`, broadcast.ID)
	providerID := "re-" + uuid.NewString()
	deliveryCleanup(t, pool, `DELETE FROM email_messages WHERE user_id = $1`, user.ID)

	require.NoError(t, repo.RecordMessage(ctx, &models.EmailMessage{Kind: models.EmailKindBroadcast, BroadcastID: broadcast.ID,
		UserID: user.ID, ProviderMessageID: providerID, Subject: "Newsletter", Status: models.EmailStatusSent}))
	require.NoError(t, repo.RecordMessage(ctx, &models.EmailMessage{Kind: models.EmailKindTransactional,
		UserID: user.ID, Subject: "Your account", Status: models.EmailStatusFailed}))

	rows, err := pool.Query(ctx, `SELECT kind, broadcast_id::text, provider_message_id, subject, status
		FROM email_messages WHERE user_id = $1 ORDER BY id`, user.ID)
	require.NoError(t, err)
	defer rows.Close()
	type row struct {
		broadcastID, provider *string
		kind, subject, status string
	}
	var got []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.kind, &r.broadcastID, &r.provider, &r.subject, &r.status))
		got = append(got, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, got, 2)
	assert.Equal(t, "broadcast", got[0].kind)
	require.NotNil(t, got[0].broadcastID)
	assert.Equal(t, broadcast.ID, *got[0].broadcastID)
	require.NotNil(t, got[0].provider)
	assert.Equal(t, providerID, *got[0].provider)
	assert.Equal(t, "sent", got[0].status)
	assert.Equal(t, "transactional", got[1].kind)
	assert.Nil(t, got[1].broadcastID, "a notice belongs to no broadcast")
	assert.Nil(t, got[1].provider, "a failed send has no provider id")
	assert.Equal(t, "failed", got[1].status)
}

func TestEmailDelivery_StoresAnEventOnceAndActsOnItOnce(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	email := deliveryTestAddress("once")
	user := newDeliveryTestUser(ctx, t, pool, email)
	svixID := newEventID()
	deliveryCleanup(t, pool, `DELETE FROM email_events WHERE svix_id = $1`, svixID)
	ev := &models.EmailEvent{SvixID: svixID, ProviderMessageID: "re-" + uuid.NewString(), EventType: "email.bounced",
		BounceType: "Permanent", OccurredAt: time.Now().UTC().Truncate(time.Millisecond)}
	effect := models.EmailEventEffect{Status: "bounced", Suppress: models.EmailSuppressedHardBounce, Recipients: []string{email}}

	stored, err := repo.ApplyEvent(ctx, ev, effect)
	require.NoError(t, err)
	assert.True(t, stored)
	first := readSuppression(ctx, t, pool, user.ID)
	require.NotNil(t, first.at)

	stored, err = repo.ApplyEvent(ctx, ev, effect)
	require.NoError(t, err)
	assert.False(t, stored, "the same svix-id is stored once")
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM email_events WHERE svix_id = $1`, svixID).Scan(&n))
	assert.Equal(t, 1, n)
	assert.Equal(t, *first.at, *readSuppression(ctx, t, pool, user.ID).at, "a replay changes nothing")

	var eventType, bounceType, provider string
	var occurred time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_type, bounce_type, provider_message_id, occurred_at
		FROM email_events WHERE svix_id = $1`, svixID).Scan(&eventType, &bounceType, &provider, &occurred))
	assert.Equal(t, "email.bounced", eventType)
	assert.Equal(t, "Permanent", bounceType)
	assert.Equal(t, ev.ProviderMessageID, provider)
	assert.True(t, ev.OccurredAt.Equal(occurred))
}

func TestEmailDelivery_AHardBounceSuppressesTheAddressWhateverItsCase(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	email := deliveryTestAddress("Bounce")
	user := newDeliveryTestUser(ctx, t, pool, email)
	bystander := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("bystander"))
	svixID := newEventID()
	deliveryCleanup(t, pool, `DELETE FROM email_events WHERE svix_id = $1`, svixID)

	_, err := repo.ApplyEvent(ctx, &models.EmailEvent{SvixID: svixID, EventType: "email.bounced", BounceType: "Permanent",
		OccurredAt: time.Now()}, models.EmailEventEffect{Status: "bounced", Suppress: models.EmailSuppressedHardBounce,
		Recipients: []string{strings.ToLower(email)}})
	require.NoError(t, err)

	s := readSuppression(ctx, t, pool, user.ID)
	require.NotNil(t, s.at)
	require.NotNil(t, s.reason)
	assert.Equal(t, "hard_bounce", *s.reason)
	assert.Nil(t, s.unsubscribed, "a hard bounce suppresses; it does not unsubscribe")
	assert.Nil(t, readSuppression(ctx, t, pool, bystander.ID).at, "another address is untouched")
}

func TestEmailDelivery_AComplaintSuppressesAndUnsubscribes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	email := deliveryTestAddress("spam")
	user := newDeliveryTestUser(ctx, t, pool, email)
	svixID := newEventID()
	deliveryCleanup(t, pool, `DELETE FROM email_events WHERE svix_id = $1`, svixID)

	_, err := repo.ApplyEvent(ctx, &models.EmailEvent{SvixID: svixID, EventType: "email.complained", OccurredAt: time.Now()},
		models.EmailEventEffect{Status: "complained", Suppress: models.EmailSuppressedComplaint, Unsubscribe: true,
			Recipients: []string{email}})
	require.NoError(t, err)

	s := readSuppression(ctx, t, pool, user.ID)
	require.NotNil(t, s.at)
	require.NotNil(t, s.reason)
	assert.Equal(t, "complaint", *s.reason)
	assert.NotNil(t, s.unsubscribed)
}

// Events can arrive out of order: the message's status is its latest event's, by when the event
// happened, and an event about a message Solvr did not record is still stored.
func TestEmailDelivery_TheLatestEventSetsTheStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	user := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("status"))
	providerID := "re-" + uuid.NewString()
	deliveryCleanup(t, pool, `DELETE FROM email_messages WHERE user_id = $1`, user.ID)
	deliveryCleanup(t, pool, `DELETE FROM email_events WHERE provider_message_id = $1`, providerID)
	require.NoError(t, repo.RecordMessage(ctx, &models.EmailMessage{Kind: models.EmailKindTransactional, UserID: user.ID,
		ProviderMessageID: providerID, Subject: "Your account", Status: models.EmailStatusSent}))

	t0 := time.Now().UTC().Truncate(time.Millisecond)
	apply := func(eventType, status string, at time.Time) {
		_, err := repo.ApplyEvent(ctx, &models.EmailEvent{SvixID: newEventID(), ProviderMessageID: providerID,
			EventType: eventType, OccurredAt: at}, models.EmailEventEffect{Status: status})
		require.NoError(t, err)
	}
	status := func() (string, *time.Time) {
		var s string
		var last *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT status, last_event_at FROM email_messages
			WHERE provider_message_id = $1`, providerID).Scan(&s, &last))
		return s, last
	}

	apply("email.delivered", "delivered", t0.Add(2*time.Second))
	s, last := status()
	assert.Equal(t, "delivered", s)
	require.NotNil(t, last)
	assert.True(t, last.Equal(t0.Add(2*time.Second)))

	apply("email.sent", "sent", t0) // arrives late
	s, _ = status()
	assert.Equal(t, "delivered", s, "an older event does not move the status back")

	apply("email.opened", "", t0.Add(5*time.Second)) // not acted on
	s, _ = status()
	assert.Equal(t, "delivered", s)

	orphan := newEventID()
	deliveryCleanup(t, pool, `DELETE FROM email_events WHERE svix_id = $1`, orphan)
	stored, err := repo.ApplyEvent(ctx, &models.EmailEvent{SvixID: orphan, ProviderMessageID: "re-unknown-" + orphan,
		EventType: "email.delivered", OccurredAt: t0}, models.EmailEventEffect{Status: "delivered"})
	require.NoError(t, err)
	assert.True(t, stored, "an event about an unrecorded message is still stored")
}

// A notice goes to one live user: one who unsubscribed from broadcasts still gets it, a
// suppressed one is reported as such, and a deleted account is not found.
func TestEmailDelivery_FindNoticeRecipient(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	repo := NewEmailDeliveryRepository(pool)
	live := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("live"))
	_, err := pool.Exec(ctx, `UPDATE users SET email_unsubscribed_at = NOW() WHERE id = $1`, live.ID)
	require.NoError(t, err)
	suppressed := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("suppressed"))
	_, err = pool.Exec(ctx, `UPDATE users SET email_suppressed_at = NOW(), email_suppressed_reason = 'hard_bounce' WHERE id = $1`, suppressed.ID)
	require.NoError(t, err)
	deleted := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("deleted"))
	_, err = pool.Exec(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, deleted.ID)
	require.NoError(t, err)

	got, isSuppressed, err := repo.FindNoticeRecipient(ctx, live.Email)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, live.ID, got.ID)
	assert.False(t, isSuppressed)

	got, isSuppressed, err = repo.FindNoticeRecipient(ctx, suppressed.Email)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, isSuppressed)

	got, _, err = repo.FindNoticeRecipient(ctx, deleted.Email)
	require.NoError(t, err)
	assert.Nil(t, got)

	got, _, err = repo.FindNoticeRecipient(ctx, "nobody-"+uuid.NewString()+"@delivery-test.solvr.dev")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// A broadcast leaves out the addresses Solvr no longer mails (PLAN-r1 R4: ListActiveEmails adds
// AND email_suppressed_at IS NULL).
func TestUserRepository_ListActiveEmails_LeavesOutSuppressedAddresses(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()
	ctx := context.Background()
	kept := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("kept"))
	gone := newDeliveryTestUser(ctx, t, pool, deliveryTestAddress("gone"))
	_, err := pool.Exec(ctx, `UPDATE users SET email_suppressed_at = NOW(), email_suppressed_reason = 'complaint' WHERE id = $1`, gone.ID)
	require.NoError(t, err)

	recipients, err := NewUserRepository(pool).ListActiveEmails(ctx)
	require.NoError(t, err)
	listed := map[string]bool{}
	for _, r := range recipients {
		listed[r.ID] = true
	}
	assert.True(t, listed[kept.ID], "a mailable user is listed")
	assert.False(t, listed[gone.ID], "a suppressed user is not listed")
}
