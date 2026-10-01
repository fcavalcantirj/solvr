package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/hub"
)

// Rooms changed by any writer and agents removed from Solvr leave every instance's public room
// projections at commit (idx 77 step 5, migration 000124). Measured on HEAD before it (idx 77
// slice 11 spike, live API): a room made private, soft-deleted, row-deleted or renamed by SQL
// stayed on GET /v1/overview until the 30 s snapshot ran out (a rename through PATCH did too:
// RoomHandler announced visibility only); a self-deleted or banned agent stayed listed as online,
// card included, in GET /v1/rooms/{slug} and /agents until its 600 s TTL ran out. Two instances
// below share a database and nothing else.

// overviewRoomName returns the room name GET /v1/overview's activity shows for slug ("" when it
// is not listed).
func overviewRoomName(t *testing.T, base, slug string) string {
	t.Helper()
	var ov struct {
		Data struct {
			Activity struct {
				Groups []struct {
					RoomSlug string `json:"room_slug"`
					RoomName string `json:"room_name"`
				} `json:"groups"`
			} `json:"activity"`
		} `json:"data"`
	}
	getJSON(t, base+"/v1/overview", &ov)
	for _, g := range ov.Data.Activity.Groups {
		if g.RoomSlug == slug {
			return g.RoomName
		}
	}
	return ""
}

// overviewRoomWithin waits until every instance's overview shows want for the room ("" = not
// listed) and fails when one still shows something else after the deadline.
func overviewRoomWithin(t *testing.T, deadline time.Duration, slug, want, what string, instances ...*roomInstance) {
	t.Helper()
	start := time.Now()
	for _, inst := range instances {
		got := overviewRoomName(t, inst.ts.URL, slug)
		for got != want && time.Since(start) < deadline {
			time.Sleep(25 * time.Millisecond)
			got = overviewRoomName(t, inst.ts.URL, slug)
		}
		require.Equal(t, want, got, "%s: the overview still shows %q %s after the change", what, got, time.Since(start).Round(time.Millisecond))
	}
	t.Logf("%s: every overview caught up in %s", what, time.Since(start).Round(time.Millisecond))
}

func TestOverview_ARoomChangedByAnyWriterLeavesEveryInstancesSnapshotAtCommit(t *testing.T) {
	t.Setenv("DATABASE_URL", newHomepageScratchURL(t))
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, key := registerRoomTestAgent(t, a.ts)
	listed := func(name string) string {
		t.Helper()
		status, out := doJSON(t, http.MethodPost, a.ts.URL+"/v1/rooms", key, `{"display_name":"`+name+`","is_private":false}`)
		require.Equal(t, http.StatusCreated, status, "create room: %v", out)
		slug := entryData(t, out)["slug"].(string)
		tok := handshakeRoomToken(t, a.ts, slug, key)
		postRoomMessage(t, a.ts.URL, slug, tok, "keeper", "a message in "+name)
		return slug
	}
	madePrivate := listed("Room made private by SQL")
	softDeleted := listed("Room soft deleted by SQL")
	rowDeleted := listed("Room row deleted by SQL")
	renamedSQL := listed("Room renamed by SQL")
	renamedAPI := listed("Room renamed through the API")
	for _, inst := range []*roomInstance{a, b} {
		for slug, name := range map[string]string{madePrivate: "Room made private by SQL", softDeleted: "Room soft deleted by SQL",
			rowDeleted: "Room row deleted by SQL", renamedSQL: "Room renamed by SQL", renamedAPI: "Room renamed through the API"} {
			require.Equal(t, name, overviewRoomName(t, inst.ts.URL, slug), "the snapshot lists the room first")
		}
	}

	const deadline = 3 * time.Second
	exec := func(sql, slug string) {
		t.Helper()
		_, err := a.pool.Exec(ctx, sql, slug)
		require.NoError(t, err, sql)
	}
	exec(`UPDATE rooms SET is_private = TRUE WHERE slug = $1`, madePrivate)
	overviewRoomWithin(t, deadline, madePrivate, "", "room made private by SQL", a, b)
	exec(`UPDATE rooms SET deleted_at = NOW() WHERE slug = $1`, softDeleted)
	overviewRoomWithin(t, deadline, softDeleted, "", "room soft deleted by SQL", a, b)
	exec(`DELETE FROM rooms WHERE slug = $1`, rowDeleted)
	overviewRoomWithin(t, deadline, rowDeleted, "", "room row deleted by SQL", a, b)
	exec(`UPDATE rooms SET display_name = 'Room renamed by SQL, new name' WHERE slug = $1`, renamedSQL)
	overviewRoomWithin(t, deadline, renamedSQL, "Room renamed by SQL, new name", "room renamed by SQL", a, b)

	status, out := doJSONAtCurrentVersion(t, http.MethodPatch, b.ts.URL+"/v1/rooms/"+renamedAPI, key, `{"display_name":"Room renamed through the API, new name"}`)
	require.Equal(t, http.StatusOK, status, "rename: %v", out)
	overviewRoomWithin(t, deadline, renamedAPI, "Room renamed through the API, new name", "room renamed through PATCH on b", a, b)
}

// publicPresence returns the agent names GET /v1/rooms/{slug}/agents lists on inst.
func publicPresence(t *testing.T, inst *roomInstance, slug, bearer string) []string {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, inst.ts.URL+"/v1/rooms/"+slug+"/agents", bearer, "")
	require.Equal(t, http.StatusOK, status, "list presence: %v", out)
	names := []string{}
	for _, rec := range out["data"].([]any) {
		names = append(names, rec.(map[string]any)["agent_name"].(string))
	}
	return names
}

func TestRoomPresence_ARemovedAgentLeavesEveryInstanceOnce(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newAgentPairRoom(t, a, false)
	onA := openLiveStream(t, a.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)
	onB := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)
	joinPresence(t, a, room.slug, room.plannerTok, "planner", 600)
	joinPresence(t, a, room.slug, room.executorTok, "executor", 600)
	require.Equal(t, []presenceFrame{{Event: "presence_join", CardName: "planner-card"}}, onB.waitPresence(t, "planner", 1))
	for _, inst := range []*roomInstance{a, b} {
		require.ElementsMatch(t, []string{"planner", "executor"}, publicPresence(t, inst, room.slug, room.executorKey))
		require.Equal(t, float64(2), roomOnlineCount(t, inst, room.slug, room.executorKey))
	}

	status, out := doJSON(t, http.MethodDelete, b.ts.URL+"/v1/agents/me", room.plannerKey, "")
	require.Equal(t, http.StatusOK, status, "the planner deletes its account through b: %v", out)

	want := []presenceFrame{{Event: "presence_join", CardName: "planner-card"}, {Event: "presence_leave"}}
	require.Equal(t, want, onA.waitPresence(t, "planner", 2), "a stream on A sees the removed agent leave, once")
	require.Equal(t, want, onB.waitPresence(t, "planner", 2), "a stream on B sees the removed agent leave, once")
	for _, inst := range []*roomInstance{a, b} {
		require.Equal(t, []string{"executor"}, publicPresence(t, inst, room.slug, room.executorKey), "the removed agent is not listed as online")
		require.Equal(t, float64(1), roomOnlineCount(t, inst, room.slug, room.executorKey))
		_, held := inst.registry.Get(hub.NewRoomID(uuid.MustParse(room.roomID)), "planner")
		require.False(t, held, "no instance keeps an in-memory entry for the removed agent")
	}
	require.Equal(t, []presenceFrame{{Event: "presence_join", CardName: "executor-card"}}, onA.presenceFrames(t, "executor"),
		"the peer that stays is untouched")
}
