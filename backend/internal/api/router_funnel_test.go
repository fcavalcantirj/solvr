package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestFunnelServerSteps_RecordedEndToEnd proves the server side of the connection
// funnel: the flow id the create-room call carries is persisted and stitches this
// room's server steps together (task step 2), and room_created, participant_joined
// (with per-actor ordinals) and first_two_way_exchange are all recorded from
// confirmed server actions (task step 3) — no browser event required.
func TestFunnelServerSteps_RecordedEndToEnd(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	suffix := time.Now().UnixNano() % 1000000000
	flow := fmt.Sprintf("f_e2e_%d", suffix)

	// --- PLANNER: register, create room WITH the flow id, handshake, join, post ---
	plannerName := fmt.Sprintf("fnplanner%d", suffix)
	_, plannerKey := registerTestAgent(t, ts, plannerName)

	createBody := fmt.Sprintf(`{"display_name":"test-funnel %d","slug":"test-funnel-%d","flow_id":"%s"}`, suffix, suffix, flow)
	createReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/v1/rooms", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+plannerKey)
	createResp, err := http.DefaultClient.Do(createReq)
	require.NoError(t, err)
	defer createResp.Body.Close()
	createRaw, _ := io.ReadAll(createResp.Body)
	require.Equal(t, http.StatusCreated, createResp.StatusCode, string(createRaw))

	sharedToken, slug := extractRoomTokenAndSlug(t, string(createRaw))

	_, plannerRoomToken := handshake(t, ts.URL, slug, plannerKey, sharedToken)
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, plannerName))
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", plannerRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"Task and first directive."}`, plannerName))

	// --- EXECUTOR: its own identity, handshake, join, post ---
	execName := fmt.Sprintf("fnexec%d", suffix)
	_, execKey := registerTestAgent(t, ts, execName)
	_, execRoomToken := handshake(t, ts.URL, slug, execKey, "")
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/join", execRoomToken,
		fmt.Sprintf(`{"agent_name":"%s"}`, execName))
	doJSON(t, http.MethodPost, ts.URL+"/r/"+slug+"/message", execRoomToken,
		fmt.Sprintf(`{"agent_name":"%s","content":"My plan for the task."}`, execName))

	// --- Inspect the recorded funnel for this room ---
	ctx := context.Background()
	var roomID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug=$1`, slug).Scan(&roomID))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE room_id=$1`, roomID) })

	events, err := db.NewFunnelEventRepository(pool).ListByRoom(ctx, roomID)
	require.NoError(t, err)

	var created, twoWay int
	var joinOrdinals []int
	for _, e := range events {
		require.Equal(t, models.FunnelSourceServer, e.SourceChannel, "server hooks record server-channel steps")
		switch e.EventName {
		case models.FunnelRoomCreated:
			created++
			require.Equal(t, flow, e.FlowID, "room_created must carry the flow id the create-room call brought")
		case models.FunnelParticipantJoined:
			joinOrdinals = append(joinOrdinals, e.Ordinal)
			require.Equal(t, flow, e.FlowID, "a join inherits the room's flow id")
		case models.FunnelFirstTwoWayExchange:
			twoWay++
			require.Equal(t, flow, e.FlowID, "the activation milestone inherits the room's flow id")
		}
	}

	require.Equal(t, 1, created, "exactly one room_created")
	require.ElementsMatch(t, []int{1, 2}, joinOrdinals, "two distinct agents joined with ordinals 1 and 2")
	require.Equal(t, 1, twoWay, "first_two_way_exchange recorded once after two distinct agents posted")
}

// TestFunnelRoutes_MountedAndPublic proves the funnel API is wired into the real
// router and reachable without authentication: the contract is public
// documentation, and a logged-out browser can report a browser step.
func TestFunnelRoutes_MountedAndPublic(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	// The contract endpoint is public documentation.
	resp, err := http.Get(ts.URL + "/v1/analytics/funnel/contract")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	require.Contains(t, string(body), models.FunnelConnectionStarted)

	// A logged-out browser reports a browser step; no credential required.
	flow := fmt.Sprintf("f_route_%d", time.Now().UnixNano()%1000000000)
	postBody := fmt.Sprintf(`{"flow_id":"%s","event":"connection_started","entry_surface":"connect_page"}`, flow)
	postResp, err := http.Post(ts.URL+"/v1/analytics/funnel", "application/json", strings.NewReader(postBody))
	require.NoError(t, err)
	pb, _ := io.ReadAll(postResp.Body)
	postResp.Body.Close()
	require.Equal(t, http.StatusAccepted, postResp.StatusCode, string(pb))

	// A server-only step cannot be self-reported by a client.
	badResp, err := http.Post(ts.URL+"/v1/analytics/funnel", "application/json",
		strings.NewReader(`{"event":"room_created"}`))
	require.NoError(t, err)
	badResp.Body.Close()
	require.Equal(t, http.StatusBadRequest, badResp.StatusCode)
}
