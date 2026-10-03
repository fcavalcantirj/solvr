package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// A browser step may name the public room or post it is attributed to. The API resolves
// that public identifier to the record's id; anything it cannot resolve as public is
// dropped, and the client's raw text is never stored.

func newSourcedFunnelHandler(t *testing.T) (*FunnelHandler, *db.FunnelEventRepository, *db.Pool) {
	t.Helper()
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	repo := db.NewFunnelEventRepository(pool)
	h := NewFunnelHandler(repo)
	h.SetSourceResolvers(db.NewRoomRepository(pool), db.NewPostRepository(pool))
	return h, repo, pool
}

func createFunnelSourceRoom(t *testing.T, pool *db.Pool, private bool) *models.Room {
	t.Helper()
	slug := "test-fsrc-" + time.Now().Format("150405.000000")[7:]
	if private {
		slug += "-p"
	}
	room, err := db.NewRoomRepository(pool).Create(context.Background(),
		models.CreateRoomParams{Slug: slug, DisplayName: slug, IsPrivate: private})
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM rooms WHERE id = $1", room.ID) //nolint:errcheck
	})
	return room
}

func ingest(t *testing.T, h *FunnelHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/analytics/funnel", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.IngestBrowserEvent(rec, req)
	return rec
}

func TestFunnelIngest_ShareVisitResolvesAPublicRoomSlugToItsID(t *testing.T) {
	h, repo, pool := newSourcedFunnelHandler(t)
	room := createFunnelSourceRoom(t, pool, false)
	flow := "f_sv_" + time.Now().Format("150405.000000")

	rec := ingest(t, h, `{"flow_id":"`+flow+`","event":"share_visit","entry_surface":"room_page","source":{"kind":"room","ref":"`+room.Slug+`"}}`)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"recorded":true`)

	events, err := repo.ListByFlow(context.Background(), flow)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, models.FunnelShareVisit, events[0].EventName)
	require.Equal(t, models.FunnelSourceKindRoom, events[0].SourceKind)
	require.Equal(t, room.ID.String(), events[0].SourceID)
}

func TestFunnelIngest_AnUnresolvableSourceIsDroppedNotStored(t *testing.T) {
	h, repo, pool := newSourcedFunnelHandler(t)
	priv := createFunnelSourceRoom(t, pool, true)

	for i, src := range []string{
		`{"kind":"room","ref":"` + priv.Slug + `"}`,
		`{"kind":"room","ref":"no-such-room-anywhere"}`,
		`{"kind":"post","ref":"6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11"}`,
		`{"kind":"post","ref":"not-a-uuid"}`,
	} {
		flow := "f_drop_" + time.Now().Format("150405.000000") + string(rune('a'+i))
		rec := ingest(t, h, `{"flow_id":"`+flow+`","event":"connection_started","source":`+src+`}`)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		events, err := repo.ListByFlow(context.Background(), flow)
		require.NoError(t, err)
		require.Len(t, events, 1, "the step itself is still counted")
		require.Empty(t, events[0].SourceKind, "source %s", src)
		require.Empty(t, events[0].SourceID, "source %s", src)
	}
}

func TestFunnelIngest_RefusesAnUnknownSourceKindOrAnOversizedRef(t *testing.T) {
	h, _, _ := newSourcedFunnelHandler(t)
	rec := ingest(t, h, `{"event":"share_visit","source":{"kind":"website","ref":"x"}}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	rec = ingest(t, h, `{"event":"share_visit","source":{"kind":"room","ref":"`+strings.Repeat("a", 201)+`"}}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
