package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage overview keeps a 30-second snapshot per instance. A room that goes private
// (or is archived or deleted) through one instance must drop out of EVERY instance's
// snapshot, not only the one that served the change.

// overviewRoom creates a public room through inst, owned by a fresh user, with two
// messages so it qualifies for the homepage preview, and returns the owner's JWT.
func overviewRoom(t *testing.T, inst *roomInstance, slug string) string {
	t.Helper()
	ctx := context.Background()
	roomPreCleanup(t, inst.pool)
	_, ownerJWT := createRoomTestUser(t, inst.pool)
	status, out := doJSON(t, "POST", inst.ts.URL+"/v1/rooms", ownerJWT,
		fmt.Sprintf(`{"display_name":"Overview %s","slug":%q,"is_private":false}`, slug, slug))
	require.Equal(t, http.StatusCreated, status, "create room: %v", out)
	roomRepo := db.NewRoomRepository(inst.pool)
	room, err := roomRepo.GetBySlug(ctx, slug)
	require.NoError(t, err)
	msgRepo := db.NewMessageRepository(inst.pool)
	for i, author := range []string{"mi_planner", "mi_executor"} {
		_, err := msgRepo.Create(ctx, models.CreateMessageParams{
			RoomID: room.ID, AuthorType: "agent", AgentName: author,
			Content: fmt.Sprintf("overview message %d", i), ContentType: "text",
		})
		require.NoError(t, err)
	}
	return ownerJWT
}

// overviewShows reports whether inst's /v1/overview currently mentions slug.
func overviewShows(t *testing.T, inst *roomInstance, slug string) bool {
	t.Helper()
	_, raw := getConsolidatedOverview(t, inst.ts.URL)
	return strings.Contains(raw, slug)
}

// waitOverviewHides waits up to d for inst's overview to stop mentioning slug.
func waitOverviewHides(t *testing.T, inst *roomInstance, slug string, d time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !overviewShows(t, inst, slug) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !overviewShows(t, inst, slug)
}

func TestRoomOverview_VisibilityChangeOnOneInstanceClearsEveryInstanceSnapshot(t *testing.T) {
	slug := hpoSlug("mi")
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	b := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	t.Cleanup(func() { hpoCleanup(t, a.pool) })
	ownerJWT := overviewRoom(t, a, slug)
	featureOnHomepage(t, a.pool, slug)

	require.True(t, overviewShows(t, a, slug), "A previews the public room")
	require.True(t, overviewShows(t, b, slug), "B previews the public room (now cached on B)")

	status, out := doJSONAtCurrentVersion(t, "PATCH", a.ts.URL+"/v1/rooms/"+slug, ownerJWT, `{"is_private":true}`)
	require.Equal(t, http.StatusOK, status, "make private through A: %v", out)

	require.False(t, overviewShows(t, a, slug), "A drops the room at once")
	require.True(t, waitOverviewHides(t, b, slug, 3*time.Second),
		"B must drop the now-private room well before its 30s snapshot expires")
}

func TestRoomOverview_InvalidationLostDuringListenerGapIsCaughtUpOnReconnect(t *testing.T) {
	slug := hpoSlug("mig")
	opts := RoomRelayOptions{ReconnectBackoff: 1500 * time.Millisecond, SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	t.Cleanup(func() { hpoCleanup(t, a.pool) })
	ownerJWT := overviewRoom(t, a, slug)
	featureOnHomepage(t, a.pool, slug)
	require.True(t, overviewShows(t, b, slug), "B previews the public room (now cached on B)")

	var killed int
	require.NoError(t, a.pool.QueryRow(context.Background(),
		`SELECT COUNT(pg_terminate_backend(pid)) FROM pg_stat_activity
		 WHERE application_name = $1 AND datname = current_database()`, RoomRelayApplicationName).Scan(&killed))
	require.GreaterOrEqual(t, killed, 2, "both instances' listeners were interrupted")

	// Made private while no instance listens: B never hears this notice.
	status, out := doJSONAtCurrentVersion(t, "PATCH", a.ts.URL+"/v1/rooms/"+slug, ownerJWT, `{"is_private":true}`)
	require.Equal(t, http.StatusOK, status, "make private through A: %v", out)

	require.True(t, waitOverviewHides(t, b, slug, 6*time.Second),
		"B must drop its snapshot when its listener reconnects, not serve it for 30s")
}
