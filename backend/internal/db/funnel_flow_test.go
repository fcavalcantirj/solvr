package db_test

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// A website visit tied to the room it produced (SPEC.md 25.7): the web server's
// skill_fetched step, the question POST /v1/rooms asks before it keeps a flow code ("does
// an earlier step carry it?"), and the operator report that reads the joined flow.

// newTestFlowCode is a well-formed flow code no other row carries, removed after the test.
func newTestFlowCode(t *testing.T, ctx context.Context, pool *db.Pool) string {
	t.Helper()
	b := make([]byte, models.FlowCodeLength)
	_, err := rand.Read(b)
	require.NoError(t, err)
	for i := range b {
		b[i] = models.FlowCodeAlphabet[int(b[i])%len(models.FlowCodeAlphabet)]
	}
	code := string(b)
	require.True(t, models.ValidFlowCode(code))
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE flow_id = $1`, code) //nolint:errcheck
	})
	return code
}

func TestFunnelEventRepository_RecordSkillFetched(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	flow := newTestFlowCode(t, ctx, pool)

	require.NoError(t, repo.RecordSkillFetched(ctx, flow, models.FunnelActorAnonymous, "", models.FunnelSurfaceAgentFetch))
	require.NoError(t, repo.RecordSkillFetched(ctx, flow, models.FunnelActorAgent, db.PseudonymizeActor("agent_skill"), models.FunnelSurfaceBrowserVisit))

	events, err := repo.ListByFlow(ctx, flow)
	require.NoError(t, err)
	require.Len(t, events, 2, "every fetch is a row: the reports count distinct flows")
	for _, e := range events {
		assert.Equal(t, models.FunnelSkillFetched, e.EventName)
		assert.Equal(t, models.FunnelSourceWebServer, e.SourceChannel)
		assert.Empty(t, e.RoomID)
		assert.Empty(t, e.Preset)
	}
	assert.Equal(t, "agent_fetch", events[0].EntrySurface)
	assert.Equal(t, models.FunnelActorAnonymous, events[0].ActorType)
	assert.Empty(t, events[0].ActorRef)
	assert.Equal(t, "browser_visit", events[1].EntrySurface)
	assert.Equal(t, models.FunnelActorAgent, events[1].ActorType)
	assert.Equal(t, db.PseudonymizeActor("agent_skill"), events[1].ActorRef)
}

// A flow is KNOWN when at least one step other than room_created carries its code.
func TestFunnelEventRepository_FlowKnown(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)

	known := func(code string) bool {
		t.Helper()
		ok, err := repo.FlowKnown(ctx, code)
		require.NoError(t, err)
		return ok
	}

	t.Run("a code no step carries is unknown", func(t *testing.T) {
		assert.False(t, known(newTestFlowCode(t, ctx, pool)))
		assert.False(t, known(""))
	})

	t.Run("a browser step makes it known", func(t *testing.T) {
		flow := newTestFlowCode(t, ctx, pool)
		require.NoError(t, repo.RecordBrowserEvent(ctx, db.BrowserFunnelEvent{
			FlowID: flow, EventName: models.FunnelConnectionStarted, ActorType: models.FunnelActorAnonymous,
		}))
		assert.True(t, known(flow))
	})

	t.Run("the web server's skill_fetched alone makes it known", func(t *testing.T) {
		flow := newTestFlowCode(t, ctx, pool)
		require.NoError(t, repo.RecordSkillFetched(ctx, flow, models.FunnelActorAnonymous, "", models.FunnelSurfaceAgentFetch))
		assert.True(t, known(flow))
	})

	t.Run("a room_created that carries it does not", func(t *testing.T) {
		flow := newTestFlowCode(t, ctx, pool)
		roomID := uuid.New()
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE room_id = $1`, roomID) //nolint:errcheck
		})
		require.NoError(t, repo.RecordRoomCreated(ctx, roomID, models.FunnelActorAgent, "ref-known", flow))
		assert.False(t, known(flow), "a room cannot vouch for the code it brought")
	})

	t.Run("a lookup that cannot run is an error, not an answer", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		_, err := repo.FlowKnown(cancelled, newTestFlowCode(t, ctx, pool))
		require.Error(t, err)
	})
}

// The report counts FLOWS, not rooms, for the website: a sentence pasted twice makes two
// rooms and still one converted flow, so the rate never passes 1. website_flow_steps is
// the same flows, step by step.
func TestActivationAnalytics_WebsiteFlowsAreCountedOncePerFlowWithTheirSteps(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewActivationAnalyticsRepository(pool)

	// A slice of the past no other test and no ambient traffic writes into.
	to := time.Now().Add(-400 * 24 * time.Hour).Truncate(time.Second)
	from := to.Add(-10 * 24 * time.Hour)
	base := from.Add(24 * time.Hour)
	_, err := pool.Exec(ctx, `DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`,
		from.Add(-24*time.Hour), to.Add(24*time.Hour))
	require.NoError(t, err)

	step := func(flow any, event, channel, surface string, room any, at time.Time) {
		t.Helper()
		var surf any
		if surface != "" {
			surf = surface
		}
		seedFunnelRow(t, ctx, pool, map[string]any{
			"flow_id": flow, "event_name": event, "source_channel": channel, "actor_type": "anonymous",
			"actor_ref": nil, "room_id": room, "ordinal": nil, "entry_surface": surf, "occurred_at": at,
		})
	}
	started := func(flow string, at time.Time) { step(flow, "connection_started", "browser", "connect_page", nil, at) }
	copied := func(flow string, at time.Time) {
		step(flow, "starter_prompt_copied", "browser", "connect_page", nil, at)
	}
	fetched := func(flow, surface string, at time.Time) { step(flow, "skill_fetched", "web_server", surface, nil, at) }
	created := func(flow any, at time.Time) { step(flow, "room_created", "server", "", uuid.New(), at) }

	// A: the whole chain, and the sentence was pasted into two agents: two rooms, one flow.
	started("aaaaaaa2", base)
	copied("aaaaaaa2", base.Add(time.Minute))
	fetched("aaaaaaa2", "agent_fetch", base.Add(2*time.Minute))
	created("aaaaaaa2", base.Add(3*time.Minute))
	created("aaaaaaa2", base.Add(4*time.Minute))
	// B: copied, then a PERSON opened the skill link in a browser. No agent read it.
	started("bbbbbbb2", base)
	copied("bbbbbbb2", base.Add(time.Minute))
	fetched("bbbbbbb2", "browser_visit", base.Add(2*time.Minute))
	// C: opened the page and left.
	started("ccccccc2", base)
	// D: every step reported more than once (a reload, an agent that fetched three times).
	started("ddddddd2", base)
	started("ddddddd2", base.Add(time.Second))
	fetched("ddddddd2", "agent_fetch", base.Add(time.Minute))
	fetched("ddddddd2", "agent_fetch", base.Add(2*time.Minute))
	fetched("ddddddd2", "agent_fetch", base.Add(3*time.Minute))
	created("ddddddd2", base.Add(4*time.Minute))
	// E: an agent that read GET /v1/connect itself: no website start. Not a website flow.
	fetched("eeeeeee2", "agent_fetch", base)
	created("eeeeeee2", base.Add(time.Minute))
	// F: started in the window, copied after it ended.
	started("fffffff2", to.Add(-time.Hour))
	copied("fffffff2", to.Add(time.Hour))
	// G: started before the window, the rest inside it. Not a flow of this window.
	started("ggggggg2", from.Add(-time.Hour))
	copied("ggggggg2", base)
	fetched("ggggggg2", "agent_fetch", base)
	// U: a room that carried no code.
	created(nil, base)

	rep, err := repo.Measure(ctx, from, to)
	require.NoError(t, err)

	assert.Equal(t, 5, rep.RoomsCreated, "A twice, D, E and U")

	require.NotNil(t, rep.FlowToRoom.Website.Denominator)
	assert.Equal(t, 5, *rep.FlowToRoom.Website.Denominator, "website flows started in the window: A, B, C, D, F")
	assert.Equal(t, 2, rep.FlowToRoom.Website.Numerator, "flows that produced a room: A (once, not twice) and D")
	require.NotNil(t, rep.FlowToRoom.Website.Rate)
	assert.InDelta(t, 2.0/5.0, *rep.FlowToRoom.Website.Rate, 1e-9)
	assert.LessOrEqual(t, *rep.FlowToRoom.Website.Rate, 1.0)
	assert.Equal(t, 1, rep.FlowToRoom.DirectAPI.Numerator, "E's room: a code no website visit started")
	assert.Nil(t, rep.FlowToRoom.DirectAPI.Rate)
	assert.Equal(t, 1, rep.FlowToRoom.Unknown.Numerator, "U's room: no code")
	assert.Nil(t, rep.FlowToRoom.Unknown.Rate)
	// The rooms of website flows are what is left: three rooms from two flows.
	assert.Equal(t, 3, rep.RoomsCreated-rep.FlowToRoom.DirectAPI.Numerator-rep.FlowToRoom.Unknown.Numerator)

	assert.Equal(t, db.WebsiteFlowSteps{Started: 5, PromptCopied: 2, SkillFetched: 2, RoomCreated: 2}, rep.WebsiteFlowSteps,
		"started A B C D F; copied A B (F copied after the window); an agent fetched for A and D (B was a person); rooms from A and D")
	assert.Equal(t, *rep.FlowToRoom.Website.Denominator, rep.WebsiteFlowSteps.Started)
	assert.Equal(t, rep.FlowToRoom.Website.Numerator, rep.WebsiteFlowSteps.RoomCreated)
}

// One website flow that made two rooms is a rate of exactly 1, never 2.
func TestActivationAnalytics_ASentencePastedTwiceIsOneConvertedFlow(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewActivationAnalyticsRepository(pool)

	to := time.Now().Add(-450 * 24 * time.Hour).Truncate(time.Second)
	from := to.Add(-5 * 24 * time.Hour)
	base := from.Add(24 * time.Hour)
	_, err := pool.Exec(ctx, `DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`,
		from.Add(-24*time.Hour), to.Add(24*time.Hour))
	require.NoError(t, err)

	row := func(event, channel string, room any, at time.Time) {
		t.Helper()
		seedFunnelRow(t, ctx, pool, map[string]any{
			"flow_id": "hhhhhhh2", "event_name": event, "source_channel": channel, "actor_type": "anonymous",
			"actor_ref": nil, "room_id": room, "ordinal": nil, "entry_surface": nil, "occurred_at": at,
		})
	}
	row("connection_started", "browser", nil, base)
	row("room_created", "server", uuid.New(), base.Add(time.Minute))
	row("room_created", "server", uuid.New(), base.Add(2*time.Minute))

	rep, err := repo.Measure(ctx, from, to)
	require.NoError(t, err)
	assert.Equal(t, 2, rep.RoomsCreated)
	require.NotNil(t, rep.FlowToRoom.Website.Denominator)
	assert.Equal(t, 1, *rep.FlowToRoom.Website.Denominator)
	assert.Equal(t, 1, rep.FlowToRoom.Website.Numerator)
	require.NotNil(t, rep.FlowToRoom.Website.Rate)
	assert.InDelta(t, 1.0, *rep.FlowToRoom.Website.Rate, 1e-9)
	assert.Equal(t, db.WebsiteFlowSteps{Started: 1, PromptCopied: 0, SkillFetched: 0, RoomCreated: 1}, rep.WebsiteFlowSteps)
}
