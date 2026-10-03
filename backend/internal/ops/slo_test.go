package ops

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The four targets of spec.json idx 79 step 1, computed from what the service
// actually recorded. A target with too little data says not_yet_measurable and
// names what is missing; it never reads as met by default.

var sloEnd = time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)

// steadyChecks returns one api and one database check every interval over
// [start, end), each with the given status.
func steadyChecks(start, end time.Time, interval time.Duration, status string) []CoreCheck {
	var out []CoreCheck
	for t := start; t.Before(end); t = t.Add(interval) {
		out = append(out, CoreCheck{Service: "api", Status: status, CheckedAt: t})
		out = append(out, CoreCheck{Service: "database", Status: status, CheckedAt: t})
	}
	return out
}

func TestTargets_AreTheOnesTheSpecAdopts(t *testing.T) {
	assert.Equal(t, 99.9, TargetAvailabilityPercent)
	assert.Equal(t, 500.0, TargetReadP95Ms)
	assert.Equal(t, 1000.0, TargetTimelineWriteP95Ms)
	assert.Equal(t, 2000.0, TargetDeliveryP95Ms)
	assert.Equal(t, 30*24*time.Hour, AvailabilityWindow)
}

func TestAvailability_AFullWindowOfOperationalChecksIsMet(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	got := Availability(steadyChecks(start, sloEnd, CheckInterval, "operational"), start, sloEnd, CheckInterval)

	require.NotNil(t, got.Percent)
	assert.InDelta(t, 100.0, *got.Percent, 1e-9)
	assert.Equal(t, growth.StatusMet, got.Status)
	assert.Zero(t, got.DowntimeSeconds)
	assert.Zero(t, got.Gaps)
}

// A degraded check (a slow database ping) is still an available service.
func TestAvailability_DegradedCountsAsAvailable(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	got := Availability(steadyChecks(start, sloEnd, CheckInterval, "degraded"), start, sloEnd, CheckInterval)
	require.NotNil(t, got.Percent)
	assert.InDelta(t, 100.0, *got.Percent, 1e-9)
	assert.Equal(t, growth.StatusMet, got.Status)
}

// An outage check of either core service makes its interval down; api and
// database down at the same instant count once.
func TestAvailability_OutageChecksAreDowntimeCountedOnce(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	checks := steadyChecks(start, sloEnd, CheckInterval, "operational")
	// 10 intervals (50 minutes) of database outage, two of them also api outage.
	for i := range checks {
		k := i / 2
		if k >= 100 && k < 110 && checks[i].Service == "database" {
			checks[i].Status = "outage"
		}
		if k >= 100 && k < 102 && checks[i].Service == "api" {
			checks[i].Status = "outage"
		}
	}
	got := Availability(checks, start, sloEnd, CheckInterval)

	assert.InDelta(t, 50*60, got.DowntimeSeconds, 1e-6)
	require.NotNil(t, got.Percent)
	window := AvailabilityWindow.Seconds()
	assert.InDelta(t, 100*(1-3000/window), *got.Percent, 1e-9)
	assert.Equal(t, growth.StatusUnmet, got.Status, "50 minutes down exceeds the 43.2-minute monthly budget of 99.9%")
	assert.Equal(t, 10, got.OutageIntervals)
}

// A gap in the checks means nothing was answering: the process was down or the
// checker was. A gap within 1.5 intervals is scheduling jitter, not downtime.
func TestAvailability_GapsBeyondToleranceAreDowntime(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	all := steadyChecks(start, sloEnd, CheckInterval, "operational")
	var checks []CoreCheck
	gapFrom := start.Add(24 * time.Hour)
	gapTo := gapFrom.Add(30 * time.Minute) // checks at gapFrom+5..+25 are missing
	for _, c := range all {
		if c.CheckedAt.After(gapFrom) && c.CheckedAt.Before(gapTo) {
			continue
		}
		checks = append(checks, c)
	}
	// Jitter: one check 6 minutes after its predecessor is not a gap.
	got := Availability(checks, start, sloEnd, CheckInterval)

	assert.Equal(t, 1, got.Gaps, "api and database silent at the same time is one gap")
	// The last check before the gap covers its own interval; the rest of the gap is down.
	assert.InDelta(t, 25*60, got.DowntimeSeconds, 1e-6)
	assert.Equal(t, growth.StatusMet, got.Status, "25 minutes fits inside the 43.2-minute monthly budget of 99.9%")

	// A second 25-minute silence takes the month past the budget.
	var twice []CoreCheck
	second := gapFrom.Add(48 * time.Hour)
	for _, c := range checks {
		if c.CheckedAt.After(second) && c.CheckedAt.Before(second.Add(30*time.Minute)) {
			continue
		}
		twice = append(twice, c)
	}
	got = Availability(twice, start, sloEnd, CheckInterval)
	assert.Equal(t, 2, got.Gaps)
	assert.InDelta(t, 50*60, got.DowntimeSeconds, 1e-6)
	assert.Equal(t, growth.StatusUnmet, got.Status)
}

func TestAvailability_JitterWithinToleranceIsNotAGap(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	checks := steadyChecks(start, sloEnd, CheckInterval, "operational")
	for i := range checks {
		checks[i].CheckedAt = checks[i].CheckedAt.Add(time.Duration(i%3) * 40 * time.Second)
	}
	got := Availability(checks, start, sloEnd, CheckInterval)
	assert.Zero(t, got.Gaps)
	assert.Zero(t, got.DowntimeSeconds)
}

// The checker stopping before the window ends is a trailing gap.
func TestAvailability_SilenceAtTheEndIsDowntime(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	// The last check is at end-65m; it covers its own 5 minutes, so the last hour is down.
	checks := steadyChecks(start, sloEnd.Add(-time.Hour), CheckInterval, "operational")
	got := Availability(checks, start, sloEnd, CheckInterval)
	assert.Equal(t, 1, got.Gaps)
	assert.InDelta(t, 60*60, got.DowntimeSeconds, 1e-6)
}

// Less than a full window of history cannot judge a monthly target.
func TestAvailability_ShortHistoryIsNotYetMeasurable(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	checks := steadyChecks(sloEnd.Add(-10*24*time.Hour), sloEnd, CheckInterval, "operational")
	got := Availability(checks, start, sloEnd, CheckInterval)
	assert.Equal(t, growth.StatusNotYetMeasurable, got.Status)
	require.NotNil(t, got.Percent, "the observed span is still reported")
	assert.InDelta(t, 100.0, *got.Percent, 1e-9)
	assert.NotEmpty(t, got.Note)
}

func TestAvailability_NoChecksIsNotYetMeasurable(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	got := Availability(nil, start, sloEnd, CheckInterval)
	assert.Equal(t, growth.StatusNotYetMeasurable, got.Status)
	assert.Nil(t, got.Percent)
}

// Checks of non-core services (ipfs) never move core-API availability.
func TestAvailability_IgnoresNonCoreServices(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	checks := steadyChecks(start, sloEnd, CheckInterval, "operational")
	for t := start; t.Before(sloEnd); t = t.Add(CheckInterval) {
		checks = append(checks, CoreCheck{Service: "ipfs", Status: "outage", CheckedAt: t})
	}
	got := Availability(checks, start, sloEnd, CheckInterval)
	assert.Equal(t, growth.StatusMet, got.Status)
}

func f64(v float64) *float64 { return &v }

func TestEvaluateP95_MetUnmetAndTooFewSamples(t *testing.T) {
	met := EvaluateP95("read_p95", "p95 ordinary read", TargetReadP95Ms, LatencyObservation{P95Ms: f64(120), Samples: 500}, "src")
	assert.Equal(t, growth.StatusMet, met.Status)
	assert.Equal(t, "ms", met.Unit)

	unmet := EvaluateP95("read_p95", "p95 ordinary read", TargetReadP95Ms, LatencyObservation{P95Ms: f64(500), Samples: 500}, "src")
	assert.Equal(t, growth.StatusUnmet, unmet.Status, "the target is strictly below 500 ms")

	few := EvaluateP95("read_p95", "p95 ordinary read", TargetReadP95Ms, LatencyObservation{P95Ms: f64(10), Samples: MinLatencySamples - 1}, "src")
	assert.Equal(t, growth.StatusNotYetMeasurable, few.Status)
	assert.NotEmpty(t, few.Missing)

	none := EvaluateP95("read_p95", "p95 ordinary read", TargetReadP95Ms, LatencyObservation{}, "src")
	assert.Equal(t, growth.StatusNotYetMeasurable, none.Status)
	assert.Nil(t, none.Measured)
}

func TestEvaluateQueueLag_AlarmsPastTheThreshold(t *testing.T) {
	now := sloEnd
	old := now.Add(-QueueLagAlarmThreshold - time.Second)
	alarm := EvaluateQueueLag("webhook_deliveries", QueueObservation{Pending: 3, Due: 2, OldestDue: &old, FailedLast24h: 1}, now)
	assert.Equal(t, QueueAlarm, alarm.Status)
	require.NotNil(t, alarm.OldestDueAgeSeconds)
	assert.InDelta(t, QueueLagAlarmThreshold.Seconds()+1, *alarm.OldestDueAgeSeconds, 1e-6)
	assert.Equal(t, QueueLagAlarmThreshold.Seconds(), alarm.ThresholdSeconds)
	assert.Equal(t, 3, alarm.Pending)
	assert.Equal(t, 1, alarm.FailedLast24h)

	fresh := now.Add(-time.Minute)
	ok := EvaluateQueueLag("webhook_deliveries", QueueObservation{Pending: 1, Due: 1, OldestDue: &fresh}, now)
	assert.Equal(t, QueueOK, ok.Status)

	empty := EvaluateQueueLag("webhook_deliveries", QueueObservation{}, now)
	assert.Equal(t, QueueOK, empty.Status)
	assert.Nil(t, empty.OldestDueAgeSeconds)
}

func TestBuildSLOReport_FourTargetsInOrderAndTheGapsNamed(t *testing.T) {
	start := sloEnd.Add(-AvailabilityWindow)
	report := BuildSLOReport(SLOObservations{
		WindowStart:   start,
		WindowEnd:     sloEnd,
		Now:           sloEnd,
		CoreChecks:    steadyChecks(start, sloEnd, CheckInterval, "operational"),
		Read:          LatencyObservation{P95Ms: f64(80), Samples: 1000},
		TimelineWrite: LatencyObservation{P95Ms: f64(1500), Samples: 200},
		Search:        LatencyObservation{P95Ms: f64(1229), Samples: 300},
	})

	require.Len(t, report.Targets, 4)
	keys := []string{report.Targets[0].Key, report.Targets[1].Key, report.Targets[2].Key, report.Targets[3].Key}
	assert.Equal(t, []string{"core_api_availability", "read_p95", "timeline_write_p95", "delivery_p95"}, keys)
	assert.Equal(t, growth.StatusMet, report.Targets[0].Status)
	assert.Equal(t, growth.StatusMet, report.Targets[1].Status)
	assert.Equal(t, growth.StatusUnmet, report.Targets[2].Status)

	delivery := report.Targets[3]
	assert.Equal(t, growth.StatusNotYetMeasurable, delivery.Status, "nothing records delivery to connected clients")
	assert.Contains(t, delivery.Missing, "connected client")

	assert.Equal(t, "search_p95", report.ExternalModel.Key)
	assert.Empty(t, report.ExternalModel.Status, "search carries the external model call; it has no target of its own")
	require.NotNil(t, report.ExternalModel.Measured)
	assert.Equal(t, 1229.0, *report.ExternalModel.Measured)

	require.Len(t, report.Queues, 1)
	assert.Equal(t, "webhook_deliveries", report.Queues[0].Queue)
	assert.NotEmpty(t, report.Missing, "unmeasured things are named")
	assert.NotEmpty(t, report.UAT, "what only the owner can do is listed, never claimed")
	assert.Contains(t, report.Targets[0].Note, "self-measured")
}
