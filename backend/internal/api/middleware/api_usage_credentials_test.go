package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 75 step 5, analytics. The usage figures are published, so they may not be moved by
// plumbing and may not be steered by a URL the API does not honour as a credential.

// Minting a stream ticket is a browser preparing to open a stream, once per (re)connect.
// It is transport, like the stream it precedes: counting it would report every person who
// merely watches a room as doing a "write".
func TestAPIUsage_AStreamTicketMintIsTransportNotUsage(t *testing.T) {
	spy := &recordingSpy{}
	r := chi.NewRouter()
	r.Use(APIUsage(spy))
	r.Post("/v1/rooms/{slug}/stream-ticket", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/rooms/some-room/stream-ticket", nil))

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Empty(t, spy.events, "a stream ticket mint must not count as API usage")
}

// The room routes read a credential from the Authorization header only, so a credential
// in the query string authenticates nobody and must not decide who the caller "is".
func TestAPIUsage_ActorIsReadFromTheHeaderOnly(t *testing.T) {
	for _, target := range []string{
		"/v1/posts?token=solvr_rt_agenttoken",
		"/v1/posts?access_token=solvr_sk_personalkey",
		"/v1/posts?access_token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig",
	} {
		spy, _ := call(t, http.MethodGet, target, nil)
		require.Len(t, spy.events, 1, target)
		assert.Equal(t, db.APIActorAnonymous, spy.events[0].ActorType, "%s: an unauthenticated URL parameter is not a credential", target)
	}
}
