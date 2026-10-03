package growth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first acquisition loop (spec.json idx 87) is planner-to-executor collaboration. Its report
// keeps example evidence, first-connection failure points, 7- and 28-day returns, and the
// difference between a second HUMAN discovering Solvr (pending source attribution) and a second
// AGENT of the same owner (deeper activation, never a new human).

func TestBuildLoopReport_SeparatesDeeperActivationFromNewHumans(t *testing.T) {
	second := 90 * time.Second
	exchange := 4 * time.Minute
	m := LoopMeasures{
		ExampleRooms: []ExampleRoomEvidence{
			{Slug: PublicDemoRoomSlug, Found: true, Instrumented: true, TimeToSecondAgent: &second, TimeToFirstExchange: &exchange},
			{Slug: "old-example", Found: true, Instrumented: false},
		},
		FirstConnections:                FirstConnectionPoints{CreatedOnly: 4, SecondJoinedNoExchange: 2, Activated: 6},
		Return7d:                        ReturnCount{Eligible: 10, Returned: 3},
		Return28d:                       ReturnCount{Eligible: 0, Returned: 0},
		SameOwnerMultiAgentRooms:        5,
		CrossOwnerRooms:                 2,
		DistinctOwnersInMultiAgentRooms: 6,
	}
	r := BuildLoopReport(m)

	assert.Contains(t, r.Positioning, "planner")
	assert.Contains(t, r.Positioning, "executor")

	require.Len(t, r.ExampleRooms, 2)
	assert.InDelta(t, 90_000, *r.ExampleRooms[0].TimeToSecondAgentMS, 1e-9)
	assert.InDelta(t, 240_000, *r.ExampleRooms[0].TimeToFirstExchangeMS, 1e-9)
	assert.Nil(t, r.ExampleRooms[1].TimeToFirstExchangeMS)
	assert.Contains(t, r.ExampleRooms[1].Note, "not instrumented")

	assert.Equal(t, 12, r.FirstConnections.Total)
	assert.Equal(t, "created_only", r.FirstConnections.MostCommonFailure)

	require.NotNil(t, r.Returns.Within7Days.Rate)
	assert.InDelta(t, 0.3, *r.Returns.Within7Days.Rate, 1e-9)
	assert.Nil(t, r.Returns.Within28Days.Rate, "no eligible creators, no rate")
	assert.Equal(t, StatusNotYetMeasurable, r.Returns.Within28Days.Status)

	assert.Equal(t, 5, r.AgentDepth.SameOwnerMultiAgentRooms)
	assert.Equal(t, 2, r.AgentDepth.CrossOwnerRooms)
	assert.Contains(t, r.AgentDepth.Note, "never")
	assert.Equal(t, StatusPendingG1Merge, r.SecondHumanDiscovery.Status)
	assert.Equal(t, StatusNotYetMeasurable, r.InitialCohort.Status, "recruiting a cohort is owner-led work, not a system verdict")
}

func TestBuildLoopReport_NoFirstConnectionsHasNoMostCommonFailure(t *testing.T) {
	r := BuildLoopReport(LoopMeasures{})
	assert.Equal(t, 0, r.FirstConnections.Total)
	assert.Equal(t, "", r.FirstConnections.MostCommonFailure)
	assert.NotNil(t, r.ExampleRooms)
}

func TestPublicDemoRoomSlug_IsTheTicTacToeExample(t *testing.T) {
	assert.Equal(t, "tictactoe-human-vs-computer-20260920", PublicDemoRoomSlug)
}
