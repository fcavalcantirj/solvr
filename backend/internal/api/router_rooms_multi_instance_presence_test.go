package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Online presence across more than one API instance: an agent joins through one instance
// and every other instance must see it — in the presence list, the agent-card lookup, the
// room's online count and the homepage's "agents online now" — renew it through a
// heartbeat, and drop it once its heartbeat expires. Presence lives in the database; no
// instance's memory is the source of truth.

func joinPresence(t *testing.T, inst *roomInstance, slug, roomTok, name string, ttl int) {
	t.Helper()
	status, out := doJSON(t, http.MethodPost, inst.ts.URL+"/r/"+slug+"/join", roomTok, fmt.Sprintf(
		`{"agent_name":%q,"ttl_seconds":%d,"card":{"name":%q,"description":"cross-instance card","skills":[]}}`,
		name, ttl, name+"-card"))
	require.Equal(t, http.StatusOK, status, "join: %v", out)
}

func presentNames(t *testing.T, inst *roomInstance, slug, roomTok string) []string {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, inst.ts.URL+"/r/"+slug+"/agents", roomTok, "")
	require.Equal(t, http.StatusOK, status, "list presence: %v", out)
	names := []string{}
	for _, rec := range out["data"].([]any) {
		names = append(names, rec.(map[string]any)["agent_name"].(string))
	}
	return names
}

func agentCard(t *testing.T, inst *roomInstance, slug, roomTok, name string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, http.MethodGet, inst.ts.URL+"/r/"+slug+"/agents/"+name, roomTok, "")
}

func roomOnlineCount(t *testing.T, inst *roomInstance, slug, bearer string) float64 {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, inst.ts.URL+"/v1/rooms/"+slug, bearer, "")
	require.Equal(t, http.StatusOK, status, "room detail: %v", out)
	return entryData(t, out)["online_count"].(float64)
}

// agentsOnlineNow reads the homepage presence figure. Each call must go to an instance
// whose 30-second overview cache has not been filled yet.
func agentsOnlineNow(t *testing.T, inst *roomInstance) int {
	t.Helper()
	overview, body := getHomepageOverview(t, inst.ts.URL)
	for _, m := range overview.Rooms.PresenceMetrics {
		if m.Key == "agents_online_now" {
			return m.Value
		}
	}
	t.Fatalf("agents_online_now missing from overview: %s", body)
	return 0
}

func TestRoomPresence_AgentJoinedOnOneInstanceIsSeenByEveryInstance(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newAgentPairRoom(t, a, false)
	before := agentsOnlineNow(t, a)

	joinPresence(t, a, room.slug, room.plannerTok, "planner", 600)

	require.Equal(t, []string{"planner"}, presentNames(t, b, room.slug, room.executorTok),
		"instance B lists the agent that joined through A")
	status, out := agentCard(t, b, room.slug, room.executorTok, "planner")
	require.Equal(t, http.StatusOK, status, "instance B returns the card of an agent that joined through A: %v", out)
	require.Equal(t, "planner-card", entryData(t, out)["name"])
	require.Equal(t, float64(1), roomOnlineCount(t, b, room.slug, room.executorKey),
		"instance B's room online count includes the agent on A")
	require.Equal(t, before+1, agentsOnlineNow(t, b),
		"instance B's homepage counts the agent that joined through A")
}

func TestRoomPresence_HeartbeatAndExpiryApplyOnEveryInstance(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	c := startRoomInstance(t, RoomRelayOptions{})
	room := newAgentPairRoom(t, a, false)
	before := agentsOnlineNow(t, a)

	joinPresence(t, a, room.slug, room.plannerTok, "planner", 2)
	// Renewed through the OTHER instance: a heartbeat is not tied to where the agent joined.
	time.Sleep(1200 * time.Millisecond)
	status, out := doJSON(t, http.MethodPost, b.ts.URL+"/r/"+room.slug+"/heartbeat", room.plannerTok, `{"agent_name":"planner"}`)
	require.Equal(t, http.StatusOK, status, "heartbeat: %v", out)
	time.Sleep(1200 * time.Millisecond) // past the original 2s, inside the renewed one
	status, _ = agentCard(t, a, room.slug, room.executorTok, "planner")
	require.Equal(t, http.StatusOK, status, "a heartbeat through B keeps the agent present on A")

	time.Sleep(2500 * time.Millisecond) // renewed TTL elapsed, no reaper has run
	for name, inst := range map[string]*roomInstance{"A": a, "B": b} {
		require.Empty(t, presentNames(t, inst, room.slug, room.executorTok), "instance %s still lists an expired agent", name)
		status, out := agentCard(t, inst, room.slug, room.executorTok, "planner")
		require.Equal(t, http.StatusNotFound, status, "instance %s still serves an expired agent's card: %v", name, out)
		require.Equal(t, float64(0), roomOnlineCount(t, inst, room.slug, room.executorKey), "instance %s online count", name)
	}
	require.Equal(t, before, agentsOnlineNow(t, c), "the homepage stops counting an expired agent")
}
