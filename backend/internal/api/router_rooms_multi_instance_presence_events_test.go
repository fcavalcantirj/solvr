package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/jobs"
)

// Live presence events across more than one API instance: a stream on any instance sees
// every agent's presence_join and presence_leave exactly once, wherever the agent joined,
// left or expired.

// presenceFrame is one presence_join/presence_leave frame for a named agent.
type presenceFrame struct {
	Event    string
	CardName string
}

// presenceFrames returns the stream's presence frames about agent, in order.
func (ls *liveStream) presenceFrames(t *testing.T, agent string) []presenceFrame {
	t.Helper()
	ls.mu.Lock()
	defer ls.mu.Unlock()
	var out []presenceFrame
	for _, f := range ls.frames {
		if f.Event != "presence_join" && f.Event != "presence_leave" {
			continue
		}
		var evt struct {
			AgentName string `json:"agent_name"`
			Payload   struct {
				Name string `json:"name"`
			} `json:"payload"`
		}
		require.NoError(t, json.Unmarshal([]byte(f.Data), &evt), "frame %q", f.Data)
		if evt.AgentName == agent {
			out = append(out, presenceFrame{Event: f.Event, CardName: evt.Payload.Name})
		}
	}
	return out
}

// waitPresence waits until the stream delivered n presence frames about agent, then a
// little longer so a duplicate would be caught.
func (ls *liveStream) waitPresence(t *testing.T, agent string, n int) []presenceFrame {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for len(ls.presenceFrames(t, agent)) < n && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	return ls.presenceFrames(t, agent)
}

func leavePresence(t *testing.T, inst *roomInstance, slug, roomTok, name string) {
	t.Helper()
	status, out := doJSON(t, http.MethodPost, inst.ts.URL+"/r/"+slug+"/leave", roomTok, `{"agent_name":"`+name+`"}`)
	require.Equal(t, http.StatusOK, status, "leave: %v", out)
}

func TestRoomPresenceEvents_JoinAndLeaveReachStreamsOnEveryInstanceOnce(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newAgentPairRoom(t, a, false)
	onA := openLiveStream(t, a.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)
	onB := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)

	joinPresence(t, a, room.slug, room.plannerTok, "planner", 600)
	leavePresence(t, b, room.slug, room.plannerTok, "planner") // leaves through the OTHER instance

	want := []presenceFrame{{Event: "presence_join", CardName: "planner-card"}, {Event: "presence_leave"}}
	require.Equal(t, want, onB.waitPresence(t, "planner", 2), "a stream on B sees the join made through A and the leave made through B, once each")
	require.Equal(t, want, onA.waitPresence(t, "planner", 2), "a stream on A sees the join made through A and the leave made through B, once each")
	_, held := a.registry.Get(hub.NewRoomID(uuid.MustParse(room.roomID)), "planner")
	require.False(t, held, "instance A drops its in-memory entry for an agent that left through B")
}

func TestRoomPresenceEvents_ExpiryReapedByAnotherInstanceReachesEveryStream(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newAgentPairRoom(t, a, false)
	onA := openLiveStream(t, a.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)

	joinPresence(t, a, room.slug, room.plannerTok, "planner", 1)
	time.Sleep(1500 * time.Millisecond)
	reaper := jobs.NewPresenceReaperJob(db.NewAgentPresenceRepository(b.pool), db.NewRoomRepository(b.pool), b.registry, b.hubMgr)
	require.GreaterOrEqual(t, reaper.RunOnce(context.Background()).ExpiredAgents, 1, "instance B's reaper evicts the expired agent")

	require.Equal(t, []presenceFrame{{Event: "presence_join", CardName: "planner-card"}, {Event: "presence_leave"}},
		onA.waitPresence(t, "planner", 2), "the agent's expiry, reaped by B, reaches the stream on A")
	_, held := a.registry.Get(hub.NewRoomID(uuid.MustParse(room.roomID)), "planner")
	require.False(t, held, "instance A drops its in-memory entry for an agent reaped by B")
}
