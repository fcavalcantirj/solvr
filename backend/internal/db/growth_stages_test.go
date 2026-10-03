package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The stage gates (spec.json idx 89) are measured from the connection funnel, the service
// checks and the moderation queues. Each test runs on its own scratch database so every count
// is exact, and seeds funnel rows directly: room_created names its creator by the same
// pseudonymous actor_ref the funnel writes, which the measurement maps back to the agent and
// the human who claimed it.

type stageFixture struct {
	t    *testing.T
	ctx  context.Context
	pool *Pool
	end  time.Time
	n    int64
}

func newStageFixture(t *testing.T) *stageFixture {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	dropLegacy()
	return &stageFixture{t: t, ctx: context.Background(), pool: pool,
		end: time.Now().UTC().Add(time.Minute).Truncate(time.Second), n: time.Now().UnixNano() % 1_000_000_000}
}

func (f *stageFixture) ago(d time.Duration) time.Time { return f.end.Add(-d) }

const day = 24 * time.Hour

func (f *stageFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *stageFixture) agent(label string, claimedBy string) string {
	f.t.Helper()
	id := fmt.Sprintf("agent_st_%s_%d", label, f.n)
	f.exec(`INSERT INTO agents (id, display_name, api_key_hash, status) VALUES ($1, $1, $2, 'active')`, id, "hash_"+id)
	if claimedBy != "" {
		f.exec(`UPDATE agents SET human_id = $1, human_claimed_at = NOW() WHERE id = $2`, claimedBy, id)
	}
	return id
}

// room records a room_created step by creator (an agent id, or a raw actor_ref when unknown) and,
// when activatedAfter >= 0, its first_two_way_exchange that long after creation.
func (f *stageFixture) room(creatorRef string, flow any, createdAt time.Time, activatedAfter time.Duration) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, actor_ref, room_id, occurred_at)
		VALUES ($1, 'room_created', 'server', 'agent', $2, $3, $4)`, flow, creatorRef, id, createdAt)
	if activatedAfter >= 0 {
		f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, room_id, occurred_at)
			VALUES ($1, 'first_two_way_exchange', 'server', 'agent', $2, $3)`, flow, id, createdAt.Add(activatedAfter))
	}
	return id
}

const notActivated = time.Duration(-1)

func TestGrowthStages_GateAIsRoomCreatedToTwoWayWithin24Hours(t *testing.T) {
	f := newStageFixture(t)
	creator := PseudonymizeActor(f.agent("creator", ""))

	// 100 eligible rooms created 2 days before end: 60 activate within 24h, 10 after 30h, 30 never.
	for i := 0; i < 100; i++ {
		after := notActivated
		switch {
		case i < 60:
			after = time.Hour
		case i < 70:
			after = 30 * time.Hour
		}
		f.room(creator, nil, f.ago(2*day), after)
	}
	// Not eligible: created inside the last 24h (too young to judge) and 32 days ago (outside).
	f.room(creator, nil, f.ago(2*time.Hour), time.Minute)
	f.room(creator, nil, f.ago(32*day), time.Minute)

	m, err := NewGrowthStageRepository(f.pool).Measure(f.ctx, f.end)
	require.NoError(t, err)
	assert.Equal(t, 100, m.GateAEligible)
	assert.Equal(t, 60, m.GateAConverted)
}

func TestGrowthStages_GateBCountsAnotherActivatedRoomWithinSevenDays(t *testing.T) {
	f := newStageFixture(t)

	// 100 owners (unclaimed agents) whose first room is 20 days before end.
	owners := make([]string, 100)
	for i := range owners {
		owners[i] = PseudonymizeActor(f.agent(fmt.Sprintf("o%d", i), ""))
		f.room(owners[i], nil, f.ago(20*day), notActivated)
	}
	// 25 return with another ACTIVATED room 3 days later: returned.
	for i := 0; i < 25; i++ {
		f.room(owners[i], nil, f.ago(17*day), time.Hour)
	}
	// 5 create a second room that never activates: a retry, not a real task.
	for i := 25; i < 30; i++ {
		f.room(owners[i], nil, f.ago(17*day), notActivated)
	}
	// 3 come back with an activated room 10 days later: outside the 7 days.
	for i := 30; i < 33; i++ {
		f.room(owners[i], nil, f.ago(10*day), time.Hour)
	}
	// An owner whose first room is only 3 days old is not yet eligible for a 7-day return.
	young := PseudonymizeActor(f.agent("young", ""))
	f.room(young, nil, f.ago(3*day), notActivated)

	m, err := NewGrowthStageRepository(f.pool).Measure(f.ctx, f.end)
	require.NoError(t, err)
	assert.Equal(t, 100, m.GateBEligible)
	assert.Equal(t, 25, m.GateBReturned)

	eligible, returned, err := NewGrowthStageRepository(f.pool).CreatorReturns(f.ctx, f.end, 28)
	require.NoError(t, err)
	assert.Equal(t, 0, eligible, "no owner's first room falls in the 30 days ending 28 days ago")
	assert.Equal(t, 0, returned)
}

func TestGrowthStages_WeeklyRoomsOwnersWorkflowsReliabilityModeration(t *testing.T) {
	f := newStageFixture(t)
	u, err := NewUserRepository(f.pool).Create(f.ctx, &models.User{
		Username: fmt.Sprintf("st%d", f.n), DisplayName: "Stage owner",
		Email: fmt.Sprintf("st%d@example.com", f.n), AuthProvider: models.AuthProviderGitHub,
		AuthProviderID: fmt.Sprintf("gh_st%d", f.n), Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	human := u.ID

	// Two claimed agents of ONE human, one unclaimed agent, one creator nobody can resolve.
	claimedA := PseudonymizeActor(f.agent("claimed_a", human))
	claimedB := PseudonymizeActor(f.agent("claimed_b", human))
	solo := PseudonymizeActor(f.agent("solo", ""))

	f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, preset, occurred_at)
		VALUES ('f_pb1', 'connection_started', 'browser', 'anonymous', 'plan-and-build', $1),
		       ('f_pb2', 'connection_started', 'browser', 'anonymous', 'plan-and-build', $1),
		       ('f_co1', 'connection_started', 'browser', 'anonymous', 'collaborate', $1)`, f.ago(3*day))
	f.room(claimedA, "f_pb1", f.ago(3*day), time.Hour)
	f.room(claimedB, "f_pb2", f.ago(2*day), time.Hour)
	f.room(solo, "f_co1", f.ago(2*day), time.Hour)
	f.room("0123456789abcdef0123456789abcdef", nil, f.ago(day), time.Hour)
	// Activated 9 days ago: not this week.
	f.room(solo, nil, f.ago(10*day), time.Hour)

	// Service checks inside the 30-day window, and one outside it.
	f.exec(`INSERT INTO service_checks (service_name, status, response_time_ms, checked_at)
		SELECT 'api', CASE WHEN g = 1 THEN 'outage' ELSE 'operational' END, 1, $1::timestamptz - g * interval '1 minute'
		  FROM generate_series(1, 200) g`, f.ago(day))
	f.exec(`INSERT INTO service_checks (service_name, status, response_time_ms, checked_at)
		SELECT 'database', 'operational', 2, $1::timestamptz - g * interval '1 minute' FROM generate_series(1, 200) g`, f.ago(day))
	f.exec(`INSERT INTO service_checks (service_name, status, response_time_ms, checked_at)
		SELECT 'ipfs', 'outage', 0, $1::timestamptz - g * interval '1 minute' FROM generate_series(1, 10) g`, f.ago(day))
	f.exec(`INSERT INTO service_checks (service_name, status, response_time_ms, checked_at) VALUES ('api', 'outage', 1, $1)`,
		f.ago(40*day))

	// Moderation: a pending flag 10 days old (backlog and volume), a pending report 2 days old
	// (volume only), a post waiting for moderation 9 days (backlog), a resolved old flag (neither).
	target := uuid.New()
	f.exec(`INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason, status, created_at)
		VALUES ('post', $1, 'human', 'r1', 'spam', 'pending', $2), ('post', $1, 'human', 'r2', 'spam', 'actioned', $2)`,
		target, f.ago(10*day))
	f.exec(`INSERT INTO reports (target_type, target_id, reporter_type, reporter_id, reason, status, created_at)
		VALUES ('post', $1, 'agent', 'r3', 'spam', 'pending', $2)`, target, f.ago(2*day))
	f.exec(`INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, moderation_state, created_at)
		VALUES ('question', 'Awaiting moderation', 'A post still waiting for a moderator.', 'human', $1, 'pending_review', 'pending', $2)`,
		human, f.ago(9*day))

	m, err := NewGrowthStageRepository(f.pool).Measure(f.ctx, f.end)
	require.NoError(t, err)

	assert.Equal(t, 4, m.WeeklyActivatedRooms)
	assert.Equal(t, 2, m.WeeklyOwners, "two agents of one human are one owner; the unclaimed agent is another")
	assert.Equal(t, 1, m.WeeklyUnknownOwnerRooms)
	assert.Equal(t, 2, m.LargestOwnerRooms)
	assert.Equal(t, []growth.WorkflowCount{
		{Preset: "plan-and-build", ActivatedRooms: 2},
		{Preset: "collaborate", ActivatedRooms: 1},
		{Preset: "unknown", ActivatedRooms: 1},
	}, m.Workflows)

	assert.Equal(t, 400, m.CoreChecks)
	assert.Equal(t, 399, m.CoreOperational)
	assert.Equal(t, []growth.ServiceUptime{
		{Service: "api", Checks: 200, Operational: 199},
		{Service: "database", Checks: 200, Operational: 200},
		{Service: "ipfs", Checks: 10, Operational: 0},
	}, m.Services)

	assert.Equal(t, 3, m.ModerationVolume, "two flags and one report created in the window")
	assert.Equal(t, 2, m.ModerationBacklog, "the 10-day pending flag and the 9-day pending post")
}
