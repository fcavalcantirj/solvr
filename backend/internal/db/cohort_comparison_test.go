package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Cohort comparison is measured from the connection funnel with one launch
// timestamp and identical definitions for every window, so the redesign can be
// judged over consistent post-launch cohorts rather than a moving all-time bag.
// These tests seed the funnel (and the rooms rows a visibility split resolves
// against) in a HISTORICAL slice that ambient NOW traffic from parallel packages
// cannot reach, so every cohort number is deterministic.

// seedCohortRoom inserts a real rooms row with an explicit id so a cohort
// visibility split has something to resolve a funnel room_id against.
func seedCohortRoom(t *testing.T, ctx context.Context, pool *db.Pool, id uuid.UUID, slug string, private bool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO rooms (id, slug, display_name, is_private)
		VALUES ($1, $2, $3, $4)`,
		id, slug, "cohort test "+slug, private)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, id) })
}

func TestCohortComparison_MeasuresConsistentPostLaunchWindows(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewCohortComparisonRepository(pool)

	now := time.Now().UTC()
	// A launch deep in the past: both the 7-day and 28-day windows have fully
	// elapsed, and the whole [launch, launch+28d) slice sits before any NOW
	// traffic other packages create in this shared database.
	launch := now.Add(-200 * 24 * time.Hour)
	stamp := launch.Format("150405.000000")
	sfx := uuid.NewString()[:8] // slug-safe unique suffix ([a-z0-9], no dots)

	// Clear anything a prior run of THIS test left inside the slice so re-runs are
	// deterministic. Only touches rows inside the historical window.
	windowEnd := launch.Add(28 * 24 * time.Hour)
	_, err := pool.Exec(ctx, `DELETE FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2`,
		launch.Add(-10*24*time.Hour), windowEnd)
	require.NoError(t, err)

	aCreator := "cc_A_creator_" + stamp
	bCreator := "cc_B_creator_" + stamp
	dCreator := "cc_D_creator_" + stamp

	// Room A: in the 7-day window, PUBLIC, NEW creator, ACTIVATED (two agents
	// exchange), a later agent session after the exchange (a return), and a Post
	// published from it.
	roomA := uuid.New()
	seedCohortRoom(t, ctx, pool, roomA, "cc-a-"+sfx, false)
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_A_" + stamp, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": aCreator, "room_id": roomA, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(24 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_A_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": aCreator, "room_id": roomA, "ordinal": 1,
		"entry_surface": nil, "occurred_at": launch.Add(24*time.Hour + time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_A_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "cc_A2_" + stamp, "room_id": roomA, "ordinal": 2,
		"entry_surface": nil, "occurred_at": launch.Add(24*time.Hour + 11*time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_A_" + stamp, "event_name": "first_two_way_exchange", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomA, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(24*time.Hour + 21*time.Second),
	})
	// A later agent session after the exchange -> Room A has a return.
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_A_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "cc_A3_" + stamp, "room_id": roomA, "ordinal": 3,
		"entry_surface": nil, "occurred_at": launch.Add(48 * time.Hour),
	})
	// A Post published from Room A -> one room-to-post publication.
	_, err = pool.Exec(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, source_room_id)
		VALUES ('post', $1, 'from a room', 'agent', $2, $3)`,
		"cc post "+stamp, aCreator, roomA)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM posts WHERE source_room_id = $1`, roomA) })

	// Room B: in the 7-day window, PRIVATE, RETURNING creator (seen before launch),
	// a second agent joined but the exchange never happened -> a second-agent
	// connection failure, not an activation.
	roomB := uuid.New()
	seedCohortRoom(t, ctx, pool, roomB, "cc-b-"+sfx, true)
	seedFunnelRow(t, ctx, pool, map[string]any{ // earlier activity: B's creator is returning
		"flow_id": "cc_B_pre_" + stamp, "event_name": "connection_started", "source_channel": "browser",
		"actor_type": "agent", "actor_ref": bCreator, "room_id": nil, "ordinal": nil,
		"entry_surface": "homepage_panel", "occurred_at": launch.Add(-120 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_B_" + stamp, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": bCreator, "room_id": roomB, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(50 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_B_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": bCreator, "room_id": roomB, "ordinal": 1,
		"entry_surface": nil, "occurred_at": launch.Add(50*time.Hour + time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_B_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "cc_B2_" + stamp, "room_id": roomB, "ordinal": 2,
		"entry_surface": nil, "occurred_at": launch.Add(50*time.Hour + 5*time.Second),
	})

	// Room C: in the 7-day window, UNKNOWN visibility (no rooms row), UNKNOWN
	// creator (no pseudonymous ref), never activated.
	roomC := uuid.New()
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": nil, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomC, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(70 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": nil, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomC, "ordinal": 1,
		"entry_surface": nil, "occurred_at": launch.Add(70*time.Hour + time.Second),
	})

	// Room D: in the 28-day window but NOT the 7-day window, PUBLIC, NEW creator,
	// ACTIVATED. It makes the 28-day window carry more rooms AND more activations
	// than the 7-day window, so the two are distinguishable.
	roomD := uuid.New()
	seedCohortRoom(t, ctx, pool, roomD, "cc-d-"+sfx, false)
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_D_" + stamp, "event_name": "room_created", "source_channel": "server",
		"actor_type": "agent", "actor_ref": dCreator, "room_id": roomD, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(10 * 24 * time.Hour),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_D_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": dCreator, "room_id": roomD, "ordinal": 1,
		"entry_surface": nil, "occurred_at": launch.Add(10*24*time.Hour + time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_D_" + stamp, "event_name": "participant_joined", "source_channel": "server",
		"actor_type": "agent", "actor_ref": "cc_D2_" + stamp, "room_id": roomD, "ordinal": 2,
		"entry_surface": nil, "occurred_at": launch.Add(10*24*time.Hour + 11*time.Second),
	})
	seedFunnelRow(t, ctx, pool, map[string]any{
		"flow_id": "cc_D_" + stamp, "event_name": "first_two_way_exchange", "source_channel": "server",
		"actor_type": "agent", "actor_ref": nil, "room_id": roomD, "ordinal": nil,
		"entry_surface": nil, "occurred_at": launch.Add(10*24*time.Hour + 31*time.Second),
	})

	rep, err := repo.Compare(ctx, launch, now)
	require.NoError(t, err)

	// Both windows use the same launch and are fully elapsed.
	assert.Equal(t, launch, rep.Launch.UTC())
	assert.Equal(t, 7, rep.Window7d.Days)
	assert.Equal(t, 28, rep.Window28d.Days)
	assert.True(t, rep.Window7d.Complete, "the 7-day window has elapsed")
	assert.True(t, rep.Window28d.Complete, "the 28-day window has elapsed")
	assert.Equal(t, launch, rep.Window7d.WindowStart.UTC())
	assert.Equal(t, launch.Add(7*24*time.Hour), rep.Window7d.WindowEnd.UTC())
	assert.Equal(t, launch.Add(28*24*time.Hour), rep.Window28d.WindowEnd.UTC())

	// 7-day window: rooms A, B, C created.
	w7 := rep.Window7d
	assert.Equal(t, 3, w7.RoomsCreated, "7d rooms created")
	assert.Equal(t, 1, w7.ActivatedRooms, "7d activated (A)")
	require.NotNil(t, w7.RoomToActivation.Rate)
	assert.InDelta(t, 1.0/3.0, *w7.RoomToActivation.Rate, 1e-9)
	assert.Equal(t, 1, w7.SecondAgentConnectionFailures, "7d second-agent failure (B joined, no exchange)")
	assert.Equal(t, 1, w7.RoomsWithReturn, "7d rooms with a later session (A)")
	assert.Equal(t, 1, w7.RoomToPostPublications, "7d room-to-post (A)")
	assert.Equal(t, 1, w7.TimeToFirstExchangeMS.Count)
	require.NotNil(t, w7.TimeToFirstExchangeMS.Median)
	assert.InDelta(t, 21000.0, *w7.TimeToFirstExchangeMS.Median, 0.5)
	assert.Equal(t, 1, w7.Visibility.PublicRooms, "7d public (A)")
	assert.Equal(t, 1, w7.Visibility.PrivateRooms, "7d private (B)")
	assert.Equal(t, 1, w7.Visibility.Unknown, "7d unknown visibility (C)")
	assert.Equal(t, 1, w7.Creators.NewCreator, "7d new creator (A)")
	assert.Equal(t, 1, w7.Creators.ReturningCreator, "7d returning creator (B)")
	assert.Equal(t, 1, w7.Creators.Unknown, "7d unknown creator (C)")

	// 28-day window: rooms A, B, C, D created.
	w28 := rep.Window28d
	assert.Equal(t, 4, w28.RoomsCreated, "28d rooms created")
	assert.Equal(t, 2, w28.ActivatedRooms, "28d activated (A, D)")
	require.NotNil(t, w28.RoomToActivation.Rate)
	assert.InDelta(t, 0.5, *w28.RoomToActivation.Rate, 1e-9)
	assert.Equal(t, 1, w28.SecondAgentConnectionFailures, "28d second-agent failure (B)")
	assert.Equal(t, 2, w28.TimeToFirstExchangeMS.Count, "28d has A and D exchanges")
	assert.Equal(t, 2, w28.Visibility.PublicRooms, "28d public (A, D)")
	assert.Equal(t, 1, w28.Visibility.PrivateRooms)
	assert.Equal(t, 2, w28.Creators.NewCreator, "28d new creators (A, D)")

	// Distinguishing more room creation from more successful collaboration: both
	// the count and the conversion are reported separately per window, so an
	// analyst can see the 28-day window has more rooms AND more activations without
	// the two being conflated.
	assert.Greater(t, w28.RoomsCreated, w7.RoomsCreated, "creation is readable on its own")
	assert.Greater(t, w28.ActivatedRooms, w7.ActivatedRooms, "successful collaboration is readable on its own")

	// External sources are marked unavailable, never zero-filled.
	assert.False(t, rep.ExternalSources.GoogleAnalytics.Available)
	assert.NotEmpty(t, rep.ExternalSources.GoogleAnalytics.Note)
	assert.False(t, rep.ExternalSources.SearchConsole.Available)
	assert.NotEmpty(t, rep.ExternalSources.SearchConsole.Note)

	assert.NotEmpty(t, rep.ExclusionsNote, "internal/probe exclusions must be documented")
	assert.NotEmpty(t, rep.ActivationDefinition)
	assert.False(t, rep.GeneratedAt.IsZero())
}

// A launch recent enough that the 28-day window has not elapsed reports the
// 7-day window complete and the 28-day window incomplete, keeping both usable
// rather than omitting or zero-filling the unfinished one.
func TestCohortComparison_IncompleteWindowIsLabeled(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewCohortComparisonRepository(pool)

	now := time.Now().UTC()
	launch := now.Add(-10 * 24 * time.Hour) // 7d elapsed, 28d not

	rep, err := repo.Compare(ctx, launch, now)
	require.NoError(t, err)

	assert.True(t, rep.Window7d.Complete, "7-day window ends before now")
	assert.False(t, rep.Window28d.Complete, "28-day window ends after now")
	assert.Equal(t, launch.Add(28*24*time.Hour), rep.Window28d.WindowEnd.UTC())
	// The incomplete window is still present and numeric, not omitted.
	assert.GreaterOrEqual(t, rep.Window28d.RoomsCreated, 0)
}

// The all-time historical figure is CONTEXT, checked against a census recomputed
// a different way in the same breath so ambient rooms land in both numbers, and
// it is labeled, never merged into a cohort window.
func TestCohortComparison_HistoricalContextMatchesCensus(t *testing.T) {
	pool, ctx := newActivationTestPool(t)
	repo := db.NewCohortComparisonRepository(pool)

	now := time.Now().UTC()
	rep, err := repo.Compare(ctx, now.Add(-200*24*time.Hour), now)
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

	assert.Equal(t, census, rep.Historical.AllTimeMultiAuthorRooms,
		"the all-time context must match a census of rooms with >=2 distinct message authors")
	assert.NotEmpty(t, rep.Historical.Note, "the historical figure must be labeled as context")
}
