package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The operator curates the homepage's featured pool (SPEC Part 26, "Featured rooms").

type fakeFeaturedStore struct {
	rooms      map[string]*db.FeaturedRoom
	public     map[string]bool
	gotAsk     *int
	gotOutcome *int
}

func newFakeFeaturedStore() *fakeFeaturedStore {
	return &fakeFeaturedStore{rooms: map[string]*db.FeaturedRoom{}, public: map[string]bool{}}
}

func (f *fakeFeaturedStore) Feature(_ context.Context, slug string, askSeq, outcomeSeq *int) (*db.FeaturedRoom, error) {
	f.gotAsk, f.gotOutcome = askSeq, outcomeSeq
	if !f.public[slug] {
		return nil, db.ErrRoomNotFound
	}
	room, ok := f.rooms[slug]
	if !ok {
		room = &db.FeaturedRoom{RoomID: uuid.New(), Slug: slug, Public: true,
			FeaturedAt: time.Date(2026, 10, 1, 0, 0, len(f.rooms), 0, time.UTC)}
		f.rooms[slug] = room
	}
	room.AskSeq, room.OutcomeSeq = askSeq, outcomeSeq
	return room, nil
}

func (f *fakeFeaturedStore) Unfeature(_ context.Context, slug string) (bool, error) {
	_, ok := f.rooms[slug]
	delete(f.rooms, slug)
	return ok, nil
}

func (f *fakeFeaturedStore) ListAll(context.Context) ([]db.FeaturedRoom, error) {
	out := make([]db.FeaturedRoom, 0, len(f.rooms))
	for _, slug := range []string{"room-a", "room-b", "room-c", "room-d", "private-one"} {
		if r, ok := f.rooms[slug]; ok {
			copy := *r
			copy.Public = f.public[slug]
			out = append(out, copy)
		}
	}
	return out, nil
}

func featuredAdminRouter(h *AdminFeaturedRoomsHandler) http.Handler {
	r := chi.NewRouter()
	r.Put("/admin/rooms/{slug}/featured", h.Feature)
	r.Delete("/admin/rooms/{slug}/featured", h.Unfeature)
	r.Get("/admin/rooms/featured", h.List)
	return r
}

func featuredAdminDo(t *testing.T, h http.Handler, method, path, body string, withKey bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if withKey {
		req.Header.Set("X-Admin-API-Key", "op-key")
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAdminFeaturedRooms_RequireTheOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	store := newFakeFeaturedStore()
	store.public["room-a"] = true
	h := featuredAdminRouter(NewAdminFeaturedRoomsHandler(store, nil))

	for _, c := range []struct{ method, path string }{
		{http.MethodPut, "/admin/rooms/room-a/featured"},
		{http.MethodDelete, "/admin/rooms/room-a/featured"},
		{http.MethodGet, "/admin/rooms/featured"},
	} {
		rec := featuredAdminDo(t, h, c.method, c.path, "", false)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, "%s %s without the key", c.method, c.path)
	}
	assert.Empty(t, store.rooms, "nothing featured without the key")
}

func TestAdminFeaturedRooms_FeatureNamesTheQuotedMessagesAndClearsTheOverview(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	store := newFakeFeaturedStore()
	store.public["room-a"] = true
	changed := 0
	h := featuredAdminRouter(NewAdminFeaturedRoomsHandler(store, func(context.Context) error { changed++; return nil }))

	rec := featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/room-a/featured", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Nil(t, store.gotAsk, "no body: the API picks the quoted messages")
	assert.Equal(t, 1, changed, "the homepage snapshot is dropped at once")

	rec = featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/room-a/featured", `{"ask_seq": 3, "outcome_seq": 12}`, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, store.gotAsk)
	assert.Equal(t, 3, *store.gotAsk)
	assert.Equal(t, 12, *store.gotOutcome)

	var body struct {
		Data struct {
			Slug       string `json:"slug"`
			AskSeq     *int   `json:"ask_seq"`
			OutcomeSeq *int   `json:"outcome_seq"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "room-a", body.Data.Slug)
	require.NotNil(t, body.Data.OutcomeSeq)
	assert.Equal(t, 12, *body.Data.OutcomeSeq)
}

func TestAdminFeaturedRooms_RejectsBadInputAndRoomsItMayNotShow(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	store := newFakeFeaturedStore()
	store.public["room-a"] = true
	h := featuredAdminRouter(NewAdminFeaturedRoomsHandler(store, nil))

	assert.Equal(t, http.StatusBadRequest, featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/room-a/featured", `{"ask_seq":`, true).Code)
	assert.Equal(t, http.StatusBadRequest, featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/room-a/featured", `{"ask_seq": 0}`, true).Code)
	assert.Equal(t, http.StatusNotFound, featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/private-one/featured", "", true).Code,
		"a private, deleted or missing room is never featured")
	assert.Empty(t, store.rooms)
}

func TestAdminFeaturedRooms_UnfeatureSaysWhetherTheRoomWasInThePool(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	store := newFakeFeaturedStore()
	store.public["room-a"] = true
	changed := 0
	h := featuredAdminRouter(NewAdminFeaturedRoomsHandler(store, func(context.Context) error { changed++; return nil }))
	featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/room-a/featured", "", true)

	rec := featuredAdminDo(t, h, http.MethodDelete, "/admin/rooms/room-a/featured", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"removed":true`)
	assert.Equal(t, 2, changed)

	rec = featuredAdminDo(t, h, http.MethodDelete, "/admin/rooms/room-a/featured", "", true)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"removed":false`)
	assert.Equal(t, 2, changed, "nothing changed, nothing to announce")
}

func TestAdminFeaturedRooms_ListMarksTheRoomsShownToday(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	store := newFakeFeaturedStore()
	for _, s := range []string{"room-a", "room-b", "room-c", "room-d"} {
		store.public[s] = true
	}
	handler := NewAdminFeaturedRoomsHandler(store, nil)
	// 2026-10-04 is day 20730; 20730 mod 4 = 2, so today shows c, d, a.
	handler.now = func() time.Time { return time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC) }
	h := featuredAdminRouter(handler)
	for _, s := range []string{"room-a", "room-b", "room-c", "room-d"} {
		featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/"+s+"/featured", "", true)
	}
	store.public["private-one"] = true
	featuredAdminDo(t, h, http.MethodPut, "/admin/rooms/private-one/featured", "", true)
	store.public["private-one"] = false // taken private after it was featured

	rec := featuredAdminDo(t, h, http.MethodGet, "/admin/rooms/featured", "", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data []struct {
			Slug       string `json:"slug"`
			Public     bool   `json:"public"`
			ShownToday bool   `json:"shown_today"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	shown := map[string]bool{}
	for _, r := range body.Data {
		shown[r.Slug] = r.ShownToday
		if r.Slug == "private-one" {
			assert.False(t, r.Public, "the pool keeps a private room but says so")
		}
	}
	assert.Len(t, body.Data, 5)
	assert.Equal(t, map[string]bool{"room-a": true, "room-b": false, "room-c": true, "room-d": true, "private-one": false}, shown)
}
