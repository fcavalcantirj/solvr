package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 4: the delivery job sends every claimed delivery and records each outcome on the
// SPEC.md Part 12.3 ladder — delivered on a 2xx, retried after 1m/5m/30m/2h, failed after
// the fifth attempt.

type fakeWebhookQueue struct {
	mu       sync.Mutex
	due      []*models.WebhookDelivery
	lease    time.Duration
	recorded map[uuid.UUID]models.WebhookDeliveryAttempt
}

func (q *fakeWebhookQueue) ClaimDueDeliveries(_ context.Context, limit int, lease time.Duration) ([]*models.WebhookDelivery, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.lease = lease
	n := min(limit, len(q.due))
	claimed := q.due[:n]
	q.due = q.due[n:]
	return claimed, nil
}

func (q *fakeWebhookQueue) RecordDeliveryAttempt(_ context.Context, id uuid.UUID, a models.WebhookDeliveryAttempt) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.recorded[id] = a
	return nil
}

// fakeWebhookSender answers each delivery with the status its webhook URL names, on the
// real retry schedule.
type fakeWebhookSender struct {
	*services.WebhookDeliveryService
	answers map[uuid.UUID]int
}

func (s fakeWebhookSender) SendQueued(_ context.Context, d *models.WebhookDelivery) (int, error) {
	status := s.answers[d.ID]
	if status == 0 {
		return 0, errors.New("connection refused")
	}
	if status < 200 || status >= 300 {
		return status, services.ErrWebhookDeliveryFailed
	}
	return status, nil
}

func TestWebhookDeliveryJob_RecordsEachOutcomeOnTheRetryLadder(t *testing.T) {
	delivered := &models.WebhookDelivery{ID: uuid.New(), Attempts: 0}
	firstFailure := &models.WebhookDelivery{ID: uuid.New(), Attempts: 0}
	fourthFailure := &models.WebhookDelivery{ID: uuid.New(), Attempts: 3}
	lastFailure := &models.WebhookDelivery{ID: uuid.New(), Attempts: 4}
	unreachable := &models.WebhookDelivery{ID: uuid.New(), Attempts: 1}
	queue := &fakeWebhookQueue{due: []*models.WebhookDelivery{delivered, firstFailure, fourthFailure, lastFailure, unreachable},
		recorded: map[uuid.UUID]models.WebhookDeliveryAttempt{}}
	sender := fakeWebhookSender{services.NewWebhookDeliveryService(nil, nil), map[uuid.UUID]int{
		delivered.ID: 204, firstFailure.ID: 500, fourthFailure.ID: 503, lastFailure.ID: 410}}

	before := time.Now()
	run, err := NewWebhookDeliveryJob(queue, sender, 0).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, WebhookDeliveryRun{Claimed: 5, Delivered: 1, Retrying: 3, Failed: 1}, run)
	require.Equal(t, DefaultWebhookDeliveryLease, queue.lease)

	require.True(t, queue.recorded[delivered.ID].Delivered)
	require.Equal(t, 204, *queue.recorded[delivered.ID].StatusCode)
	require.Nil(t, queue.recorded[delivered.ID].NextAttemptAt)

	retryAfter := func(d *models.WebhookDelivery, want time.Duration) {
		t.Helper()
		got := queue.recorded[d.ID]
		require.False(t, got.Delivered)
		require.NotEmpty(t, got.Error)
		require.NotNil(t, got.NextAttemptAt, "attempt %d is retried", d.Attempts+1)
		require.WithinDuration(t, before.Add(want), *got.NextAttemptAt, 5*time.Second)
	}
	retryAfter(firstFailure, time.Minute)  // attempt 2 after 1 minute
	retryAfter(unreachable, 5*time.Minute) // attempt 3 after 5 minutes
	retryAfter(fourthFailure, 2*time.Hour) // attempt 5 after 2 hours
	require.Equal(t, 500, *queue.recorded[firstFailure.ID].StatusCode)
	require.Nil(t, queue.recorded[unreachable.ID].StatusCode, "no answer, no status")

	final := queue.recorded[lastFailure.ID]
	require.False(t, final.Delivered)
	require.Nil(t, final.NextAttemptAt, "the fifth attempt was the last")
	require.Equal(t, 410, *final.StatusCode)

	again, err := NewWebhookDeliveryJob(queue, sender, 0).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, WebhookDeliveryRun{}, again, "nothing due, nothing sent")
}
