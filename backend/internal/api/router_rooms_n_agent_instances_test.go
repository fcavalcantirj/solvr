package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// N-agent collaboration spread over two API instances (steps 4 and 5 of "Verify N-agent
// collaboration end to end across clients and service instances"): concurrent writers on
// both instances, one participant disconnecting and rejoining through the other instance,
// retried writes, and private-room admission and individual revocation.

// concurrentRound has every listed participant write perAgent messages at the same time,
// each through its own instance, each write carrying a client_entry_id. Writes of one
// participant are sequential; participants run in parallel. It returns the writes per
// participant in submission order.
func (r *nAgentRoom) concurrentRound(t *testing.T, insts []*roomInstance, who []int, round string, perAgent int) map[int][]int64 {
	t.Helper()
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		errs   []string
		out    = map[int][]int64{}
		starts = make(chan struct{})
	)
	for _, i := range who {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-starts
			a := r.agents[i]
			for k := 0; k < perAgent; k++ {
				status, id, err := postEntryRaw(insts[i].ts.URL, r.slug, a.tok, map[string]any{
					"body":            fmt.Sprintf("%s %s #%d", a.name, round, k),
					"client_entry_id": fmt.Sprintf("%s-%s-%d", a.name, round, k),
				})
				mu.Lock()
				if err != nil || status != http.StatusCreated {
					errs = append(errs, fmt.Sprintf("%s %s #%d: status %d err %v", a.name, round, k, status, err))
				} else {
					out[i] = append(out[i], id)
				}
				mu.Unlock()
			}
		}(i)
	}
	close(starts)
	wg.Wait()
	require.Empty(t, errs, "every concurrent write is accepted")
	return out
}

// --- Step 4: two instances, concurrent writers, disconnect + rejoin, idempotent retry. ---

func TestNAgentRoom_TwoInstancesConcurrentWritesRejoinReplayAndIdempotentRetry(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	pool := nAgentDB(t)
	roomPreCleanup(t, pool)
	room := newNAgentRoom(t, a, pool, 4, false)
	insts := []*roomInstance{a, a, b, b} // planner + executor-1 on A, executor-2 + reviewer on B
	streams := make([]*liveStream, 4)
	for i, ag := range room.agents {
		streams[i] = openLiveStream(t, insts[i].ts.URL+"/v1/rooms/"+room.slug+"/stream", ag.tok)
	}

	round1 := room.concurrentRound(t, insts, []int{0, 1, 2, 3}, "r1", 3)
	committed := entriesAfter(t, a.ts.URL, room.slug, room.agents[0].key, 0, "")
	require.Equal(t, committed, streams[3].waitIDs(t, len(committed)), "reviewer on B saw round 1")
	lastSeen := committed[len(committed)-1]

	// The reviewer disconnects; the other three keep writing on both instances.
	streams[3].stop()
	round2 := room.concurrentRound(t, insts, []int{0, 1, 2}, "r2", 2)

	// The reviewer's retry of a write it already made (same client_entry_id, same body),
	// sent through the OTHER instance, returns the original entry and stores nothing new.
	status, id, err := postEntryRaw(a.ts.URL, room.slug, room.agents[3].tok, map[string]any{
		"body": "reviewer r1 #2", "client_entry_id": "reviewer-r1-2"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "an identical retry replays")
	require.Equal(t, round1[3][2], id, "the retry returns the original entry")

	// The reviewer rejoins through instance A and resumes from its last seen entry.
	insts[3] = a
	room.join(t, a, room.agents[3])
	resumed := openLiveStream(t, a.ts.URL+"/v1/rooms/"+room.slug+"/stream?lastEventId="+strconv.FormatInt(lastSeen, 10), room.agents[3].tok)
	round3 := room.concurrentRound(t, insts, []int{0, 1, 2, 3}, "r3", 1)

	committed = entriesAfter(t, a.ts.URL, room.slug, room.agents[0].key, 0, "")
	for i := 0; i < 3; i++ {
		require.Equal(t, committed, streams[i].waitIDs(t, len(committed)), "%s is unaffected by the reviewer's reconnect", room.agents[i].name)
	}
	var missedAndLive []int64
	for _, id := range committed {
		if id > lastSeen {
			missedAndLive = append(missedAndLive, id)
		}
	}
	require.Equal(t, missedAndLive, resumed.waitIDs(t, len(missedAndLive)), "replay of what it missed, then live, no hole and no duplicate")

	// Exactly the accepted writes are stored, each participant's in its own order.
	msgs := room.messages(t, b.ts.URL, room.agents[1].tok)
	require.Len(t, msgs, 4*3+3*2+4*1, "no write lost, the retry stored nothing")
	byAuthor := map[string][]int64{}
	for _, m := range msgs {
		byAuthor[m["author_id"].(string)] = append(byAuthor[m["author_id"].(string)], int64(m["id"].(float64)))
	}
	for i, ag := range room.agents {
		want := append(append(append([]int64{}, round1[i]...), round2[i]...), round3[i]...)
		require.Equal(t, want, byAuthor[ag.id], "%s's writes are stored once, in its submission order", ag.name)
	}

	// Rejoining is not a new participant; the room activated once.
	require.Equal(t, 4, room.funnelSteps(t, "participant_joined"))
	require.Equal(t, 1, room.funnelSteps(t, "first_two_way_exchange"))
	require.Equal(t, 1, room.activations(t))
}

// --- Step 4/6: first messages racing on two instances activate the room exactly once. ---

func TestNAgentRoom_ConcurrentFirstMessagesOnTwoInstancesActivateOnce(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	pool := nAgentDB(t)
	roomPreCleanup(t, pool)
	for attempt := 0; attempt < 5; attempt++ {
		room := newNAgentRoom(t, a, pool, 8, false)
		insts := []*roomInstance{a, b, a, b, a, b, a, b}
		room.concurrentRound(t, insts, []int{0, 1, 2, 3, 4, 5, 6, 7}, "first", 1)
		require.Equal(t, 1, room.activations(t), "attempt %d: one room.activated entry", attempt)
		require.Equal(t, 1, room.funnelSteps(t, "first_two_way_exchange"), "attempt %d: one funnel milestone", attempt)
	}
}

// --- Step 5: private-room admission and individual revocation across instances. ---

func TestNAgentRoom_PrivateAdmissionAndIndividualRevocationAcrossInstances(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	pool := nAgentDB(t)
	roomPreCleanup(t, pool)
	room := newNAgentRoom(t, a, pool, 4, true)
	planner, exec1, exec2, reviewer := room.agents[0], room.agents[1], room.agents[2], room.agents[3]
	_, outsiderKey := registerRoomTestAgent(t, a.ts)
	_, strangerJWT := createRoomTestUser(t, pool)

	blocked := func(inst *roomInstance, what, path, bearer string) {
		t.Helper()
		st := statusWithin(t, inst.ts.URL+path, bearer)
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, st, "%s must be refused, got %d", what, st)
	}
	// A browser reaches a private stream only through a ticket; none is minted for a caller
	// who has no access.
	noTicket := func(inst *roomInstance, who, bearer string) {
		t.Helper()
		st, _ := doJSON(t, http.MethodPost, inst.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", bearer, "")
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, st, "%s must get no stream ticket, got %d", who, st)
	}
	readsBlocked := func(inst *roomInstance, who, bearer string) {
		t.Helper()
		base := "/v1/rooms/" + room.slug
		blocked(inst, who+" room detail", base, bearer)
		blocked(inst, who+" entries", base+"/entries", bearer)
		blocked(inst, who+" messages", base+"/messages", bearer)
		blocked(inst, who+" agents", base+"/agents", bearer)
		blocked(inst, who+" stream", base+"/stream", bearer)
	}

	for _, inst := range []*roomInstance{a, b} {
		st, _ := handshake(t, inst.ts.URL, room.slug, outsiderKey, "")
		require.Equal(t, http.StatusForbidden, st, "an agent the owner never admitted cannot handshake")
		readsBlocked(inst, "anonymous", "")
		readsBlocked(inst, "unadmitted agent", outsiderKey)
		readsBlocked(inst, "unrelated human", strangerJWT)
		noTicket(inst, "unrelated human browser stream", strangerJWT)
	}

	// Admitted members collaborate across both instances with their own tokens.
	reviewerOnB := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", reviewer.tok)
	plan := room.post(t, a.ts.URL, planner, "private plan", 0)
	room.post(t, b.ts.URL, exec1, "exec-1 done", plan)
	room.post(t, b.ts.URL, exec2, "exec-2 done", plan)

	// The owner revokes executor-2 alone, through instance A.
	st, out := doJSON(t, http.MethodDelete, a.ts.URL+"/v1/rooms/"+room.slug+"/members/"+exec2.id, room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, st, "revoke: %v", out)

	for _, inst := range []*roomInstance{a, b} {
		readsBlocked(inst, "revoked room token", exec2.tok)
		readsBlocked(inst, "revoked agent key", exec2.key)
		noTicket(inst, "revoked browser-style stream", exec2.tok)
		status, _, err := postEntryRaw(inst.ts.URL, room.slug, exec2.tok, map[string]any{"body": "still here?"})
		require.NoError(t, err)
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, status, "a revoked token cannot write")
		hs, _ := handshake(t, inst.ts.URL, room.slug, exec2.key, "")
		require.Equal(t, http.StatusForbidden, hs, "a revoked agent cannot handshake back in")
	}

	// Everyone else continues on both instances, unaffected.
	r1 := room.post(t, b.ts.URL, reviewer, "review: exec-1 approved", plan)
	room.post(t, a.ts.URL, planner, "exec-1's part stands; exec-2 was removed", r1)
	committed := entriesAfter(t, a.ts.URL, room.slug, planner.tok, 0, "")
	require.Equal(t, committed, reviewerOnB.waitIDs(t, len(committed)), "the reviewer's stream on B keeps receiving")
	require.Len(t, room.messages(t, b.ts.URL, exec1.tok), 5)

	// A private room never shows up in public discovery.
	status, list := doJSON(t, http.MethodGet, a.ts.URL+"/v1/rooms?q="+room.slug, "", "")
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, list["data"], "a private room is not publicly listed")
}

// statusWithin is getStatus bounded by a deadline, so a stream that is wrongly opened
// (200 and held open) fails the check instead of hanging the run.
func statusWithin(t *testing.T, url, bearer string) int {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "GET %s", url)
	resp.Body.Close()
	return resp.StatusCode
}
