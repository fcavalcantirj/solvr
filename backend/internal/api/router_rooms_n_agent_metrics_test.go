package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// Step 6 of "Verify N-agent collaboration end to end across clients and service
// instances": a group room counts as ONE activation, its participant and message counts
// are exact, the public surfaces carry no traffic or growth figures, and the public view
// shows permitted observers exactly the public room — never the private one.

func TestNAgentRoom_OneActivationExactCountsNoTrafficLeakAndPermittedPublicView(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	pool := nAgentDB(t)
	roomPreCleanup(t, pool)
	windowStart := time.Now().Add(-time.Second)

	public := newNAgentRoom(t, a, pool, 4, false)
	private := newNAgentRoom(t, a, pool, 3, true)
	plan := public.post(t, a.ts.URL, public.agents[0], "public plan", 0)
	for _, ag := range public.agents {
		public.post(t, a.ts.URL, ag, ag.name+" update one", plan)
	}
	for _, ag := range public.agents[1:] {
		public.post(t, a.ts.URL, ag, ag.name+" update two", plan)
	}
	const publicMessages = 1 + 4 + 3
	privatePlan := private.post(t, a.ts.URL, private.agents[0], "private plan", 0)
	for _, ag := range private.agents[1:] {
		private.post(t, a.ts.URL, ag, ag.name+" private update", privatePlan)
	}

	// One activation per room, however many agents took part.
	for _, r := range []*nAgentRoom{public, private} {
		require.Equal(t, 1, r.activations(t), "%s: one room.activated entry", r.slug)
		require.Equal(t, 1, r.funnelSteps(t, "first_two_way_exchange"), "%s: one funnel milestone", r.slug)
		require.Equal(t, len(r.agents), r.funnelSteps(t, "participant_joined"), "%s: one join per participant", r.slug)
	}

	// Operator activation measurement over this test's window: two rooms, two
	// activations (not a count of pairs), the largest room at four participants.
	rep, err := db.NewActivationAnalyticsRepository(pool).Measure(context.Background(), windowStart, time.Now().Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, 2, rep.RoomsCreated)
	require.Equal(t, 2, rep.ActivatedRooms)
	require.Equal(t, 2, rep.Participants.RoomsWithParticipants)
	require.Equal(t, 4, rep.Participants.MaxParticipants)
	require.Equal(t, 2, rep.Participants.RoomsWithLaterJoins, "both rooms grew past the activating pair")

	// ...and that report is operator-only: no public, agent or ordinary human read.
	t.Setenv("ADMIN_API_KEY", "n-agent-operator-key")
	_, memberJWT := createRoomTestUser(t, pool)
	for who, bearer := range map[string]string{"anonymous": "", "agent": public.agents[0].key, "human": memberJWT} {
		st := statusWithin(t, a.ts.URL+"/admin/activation-analytics", bearer)
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, st, "%s reads the operator report", who)
	}
	req, err := http.NewRequest(http.MethodGet, a.ts.URL+"/admin/activation-analytics", nil)
	require.NoError(t, err)
	req.Header.Set(handlers.OperatorAccessHeader, "n-agent-operator-key")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "the operator still reads it")

	// Public view, anonymous observer: exact counts for the public room.
	status, out := doJSON(t, http.MethodGet, a.ts.URL+"/v1/rooms/"+public.slug, "", "")
	require.Equal(t, http.StatusOK, status)
	detail := entryData(t, out)
	require.Equal(t, float64(publicMessages), detail["room"].(map[string]any)["message_count"])
	require.Len(t, detail["recent_messages"], publicMessages)
	require.Equal(t, float64(4), detail["online_count"])
	require.Equal(t, "conversation_started", detail["connection_status"])
	card := publicRoomCard(t, a.ts.URL, public.slug)
	require.Equal(t, float64(4), card["unique_participant_count"])
	require.Equal(t, float64(publicMessages), card["message_count"])
	require.Len(t, public.messages(t, a.ts.URL, ""), publicMessages, "anonymous observers read the public transcript")

	// The private room: members read it, the public never does.
	require.Len(t, private.messages(t, a.ts.URL, private.agents[1].tok), 3)
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, statusWithin(t, a.ts.URL+"/v1/rooms/"+private.slug, ""))
	_, list := doJSON(t, http.MethodGet, a.ts.URL+"/v1/rooms?q="+private.slug, "", "")
	require.Empty(t, list["data"])

	// Homepage with BOTH rooms selected: only the public one is previewed, with exact
	// counts; the private room's slug, title and text appear nowhere; no field names a
	// traffic or growth figure.
	previews, body := homepagePreviews(t, private.slug+","+public.slug)
	require.Len(t, previews, 1)
	require.Equal(t, public.slug, previews[0]["slug"])
	require.Equal(t, float64(publicMessages), previews[0]["message_count"])
	require.Equal(t, float64(4), previews[0]["participant_count"])
	require.NotContains(t, body, private.slug)
	require.NotContains(t, body, "private plan")
	var overview any
	require.NoError(t, json.Unmarshal([]byte(body), &overview))
	for _, key := range jsonKeys(overview) {
		require.Empty(t, handlers.PrivateAnalyticsTermInKey(key), "public overview field %q names a private analytics figure", key)
	}
	require.False(t, strings.Contains(strings.ToLower(body), "funnel"), "no funnel data on the public overview")
}

// jsonKeys returns every object key in a decoded JSON value, recursively.
func jsonKeys(v any) []string {
	var keys []string
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			keys = append(keys, k)
			keys = append(keys, jsonKeys(child)...)
		}
	case []any:
		for _, child := range x {
			keys = append(keys, jsonKeys(child)...)
		}
	}
	return keys
}
