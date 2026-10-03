package growth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The one-million goal is a FUTURE OUTCOME counted in participant identities over a rolling
// 30-day window. These tests hold the evaluation to the success condition spec.json idx 86
// sets: sustained adoption across consecutive windows, never a one-window spike, and never a
// figure that reads as verified unique people.

func TestParticipantGoal_IsOneMillionOverARolling30DayWindow(t *testing.T) {
	assert.Equal(t, 1_000_000, MonthlyActiveParticipantGoal)
	assert.Equal(t, 30, ParticipantWindowDays)
}

func TestEvaluateTarget_BelowTheGoalIsUnmet(t *testing.T) {
	ev := EvaluateTarget(1_000_000, 4_200, 3_900, 1_100)
	assert.Equal(t, StatusUnmet, ev.Status)
	assert.Equal(t, 4_200, ev.Measured)
	assert.Equal(t, 3_900, ev.PreviousWindow)
	assert.Equal(t, 1_100, ev.Returning)
	assert.Equal(t, 30, ev.WindowDays)
}

func TestEvaluateTarget_AOneWindowSpikeIsNotMet(t *testing.T) {
	// The current window clears the goal, the window before it did not: that is a spike,
	// not sustained adoption.
	ev := EvaluateTarget(10, 25, 3, 2)
	assert.Equal(t, StatusUnmet, ev.Status)
	assert.Contains(t, ev.Note, "sustained")
}

func TestEvaluateTarget_TwoConsecutiveWindowsAtTheGoalAreMet(t *testing.T) {
	ev := EvaluateTarget(10, 12, 10, 8)
	assert.Equal(t, StatusMet, ev.Status)
}

func TestEvaluateTarget_SaysItIsAFutureOutcomeNotALaunchClaim(t *testing.T) {
	ev := EvaluateTarget(MonthlyActiveParticipantGoal, 0, 0, 0)
	assert.Contains(t, ev.Note, "future outcome")
	assert.Contains(t, ev.Note, "not a launch acceptance claim")
	assert.Contains(t, ev.Note, "registered")
}

func sampleMeasures() ParticipantMeasures {
	end := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	return ParticipantMeasures{
		WindowStart:         end.Add(-30 * 24 * time.Hour),
		WindowEnd:           end,
		PreviousWindowStart: end.Add(-60 * 24 * time.Hour),
		Humans:              3, Agents: 5,
		PreviousHumans: 2, PreviousAgents: 1,
		ReturningHumans: 1, ReturningAgents: 1,
		KnownOverlap: 1, UnresolvedOverlap: 2,
		ByAction: []ActionIdentities{{Action: "search", Humans: 1, Agents: 2}},
		Anonymous: AnonymousMeasures{
			Searches: 4, PostViews: 6, BrowserSteps: 3, Flows: 2, FlowsAttributedToIdentity: 1,
		},
		Traffic: TrafficMeasures{
			APIRequests:        9,
			APIRequestsByActor: map[string]int{"agent": 5, "human": 3, "anonymous": 1},
			APIRequestsByKind:  map[string]int{"poll": 4, "write": 3, "search": 2},
			SearchesBySearcher: map[string]int{"agent": 2, "human": 1, "anonymous": 4},
			MonitoringSearches: 1, PostViewsRecorded: 7, ActiveRooms: 2, Activations: 1,
			HumanRegistrations: 8, AgentRegistrations: 10,
		},
	}
}

func TestBuildParticipantReport_KeepsHumansAgentsAndAnonymousSeparate(t *testing.T) {
	r := BuildParticipantReport(sampleMeasures(), MonthlyActiveParticipantGoal)

	assert.Equal(t, 3, r.Humans.Active)
	assert.Equal(t, 5, r.AgentIdentities.Active)
	assert.Contains(t, r.AgentIdentities.Disclosure, "may belong to one person")

	// The combined figure is a sum of identities with its overlap shown, never people.
	assert.Equal(t, 8, r.Combined.Identities)
	assert.Equal(t, 1, r.Combined.KnownOverlap)
	assert.Equal(t, 2, r.Combined.UnresolvedOverlap)
	assert.Contains(t, r.Combined.Label, "not verified unique people")

	// Anonymous visitors cannot be estimated without a visitor identifier: absent, not zero.
	assert.Nil(t, r.Anonymous.EstimatedEngagedVisitors)
	assert.False(t, r.Anonymous.Available)
	assert.Equal(t, 1, r.Anonymous.Flows.AttributedToIdentity)
	assert.Equal(t, 1, r.Anonymous.Flows.AnonymousOnly)
	assert.Equal(t, 2, r.Anonymous.Flows.Total)

	// Traffic is its own block; sessions and page views are unavailable, not zero.
	assert.Nil(t, r.Traffic.Sessions)
	assert.Nil(t, r.Traffic.PageViews)
	assert.Equal(t, 9, r.Traffic.APIRequests.Total)
	assert.Equal(t, 8, r.Traffic.Registrations.Humans)
	assert.Contains(t, r.Traffic.Registrations.Note, "never counted as participants")

	assert.Equal(t, 8, r.Target.Measured)
	assert.Equal(t, 3, r.Target.PreviousWindow)
	assert.Equal(t, 2, r.Target.Returning)
	assert.Equal(t, StatusUnmet, r.Target.Status)
}

func TestBuildParticipantReport_StatesTheMergeBasisAndExclusions(t *testing.T) {
	r := BuildParticipantReport(sampleMeasures(), MonthlyActiveParticipantGoal)
	d := r.Definitions

	assert.Contains(t, d.MergeBasis, "flow_id")
	assert.Contains(t, d.MergeBasis, "server-issued")
	for _, never := range []string{"cookies", "fingerprinting", "IP address", "user agent"} {
		assert.Contains(t, d.MergeBasis, never, "the merge basis must rule out %s", never)
	}
	for _, excluded := range []string{"registration", "heartbeat", "health check", "invitation", "monitoring"} {
		assert.Contains(t, strings.ToLower(d.Exclusions), excluded)
	}
	assert.Contains(t, d.Sessions, "two CLI sessions")
	assert.Contains(t, d.Privacy, "operator")
}

func TestBuildParticipantReport_UnavailableFiguresSerializeAsNull(t *testing.T) {
	raw, err := json.Marshal(BuildParticipantReport(sampleMeasures(), MonthlyActiveParticipantGoal))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))

	anon := body["anonymous"].(map[string]any)
	v, present := anon["estimated_engaged_visitors"]
	assert.True(t, present)
	assert.Nil(t, v)

	traffic := body["traffic"].(map[string]any)
	for _, k := range []string{"sessions", "page_views"} {
		v, present := traffic[k]
		assert.True(t, present, "%s must be present and null", k)
		assert.Nil(t, v)
	}
	_, hasTarget := body["monthly_active_participant_target"]
	assert.True(t, hasTarget)
}
