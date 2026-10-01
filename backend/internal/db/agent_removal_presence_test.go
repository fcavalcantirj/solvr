package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// A removed agent (self-deleted or banned) can no longer join or heartbeat: its room tokens
// stop resolving (idx 75). Its presence rows stayed, so measured on HEAD before migration 000124
// (idx 77 slice 11 spike, live API) GET /v1/rooms/{slug} and GET /v1/rooms/{slug}/agents kept
// listing it as online, with its card, until its TTL ran out (600 s by default), and the homepage
// counted it. The removal now ends its presence in the same transaction and announces each leave
// on RoomPresenceChannel, so every instance's streams show it (hub.PresenceChange, origin
// "database"). Its membership and tokens are left as they are.

// presenceLeave is the part of a hub.PresenceChange notice these tests read.
type presenceLeave struct {
	RoomID    string `json:"room_id"`
	AgentName string `json:"agent_name"`
	Joined    bool   `json:"joined"`
	Origin    string `json:"origin"`
}

// presenceNotices returns the presence notices committed since its previous call (sentinel
// read, like overviewNotices.since).
func presenceNotices(ctx context.Context, t *testing.T, pool *Pool) func() []presenceLeave {
	t.Helper()
	conn, err := pgx.ConnectConfig(ctx, pool.pool.Config().ConnConfig.Copy())
	require.NoError(t, err, "connect listener")
	t.Cleanup(func() { conn.Close(context.Background()) })
	_, err = conn.Exec(ctx, "LISTEN "+RoomPresenceChannel)
	require.NoError(t, err)
	return func() []presenceLeave {
		t.Helper()
		_, err := pool.Exec(ctx, `SELECT pg_notify($1, 'sentinel')`, RoomPresenceChannel)
		require.NoError(t, err)
		out := []presenceLeave{}
		for {
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			msg, err := conn.WaitForNotification(wctx)
			cancel()
			require.NoError(t, err, "the sentinel notice never arrived")
			if msg.Payload == "sentinel" {
				return out
			}
			var c presenceLeave
			require.NoError(t, json.Unmarshal([]byte(msg.Payload), &c), "payload %q", msg.Payload)
			out = append(out, c)
		}
	}
}

func TestAgentRemoval_EndsItsRoomPresenceAndAnnouncesEachLeave(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	suffix := uuid.NewString()[:8]
	removed := authorAgent(ctx, t, pool, "arp_removed_"+suffix)
	banned := authorAgent(ctx, t, pool, "arp_banned_"+suffix)
	stays := authorAgent(ctx, t, pool, "arp_stays_"+suffix)
	r1 := insertActivityRoom(ctx, t, pool, "arp-room-one")
	r2 := insertActivityRoom(ctx, t, pool, "arp-room-two")
	members, presence := NewRoomMemberRepository(pool), NewAgentPresenceRepository(pool)
	present := func(room uuid.UUID, agent, name string) {
		t.Helper()
		_, err := members.Add(ctx, models.AddRoomMemberParams{RoomID: room, AgentID: agent, Role: models.RoleMember})
		require.NoError(t, err)
		_, err = presence.Upsert(ctx, models.UpsertAgentPresenceParams{
			RoomID: room, AgentID: agent, AgentName: name, CardJSON: json.RawMessage(`{"name":"` + name + `"}`), TTLSeconds: 600,
		})
		require.NoError(t, err)
	}
	present(r1, removed, "removed-in-one")
	present(r2, removed, "removed-in-two")
	present(r1, banned, "banned")
	present(r1, stays, "stays-in-one")
	present(r2, stays, "stays-in-two")
	names := func(room uuid.UUID) []string {
		t.Helper()
		rows, err := presence.ListByRoom(ctx, room)
		require.NoError(t, err)
		out := []string{}
		for _, rec := range rows {
			out = append(out, rec.AgentName)
		}
		return out
	}
	activeMembership := func(agent string) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_members WHERE agent_id = $1 AND revoked_at IS NULL`, agent).Scan(&n))
		return n
	}
	since := presenceNotices(ctx, t, pool)
	require.Empty(t, since(), "joining announces nothing from the database")

	_, err := pool.Exec(ctx, `UPDATE agents SET display_name = 'renamed', bio = 'a bio' WHERE id = $1`, stays)
	require.NoError(t, err)
	require.Empty(t, since(), "an edit of a live agent ends nothing")

	require.NoError(t, NewAgentRepository(pool).Delete(ctx, removed))
	require.ElementsMatch(t, []presenceLeave{
		{RoomID: r1.String(), AgentName: "removed-in-one", Origin: "database"},
		{RoomID: r2.String(), AgentName: "removed-in-two", Origin: "database"},
	}, since(), "a self-deleted agent leaves every room it was present in, once each")
	require.ElementsMatch(t, []string{"banned", "stays-in-one"}, names(r1))
	require.ElementsMatch(t, []string{"stays-in-two"}, names(r2))
	require.Equal(t, 2, activeMembership(removed), "the membership is left as it is")

	require.ErrorIs(t, NewAgentRepository(pool).Delete(ctx, removed), ErrAgentNotFound)
	require.Empty(t, since(), "deleting an already deleted agent announces nothing")

	bans := NewAccountBanRepository(pool)
	_, err = bans.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: stays, Reason: "dry run", DryRun: true})
	require.NoError(t, err)
	require.Empty(t, since(), "a dry-run ban rolls back: nothing ends, nothing is announced")
	require.ElementsMatch(t, []string{"banned", "stays-in-one"}, names(r1))

	_, err = bans.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: banned, Reason: "slice 11 test"})
	require.NoError(t, err)
	require.Equal(t, []presenceLeave{{RoomID: r1.String(), AgentName: "banned", Origin: "database"}}, since(),
		"a banned agent leaves the room it was present in")
	require.ElementsMatch(t, []string{"stays-in-one"}, names(r1))

	_, err = bans.BanAccount(ctx, BanRequest{AccountType: "agent", AccountID: removed, Reason: "already deleted"})
	require.NoError(t, err)
	require.Empty(t, since(), "banning an already deleted agent announces nothing")
	require.ElementsMatch(t, []string{"stays-in-one"}, names(r1))
	require.ElementsMatch(t, []string{"stays-in-two"}, names(r2))
}
