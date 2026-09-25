package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// Room delivery across restarts and more than one API instance (task: "Preserve room
// delivery and statistics across restarts and more than one API instance"). Each
// "instance" below is a full router with its own connection pool, hub manager and room
// relay — nothing in memory is shared, only the database, exactly as two API processes.
// Entries are committed first; a Postgres notification (sent only at commit) wakes every
// instance, which reads the committed entries after its cursor and fans them out.

type roomInstance struct {
	ts   *httptest.Server
	stop func()
}

// startRoomInstance runs one API instance against DATABASE_URL and waits until its relay
// listens for cross-instance notifications.
func startRoomInstance(t *testing.T, opts RoomRelayOptions) *roomInstance {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(ctx, registry, slog.Default(), 0)
	relay := StartRoomRelay(ctx, pool, hubMgr, opts)
	ts := httptest.NewServer(NewRouter(pool, hubMgr, registry))
	select {
	case <-relay.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("room relay never started listening")
	}
	stop := sync.OnceFunc(func() {
		ts.CloseClientConnections()
		ts.Close()
		cancel()
		relay.Wait()
		hubMgr.WaitIdle()
		pool.Close()
	})
	t.Cleanup(stop)
	return &roomInstance{ts: ts, stop: stop}
}

// liveStream is an SSE stream whose frames can be read while it is still open.
type liveStream struct {
	mu     sync.Mutex
	frames []sseFrame
	stop   func()
}

func openLiveStream(t *testing.T, url, bearer string) *liveStream {
	t.Helper()
	ls := &liveStream{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	ready, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		openSSEFrames(ctx, t, url, bearer, &ls.frames, &ls.mu, ready)
	}()
	<-ready
	ls.stop = sync.OnceFunc(func() { cancel(); <-done })
	t.Cleanup(ls.stop)
	time.Sleep(200 * time.Millisecond) // subscribed before the test writes
	return ls
}

func (ls *liveStream) ids() []int64 {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	return timelineIDs(ls.frames)
}

// waitIDs waits until the stream delivered n timeline frames and returns their ids; it
// then waits briefly more so a duplicate or stray frame would be caught.
func (ls *liveStream) waitIDs(t *testing.T, n int) []int64 {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for len(ls.ids()) < n && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	return ls.ids()
}

// twoAgentRoom creates a private room with a planner and an executor as members and
// returns the slug plus each agent's API key and room token.
type twoAgentRoom struct {
	slug                     string
	plannerKey, plannerTok   string
	executorKey, executorTok string
	pool                     *db.Pool
	roomID                   string
}

func newTwoAgentRoom(t *testing.T, a *roomInstance) *twoAgentRoom {
	t.Helper()
	pool, err := db.NewPool(context.Background(), os.Getenv("DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	roomPreCleanup(t, pool)
	_, ownerJWT := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, a.ts.URL, ownerJWT, true)
	r := &twoAgentRoom{slug: slug, pool: pool}
	var memberID string
	memberID, r.plannerKey = registerRoomTestAgent(t, a.ts)
	status, out := doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "admit planner: %v", out)
	memberID, r.executorKey = registerRoomTestAgent(t, a.ts)
	status, out = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+slug+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "admit executor: %v", out)
	r.plannerTok = handshakeRoomToken(t, a.ts, slug, r.plannerKey)
	r.executorTok = handshakeRoomToken(t, a.ts, slug, r.executorKey)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT id::text FROM rooms WHERE slug=$1`, slug).Scan(&r.roomID))
	return r
}

// commitBehindTheAPI commits a message straight to the database, as an instance that
// crashed before announcing it would: only a notification or a cursor catch-up can
// deliver it.
func (r *twoAgentRoom) commitBehindTheAPI(t *testing.T, body string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, r.pool.QueryRow(context.Background(),
		`INSERT INTO room_entries (room_id, kind, author_type, actor_label, body, content_type, extension)
		 VALUES ($1::uuid, 'message', 'agent', 'behind-the-api', $2, 'text', '{}') RETURNING id`,
		r.roomID, body).Scan(&id))
	return id
}

// committed is the room's committed timeline after afterID (every message and event,
// including system entries such as room.activated), in order: what every stream owes.
func (r *twoAgentRoom) committed(t *testing.T, inst *roomInstance, afterID int64) []int64 {
	t.Helper()
	return entriesAfter(t, inst.ts.URL, r.slug, r.plannerKey, afterID, "")
}

func TestRoomRelay_PlannerAndExecutorOnDifferentInstancesSeeEachOther(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newTwoAgentRoom(t, a)

	planner := openLiveStream(t, a.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.plannerKey)
	executor := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)

	p1 := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "plan-step-1")
	e1 := postRoomMessage(t, b.ts.URL, room.slug, room.executorTok, "executor", "done-step-1")
	p2 := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "plan-step-2")

	want := room.committed(t, b, 0)
	require.Subset(t, want, []int64{p1, e1, p2})
	require.Equal(t, want, executor.waitIDs(t, len(want)), "executor on instance B receives the planner's entries from A, in order, once")
	require.Equal(t, want, planner.waitIDs(t, len(want)), "planner on instance A receives the executor's entry from B, in order, once")
}

func TestRoomRelay_UncommittedEntryIsNeverAnnounced(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newTwoAgentRoom(t, a)
	executor := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)

	ctx := context.Background()
	tx, err := room.pool.BeginTx(ctx)
	require.NoError(t, err)
	var id int64
	require.NoError(t, tx.QueryRow(ctx,
		`INSERT INTO room_entries (room_id, kind, author_type, actor_label, body, content_type, extension)
		 VALUES ($1::uuid, 'message', 'agent', 'tx', 'pending', 'text', '{}') RETURNING id`, room.roomID).Scan(&id))
	time.Sleep(500 * time.Millisecond)
	require.Empty(t, executor.ids(), "an entry must not be announced before it commits")

	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, []int64{id}, executor.waitIDs(t, 1), "the commit announces it")

	tx, err = room.pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		`INSERT INTO room_entries (room_id, kind, author_type, actor_label, body, content_type, extension)
		 VALUES ($1::uuid, 'message', 'agent', 'tx', 'rolled back', 'text', '{}')`, room.roomID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	next := room.commitBehindTheAPI(t, "after rollback")
	require.Equal(t, []int64{id, next}, executor.waitIDs(t, 2), "a rolled-back entry is never delivered")
}

func TestRoomRelay_ListenerInterruptionRecoversEveryEntryInOrder(t *testing.T) {
	// No periodic sweep: only the reconnect catch-up can recover what the gap lost.
	opts := RoomRelayOptions{ReconnectBackoff: 1500 * time.Millisecond, SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newTwoAgentRoom(t, a)
	executor := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)

	ctx := context.Background()
	var killed int
	require.NoError(t, room.pool.QueryRow(ctx,
		`SELECT COUNT(pg_terminate_backend(pid)) FROM pg_stat_activity
		 WHERE application_name = $1 AND datname = current_database()`, RoomRelayApplicationName).Scan(&killed))
	require.GreaterOrEqual(t, killed, 2, "both instances' listeners were interrupted")

	// Committed while no instance listens: these notifications are lost.
	var want []int64
	for i := 0; i < 5; i++ {
		want = append(want, room.commitBehindTheAPI(t, "during-gap-"+strconv.Itoa(i)))
	}
	require.Equal(t, want, executor.waitIDs(t, 5), "cursor recovery delivers every committed entry, in order, once")

	// The reconnected listener carries live traffic again.
	after := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "after-recovery")
	all := room.committed(t, b, 0)
	require.Contains(t, all, after)
	require.Equal(t, all, executor.waitIDs(t, len(all)))
}

func TestRoomRelay_RestartedInstanceResumesTheStreamWithoutHolesOrDuplicates(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	room := newTwoAgentRoom(t, a)

	executor := openLiveStream(t, b.ts.URL+"/v1/rooms/"+room.slug+"/stream", room.executorKey)
	p1 := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "before-restart")
	require.Equal(t, []int64{p1}, executor.waitIDs(t, 1))

	b.stop() // instance B goes down; its stream ends
	executor.stop()
	p2 := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "while-down-1")
	e2 := postRoomMessage(t, a.ts.URL, room.slug, room.executorTok, "executor", "while-down-2")

	b2 := startRoomInstance(t, RoomRelayOptions{})
	resumed := openLiveStream(t, b2.ts.URL+"/v1/rooms/"+room.slug+"/stream?lastEventId="+strconv.FormatInt(p1, 10), room.executorKey)
	p3 := postRoomMessage(t, a.ts.URL, room.slug, room.plannerTok, "planner", "after-restart")
	want := room.committed(t, a, p1)
	require.Subset(t, want, []int64{p2, e2, p3})
	require.Equal(t, want, resumed.waitIDs(t, len(want)), "replay then live, no hole and no duplicate")
}
