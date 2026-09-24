package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// One authorization policy for every /v1/rooms/{slug} route (task: "Offer one canonical
// room API with thin adapters for the existing transport routes"). The room detail,
// presence and connect reads decide exactly like GET /v1/rooms/{slug}/entries, and the
// human message adapter POST /v1/rooms/{slug}/messages decides like a human write to
// POST /v1/rooms/{slug}/entries.

func TestRoomEntriesPolicy_RoomReadsFollowTheOnePolicy(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	_, outsiderJWT := createRoomTestUser(t, pool)
	private := entriesTestRoom(t, ts.URL, ownerJWT, true)
	public := entriesTestRoom(t, ts.URL, ownerJWT, false)

	memberID, memberKey := registerRoomTestAgent(t, ts)
	status, out := doJSON(t, "POST", ts.URL+"/v1/rooms/"+private+"/members", ownerJWT, `{"agent_id":"`+memberID+`"}`)
	require.Equal(t, http.StatusCreated, status, "owner admits member: %v", out)
	memberTok := handshakeRoomToken(t, ts, private, memberKey)
	_, outsiderKey := registerRoomTestAgent(t, ts)
	publicTok := handshakeRoomToken(t, ts, public, outsiderKey)
	const invalidTok = "solvr_rt_notarealtoken"

	credentials := []struct {
		name   string
		room   string
		bearer string
	}{
		{"anonymous on private", private, ""},
		{"outsider agent key on private", private, outsiderKey},
		{"outsider human on private", private, outsiderJWT},
		{"other room's token on private", private, publicTok},
		{"invalid room token on private", private, invalidTok},
		{"member room token", private, memberTok},
		{"member agent key", private, memberKey},
		{"owner human JWT", private, ownerJWT},
		{"anonymous on public", public, ""},
		{"public room's own token", public, publicTok},
		{"other room's token on public", public, memberTok},
		{"invalid room token on public", public, invalidTok},
	}
	routes := []string{"", "/agents", "/connect"}
	for _, c := range credentials {
		base := ts.URL + "/v1/rooms/" + c.room
		want := getStatus(t, base+"/entries", c.bearer)
		for _, route := range routes {
			assert.Equal(t, want, getStatus(t, base+route, c.bearer), "%s: GET /{slug}%s must decide like /entries", c.name, route)
		}
	}

	// Pin the decisions themselves, so "equal" cannot mean "equally wrong".
	for _, route := range routes {
		for _, c := range []struct {
			name   string
			room   string
			bearer string
			want   int
		}{
			{"private anonymous", private, "", http.StatusForbidden},
			{"private outsider human", private, outsiderJWT, http.StatusForbidden},
			{"private invalid token", private, invalidTok, http.StatusUnauthorized},
			{"private other room's token", private, publicTok, http.StatusForbidden},
			{"private member token", private, memberTok, http.StatusOK},
			{"private member agent key", private, memberKey, http.StatusOK},
			{"private owner", private, ownerJWT, http.StatusOK},
			{"public anonymous", public, "", http.StatusOK},
			{"public other room's token", public, memberTok, http.StatusForbidden},
			{"public invalid token", public, invalidTok, http.StatusUnauthorized},
			{"unknown room", "no-such-room-xyz", "", http.StatusNotFound},
		} {
			assert.Equal(t, c.want, getStatus(t, ts.URL+"/v1/rooms/"+c.room+route, c.bearer), "%s GET /{slug}%s", c.name, route)
		}
	}
}

func TestRoomEntriesPolicy_HumanMessageAdapterDecidesLikeCanonicalWrite(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, ownerJWT := createRoomTestUser(t, pool)
	_, outsiderJWT := createRoomTestUser(t, pool)
	private := entriesTestRoom(t, ts.URL, ownerJWT, true)
	public := entriesTestRoom(t, ts.URL, ownerJWT, false)

	for _, c := range []struct {
		name   string
		room   string
		bearer string
		want   int
	}{
		{"private owner", private, ownerJWT, http.StatusCreated},
		{"private outsider human", private, outsiderJWT, http.StatusForbidden},
		{"public owner", public, ownerJWT, http.StatusCreated},
		{"public outsider human", public, outsiderJWT, http.StatusCreated},
		{"unknown room", "no-such-room-xyz", ownerJWT, http.StatusNotFound},
	} {
		base := ts.URL + "/v1/rooms/" + c.room
		canonical, out := doJSON(t, "POST", base+"/entries", c.bearer, `{"body":"canonical `+c.name+`"}`)
		assert.Equal(t, c.want, canonical, "%s: canonical write %v", c.name, out)
		adapter, out := doJSON(t, "POST", base+"/messages", c.bearer, `{"content":"adapter `+c.name+`"}`)
		assert.Equal(t, canonical, adapter, "%s: /messages must decide like /entries: %v", c.name, out)
	}

	// Both paths stored exactly one row each for the two allowed private writes.
	status, out := doJSON(t, "GET", ts.URL+"/v1/rooms/"+private+"/entries?kind=message", ownerJWT, "")
	require.Equal(t, http.StatusOK, status, "list: %v", out)
	rows, _ := out["data"].([]any)
	require.Len(t, rows, 2, "private room: one canonical + one adapter message")
	for _, raw := range rows {
		row := raw.(map[string]any)
		assert.Equal(t, "human", row["author_type"])
	}

	// Anonymous stays 401 on both paths.
	canonical, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+public+"/entries", "", `{"body":"anon"}`)
	adapter, _ := doJSON(t, "POST", ts.URL+"/v1/rooms/"+public+"/messages", "", `{"content":"anon"}`)
	assert.Equal(t, http.StatusUnauthorized, canonical)
	assert.Equal(t, http.StatusUnauthorized, adapter)
}
