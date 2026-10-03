package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Task idx 84: the three published guides are evidence-backed. Each test is the guide's
// workflow run over plain HTTP by agents that have nothing but net/http and the prompt
// text the real router serves (router_connect_http_only_test.go's harness), following
// it literally. A guide states only what these tests run; named CLI clients are not
// claimed until they are tested live.

// guideStart reads GET /v1/connect for a preset and returns its first prompt's text.
func guideStart(t *testing.T, base, preset string) string {
	t.Helper()
	start := httpOnlyGet(t, base+"/v1/connect?preset="+preset+"&visibility=public")
	selected, _ := start["selected"].(map[string]any)
	require.Equal(t, preset, selected["preset"])
	prompt, _ := start["prompt"].(map[string]any)
	text, _ := prompt["text"].(string)
	require.NotEmpty(t, text)
	return text
}

// roomMessages reads a room's message bodies in timeline order.
func roomMessages(t *testing.T, base, slug string) []string {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms/"+slug+"/entries?kind=message&limit=100", "", "")
	require.Equal(t, http.StatusOK, status, "entries: %v", out)
	rows, _ := out["data"].([]any)
	bodies := []string{}
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		body, _ := row["body"].(string)
		bodies = append(bodies, body)
	}
	return bodies
}

// Guide: connect a planner and an executor (preset plan-and-build).
func TestGuide_PlannerAndExecutor(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gp%d", n), fmt.Sprintf("test guide planner %d", n))
	planner.follow(t, ts.URL, guideStart(t, ts.URL, "plan-and-build"))
	slug := planner.vars["ROOM_SLUG"]
	require.Equal(t, []string{
		"POST /v1/agents/register", "POST /v1/rooms", "POST /v1/rooms/" + slug + "/handshake",
		"POST /r/" + slug + "/join", "POST /v1/rooms/" + slug + "/entries",
		"POST /v1/rooms/" + slug + "/entries/" + planner.vars["ENTRY_ID"] + "/pin", "GET /v1/rooms/" + slug + "/entries",
	}, planner.calls, "the planner's steps (the plan is pinned as the directive)")

	executor := newHTTPOnlyAgent(fmt.Sprintf("roomtest_ge%d", n), "")
	executor.follow(t, ts.URL, httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug+"/connect")["prompt"].(string))
	require.Contains(t, executor.calls, "POST /v1/rooms/"+slug+"/handshake")
	require.Contains(t, executor.calls, "GET /v1/rooms/"+slug+"/entries", "the executor reads the planner's message")
	require.Contains(t, executor.calls, "POST /v1/rooms/"+slug+"/entries", "the executor replies")
	requireAuthoredEntries(t, ts.URL, slug, planner, executor)
}

// Guide: connect a builder and a reviewer (preset build-and-review, reviewer role).
func TestGuide_BuilderAndReviewer(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	builder := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gb%d", n), fmt.Sprintf("test guide builder %d", n))
	builder.follow(t, ts.URL, guideStart(t, ts.URL, "build-and-review"))
	slug := builder.vars["ROOM_SLUG"]
	require.NotEmpty(t, slug)

	reviewer := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gr%d", n), "")
	reviewer.follow(t, ts.URL, httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug+"/connect?role=reviewer")["prompt"].(string))
	require.Contains(t, reviewer.calls, "GET /v1/rooms/"+slug+"/entries", "the reviewer reads the builder's work")
	require.Contains(t, reviewer.calls, "POST /v1/rooms/"+slug+"/entries", "the reviewer posts its review")
	requireAuthoredEntries(t, ts.URL, slug, builder, reviewer)
}

// Guide: resume collaboration across two CLIs. The executor's CLI exits; while it is
// away the planner keeps posting; a second CLI starts over from the room's prompt, as
// the prompt's RESUMING section says, reads the room, finds the messages it missed and
// continues after them.
func TestGuide_ResumeAcrossTwoCLIs(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gq%d", n), fmt.Sprintf("test guide resume %d", n))
	planner.follow(t, ts.URL, guideStart(t, ts.URL, "plan-and-build"))
	slug := planner.vars["ROOM_SLUG"]
	roomPrompt := httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug+"/connect")["prompt"].(string)
	require.Contains(t, roomPrompt, "RESUMING", "the prompt itself teaches the resume")
	require.Contains(t, roomPrompt, "start over from this prompt and read the room to catch up")

	first := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gx%d", n), "")
	first.follow(t, ts.URL, roomPrompt)

	// The first CLI exits. The planner posts while it is away.
	missed := fmt.Sprintf("step two while you were away %d", n)
	status, _, err := postEntryRaw(ts.URL, slug, planner.vars["YOUR_ROOM_TOKEN"],
		map[string]any{"body": missed, "client_entry_id": fmt.Sprintf("away-%d", n)})
	require.NoError(t, err)
	require.Less(t, status, 300)

	// A second CLI starts over from the same prompt.
	second := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gy%d", n), "")
	second.message = fmt.Sprintf("resumed after step two %d", n)
	second.follow(t, ts.URL, roomPrompt)
	require.Contains(t, second.calls, "GET /v1/rooms/"+slug+"/entries", "the second CLI reads the room to catch up")

	bodies := roomMessages(t, ts.URL, slug)
	require.Equal(t, []string{planner.message, first.message, missed, second.message}, bodies,
		"the room carries the missed message before the resumed reply, nothing repeated")
}
