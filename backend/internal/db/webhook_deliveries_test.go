package db

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4, webhook half: a webhook is stored with its signing secret sealed, a notification
// event of schema version 1 queues one delivery per subscribed webhook in the same statement,
// and the queue hands each due delivery to one sender at a time — with the same delivery ID
// and data on every attempt. Each test runs on its own scratch database.

const webhookTestSealSecret = "webhook-test-seal-secret-32-chars!!"

func newWebhookTestRepo(pool *Pool) *WebhookRepository {
	return NewWebhookRepository(pool).WithSealSecret(webhookTestSealSecret)
}

func createTestWebhook(t *testing.T, repo *WebhookRepository, agentID string, status models.WebhookStatus, events ...string) *models.Webhook {
	t.Helper()
	w := &models.Webhook{AgentID: agentID, URL: "https://receiver.example/hooks/" + uuid.NewString()[:8],
		Events: events, Secret: "whsec-" + uuid.NewString(), SecretHash: "unused", Status: status}
	require.NoError(t, repo.Create(context.Background(), w))
	require.NotEqual(t, uuid.Nil, w.ID)
	return w
}

func deliveriesOf(t *testing.T, pool *Pool, notificationID string) map[uuid.UUID]string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT webhook_id, event FROM webhook_deliveries WHERE notification_id = $1`, notificationID)
	require.NoError(t, err)
	defer rows.Close()
	got := map[uuid.UUID]string{}
	for rows.Next() {
		var id uuid.UUID
		var event string
		require.NoError(t, rows.Scan(&id, &event))
		got[id] = event
	}
	require.NoError(t, rows.Err())
	return got
}

func TestWebhookRepository_StoresTheSigningSecretSealedAndManagesSubscriptions(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := newWebhookTestRepo(pool)

	w := createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)
	var sealed []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT signing_secret FROM webhooks WHERE id = $1`, w.ID).Scan(&sealed))
	require.NotEmpty(t, sealed)
	require.False(t, bytes.Contains(sealed, []byte(w.Secret)), "the database holds the secret only sealed")

	found, err := repo.FindByID(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, s.agentID, found.AgentID)
	require.Equal(t, []string{models.NotificationReplyRemoved}, found.Events)
	require.Empty(t, found.Secret, "a read never opens the secret")

	found.Events = []string{models.NotificationPostApproved, models.NotificationReplyFlagged}
	found.Status = models.WebhookStatusPaused
	require.NoError(t, repo.Update(ctx, found))
	listed, err := repo.List(ctx, s.agentID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, found.Events, listed[0].Events)
	require.Equal(t, models.WebhookStatusPaused, listed[0].Status)
	var after []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT signing_secret FROM webhooks WHERE id = $1`, w.ID).Scan(&after))
	require.Equal(t, sealed, after, "an update without a new secret keeps the sealed one")

	agent, err := repo.FindAgent(ctx, s.agentID)
	require.NoError(t, err)
	require.Equal(t, s.agentID, agent.ID)
	_, err = repo.FindAgent(ctx, "agent_absent_"+uuid.NewString()[:8])
	require.ErrorIs(t, err, ErrAgentNotFound)

	require.NoError(t, repo.Delete(ctx, w.ID))
	_, err = repo.FindByID(ctx, w.ID)
	require.ErrorIs(t, err, ErrWebhookNotFound)
	require.ErrorIs(t, repo.Delete(ctx, w.ID), ErrWebhookNotFound)

	require.ErrorIs(t, NewWebhookRepository(pool).Create(ctx, &models.Webhook{AgentID: s.agentID,
		URL: "https://receiver.example/x", Events: []string{models.NotificationPostApproved}, Secret: "s",
		SecretHash: "unused", Status: models.WebhookStatusActive}), ErrWebhookSecretUnsealable,
		"without a seal secret a webhook cannot be stored: nothing could sign its deliveries")
}

func TestWebhookDeliveries_AnEventQueuesOneDeliveryPerSubscribedWebhook(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	other := seedNotificationSubjects(t, pool)
	repo := newWebhookTestRepo(pool)
	notifications := NewNotificationsRepository(pool)

	subscribed := createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationPostApproved, models.NotificationReplyRemoved)
	failing := createTestWebhook(t, repo, s.agentID, models.WebhookStatusFailing, models.NotificationReplyRemoved)
	createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationPostApproved)
	createTestWebhook(t, repo, s.agentID, models.WebhookStatusPaused, models.NotificationReplyRemoved)
	createTestWebhook(t, repo, s.agentID, models.WebhookStatusDisabled, models.NotificationReplyRemoved)
	createTestWebhook(t, repo, other.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)

	event, err := notifications.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)
	require.Equal(t, map[uuid.UUID]string{subscribed.ID: models.NotificationReplyRemoved, failing.ID: models.NotificationReplyRemoved},
		deliveriesOf(t, pool, event.ID), "the active and failing subscriptions of this agent to this event, nothing else")

	var data string
	var version int
	var occurred time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT data::text, schema_version, occurred_at FROM webhook_deliveries
		WHERE notification_id = $1 AND webhook_id = $2`, event.ID, subscribed.ID).Scan(&data, &version, &occurred))
	require.Equal(t, 1, version)
	require.WithinDuration(t, event.CreatedAt, occurred, time.Millisecond)
	require.JSONEq(t, `{"notification_id":"`+event.ID+`","agent_id":"`+s.agentID+`",
		"subject":{"post_id":"`+s.postID+`","reply_id":"`+s.replyID+`"},
		"title":"Your reply was removed","body":"Your reply did not pass moderation","link":"/posts/`+s.postID+`"}`, data)

	outside := replyRemovedEvent(s, nil, nil)
	outside.SchemaVersion = 0
	legacy, err := notifications.Create(ctx, outside)
	require.NoError(t, err)
	require.Empty(t, deliveriesOf(t, pool, legacy.ID), "an event outside the contract is never delivered")

	blog, err := notifications.Create(ctx, &models.Notification{AgentID: &s.agentID, Type: models.NotificationBlogPostRejected,
		Title: "Blog post needs changes", SchemaVersion: models.NotificationSchemaVersion})
	require.NoError(t, err)
	require.Empty(t, deliveriesOf(t, pool, blog.ID), "no webhook subscribes to blog_post_rejected")
	var blogData string
	blogHook := createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationBlogPostRejected)
	blog2, err := notifications.Create(ctx, &models.Notification{AgentID: &s.agentID, Type: models.NotificationBlogPostRejected,
		Title: "Blog post needs changes", SchemaVersion: models.NotificationSchemaVersion})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT data::text FROM webhook_deliveries WHERE notification_id = $1 AND webhook_id = $2`,
		blog2.ID, blogHook.ID).Scan(&blogData))
	var blogPayload map[string]any
	require.NoError(t, json.Unmarshal([]byte(blogData), &blogPayload))
	require.Equal(t, map[string]any{}, blogPayload["subject"], "an event without a subject delivers an empty subject")

	var userID string
	username := "wh" + uuid.NewString()[:8]
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		VALUES ($1, $2, $3, 'email', $4, $5) RETURNING id::text`, username, username, username+"@example.com", username,
		strings.ToUpper(username[2:10])).Scan(&userID))
	human, err := notifications.Create(ctx, &models.Notification{UserID: &userID, Type: models.NotificationPostApproved,
		Title: "Approved", SchemaVersion: models.NotificationSchemaVersion, Subject: models.NotificationSubject{PostID: &s.postID}})
	require.NoError(t, err)
	require.Empty(t, deliveriesOf(t, pool, human.ID), "webhooks belong to agents")

	_, err = pool.Exec(ctx, `INSERT INTO webhook_deliveries (webhook_id, notification_id, event, schema_version, data, occurred_at)
		VALUES ($1, $2, 'reply.removed', 1, '{}', NOW())`, subscribed.ID, event.ID)
	require.Error(t, err, "one delivery per webhook and notification")
}

func TestWebhookDeliveries_TwoInstancesNeverClaimTheSameDelivery(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := newWebhookTestRepo(pool)
	notifications := NewNotificationsRepository(pool)
	hook := createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)
	const queued = 30
	for i := 0; i < queued; i++ {
		_, err := notifications.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
		require.NoError(t, err)
	}

	second, err := NewPool(ctx, pool.pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(second.Close)
	instances := []*WebhookRepository{repo, newWebhookTestRepo(second)}
	claims := make([][]*models.WebhookDelivery, len(instances))
	var wg sync.WaitGroup
	for i, instance := range instances {
		wg.Add(1)
		go func(i int, instance *WebhookRepository) {
			defer wg.Done()
			for round := 0; round < 10; round++ {
				got, err := instance.ClaimDueDeliveries(ctx, 4, time.Minute)
				if err != nil {
					t.Errorf("instance %d: %v", i, err)
					return
				}
				claims[i] = append(claims[i], got...)
			}
		}(i, instance)
	}
	wg.Wait()

	seen := map[uuid.UUID]int{}
	for i, claimed := range claims {
		for _, d := range claimed {
			seen[d.ID]++
			require.Equal(t, hook.ID, d.WebhookID, "instance %d", i)
			require.Equal(t, hook.URL, d.Webhook.URL)
			require.Equal(t, hook.Secret, d.Webhook.Secret, "the claim opens the secret for the signature")
			require.Equal(t, models.NotificationReplyRemoved, d.Event)
			require.Equal(t, 1, d.SchemaVersion)
			require.Equal(t, 0, d.Attempts)
		}
	}
	require.Len(t, seen, queued, "every due delivery was claimed")
	for id, n := range seen {
		require.Equal(t, 1, n, "delivery %s was claimed by %d senders", id, n)
	}

	again, err := repo.ClaimDueDeliveries(ctx, queued, time.Minute)
	require.NoError(t, err)
	require.Empty(t, again, "a leased delivery is not claimed again")

	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET leased_until = NOW() - INTERVAL '1 second'`)
	require.NoError(t, err)
	expired, err := repo.ClaimDueDeliveries(ctx, queued, time.Minute)
	require.NoError(t, err)
	require.Len(t, expired, queued, "a lease that ran out (its sender died) is claimed again")
	for _, d := range expired {
		require.Equal(t, 1, seen[d.ID], "the same delivery ID comes back")
	}
}

func TestWebhookDeliveries_AttemptsKeepTheIDAndDataUntilDeliveredOrFailed(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	repo := newWebhookTestRepo(pool)
	notifications := NewNotificationsRepository(pool)
	createTestWebhook(t, repo, s.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)
	_, err := notifications.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)

	first, err := repo.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	d := first[0]

	status := 503
	retryAt := time.Now().Add(time.Minute)
	require.NoError(t, repo.RecordDeliveryAttempt(ctx, d.ID, models.WebhookDeliveryAttempt{
		StatusCode: &status, Error: "webhook returned 503", NextAttemptAt: &retryAt}))
	none, err := repo.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, none, "a retry waits for its time")

	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = NOW() WHERE id = $1`, d.ID)
	require.NoError(t, err)
	retried, err := repo.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	require.Equal(t, d.ID, retried[0].ID)
	require.Equal(t, 1, retried[0].Attempts)
	require.Equal(t, string(d.Data), string(retried[0].Data), "every attempt carries the same data")
	require.True(t, d.OccurredAt.Equal(retried[0].OccurredAt))

	ok := 204
	require.NoError(t, repo.RecordDeliveryAttempt(ctx, d.ID, models.WebhookDeliveryAttempt{Delivered: true, StatusCode: &ok}))
	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = NOW() - INTERVAL '1 hour'`)
	require.NoError(t, err)
	done, err := repo.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, done, "a delivered event is never sent again")
	var state string
	var attempts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, attempts FROM webhook_deliveries WHERE id = $1`, d.ID).Scan(&state, &attempts))
	require.Equal(t, "delivered", state)
	require.Equal(t, 2, attempts)

	_, err = notifications.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)
	last, err := repo.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, last, 1)
	require.NoError(t, repo.RecordDeliveryAttempt(ctx, last[0].ID, models.WebhookDeliveryAttempt{Error: "connection refused"}))
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, last[0].ID).Scan(&state))
	require.Equal(t, "failed", state, "an attempt with no retry left fails the delivery")
}

func TestWebhookDeliveries_ASecretThisServerCannotOpenFailsTheDelivery(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	createTestWebhook(t, newWebhookTestRepo(pool), s.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)
	event, err := NewNotificationsRepository(pool).Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)

	rotated := NewWebhookRepository(pool).WithSealSecret("another-server-seal-secret-32-chars")
	got, err := rotated.ClaimDueDeliveries(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Empty(t, got, "nothing is sent unsigned")
	var state, lastError string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, COALESCE(last_error, '') FROM webhook_deliveries WHERE notification_id = $1`,
		event.ID).Scan(&state, &lastError))
	require.Equal(t, "failed", state)
	require.Contains(t, lastError, "signing secret")
}
