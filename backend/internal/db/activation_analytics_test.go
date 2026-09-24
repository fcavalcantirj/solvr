package db_test

import (
	"context"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Activation is measured, not asserted into existence. These tests seed the
// connection funnel in a HISTORICAL window (a slice of the past that ambient
// test traffic — created at NOW by other packages sharing this database —
// cannot reach) so the funnel-based numbers are deterministic, and check the
// message-based historical figure against a census recomputed in the same
// breath so ambient rows land in both numbers.

func newActivationTestPool(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

// seedFunnelRow inserts one funnel_events row with an explicit occurred_at so a
// test can place a step at a chosen instant. Test-only: production records
// steps at NOW through the repository.
func seedFunnelRow(t *testing.T, ctx context.Context, pool *db.Pool, ev map[string]any) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref,
			room_id, ordinal, entry_surface, occurred_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		ev["flow_id"], ev["event_name"], ev["source_channel"], ev["actor_type"],
		ev["actor_ref"], ev["room_id"], ev["ordinal"], ev["entry_surface"], ev["occurred_at"],
	)
	require.NoError(t, err)
}

func TestActivationAnalytics_MeasuresProspectiveActivationInAHistoricalWindow(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewActivationAnalyticsRepository(pool)

	// A window that ends before any ambient (NOW) traffic can appear in it.
	to := time.Now().Add(-90 * 24 * time.Hour)
	from := to.Add(-10 * 24 * time.Hour)
	base := from.Add(24 * time.Hour) // room_created instants sit inside [from,to)

	// Clean any funnel rows a prior run of THIS test left in the historical slice
	// (its room_ids are fresh UUIDs, but flow ids are stable), so re-runs are
	// deterministic. Only touches rows inside the historical window.
	_, err := pool.Exec(ctx, `DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`, from, to)
	require.NoError(t, err)

	// Room W: website origin. Started in browser, room created, two agents joined,
	// two-way exchange reached. This is one activated room.
	roomW := uuid.New()
	flowW := "act_test_web_" + base.Format("150405.000000")
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "connection_started", "source_channel": "browser",
		"actor_type": "anonymous", "actor_ref": nil, "room_id": nil, "ordinal": nil,
		"entry_surface": "homepage_panel", "occurred_at": base,
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aW1", "room_id": roomW, "ordinal": nil,
		"entry_surface": nil, "occurred_at": base.Add(2 * time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aW1", "room_id": roomW, "ordinal": 1,
		"entry_surface": nil, "occurred_at": base.Add(3 * time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aW2", "room_id": roomW, "ordinal": 2,
		"entry_surface": nil, "occurred_at": base.Add(12 * time.Second), // +10s after create
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "first_two_way_exchange", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomW, "ordinal": nil,
		"entry_surface": nil, "occurred_at": base.Add(22 * time.Second), // +20s after create
	})
	// A browser viewer of the room and a repeated exchange attempt: neither may
	// inflate the counts. room_viewed is not activation; first_two_way is deduped
	// by the unique index so a second insert simply does nothing.
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowW, "event_name": "room_viewed", "source_channel": "browser",
		"actor_type": "anonymous", "actor_ref": nil, "room_id": nil, "ordinal": nil,
		"entry_surface": "room_page", "occurred_at": base.Add(30 * time.Second),
	})

	// Room D: direct-API origin. A room created with a flow id but no browser
	// connection_started (an agent created it straight from a pasted prompt).
	// Eight participants join — a room with eight agents is still ONE room and,
	// once it reaches a two-way exchange, ONE activation.
	roomD := uuid.New()
	flowD := "act_test_direct_" + base.Format("150405.000000")
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowD, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aD1", "room_id": roomD, "ordinal": nil,
		"entry_surface": nil, "occurred_at": base.Add(48 * time.Hour),
	})
	for i := 1; i <= 8; i++ {
		seedFunnelRow(t, ctx, pool, map[string]any{
			"flow_id": flowD, "event_name": "participant_joined", "source_channel": "server",
			"actor_type": "agent", "actor_ref": fmt.Sprintf("aD%d", i), "room_id": roomD, "ordinal": i,
			"entry_surface": nil, "occurred_at": base.Add(48*time.Hour + time.Duration(i)*time.Second),
		})
	}
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": flowD, "event_name": "first_two_way_exchange", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomD, "ordinal": nil,
		"entry_surface": nil, "occurred_at": base.Add(48*time.Hour + 40*time.Second), // +40s after create
	})

	// Room U: unknown origin. A room created with NO flow id, and it never reaches
	// a two-way exchange. It counts toward rooms created (unknown origin) but not
	// activation.
	roomU := uuid.New()
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": nil, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aU1", "room_id": roomU, "ordinal": nil,
		"entry_surface": nil, "occurred_at": base.Add(72 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": nil, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "aU1", "room_id": roomU, "ordinal": 1,
		"entry_surface": nil, "occurred_at": base.Add(72*time.Hour + time.Second),
	})

	// A second website flow that started in the browser but never created a room:
	// it lifts the flow->room denominator (two website flows started) above the
	// numerator (one website room created).
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "act_test_web_abandoned_" + base.Format("150405.000000"),
		"event_name": "connection_started", "source_channel": "browser",
		"actor_type": "anonymous", "actor_ref": nil, "room_id": nil, "ordinal": nil,
		"entry_surface": "homepage_panel", "occurred_at": base.Add(96 * time.Hour),
	})

	rep, err := repo.Measure(ctx, from, to)
	require.NoError(t, err)

	// Three rooms created in the window (W website, D direct, U unknown).
	assert.Equal(t, 3, rep.RoomsCreated, "rooms created in window")
	// Two reached a two-way exchange (W and D). U did not.
	assert.Equal(t, 2, rep.ActivatedRooms, "activated rooms (strict prospective)")

	// room->activation: 2 of 3, a valid denominator.
	require.NotNil(t, rep.RoomToActivation.Denominator)
	assert.Equal(t, 3, *rep.RoomToActivation.Denominator)
	assert.Equal(t, 2, rep.RoomToActivation.Numerator)
	require.NotNil(t, rep.RoomToActivation.Rate)
	assert.InDelta(t, 2.0/3.0, *rep.RoomToActivation.Rate, 1e-9)

	// flow->room by origin. Website: two flows started, one made a room.
	require.NotNil(t, rep.FlowToRoom.Website.Denominator)
	assert.Equal(t, 2, *rep.FlowToRoom.Website.Denominator, "website flows started")
	assert.Equal(t, 1, rep.FlowToRoom.Website.Numerator, "website rooms created")
	require.NotNil(t, rep.FlowToRoom.Website.Rate)
	assert.InDelta(t, 0.5, *rep.FlowToRoom.Website.Rate, 1e-9)
	// Direct API: one room, denominator unknown (no browser start to count).
	assert.Nil(t, rep.FlowToRoom.DirectAPI.Denominator, "direct-api has no browser start denominator")
	assert.Equal(t, 1, rep.FlowToRoom.DirectAPI.Numerator)
	assert.Nil(t, rep.FlowToRoom.DirectAPI.Rate)
	// Unknown: one room (no flow id), denominator unknown.
	assert.Nil(t, rep.FlowToRoom.Unknown.Denominator)
	assert.Equal(t, 1, rep.FlowToRoom.Unknown.Numerator)
	assert.Nil(t, rep.FlowToRoom.Unknown.Rate)
	// The three origin numerators partition rooms created.
	assert.Equal(t, rep.RoomsCreated,
		rep.FlowToRoom.Website.Numerator+rep.FlowToRoom.DirectAPI.Numerator+rep.FlowToRoom.Unknown.Numerator)

	// Durations. Only W has an ordinal-2 join (D's joins are ordinals 1..8, so it
	// also has an ordinal-2 join — both W and D contribute).
	// time to second agent: W = 10s (10000ms), D = 2s (2000ms).
	assert.Equal(t, 2, rep.TimeToSecondAgentMS.Count)
	require.NotNil(t, rep.TimeToSecondAgentMS.Median)
	assert.InDelta(t, percentileCont([]float64{2000, 10000}, 0.5), *rep.TimeToSecondAgentMS.Median, 0.5)
	require.NotNil(t, rep.TimeToSecondAgentMS.P90)
	assert.InDelta(t, percentileCont([]float64{2000, 10000}, 0.9), *rep.TimeToSecondAgentMS.P90, 0.5)

	// time to first exchange: W = 20s (20000ms), D = 40s (40000ms).
	assert.Equal(t, 2, rep.TimeToFirstExchangeMS.Count)
	require.NotNil(t, rep.TimeToFirstExchangeMS.Median)
	assert.InDelta(t, percentileCont([]float64{20000, 40000}, 0.5), *rep.TimeToFirstExchangeMS.Median, 0.5)
	require.NotNil(t, rep.TimeToFirstExchangeMS.P90)
	assert.InDelta(t, percentileCont([]float64{20000, 40000}, 0.9), *rep.TimeToFirstExchangeMS.P90, 0.5)

	// Participants: W(2) and D(8) and U(1) have joins -> 3 rooms with participants.
	// Max participants is 8. Rooms with later joins (a 3rd+ agent) is 1 (only D).
	assert.Equal(t, 3, rep.Participants.RoomsWithParticipants)
	assert.Equal(t, 8, rep.Participants.MaxParticipants)
	assert.Equal(t, 1, rep.Participants.RoomsWithLaterJoins)

	assert.False(t, rep.GeneratedAt.IsZero())
	assert.NotEmpty(t, rep.ActivationDefinition)
}

// One agent using two display names is one authenticated identity: it must not
// reach a two-way exchange, so it is not activated. This drives the real
// milestone recorder, not a hand-written funnel row, so the definition the
// product enforces is what the measurement counts.
func TestActivationAnalytics_TwoDisplayNamesOneIdentity_DoNotActivate(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	funnel := db.NewFunnelEventRepository(pool)

	stamp := time.Now().Format("150405")
	slug := "act-test-twonames-" + stamp

	var room uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO rooms (slug, display_name, token_hash, is_private)
		VALUES ($1, 'two names one identity', 'hash_act_twonames_test', false)
		RETURNING id`, slug).Scan(&room)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, room) })

	// Two messages, same authenticated author_id, DIFFERENT display names.
	author := "agent-single-" + stamp
	for i, name := range []string{author + "-planner", author + "-executor"} {
		_, err := pool.Exec(ctx, `
			INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
			VALUES ($1,$2,'agent',$3,$4)`,
			room, name, author, fmt.Sprintf("msg %d", i))
		require.NoError(t, err)
	}

	recorded, err := funnel.RecordFirstTwoWayExchange(ctx, room)
	require.NoError(t, err)
	assert.False(t, recorded,
		"two display names of ONE authenticated identity must not record a two-way exchange")
}

// The message-based historical figure is checked against a census recomputed a
// different way in the same breath, so ambient rooms created by other packages
// land in both numbers and only a wrong definition shows up as a disagreement.
func TestActivationAnalytics_HistoricalMultiAuthorMatchesCensus(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewActivationAnalyticsRepository(pool)

	to := time.Now()
	from := to.Add(-30 * 24 * time.Hour)
	rep, err := repo.Measure(ctx, from, to)
	require.NoError(t, err)

	var census int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT room_id
			  FROM messages
			 WHERE author_type = 'agent' AND deleted_at IS NULL
			 GROUP BY room_id
			HAVING COUNT(DISTINCT agent_name) >= 2
		) q`).Scan(&census)
	require.NoError(t, err)

	assert.Equal(t, census, rep.HistoricalMultiAuthorRooms,
		"the historical multi-author figure must match a census of rooms with >=2 distinct message authors")
	assert.NotEmpty(t, rep.HistoricalNote, "the historical figure must be labeled, not silently mixed with activation")
}

// percentileCont replicates PostgreSQL's percentile_cont (continuous percentile
// with linear interpolation) so the test's oracle matches the repository's SQL.
func percentileCont(values []float64, p float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := math.Floor(rank)
	hi := math.Ceil(rank)
	if lo == hi {
		return sorted[int(lo)]
	}
	return sorted[int(lo)] + (rank-lo)*(sorted[int(hi)]-sorted[int(lo)])
}
