package db_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Share attribution (idx 88 step 4): a room created from a public source records that
// source on its room_created step, and every later server step of the room — joins and
// the first two-way exchange — inherits it, the way they inherit flow_id.

func TestFunnelEventRepository_SourceTravelsFromRoomCreationThroughActivation(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	msgs := db.NewMessageRepository(pool)
	origin := createMemberTestRoom(ctx, t, pool, "fn-src-origin-"+time.Now().Format("150405"), false)
	room := createMemberTestRoom(ctx, t, pool, "fn-src-fresh-"+time.Now().Format("150405"), false)
	src := db.FunnelSource{Kind: models.FunnelSourceKindRoom, ID: origin.ID}

	require.NoError(t, repo.RecordRoomCreatedFrom(ctx, room.ID, models.FunnelActorAgent, "planner", "f_src_flow", src))
	for _, a := range []string{"planner", "executor"} {
		_, rec, err := repo.RecordParticipantJoined(ctx, room.ID, models.FunnelActorAgent, a)
		require.NoError(t, err)
		require.True(t, rec)
		_, err = msgs.Create(ctx, models.CreateMessageParams{
			RoomID: room.ID, AuthorType: "agent", AgentName: a, Content: "hi", ContentType: "text",
		})
		require.NoError(t, err)
	}
	rec, err := repo.RecordFirstTwoWayExchange(ctx, room.ID)
	require.NoError(t, err)
	require.True(t, rec)

	events, err := repo.ListByRoom(ctx, room.ID)
	require.NoError(t, err)
	require.Len(t, events, 4)
	for _, e := range events {
		require.Equal(t, models.FunnelSourceKindRoom, e.SourceKind, "%s", e.EventName)
		require.Equal(t, origin.ID.String(), e.SourceID, "%s", e.EventName)
	}
}

func TestFunnelEventRepository_AnUnsourcedRoomStaysUnsourced(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	room := createMemberTestRoom(ctx, t, pool, "fn-src-none-"+time.Now().Format("150405"), false)

	require.NoError(t, repo.RecordRoomCreated(ctx, room.ID, models.FunnelActorAgent, "solo", "f_nosrc"))
	_, _, err := repo.RecordParticipantJoined(ctx, room.ID, models.FunnelActorAgent, "solo")
	require.NoError(t, err)

	events, err := repo.ListByRoom(ctx, room.ID)
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, e := range events {
		require.Empty(t, e.SourceKind)
		require.Empty(t, e.SourceID)
	}
}

func TestFunnelEventRepository_BrowserShareStepsCarryTheirSource(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	postID := uuid.New()
	flow := "f_share_" + time.Now().Format("150405.000000")

	for _, name := range []string{models.FunnelShareVisit, models.FunnelShareLinkCopied} {
		require.NoError(t, repo.RecordBrowserEvent(ctx, db.BrowserFunnelEvent{
			FlowID: flow, EventName: name, ActorType: models.FunnelActorAnonymous,
			EntrySurface: "post_page", Source: db.FunnelSource{Kind: models.FunnelSourceKindPost, ID: postID},
		}))
	}
	events, err := repo.ListByFlow(ctx, flow)
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, e := range events {
		require.Equal(t, models.FunnelSourceKindPost, e.SourceKind)
		require.Equal(t, postID.String(), e.SourceID)
	}
}

func TestFunnelEvents_SourceKindAndIDAreBothOrNeither(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	_, err := pool.Exec(ctx, `INSERT INTO funnel_events (event_name, source_channel, actor_type, source_kind)
		VALUES ('share_visit', 'browser', 'anonymous', 'room')`)
	require.Error(t, err, "a source kind without an id must be refused")
	_, err = pool.Exec(ctx, `INSERT INTO funnel_events (event_name, source_channel, actor_type, source_kind, source_id)
		VALUES ('share_visit', 'browser', 'anonymous', 'website', $1)`, uuid.New())
	require.Error(t, err, "an unknown source kind must be refused")
}

func TestFunnelEventContract_NamesTheShareSteps(t *testing.T) {
	require.True(t, models.IsBrowserFunnelEvent(models.FunnelShareVisit))
	require.True(t, models.IsBrowserFunnelEvent(models.FunnelShareLinkCopied))
	names := map[string]bool{}
	for _, spec := range models.FunnelEventContract() {
		names[spec.Name] = true
	}
	require.True(t, names[models.FunnelShareVisit])
	require.True(t, names[models.FunnelShareLinkCopied])
}
