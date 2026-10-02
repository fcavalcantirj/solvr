package jobs

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// DefaultWebhookDeliveryInterval is how often due webhook deliveries are sent: attempt 1 of
// an event goes out within this of the notification (SPEC.md Part 12.3 "immediate").
const DefaultWebhookDeliveryInterval = 10 * time.Second

// DefaultWebhookDeliveryBatchSize is the most deliveries one run sends.
const DefaultWebhookDeliveryBatchSize = 20

// DefaultWebhookDeliveryLease is how long a claimed delivery stays with its sender. A run
// sends its batch at once and each send ends within the 10-second success window, so a lease
// runs out only when the sender died; another instance then sends it, with the same ID.
const DefaultWebhookDeliveryLease = 2 * time.Minute

// WebhookDeliveryQueue hands out due deliveries and records their attempts. Implemented by
// db.WebhookRepository.
type WebhookDeliveryQueue interface {
	ClaimDueDeliveries(ctx context.Context, limit int, lease time.Duration) ([]*models.WebhookDelivery, error)
	RecordDeliveryAttempt(ctx context.Context, id uuid.UUID, attempt models.WebhookDeliveryAttempt) error
}

// WebhookSender sends one attempt and owns the retry schedule. Implemented by
// services.WebhookDeliveryService.
type WebhookSender interface {
	SendQueued(ctx context.Context, d *models.WebhookDelivery) (int, error)
	ShouldRetry(attempt int) bool
	GetNextRetryDelay(attempt int) time.Duration
}

// WebhookDeliveryRun is what one run did.
type WebhookDeliveryRun struct {
	Claimed   int
	Delivered int
	Retrying  int // failed attempts with a retry scheduled
	Failed    int // failed attempts that were the last one
}

// WebhookDeliveryJob sends the queued webhook deliveries (task idx 78 step 4). Every API
// instance runs it; the queue gives each due delivery to one of them at a time.
type WebhookDeliveryJob struct {
	queue  WebhookDeliveryQueue
	sender WebhookSender
	batch  int
}

// NewWebhookDeliveryJob creates the job; a batch <= 0 uses the default.
func NewWebhookDeliveryJob(queue WebhookDeliveryQueue, sender WebhookSender, batch int) *WebhookDeliveryJob {
	if batch <= 0 {
		batch = DefaultWebhookDeliveryBatchSize
	}
	return &WebhookDeliveryJob{queue: queue, sender: sender, batch: batch}
}

// RunOnce claims a batch of due deliveries, sends them at once and records each outcome:
// delivered on a 2xx; otherwise retried on the schedule, or failed after the last attempt.
func (j *WebhookDeliveryJob) RunOnce(ctx context.Context) (WebhookDeliveryRun, error) {
	due, err := j.queue.ClaimDueDeliveries(ctx, j.batch, DefaultWebhookDeliveryLease)
	if err != nil {
		return WebhookDeliveryRun{}, err
	}
	run := WebhookDeliveryRun{Claimed: len(due)}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, d := range due {
		wg.Add(1)
		go func(d *models.WebhookDelivery) {
			defer wg.Done()
			outcome := j.attempt(ctx, d)
			if err := j.queue.RecordDeliveryAttempt(ctx, d.ID, outcome); err != nil {
				log.Printf("Webhook delivery job: record attempt of %s: %v", d.ID, err)
			}
			mu.Lock()
			defer mu.Unlock()
			switch {
			case outcome.Delivered:
				run.Delivered++
			case outcome.NextAttemptAt != nil:
				run.Retrying++
			default:
				run.Failed++
			}
		}(d)
	}
	wg.Wait()
	return run, nil
}

// attempt sends d once and decides what comes next.
func (j *WebhookDeliveryJob) attempt(ctx context.Context, d *models.WebhookDelivery) models.WebhookDeliveryAttempt {
	status, err := j.sender.SendQueued(ctx, d)
	var outcome models.WebhookDeliveryAttempt
	if status != 0 {
		outcome.StatusCode = &status
	}
	if err == nil {
		outcome.Delivered = true
		return outcome
	}
	outcome.Error = err.Error()
	if number := d.Attempts + 1; j.sender.ShouldRetry(number) {
		next := time.Now().Add(j.sender.GetNextRetryDelay(number))
		outcome.NextAttemptAt = &next
	}
	return outcome
}

// RunScheduled runs the job every interval until ctx is cancelled.
func (j *WebhookDeliveryJob) RunScheduled(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if run, err := j.RunOnce(ctx); err != nil {
			log.Printf("Webhook delivery job: %v", err)
		} else if run.Claimed > 0 {
			log.Printf("Webhook delivery job: %d claimed, %d delivered, %d retrying, %d failed",
				run.Claimed, run.Delivered, run.Retrying, run.Failed)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
