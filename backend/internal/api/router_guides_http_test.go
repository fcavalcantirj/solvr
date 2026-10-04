package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The published guides are evidence-backed (task idx 84, v1.3.5). Each guide shows one
// example sentence from GET /v1/connect/examples; each test is that guide run over plain
// HTTP by fresh agents that have nothing but net/http, the sentence, and the skill's
// recipes (router_connect_http_only_test.go's harness), following them literally. A
// guide states only what these tests run; named CLI clients are not claimed until they
// are tested live.

// guideSentence is a guide's example sentence, as GET /v1/connect/examples serves it.
func guideSentence(t *testing.T, base, preset string) sentence {
	t.Helper()
	examples := httpOnlyGet(t, base+"/v1/connect/examples")
	presets, _ := examples["presets"].([]any)
	for _, raw := range presets {
		p, _ := raw.(map[string]any)
		if p["value"] == preset {
			return decodeSentence(t, p["prompt"])
		}
	}
	t.Fatalf("no example sentence for %s", preset)
	return sentence{}
}

// roomMessages reads a room's message bodies in timeline order.
func roomMessages(t *testing.T, base, slug, credential string) []string {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms/"+slug+"/entries?kind=message&limit=100", credential, "")
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

// Guide: connect a planner and an executor (Plan & execute).
func TestGuide_PlannerAndExecutor(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	s := guideSentence(t, ts.URL, "plan-and-build")
	require.Equal(t, "ship the signup page", s.segment("intent"))
	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gp%d", n), fmt.Sprintf("test guide planner %d", n))
	planner.startRoom(t, ts.URL, s)
	slug := planner.vars["ROOM_SLUG"]
	require.Equal(t, startCalls(planner), planner.calls, "the planner's steps (the plan is pinned as the directive)")

	executor := newHTTPOnlyAgent(fmt.Sprintf("roomtest_ge%d", n), "")
	executor.joinRoom(t, ts.URL, roomSentence(t, ts.URL, slug, "executor", ""))
	require.Contains(t, executor.seen, planner.message, "the executor reads the planner's message")
	planner.read(t, ts.URL)
	require.Contains(t, planner.seen, executor.message, "the planner reads the executor's reply")
	requireAuthoredEntries(t, ts.URL, slug, "", planner, executor)
}

// Guide: connect a builder and a reviewer (Build & review).
func TestGuide_BuilderAndReviewer(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	s := guideSentence(t, ts.URL, "build-and-review")
	require.Equal(t, "add API rate limiting", s.segment("intent"))
	builder := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gb%d", n), fmt.Sprintf("test guide builder %d", n))
	builder.startRoom(t, ts.URL, s)
	slug := builder.vars["ROOM_SLUG"]
	require.NotEmpty(t, slug)

	reviewer := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gr%d", n), "")
	reviewer.joinRoom(t, ts.URL, roomSentence(t, ts.URL, slug, "reviewer", ""))
	require.Contains(t, reviewer.seen, builder.message, "the reviewer reads the builder's work")
	builder.read(t, ts.URL)
	require.Contains(t, builder.seen, reviewer.message, "the builder reads the review")
	requireAuthoredEntries(t, ts.URL, slug, "", builder, reviewer)
}

// Guide: share context between two agents (Share context, a private room). The learner
// follows the example sentence; the expert gives its id to the human, who relays it; the
// learner admits it; the expert reads the question and answers.
func TestGuide_ShareContext(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	s := guideSentence(t, ts.URL, "collaborate")
	require.Equal(t, "learn our billing code", s.segment("intent"))
	require.Equal(t, "private", s.segment("visibility"))
	learner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gs%d", n), fmt.Sprintf("test guide share %d", n))
	learner.startRoom(t, ts.URL, s)
	slug := learner.vars["ROOM_SLUG"]
	require.Equal(t, startCalls(learner), learner.calls, "the learner's steps (the question is pinned as the directive)")

	expertSentence := roomSentence(t, ts.URL, slug, "expert", learner.vars["YOUR_AGENT_API_KEY"])
	expert := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gt%d", n), "")
	expert.register(t, ts.URL)
	learner.admit(t, ts.URL, expert.id)
	expert.joinRoom(t, ts.URL, expertSentence)
	require.Contains(t, expert.seen, learner.message, "the expert reads the question")
	learner.read(t, ts.URL)
	require.Contains(t, learner.seen, expert.message, "the learner reads the answer")
	requireAuthoredEntries(t, ts.URL, slug, learner.vars["YOUR_ROOM_TOKEN"], learner, expert)
}

// Guide: resume collaboration across two CLIs. The executor's CLI exits; while it is away
// the planner keeps posting; a second CLI starts over from the room's sentence, as the
// skill's RESUMING step says, reads the room, finds the messages it missed and continues
// after them.
func TestGuide_ResumeAcrossTwoCLIs(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	resuming := skillRooms(t)[strings.Index(skillRooms(t), "RESUMING"):]
	require.Contains(t, resuming, "start over from your prompt and read the room to catch up", "the skill teaches the resume")

	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gq%d", n), fmt.Sprintf("test guide resume %d", n))
	planner.startRoom(t, ts.URL, guideSentence(t, ts.URL, "plan-and-build"))
	slug := planner.vars["ROOM_SLUG"]
	roomPrompt := roomSentence(t, ts.URL, slug, "executor", "")

	first := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gx%d", n), "")
	first.joinRoom(t, ts.URL, roomPrompt)

	// The first CLI exits. The planner posts while it is away.
	missed := fmt.Sprintf("step two while you were away %d", n)
	status, _, err := postEntryRaw(ts.URL, slug, planner.vars["YOUR_ROOM_TOKEN"],
		map[string]any{"body": missed, "client_entry_id": fmt.Sprintf("away-%d", n)})
	require.NoError(t, err)
	require.Less(t, status, 300)

	// A second CLI starts over from the same sentence.
	second := newHTTPOnlyAgent(fmt.Sprintf("roomtest_gy%d", n), "")
	second.message = fmt.Sprintf("resumed after step two %d", n)
	second.joinRoom(t, ts.URL, roomPrompt)
	require.Contains(t, second.seen, missed, "the second CLI reads the room to catch up")

	bodies := roomMessages(t, ts.URL, slug, "")
	require.Equal(t, []string{planner.message, first.message, missed, second.message}, bodies,
		"the room carries the missed message before the resumed reply, nothing repeated")
}
