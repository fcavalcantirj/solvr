package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The acquisition-loop measures (spec.json idx 87) come from the funnel and the rooms table on a
// scratch database: example-room connection evidence, where each owner's FIRST connection
// stopped, and whether a multi-agent room joined agents of one owner (deeper activation) or of
// several (collaboration across people).

func (f *stageFixture) joined(room uuid.UUID, agentRef string, ordinal int, at time.Time) {
	f.exec(`INSERT INTO funnel_events (event_name, source_channel, actor_type, actor_ref, room_id, ordinal, occurred_at)
		VALUES ('participant_joined', 'server', 'agent', $1, $2, $3, $4)`, agentRef, room, ordinal, at)
}

func (f *stageFixture) roomRow(slug string, private bool, at time.Time) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	require.NoError(f.t, f.pool.QueryRow(f.ctx, `
		INSERT INTO rooms (slug, display_name, is_private, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		slug, "Loop room "+slug, private, at).Scan(&id))
	return id
}

func TestAcquisitionLoop_ExampleEvidenceFailurePointsAndAgentDepth(t *testing.T) {
	f := newStageFixture(t)

	// Example rooms: one instrumented through the funnel, one older than the funnel, one private.
	demo := f.roomRow("loop-demo", false, f.ago(5*day))
	planner, executor := PseudonymizeActor(f.agent("planner", "")), PseudonymizeActor(f.agent("executor", ""))
	f.exec(`INSERT INTO funnel_events (event_name, source_channel, actor_type, actor_ref, room_id, occurred_at)
		VALUES ('room_created', 'server', 'agent', $1, $2, $3)`, planner, demo, f.ago(5*day))
	f.joined(demo, planner, 1, f.ago(5*day).Add(10*time.Second))
	f.joined(demo, executor, 2, f.ago(5*day).Add(90*time.Second))
	f.exec(`INSERT INTO funnel_events (event_name, source_channel, actor_type, room_id, occurred_at)
		VALUES ('first_two_way_exchange', 'server', 'agent', $1, $2)`, demo, f.ago(5*day).Add(4*time.Minute))
	f.roomRow("loop-old", false, f.ago(200*day))
	f.roomRow("loop-private", true, f.ago(3*day))

	// First connections in the window: one owner stops at creation, one when the second agent
	// joined without an exchange, the demo's creator activated. An owner whose first room predates
	// the window is not a first connection, even with a new room inside it.
	stuck := PseudonymizeActor(f.agent("stuck", ""))
	f.room(stuck, nil, f.ago(4*day), notActivated)
	half := PseudonymizeActor(f.agent("half", ""))
	halfRoom := f.room(half, nil, f.ago(3*day), notActivated)
	f.joined(halfRoom, half, 1, f.ago(3*day))
	f.joined(halfRoom, PseudonymizeActor(f.agent("guest", "")), 2, f.ago(3*day).Add(time.Minute))
	veteran := PseudonymizeActor(f.agent("veteran", ""))
	f.room(veteran, nil, f.ago(40*day), notActivated)
	f.room(veteran, nil, f.ago(2*day), notActivated)

	// Agent depth: two agents of ONE human in a room (deeper activation) and an agent of that human
	// with an unclaimed agent (two owners).
	u, err := NewUserRepository(f.pool).Create(f.ctx, &models.User{
		Username: "loopowner" + uuid.NewString()[:6], DisplayName: "Loop owner",
		Email: uuid.NewString()[:8] + "@example.com", AuthProvider: models.AuthProviderGitHub,
		AuthProviderID: "gh_" + uuid.NewString()[:8], Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	mine1, mine2 := PseudonymizeActor(f.agent("mine1", u.ID)), PseudonymizeActor(f.agent("mine2", u.ID))
	sameOwner := f.room(mine1, nil, f.ago(6*day), time.Hour)
	f.joined(sameOwner, mine1, 1, f.ago(6*day))
	f.joined(sameOwner, mine2, 2, f.ago(6*day).Add(time.Minute))
	crossOwner := f.room(mine1, nil, f.ago(5*day), time.Hour)
	f.joined(crossOwner, mine1, 1, f.ago(5*day))
	f.joined(crossOwner, PseudonymizeActor(f.agent("stranger", "")), 2, f.ago(5*day).Add(time.Minute))

	m, err := NewAcquisitionLoopRepository(f.pool).Measure(f.ctx, f.end, []string{"loop-demo", "loop-old", "loop-private", "loop-missing"})
	require.NoError(t, err)

	require.Len(t, m.ExampleRooms, 4)
	demoEv := m.ExampleRooms[0]
	assert.True(t, demoEv.Found)
	assert.True(t, demoEv.Instrumented)
	require.NotNil(t, demoEv.TimeToSecondAgent)
	assert.Equal(t, 90*time.Second, *demoEv.TimeToSecondAgent)
	require.NotNil(t, demoEv.TimeToFirstExchange)
	assert.Equal(t, 4*time.Minute, *demoEv.TimeToFirstExchange)
	assert.Equal(t, growth.ExampleRoomEvidence{Slug: "loop-old", Found: true}, m.ExampleRooms[1])
	assert.Equal(t, growth.ExampleRoomEvidence{Slug: "loop-private"}, m.ExampleRooms[2], "a private room is never an example")
	assert.Equal(t, growth.ExampleRoomEvidence{Slug: "loop-missing"}, m.ExampleRooms[3])

	// First rooms in the window: stuck (created only), half (second joined, no exchange), demo's
	// planner (activated), mine1 (activated; its first room is the same-owner room).
	assert.Equal(t, growth.FirstConnectionPoints{CreatedOnly: 1, SecondJoinedNoExchange: 1, Activated: 2}, m.FirstConnections)

	assert.Equal(t, 1, m.SameOwnerMultiAgentRooms)
	assert.Equal(t, 3, m.CrossOwnerRooms, "the demo, the half room and the cross-owner room each joined two owners")
	assert.Equal(t, 6, m.DistinctOwnersInMultiAgentRooms)
}
