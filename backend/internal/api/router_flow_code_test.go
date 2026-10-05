package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// A website visit tied to the room it produced, through the real router (SPEC.md 25.6 and
// 25.7). The sentence carries the visit's flow code on its skill link; the browser reports
// its steps with it; the web server reports the fetch of the link; the agent hands the code
// back when it creates the room. A code is kept only when it is well formed and an earlier
// step already carries it, and a room is never refused because of it.

// visitConnect reads GET /v1/connect the way a visitor's browser does and returns the flow
// code of the visit and its sentence. The flow's funnel rows are removed after the test.
func visitConnect(t *testing.T, pool *db.Pool, base, query string) (string, sentence) {
	t.Helper()
	start := httpOnlyGet(t, base+"/v1/connect"+query)
	selected, _ := start["selected"].(map[string]any)
	code, _ := selected["flow_id"].(string)
	require.True(t, models.ValidFlowCode(code), "selected.flow_id is a flow code: %q", code)
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE flow_id = $1`, code) //nolint:errcheck
	})
	return code, decodeSentence(t, start["prompt"])
}

// reportFunnel posts one step to the public ingest, with the headers a caller chooses.
func reportFunnel(t *testing.T, base, body string) int {
	t.Helper()
	status, out := doJSON(t, http.MethodPost, base+"/v1/analytics/funnel", "", body)
	require.Equalf(t, http.StatusAccepted, status, "%s -> %v", body, out)
	return status
}

// createRoomAs creates a room with a raw JSON body fragment for flow_id (so a test can
// send a string, a number or nothing) and returns the room's id.
func createRoomAs(t *testing.T, ts *httptest.Server, pool *db.Pool, agentKey, flowField string) string {
	t.Helper()
	n := time.Now().UnixNano() % 1000000000
	body := fmt.Sprintf(`{"display_name":"test attribution %d","slug":"test-attribution-%d"%s}`, n, n, flowField)
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/rooms", agentKey, body)
	require.Equalf(t, http.StatusCreated, status, "a room is never refused because of flow_id: %s -> %v", body, out)
	data, _ := out["data"].(map[string]any)
	id, _ := data["id"].(string)
	require.NotEmpty(t, id, "%v", out)
	require.Equal(t, fmt.Sprintf("test attribution %d", n), data["display_name"])
	require.Equal(t, fmt.Sprintf("test-attribution-%d", n), data["slug"])
	require.NotContains(t, data, "flow_id", "the room itself carries nothing of the flow")
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE room_id = $1::uuid`, id) //nolint:errcheck
	})
	return id
}

// roomCreatedStep reads the room's room_created step: how many there are and its flow id.
func roomCreatedStep(t *testing.T, pool *db.Pool, roomID string) (rows int, flow string) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT COUNT(*), COALESCE(MAX(flow_id), '')
		  FROM funnel_events WHERE room_id = $1::uuid AND event_name = 'room_created'`, roomID).Scan(&rows, &flow))
	return rows, flow
}

func TestFlowCode_AKnownCodeTiesTheRoomToTheVisit(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	n := time.Now().UnixNano() % 100000000

	code, _ := visitConnect(t, pool, ts.URL, "")
	reportFunnel(t, ts.URL, `{"event":"connection_started","flow_id":"`+code+`","entry_surface":"connect_page"}`)

	_, key := registerTestAgent(t, ts, fmt.Sprintf("roomtest_fk%d", n))
	roomID := createRoomAs(t, ts, pool, key, `,"flow_id":"`+code+`"`)

	rows, flow := roomCreatedStep(t, pool, roomID)
	require.Equal(t, 1, rows)
	require.Equal(t, code, flow, "the room is tied to the visit that copied the sentence")
}

// The web server's skill_fetched alone makes a code known: an agent that read
// GET /v1/connect itself, fetched the skill link and created its room is one flow too.
func TestFlowCode_TheSkillFetchAloneMakesACodeKnown(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	n := time.Now().UnixNano() % 100000000

	code, _ := visitConnect(t, pool, ts.URL, "")
	reportFunnel(t, ts.URL, `{"event":"skill_fetched","flow_id":"`+code+`","request_mode":""}`)

	_, key := registerTestAgent(t, ts, fmt.Sprintf("roomtest_fs%d", n))
	roomID := createRoomAs(t, ts, pool, key, `,"flow_id":"`+code+`"`)
	_, flow := roomCreatedStep(t, pool, roomID)
	require.Equal(t, code, flow)
}

// A well-formed code no earlier step carries is dropped: the room is created all the same
// and its room_created step has no flow id. A room cannot vouch for the code it brought,
// so sending the same unknown code again changes nothing.
func TestFlowCode_AnUnknownCodeIsDroppedAndTheRoomIsCreatedAllTheSame(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	n := time.Now().UnixNano() % 100000000

	// Minted by the API, never reported by any step: well formed and unknown.
	code, _ := visitConnect(t, pool, ts.URL, "")
	_, key := registerTestAgent(t, ts, fmt.Sprintf("roomtest_fu%d", n))

	for attempt := 1; attempt <= 2; attempt++ {
		roomID := createRoomAs(t, ts, pool, key, `,"flow_id":"`+code+`"`)
		rows, flow := roomCreatedStep(t, pool, roomID)
		require.Equal(t, 1, rows, "attempt %d: room_created is still recorded", attempt)
		require.Equal(t, "", flow, "attempt %d: an unknown code attributes nothing", attempt)
	}
	var carried int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM funnel_events WHERE flow_id = $1`, code).Scan(&carried))
	require.Equal(t, 0, carried, "the unknown code is stored nowhere")
}

// Anything that is not exactly a flow code is dropped the same way, whatever JSON it is.
// A value longer than the column allows used to lose the whole room_created row; now the
// step is recorded without it.
func TestFlowCode_AMalformedCodeIsDroppedAndTheRoomIsCreatedAllTheSame(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	n := time.Now().UnixNano() % 100000000
	_, key := registerTestAgent(t, ts, fmt.Sprintf("roomtest_fm%d", n))

	// A known code, to show that only an exact match of it is ever kept.
	known, _ := visitConnect(t, pool, ts.URL, "")
	reportFunnel(t, ts.URL, `{"event":"connection_started","flow_id":"`+known+`"}`)

	for _, field := range []string{
		`,"flow_id":"f_0123456789abcdef01234567"`,
		`,"flow_id":"` + strings.ToUpper(known) + `"`,
		`,"flow_id":" ` + known + `"`,
		`,"flow_id":"` + known + `\n"`,
		`,"flow_id":"` + known + known + `"`,
		`,"flow_id":"` + strings.Repeat("a", 300) + `"`,
		`,"flow_id":""`,
		`,"flow_id":null`,
		`,"flow_id":12345678`,
		`,"flow_id":true`,
		`,"flow_id":["` + known + `"]`,
		`,"flow_id":{"code":"` + known + `"}`,
		``,
	} {
		roomID := createRoomAs(t, ts, pool, key, field)
		rows, flow := roomCreatedStep(t, pool, roomID)
		require.Equal(t, 1, rows, "%s: room_created is recorded", field)
		require.Equal(t, "", flow, "%s: nothing is attributed", field)
	}
}

// A code made only of digits may arrive as a bare JSON number. The body is not refused
// for it, and the number is read as the code it spells.
func TestFlowCode_ACodeSentAsABareNumberIsStillRead(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	n := time.Now().UnixNano() % 100000000
	const digits = "23456789" // a well-formed code that happens to be a number
	require.True(t, models.ValidFlowCode(digits))
	forget := func() {
		pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE flow_id = $1`, digits) //nolint:errcheck
	}
	forget()
	t.Cleanup(forget)

	_, key := registerTestAgent(t, ts, fmt.Sprintf("roomtest_fn%d", n))
	_, flow := roomCreatedStep(t, pool, createRoomAs(t, ts, pool, key, `,"flow_id":`+digits))
	require.Equal(t, "", flow, "unknown: dropped like any other")

	reportFunnel(t, ts.URL, `{"event":"skill_fetched","flow_id":"`+digits+`"}`)
	_, flow = roomCreatedStep(t, pool, createRoomAs(t, ts, pool, key, `,"flow_id":`+digits))
	require.Equal(t, digits, flow, "known: kept, although it came as a number")
}

// The whole chain, each party doing only its own part: the visitor's browser, the site's
// web server, and an agent that has nothing but the sentence and the skill.
func TestFlowCode_TheWholeChainFromTheSentenceToTheRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	// The visitor opens /connect, types an intent, flips the visibility back: three reads,
	// one flow, because the browser sends the first answer's code back as ?flow=.
	code, _ := visitConnect(t, pool, ts.URL, "")
	reportFunnel(t, ts.URL, `{"event":"connection_started","flow_id":"`+code+`","entry_surface":"connect_page","preset":"plan-and-build","instruction_version":"2.1"}`)
	again, _ := visitConnect(t, pool, ts.URL, "?flow="+code+"&visibility=private")
	require.Equal(t, code, again)
	final, copied := visitConnect(t, pool, ts.URL, fmt.Sprintf("?flow=%s&intent=ship+the+signup+page+%d", code, n))
	require.Equal(t, code, final)
	require.Equal(t, code, copied.flowCode(), "the sentence the visitor copies carries the flow code on its skill link")
	reportFunnel(t, ts.URL, `{"event":"starter_prompt_copied","flow_id":"`+code+`","entry_surface":"connect_page","preset":"plan-and-build","role":"planner","instruction_version":"2.1"}`)

	// The agent fetches the link it was given; the web server reports it (no Sec-Fetch-Mode:
	// an HTTP client, not a browser navigation).
	reportFunnel(t, ts.URL, `{"event":"skill_fetched","flow_id":"`+code+`","request_mode":""}`)

	// The agent follows the skill literally: the link carried ?f=<code>, so the create body
	// gains flow_id.
	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_fc%d", n), fmt.Sprintf("test flow chain %d", n))
	planner.startRoom(t, ts.URL, copied)
	require.Equal(t, code, planner.flow)
	require.Equal(t, startCalls(planner), planner.calls)
	slug := planner.vars["ROOM_SLUG"]
	var roomID string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT id::text FROM rooms WHERE slug = $1`, slug).Scan(&roomID))
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM funnel_events WHERE room_id = $1::uuid`, roomID) //nolint:errcheck
	})

	// One flow, joined end to end: both browser steps, the fetch, the room and its first join.
	events, err := db.NewFunnelEventRepository(pool).ListByFlow(context.Background(), code)
	require.NoError(t, err)
	type step struct{ event, channel, surface, room string }
	var got []step
	for _, e := range events {
		got = append(got, step{e.EventName, e.SourceChannel, e.EntrySurface, e.RoomID})
	}
	require.Equal(t, []step{
		{"connection_started", "browser", "connect_page", ""},
		{"starter_prompt_copied", "browser", "connect_page", ""},
		{"skill_fetched", "web_server", "agent_fetch", ""},
		{"room_created", "server", "", roomID},
		{"participant_joined", "server", "", roomID},
	}, got)
}

// A person who opens the skill link in a browser is recorded apart from an agent's fetch.
func TestFlowCode_APersonOpeningTheSkillLinkIsABrowserVisit(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	code, _ := visitConnect(t, pool, ts.URL, "")
	reportFunnel(t, ts.URL, `{"event":"skill_fetched","flow_id":"`+code+`","request_mode":"navigate","entry_surface":"agent_fetch"}`)
	events, err := db.NewFunnelEventRepository(pool).ListByFlow(context.Background(), code)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "browser_visit", events[0].EntrySurface, "the API's reading of request_mode, not the client's claim")
	require.Equal(t, "web_server", events[0].SourceChannel)

	// A malformed code is the usual validation answer, and nothing is stored.
	status, out := doJSON(t, http.MethodPost, ts.URL+"/v1/analytics/funnel", "", `{"event":"skill_fetched","flow_id":"not-a-code"}`)
	require.Equal(t, http.StatusBadRequest, status, "%v", out)
	failure, _ := out["error"].(map[string]any)
	require.Equal(t, "VALIDATION_ERROR", failure["code"])
}
