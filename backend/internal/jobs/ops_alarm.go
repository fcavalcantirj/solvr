package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

// DefaultOpsAlarmInterval is how often the delivery queue lag is judged.
const DefaultOpsAlarmInterval = 5 * time.Minute

// QueueLagReader reads the webhook delivery backlog (db.OpsSLORepository).
type QueueLagReader interface {
	WebhookQueue(ctx context.Context, now time.Time) (ops.QueueObservation, error)
}

// OpsAlarmJob judges the webhook delivery queue lag against ops.QueueLagAlarmThreshold
// and logs a WARN when it alarms (spec.json idx 79 step 5). The same judgement is in
// GET /admin/ops/slo; this job makes it visible without anyone asking. Paging on the
// WARN is the owner's (UAT).
type OpsAlarmJob struct {
	reader QueueLagReader
	logger *slog.Logger
	now    func() time.Time
}

// NewOpsAlarmJob creates the queue-lag alarm job.
func NewOpsAlarmJob(reader QueueLagReader, logger *slog.Logger) *OpsAlarmJob {
	if logger == nil {
		logger = slog.Default()
	}
	return &OpsAlarmJob{reader: reader, logger: logger, now: time.Now}
}

// RunOnce reads the queue, judges it and warns on alarm.
func (j *OpsAlarmJob) RunOnce(ctx context.Context) (ops.QueueLag, error) {
	now := j.now()
	obs, err := j.reader.WebhookQueue(ctx, now)
	if err != nil {
		j.logger.Warn("ops alarm: webhook delivery queue unreadable", "error", err)
		return ops.QueueLag{}, err
	}
	lag := ops.EvaluateQueueLag("webhook_deliveries", obs, now)
	if lag.Status == ops.QueueAlarm {
		j.logger.Warn("ops alarm: webhook delivery queue lag",
			"queue", lag.Queue,
			"oldest_due_age_seconds", int64(*lag.OldestDueAgeSeconds),
			"threshold_seconds", int64(lag.ThresholdSeconds),
			"due", lag.Due,
			"pending", lag.Pending,
			"failed_last_24h", lag.FailedLast24h)
	}
	return lag, nil
}

// RunScheduled runs once at start, then on the interval until ctx ends.
func (j *OpsAlarmJob) RunScheduled(ctx context.Context, interval time.Duration) {
	_, _ = j.RunOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = j.RunOnce(ctx)
		}
	}
}
