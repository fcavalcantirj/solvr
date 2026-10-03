package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/ops"
)

// The operations report reads what the service recorded (spec.json idx 79 step 1):
// service_checks for availability, api_request_events.duration_ms for read and
// timeline-write latency, search_queries for the external-model figure and
// webhook_deliveries for queue lag. Each test runs on its own scratch database, so
// every figure is exact.

func TestOpsSLO_ObservesChecksLatenciesAndSearchesInTheWindow(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewOpsSLORepository(pool)

	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	start := end.Add(-ops.AvailabilityWindow)

	// Two service checks inside the window, one before it, one of a non-core service.
	_, err := pool.Exec(ctx, `INSERT INTO service_checks (service_name, status, checked_at) VALUES
		('api', 'operational', $1), ('database', 'outage', $1),
		('api', 'operational', $2), ('ipfs', 'outage', $1)`, end.Add(-time.Hour), start.Add(-time.Hour))
	require.NoError(t, err)

	usage := NewAPIUsageRepository(pool)
	ms := func(v int) *int { return &v }
	var events []APIRequestEvent
	for i := 1; i <= 100; i++ { // reads of 1..100 ms: p95 = 95.05
		events = append(events, APIRequestEvent{RouteTemplate: "/v1/posts", Method: "GET", Operation: "knowledge.post.list",
			OperationFamily: APIFamilyKnowledge, OperationKind: APIOperationPoll, ActorType: APIActorAnonymous,
			StatusClass: 2, OccurredAt: end.Add(-time.Minute), DurationMs: ms(i)})
	}
	events = append(events,
		// Not ordinary reads: a rejected read, an unmeasured read, a read outside the window, a search.
		APIRequestEvent{RouteTemplate: "/v1/posts", Method: "GET", Operation: "knowledge.post.list", OperationFamily: APIFamilyKnowledge,
			OperationKind: APIOperationPoll, ActorType: APIActorAnonymous, StatusClass: 4, OccurredAt: end.Add(-time.Minute), DurationMs: ms(9000)},
		APIRequestEvent{RouteTemplate: "/v1/posts", Method: "GET", Operation: "knowledge.post.list", OperationFamily: APIFamilyKnowledge,
			OperationKind: APIOperationPoll, ActorType: APIActorAnonymous, StatusClass: 2, OccurredAt: end.Add(-time.Minute)},
		APIRequestEvent{RouteTemplate: "/v1/posts", Method: "GET", Operation: "knowledge.post.list", OperationFamily: APIFamilyKnowledge,
			OperationKind: APIOperationPoll, ActorType: APIActorAnonymous, StatusClass: 2, OccurredAt: start.Add(-time.Minute), DurationMs: ms(9000)},
		APIRequestEvent{RouteTemplate: "/v1/search", Method: "GET", Operation: "knowledge.search", OperationFamily: APIFamilyKnowledge,
			OperationKind: APIOperationSearch, ActorType: APIActorAnonymous, StatusClass: 2, OccurredAt: end.Add(-time.Minute), DurationMs: ms(9000)},
	)
	// Accepted timeline writes on every timeline route, plus a refused one and a post create.
	for i, route := range []string{"/v1/rooms/{slug}/entries", "/v1/rooms/{slug}/messages", "/r/{slug}/message", "/r/{slug}/events"} {
		events = append(events, APIRequestEvent{RouteTemplate: route, Method: "POST", Operation: "room.message.send",
			OperationFamily: APIFamilyRoom, OperationKind: APIOperationWrite, ActorType: APIActorAgent,
			StatusClass: 2, OccurredAt: end.Add(-time.Minute), DurationMs: ms(100 * (i + 1))})
	}
	events = append(events,
		APIRequestEvent{RouteTemplate: "/v1/rooms/{slug}/entries", Method: "POST", Operation: "room.message.send", OperationFamily: APIFamilyRoom,
			OperationKind: APIOperationWrite, ActorType: APIActorAgent, StatusClass: 4, OccurredAt: end.Add(-time.Minute), DurationMs: ms(9000)},
		APIRequestEvent{RouteTemplate: "/v1/posts", Method: "POST", Operation: "knowledge.post.create", OperationFamily: APIFamilyKnowledge,
			OperationKind: APIOperationWrite, ActorType: APIActorAgent, StatusClass: 2, OccurredAt: end.Add(-time.Minute), DurationMs: ms(9000)},
	)
	require.NoError(t, usage.RecordRequests(ctx, events))

	_, err = pool.Exec(ctx, `INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms, searched_at)
		VALUES ('a', 'a', 1, 'hybrid', 1000, $1), ('b', 'b', 1, 'hybrid', 2000, $1), ('c', 'c', 1, 'hybrid', 50000, $2)`,
		end.Add(-time.Minute), start.Add(-time.Minute))
	require.NoError(t, err)

	obs, err := repo.ObserveSLO(ctx, start, end, end)
	require.NoError(t, err)

	assert.Equal(t, start, obs.WindowStart)
	assert.Equal(t, end, obs.WindowEnd)
	require.Len(t, obs.CoreChecks, 2, "core checks inside the window only")
	for _, c := range obs.CoreChecks {
		assert.Contains(t, ops.CoreServices, c.Service)
	}

	assert.Equal(t, 100, obs.Read.Samples, "2xx measured GETs in the window, search excluded")
	require.NotNil(t, obs.Read.P95Ms)
	assert.InDelta(t, 95.05, *obs.Read.P95Ms, 1e-9)

	assert.Equal(t, 4, obs.TimelineWrite.Samples, "accepted writes on the four timeline routes")
	require.NotNil(t, obs.TimelineWrite.P95Ms)
	assert.InDelta(t, 385, *obs.TimelineWrite.P95Ms, 1e-9)

	assert.Equal(t, 2, obs.Search.Samples)
	require.NotNil(t, obs.Search.P95Ms)
	assert.InDelta(t, 1950, *obs.Search.P95Ms, 1e-9)
}

func TestOpsSLO_EmptyWindowHasNoP95(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	end := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	obs, err := NewOpsSLORepository(pool).ObserveSLO(context.Background(), end.Add(-ops.AvailabilityWindow), end, end)
	require.NoError(t, err)
	assert.Nil(t, obs.Read.P95Ms)
	assert.Zero(t, obs.Read.Samples)
	assert.Nil(t, obs.TimelineWrite.P95Ms)
	assert.Nil(t, obs.Search.P95Ms)
	assert.Empty(t, obs.CoreChecks)
}

// A delivery whose next attempt is due and still undelivered is lag; one scheduled
// for a later retry is pending, not due; failed ones are counted for the last day.
func TestOpsSLO_WebhookQueueLag(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	s := seedNotificationSubjects(t, pool)
	hooks := newWebhookTestRepo(pool)
	notifications := NewNotificationsRepository(pool)
	repo := NewOpsSLORepository(pool)

	for i := 0; i < 4; i++ {
		createTestWebhook(t, hooks, s.agentID, models.WebhookStatusActive, models.NotificationReplyRemoved)
	}
	_, err := notifications.Create(ctx, replyRemovedEvent(s, &s.postID, &s.replyID))
	require.NoError(t, err)

	now := time.Now().UTC()
	var ids []string
	rows, err := pool.Query(ctx, `SELECT id::text FROM webhook_deliveries ORDER BY id`)
	require.NoError(t, err)
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	rows.Close()
	require.Len(t, ids, 4, "one delivery per subscribed webhook")

	oldest := now.Add(-20 * time.Minute)
	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE id = $1::uuid`, ids[0], oldest)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE id = $1::uuid`, ids[1], now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET next_attempt_at = $2 WHERE id = $1::uuid`, ids[2], now.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE webhook_deliveries SET status = 'failed', last_attempt_at = $2 WHERE id = $1::uuid`, ids[3], now.Add(-time.Hour))
	require.NoError(t, err)

	q, err := repo.WebhookQueue(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 3, q.Pending)
	assert.Equal(t, 2, q.Due)
	require.NotNil(t, q.OldestDue)
	assert.WithinDuration(t, oldest, *q.OldestDue, time.Millisecond)
	assert.Equal(t, 1, q.FailedLast24h)

	lag := ops.EvaluateQueueLag("webhook_deliveries", q, now)
	assert.Equal(t, ops.QueueAlarm, lag.Status, "a delivery due 20 minutes ago is past the 5-minute threshold")

	obs, err := repo.ObserveSLO(ctx, now.Add(-ops.AvailabilityWindow), now, now)
	require.NoError(t, err)
	assert.Equal(t, q, obs.Webhooks, "the report carries the same queue observation")
}
