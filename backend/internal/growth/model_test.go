package growth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The acquisition model (spec.json idx 90) makes the planning arithmetic explicit and testable:
// A_next = A·r + new + reactivated − duplicates, the inflow a population needs just to stay flat,
// and the qualified visits that inflow implies at an activation rate. The percentages are
// hypothetical sensitivity inputs; observed counts replace inputs as they are measured, and
// measured cohort survival replaces the single retention term.

func TestModel_RetainingEightyPercentOfAMillionNeeds200kAMonth(t *testing.T) {
	inflow := InflowToStayFlat(1_000_000, 0.80)
	assert.InDelta(t, 200_000, inflow, 1e-6)

	visits, ok := QualifiedVisitsNeeded(inflow, 0.10)
	require.True(t, ok)
	assert.InDelta(t, 2_000_000, visits, 1e-6)

	_, ok = QualifiedVisitsNeeded(inflow, 0)
	assert.False(t, ok, "a zero activation rate implies no finite number of visits")

	// The flat case closes: a million at 80% retention plus 200k in stays a million.
	assert.InDelta(t, 1_000_000, NextActive(1_000_000, 0.80, 150_000, 50_000, 0), 1e-6)

	ex := GoalArithmetic()
	assert.True(t, ex.Hypothetical)
	assert.InDelta(t, 200_000, ex.InflowToStayFlat, 1e-6)
	assert.InDelta(t, 2_000_000, ex.QualifiedVisitsToStayFlat, 1e-6)
}

func TestModel_NextActiveSubtractsDuplicatesAndSteadyStateFollows(t *testing.T) {
	assert.InDelta(t, 100*0.5+20+5-3, NextActive(100, 0.5, 20, 5, 3), 1e-9)
	assert.InDelta(t, 44, SteadyState(22, 0.5), 1e-9)
	_, ok := SteadyStateOK(22, 1.0)
	assert.False(t, ok, "full retention has no finite steady state")
}

func TestProjectWithCohorts_EqualsTheSimpleModelForGeometricSurvival(t *testing.T) {
	r := 0.7
	survival := []float64{1, r, r * r, r * r * r, r * r * r * r}
	cohorts := []Cohort{{StartMonth: 0, Size: 1000}, {StartMonth: 1, Size: 200}, {StartMonth: 2, Size: 200},
		{StartMonth: 3, Size: 200}, {StartMonth: 4, Size: 200}}
	projected := ProjectWithCohorts(cohorts, survival, 5)

	simple := 1000.0
	assert.InDelta(t, simple, projected[0], 1e-6)
	for m := 1; m < 5; m++ {
		simple = NextActive(simple, r, 200, 0, 0)
		assert.InDelta(t, simple, projected[m], 1e-6, "month %d", m)
	}
}

func TestProjectWithCohorts_AMeasuredCurveDivergesFromOneRate(t *testing.T) {
	// Most loss in the first month, then a loyal core: one retention rate cannot express it.
	measured := []float64{1, 0.4, 0.35, 0.33}
	cohorts := []Cohort{{StartMonth: 0, Size: 1000}}
	got := ProjectWithCohorts(cohorts, measured, 4)
	assert.InDelta(t, 330, got[3], 1e-6)
	assert.Greater(t, got[3]-1000*0.4*0.4*0.4, 200.0, "a single 40% rate would predict 64")
	// Past the measured ages the curve holds its last value rather than inventing decay.
	assert.InDelta(t, 330, ProjectWithCohorts(cohorts, measured, 6)[5], 1e-6)
}

func TestSurvivalFromCohorts_WeightsEachAgeByTheCohortsThatReachedIt(t *testing.T) {
	obs := []CohortObservation{
		{CohortMonth: "2026-06", ActiveByAge: []int{2, 1, 2}},
		{CohortMonth: "2026-07", ActiveByAge: []int{1, 0}},
		{CohortMonth: "2026-08", ActiveByAge: []int{1}},
	}
	s := SurvivalFromCohorts(obs)
	require.Len(t, s, 3)
	assert.InDelta(t, 1.0, s[0], 1e-9)
	assert.InDelta(t, 1.0/3.0, s[1], 1e-9, "(1+0)/(2+1): only the cohorts old enough count")
	assert.InDelta(t, 1.0, s[2], 1e-9)
	assert.Empty(t, SurvivalFromCohorts(nil))
}

func observedHumans() PopulationFlows {
	return PopulationFlows{Active: 300, Retained: 120, New: 150, Reactivated: 30, PreviousActive: 200}
}

func TestScenarios_PercentagesAreHypotheticalAndCountsAreObserved(t *testing.T) {
	scenarios := BuildScenarios("humans", observedHumans(), 0)
	require.Len(t, scenarios, 3)
	names := []string{}
	for _, s := range scenarios {
		names = append(names, s.Name)
		assert.Equal(t, "humans", s.Population)
		assert.Equal(t, SourceObserved, s.StartActive.Source)
		assert.InDelta(t, 300, s.StartActive.Value, 1e-9)
		assert.Equal(t, SourceObserved, s.NewActivated.Source)
		assert.Equal(t, SourceObserved, s.Reactivated.Source)
		assert.Equal(t, SourceHypothetical, s.ActivationRate.Source)
		require.Len(t, s.Projection, ScenarioMonths)
		assert.InDelta(t, NextActive(300, s.MonthlyRetention.Value, 150, 30, 0), s.Projection[0], 1e-6)
	}
	assert.Equal(t, []string{"conservative", "base", "optimistic"}, names)

	// 200 previously active is enough sample: the base case uses the measured 60% retention.
	assert.Equal(t, SourceObserved, scenarios[1].MonthlyRetention.Source)
	assert.InDelta(t, 0.60, scenarios[1].MonthlyRetention.Value, 1e-9)
	assert.Equal(t, SourceHypothetical, scenarios[0].MonthlyRetention.Source)
	assert.Equal(t, SourceHypothetical, scenarios[2].MonthlyRetention.Source)
	assert.Less(t, scenarios[0].MonthlyRetention.Value, scenarios[2].MonthlyRetention.Value)
}

func TestScenarios_TooLittleHistoryKeepsRetentionHypothetical(t *testing.T) {
	small := PopulationFlows{Active: 9, Retained: 3, New: 6, PreviousActive: 5}
	base := BuildScenarios("agents", small, 0)[1]
	assert.Equal(t, SourceHypothetical, base.MonthlyRetention.Source)
}

func TestChannels_ArePendingAndPaidAcquisitionWaits(t *testing.T) {
	channels := ChannelComparison()
	keys := []string{}
	for _, c := range channels {
		keys = append(keys, c.Channel)
		assert.Equal(t, StatusPendingG1Merge, c.Status)
		assert.Nil(t, c.RetainedActivations)
		assert.Nil(t, c.CostPerRetainedActivation)
	}
	assert.Equal(t, []string{"seo", "public_room_sharing", "agent_ecosystem_referrals", "direct"}, keys)

	ready, why := PaidAcquisitionReady(false, false)
	assert.False(t, ready)
	assert.Contains(t, why, "retention")
	ready, _ = PaidAcquisitionReady(true, false)
	assert.False(t, ready, "measured retention without cost is not enough")
	ready, _ = PaidAcquisitionReady(true, true)
	assert.True(t, ready)
}

func TestClassifyBottleneck_ChecksCapacityThenConnectionThenRepeatThenReach(t *testing.T) {
	met, unmet, unknown := Gate{Status: StatusMet}, Gate{Status: StatusUnmet}, Gate{Status: StatusNotYetMeasurable}

	assert.Equal(t, BottleneckCapacity, ClassifyBottleneck(unmet, unmet, unmet).Bottleneck)
	assert.Equal(t, BottleneckConnection, ClassifyBottleneck(met, unmet, unmet).Bottleneck)
	assert.Equal(t, BottleneckRepeatUsage, ClassifyBottleneck(met, met, unmet).Bottleneck)
	assert.Equal(t, BottleneckReach, ClassifyBottleneck(met, met, met).Bottleneck)

	b := ClassifyBottleneck(met, unknown, unmet)
	assert.Equal(t, StatusNotYetMeasurable, b.Bottleneck)
	assert.Equal(t, []string{"connection_success"}, b.Missing)
}
