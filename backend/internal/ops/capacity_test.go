package ops

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// spec.json idx 79 step 3: for planning only, 1,000,000 monthly active participants, 10%
// daily activity and 20 requests per daily participant is 2,000,000 requests a day; measured
// polling and stream costs and burst factors are added separately. Every input says whether
// it was observed or is a hypothetical assumption, and the plan never calls itself a forecast.

func hyp(v float64) growth.Input { return growth.Input{Value: v, Source: growth.SourceHypothetical} }
func obs(v float64) growth.Input { return growth.Input{Value: v, Source: growth.SourceObserved} }

func TestCapacityPlan_TheSpecsPlanningArithmetic(t *testing.T) {
	plan := CapacityPlan(CapacityInputs{
		MonthlyActiveParticipants:   hyp(1_000_000),
		DailyActiveShare:            hyp(0.10),
		RequestsPerDailyParticipant: hyp(20),
		BurstFactor:                 hyp(1),
	})
	assert.Equal(t, 100_000.0, plan.DailyActiveParticipants)
	assert.Equal(t, 2_000_000.0, plan.BaseRequestsPerDay)
	assert.InDelta(t, 23.148148, plan.BaseMeanRPS, 1e-6)
	assert.Equal(t, plan.BaseRequestsPerDay, plan.TotalRequestsPerDay, "no polling or streams were added")
	assert.InDelta(t, plan.BaseMeanRPS, plan.PeakRPS, 1e-9)
	assert.Nil(t, plan.CapacityMultiple, "no measured capacity, no multiple")
}

func TestCapacityPlan_AddsPollingStreamsAndBurstSeparately(t *testing.T) {
	plan := CapacityPlan(CapacityInputs{
		MonthlyActiveParticipants:   hyp(1_000_000),
		DailyActiveShare:            hyp(0.10),
		RequestsPerDailyParticipant: hyp(20),
		// 20% of daily participants poll every 30 s for 2 hours: 240 polls each.
		PollingShare:        hyp(0.20),
		PollIntervalSeconds: hyp(30),
		PollingHoursPerDay:  hyp(2),
		// 10% hold a stream 6 hours a day: 2,500 concurrent on average.
		StreamShare:            hyp(0.10),
		StreamHoursPerDay:      hyp(6),
		StreamHeartbeatSeconds: obs(30),
		BurstFactor:            obs(10),
		MeasuredCapacityRPS:    obs(250),
	})

	assert.Equal(t, 20_000*240.0, plan.PollingRequestsPerDay)
	assert.Equal(t, 2_000_000.0+4_800_000.0, plan.TotalRequestsPerDay)
	assert.InDelta(t, 6_800_000.0/86_400, plan.MeanRPS, 1e-9)
	assert.InDelta(t, 10*6_800_000.0/86_400, plan.PeakRPS, 1e-9)
	assert.InDelta(t, 2_500, plan.MeanConcurrentStreams, 1e-9)
	assert.InDelta(t, 25_000, plan.PeakConcurrentStreams, 1e-9)
	assert.InDelta(t, 2_500.0/30, plan.StreamHeartbeatFramesPerSecond, 1e-9)
	require.NotNil(t, plan.CapacityMultiple)
	assert.InDelta(t, plan.PeakRPS/250, *plan.CapacityMultiple, 1e-9,
		"how many measured-capacity units the peak needs")
}

// The plan lists every input with its source, so a table built from it shows which
// numbers are assumptions.
func TestCapacityPlan_RowsCarryEveryInputsSource(t *testing.T) {
	plan := CapacityPlan(DefaultCapacityInputs())
	require.NotEmpty(t, plan.Rows)
	sources := map[string]bool{}
	for _, row := range plan.Rows {
		assert.NotEmpty(t, row.Label)
		assert.Contains(t, []string{growth.SourceObserved, growth.SourceHypothetical, SourceDerived}, row.Source, row.Label)
		sources[row.Source] = true
	}
	assert.True(t, sources[growth.SourceHypothetical], "the spec's 1,000,000 MAP is an assumption")
	assert.True(t, sources[SourceDerived])
	assert.Equal(t, 2_000_000.0, plan.BaseRequestsPerDay)
	assert.Contains(t, plan.Note, "not a forecast")
}

func TestDefaultCapacityInputs_AreTheSpecsAssumptions(t *testing.T) {
	in := DefaultCapacityInputs()
	assert.Equal(t, hyp(1_000_000), in.MonthlyActiveParticipants)
	assert.Equal(t, hyp(0.10), in.DailyActiveShare)
	assert.Equal(t, hyp(20), in.RequestsPerDailyParticipant)
	assert.Equal(t, obs(30), in.StreamHeartbeatSeconds, "the SSE heartbeat interval is read from the code")
}
