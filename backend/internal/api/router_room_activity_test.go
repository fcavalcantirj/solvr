package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type cancelRequestKey struct{}

type cancelAfterInsertMark struct{}

// cancelAfterMessageInsert ends a request's context as soon as its message INSERT
// returns: the caller is gone right after the entry is stored, as when a client
// disconnects or its deadline fires mid-request.
type cancelAfterMessageInsert struct{ fired atomic.Int32 }

func (c *cancelAfterMessageInsert) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "INSERT INTO messages") {
		return context.WithValue(ctx, cancelAfterInsertMark{}, true)
	}
	return ctx
}

func (c *cancelAfterMessageInsert) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(cancelAfterInsertMark{}) == nil || data.Err != nil {
		return
	}
	if cancel, ok := ctx.Value(cancelRequestKey{}).(context.CancelFunc); ok {
		c.fired.Add(1)
		cancel()
	}
}

// Room activity is a projection of the timeline (idx 77): a message the API stored is
// counted and is the room's latest activity even when the caller's context ends right
// after the insert, because the count moves in the insert's own transaction.
func TestRoomActivity_ACallerThatLeavesRightAfterTheInsertIsStillCounted(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	tracer := &cancelAfterMessageInsert{}
	pool, err := db.NewPool(context.Background(), dbURL, db.WithQueryTracer(tracer))
	require.NoError(t, err)
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		router.ServeHTTP(w, r.WithContext(context.WithValue(ctx, cancelRequestKey{}, cancel)))
	}))
	defer func() {
		ts.Close()
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-activity-%')")
		pool.Exec(ctx, "DELETE FROM rooms WHERE slug LIKE 'test-activity-%'")
		pool.Exec(ctx, "DELETE FROM agents WHERE id LIKE 'agent_roomtest_%'")
		pool.Exec(ctx, "DELETE FROM users WHERE username LIKE 'roomtest_%'")
		pool.Close()
	}()

	_, jwt := createRoomTestUser(t, pool)
	slug := fmt.Sprintf("test-activity-%d", os.Getpid())
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms", jwt,
		fmt.Sprintf(`{"display_name":"Activity %s","slug":%q,"is_private":false}`, slug, slug))
	require.Equal(t, http.StatusCreated, status, "create room: %v", out)
	agentID, agentKey := registerRoomTestAgent(t, ts)
	roomTok := handshakeRoomToken(t, ts, slug, agentKey)

	// Through each write route: canonical entries, the agent transport, the human route.
	writes := []struct{ url, bearer, body string }{
		{ts.URL + "/v1/rooms/" + slug + "/entries", roomTok, `{"body":"plan: one"}`},
		{ts.URL + "/r/" + slug + "/message", roomTok, `{"agent_name":"` + agentID + `","content":"build: two"}`},
		{ts.URL + "/v1/rooms/" + slug + "/messages", jwt, `{"content":"looks good"}`},
	}
	// The caller is gone after the insert, so the status it would have read is not the
	// point (a route that reads the entry back answers 500 on the ended context); what
	// was stored is.
	for _, w := range writes {
		status, _ := doJSON(t, "POST", w.url, w.bearer, w.body)
		t.Logf("%s answered %d to a caller that had already left", w.url, status)
	}
	require.Equal(t, int32(len(writes)), tracer.fired.Load(), "every write lost its caller right after the insert")

	var stored, counted int
	var lastActive, newest string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT r.message_count,
		       (SELECT COUNT(*) FROM room_entries e WHERE e.room_id = r.id AND e.kind = 'message' AND e.deleted_at IS NULL),
		       r.last_active_at::text,
		       (SELECT MAX(e.created_at) FROM room_entries e WHERE e.room_id = r.id AND e.kind = 'message')::text
		FROM rooms r WHERE r.slug = $1`, slug).Scan(&counted, &stored, &lastActive, &newest))
	require.Equal(t, len(writes), stored, "every write stored its entry")
	require.Equal(t, stored, counted, "message_count counts every stored message")
	require.Equal(t, newest, lastActive, "last_active_at is the newest stored message")
}
