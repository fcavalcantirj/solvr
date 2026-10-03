package jobs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

// The queue-lag alarm (spec.json idx 79 step 5): every 5 minutes the webhook delivery
// backlog is read and judged against ops.QueueLagAlarmThreshold; an alarm is a WARN log
// line an external log monitor can page on (paging itself is the owner's, UAT).

type fakeQueueReader struct {
	obs   ops.QueueObservation
	err   error
	calls int
}

func (f *fakeQueueReader) WebhookQueue(_ context.Context, _ time.Time) (ops.QueueObservation, error) {
	f.calls++
	return f.obs, f.err
}

func alarmJob(reader QueueLagReader, buf *bytes.Buffer, now time.Time) *OpsAlarmJob {
	j := NewOpsAlarmJob(reader, slog.New(slog.NewTextHandler(buf, nil)))
	j.now = func() time.Time { return now }
	return j
}

func TestOpsAlarmJob_WarnsWhenTheOldestDueDeliveryIsPastTheThreshold(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	old := now.Add(-20 * time.Minute)
	var buf bytes.Buffer
	lag, err := alarmJob(&fakeQueueReader{obs: ops.QueueObservation{Pending: 4, Due: 2, OldestDue: &old}}, &buf, now).RunOnce(context.Background())

	require.NoError(t, err)
	assert.Equal(t, ops.QueueAlarm, lag.Status)
	assert.Contains(t, buf.String(), "level=WARN")
	assert.Contains(t, buf.String(), "ops alarm: webhook delivery queue lag")
	assert.Contains(t, buf.String(), "oldest_due_age_seconds=1200")
	assert.Contains(t, buf.String(), "threshold_seconds=300")
}

func TestOpsAlarmJob_StaysQuietWhenTheQueueKeepsUp(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	fresh := now.Add(-time.Minute)
	var buf bytes.Buffer
	lag, err := alarmJob(&fakeQueueReader{obs: ops.QueueObservation{Pending: 1, Due: 1, OldestDue: &fresh}}, &buf, now).RunOnce(context.Background())

	require.NoError(t, err)
	assert.Equal(t, ops.QueueOK, lag.Status)
	assert.NotContains(t, buf.String(), "level=WARN")
}

func TestOpsAlarmJob_AReadFailureIsReportedNotSwallowed(t *testing.T) {
	var buf bytes.Buffer
	_, err := alarmJob(&fakeQueueReader{err: errors.New("db down")}, &buf, time.Now()).RunOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, buf.String(), "level=WARN", "an unreadable queue is itself worth a warning")
}

func TestOpsAlarmJob_RunsAtStartThenOnTheInterval(t *testing.T) {
	reader := &fakeQueueReader{}
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	alarmJob(reader, &buf, time.Now()).RunScheduled(ctx, 50*time.Millisecond)
	assert.GreaterOrEqual(t, reader.calls, 2, "once at start and at least once on the ticker")
}

// The availability computation assumes the health check cadence.
func TestOpsCheckInterval_IsTheHealthCheckInterval(t *testing.T) {
	assert.Equal(t, DefaultHealthCheckInterval, ops.CheckInterval)
	assert.Equal(t, 5*time.Minute, DefaultOpsAlarmInterval)
}
