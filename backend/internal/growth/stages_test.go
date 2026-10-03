package growth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The staged plan (spec.json idx 89) is a sequence of separately verifiable gates. These tests
// hold its evaluation to the plan's rules: every gate is judged on its own threshold and sample,
// a proposed target is labeled proposed, a stage is met only when all its gates are met AND the
// stage before it is met, an owner observation is never auto-passed, and no date or budget is
// invented.

func gateByKey(t *testing.T, s Stage, key string) Gate {
	t.Helper()
	for _, g := range s.Gates {
		if g.Key == key {
			return g
		}
	}
	t.Fatalf("stage %d has no gate %s", s.Number, key)
	return Gate{}
}

func TestRateGate_JudgesTheThresholdOnlyWithEnoughSample(t *testing.T) {
	met := RateGate("a", "conversion", 60, 100, 0.60, 100, true)
	assert.Equal(t, StatusMet, met.Status)
	require.NotNil(t, met.Measured)
	assert.InDelta(t, 0.60, *met.Measured, 1e-9)
	assert.Equal(t, 100, *met.Sample)
	assert.Equal(t, 100, *met.MinSample)
	assert.True(t, met.ProposedMinSample)

	assert.Equal(t, StatusUnmet, RateGate("a", "conversion", 59, 100, 0.60, 100, true).Status)

	small := RateGate("a", "conversion", 60, 99, 0.60, 100, true)
	assert.Equal(t, StatusNotYetMeasurable, small.Status, "too little sample is not a verdict")
	require.NotNil(t, small.Measured, "the measured rate is still shown")

	empty := RateGate("a", "conversion", 0, 0, 0.60, 100, false)
	assert.Equal(t, StatusNotYetMeasurable, empty.Status)
	assert.Nil(t, empty.Measured, "no denominator, no rate")
}

func TestCreatorReturnGate_TwentyFivePercentOfAHundred(t *testing.T) {
	assert.Equal(t, StatusMet, RateGate("b", "return", 25, 100, 0.25, 100, false).Status)
	assert.Equal(t, StatusUnmet, RateGate("b", "return", 24, 100, 0.25, 100, false).Status)
	assert.Equal(t, StatusNotYetMeasurable, RateGate("b", "return", 30, 99, 0.25, 100, false).Status)
}

func TestCombineStageStatus_IsSequentialAndStrict(t *testing.T) {
	met := Gate{Status: StatusMet}
	assert.Equal(t, StatusBlockedByPreviousStage, CombineStageStatus(false, []Gate{met}))
	assert.Equal(t, StatusMet, CombineStageStatus(true, []Gate{met, met}))
	assert.Equal(t, StatusUnmet, CombineStageStatus(true, []Gate{met, {Status: StatusUnmet}, {Status: StatusPendingG1Merge}}))
	assert.Equal(t, StatusPendingG1Merge, CombineStageStatus(true, []Gate{met, {Status: StatusPendingG1Merge}, {Status: StatusNotYetMeasurable}}))
	assert.Equal(t, StatusNotYetMeasurable, CombineStageStatus(true, []Gate{met, {Status: StatusNotYetMeasurable}}))
}

func healthyStage1() StageMeasures {
	return StageMeasures{
		WeeklyActivatedRooms: 140, WeeklyOwners: 37, LargestOwnerRooms: 12,
		Workflows:     []WorkflowCount{{Preset: "plan-and-build", ActivatedRooms: 90}, {Preset: "collaborate", ActivatedRooms: 50}},
		GateAEligible: 200, GateAConverted: 130,
		GateBEligible: 120, GateBReturned: 40,
		CoreChecks: 17280, CoreOperational: 17270,
		ModerationVolume: 12, ModerationBacklog: 0,
	}
}

func TestEvaluateStages_OwnerObservationsAreNeverAutoPassed(t *testing.T) {
	r := EvaluateStages(healthyStage1(), TargetInputs{Current: 50, Previous: 40, Returning: 20})
	require.Len(t, r.Stages, 4)

	s1 := r.Stages[0]
	assert.Equal(t, StatusMet, gateByKey(t, s1, "weekly_activated_rooms").Status)
	assert.Equal(t, StatusMet, gateByKey(t, s1, "independent_owners").Status)
	assert.Equal(t, StatusMet, gateByKey(t, s1, "gate_a_two_way_within_24h").Status)
	assert.Equal(t, StatusMet, gateByKey(t, s1, "gate_b_creator_return_7d").Status)
	assert.Equal(t, StatusNotYetMeasurable, gateByKey(t, s1, "connect_unaided").Status)
	repeated := gateByKey(t, s1, "repeated_workflow")
	assert.Equal(t, StatusNotYetMeasurable, repeated.Status)
	assert.Contains(t, repeated.Evidence, "plan-and-build")

	// The measurable gates pass, but an owner must still observe two of them.
	assert.Equal(t, StatusNotYetMeasurable, s1.Status)
	for _, later := range r.Stages[1:] {
		assert.Equal(t, StatusBlockedByPreviousStage, later.Status, "stage %d", later.Number)
	}
}

func TestEvaluateStages_LaterGatesAreStillMeasuredWhileBlocked(t *testing.T) {
	r := EvaluateStages(healthyStage1(), TargetInputs{Current: 50, Previous: 40})

	s2 := r.Stages[1]
	participants := gateByKey(t, s2, "monthly_active_participants")
	assert.Equal(t, StatusUnmet, participants.Status)
	assert.InDelta(t, 50, *participants.Measured, 1e-9)
	assert.Equal(t, StatusPendingG1Merge, gateByKey(t, s2, "activation_retention_by_source").Status)

	s3 := r.Stages[2]
	reliability := gateByKey(t, s3, "core_service_reliability")
	assert.Equal(t, StatusMet, reliability.Status, "17270/17280 clears 99.5%%")
	assert.True(t, reliability.ProposedThreshold)
	assert.Equal(t, StatusNotYetMeasurable, gateByKey(t, s3, "cost_per_activated_room").Status)
	assert.Equal(t, StatusMet, gateByKey(t, s3, "moderation_backlog").Status)
	assert.Equal(t, StatusPendingG1Merge, gateByKey(t, s3, "acquisition_channels").Status)

	s4 := r.Stages[3]
	assert.Equal(t, StatusPendingG1Merge, gateByKey(t, s4, "retained_channel_cohorts").Status)
	assert.Equal(t, StatusNotYetMeasurable, gateByKey(t, s4, "tested_capacity").Status)
}

func TestEvaluateStages_UnmetStage1IsRecordedAsUnmet(t *testing.T) {
	m := healthyStage1()
	m.WeeklyActivatedRooms = 7
	m.GateAConverted = 100 // 50% of 200
	r := EvaluateStages(m, TargetInputs{})
	s1 := r.Stages[0]
	assert.Equal(t, StatusUnmet, gateByKey(t, s1, "weekly_activated_rooms").Status)
	assert.Equal(t, StatusUnmet, gateByKey(t, s1, "gate_a_two_way_within_24h").Status)
	assert.Equal(t, StatusUnmet, s1.Status)
}

func TestEvaluateStages_ASingleOwnerIsNotIndependent(t *testing.T) {
	m := healthyStage1()
	m.WeeklyOwners = 1
	g := gateByKey(t, EvaluateStages(m, TargetInputs{}).Stages[0], "independent_owners")
	assert.Equal(t, StatusUnmet, g.Status)
	assert.True(t, g.ProposedThreshold)
}

func TestEvaluateStages_ABacklogOrNoChecksAreHonest(t *testing.T) {
	m := healthyStage1()
	m.ModerationBacklog = 3
	m.CoreChecks, m.CoreOperational = 0, 0
	s3 := EvaluateStages(m, TargetInputs{}).Stages[2]
	assert.Equal(t, StatusUnmet, gateByKey(t, s3, "moderation_backlog").Status)
	assert.Equal(t, StatusNotYetMeasurable, gateByKey(t, s3, "core_service_reliability").Status)
}

func TestEvaluateStages_InventsNoDatesOrBudgets(t *testing.T) {
	r := EvaluateStages(healthyStage1(), TargetInputs{})
	raw, err := json.Marshal(r)
	require.NoError(t, err)
	var body struct {
		Stages []map[string]any `json:"stages"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Len(t, body.Stages, 4)
	for _, s := range body.Stages {
		for _, k := range []string{"deadline", "budget"} {
			v, present := s[k]
			assert.True(t, present, "%s must be present and null", k)
			assert.Nil(t, v)
		}
	}
	assert.Contains(t, r.Note, "observed growth")
	assert.Contains(t, r.Note, "Shipping")
	for i, s := range r.Stages {
		assert.Equal(t, i+1, s.Number)
	}
	assert.Equal(t, []int{100, 10_000, 100_000, 1_000_000}, []int{
		StageOneWeeklyActivatedRooms, StageTwoParticipants, StageThreeParticipants, MonthlyActiveParticipantGoal,
	})
}
