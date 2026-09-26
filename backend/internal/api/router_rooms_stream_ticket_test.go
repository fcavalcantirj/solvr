package api

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// idx 75 step 2: a browser EventSource cannot send an Authorization header, so it opens
// the stream with a short-lived stream TICKET instead of a long-lived bearer credential in
// the URL. The ticket is minted by an authenticated POST, is bound to one room, opens only
// the read stream, and the stream it opens is still cut when the underlying access ends.

func ticketSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "test-jwt-secret-32-chars-long!!" // the router's own fallback
}

// mintTicketAt asks the API at baseURL for a stream ticket for room slug with bearer.
func mintTicketAt(t *testing.T, baseURL, slug, bearer string) string {
	t.Helper()
	status, out := doJSON(t, "POST", baseURL+"/v1/rooms/"+slug+"/stream-ticket", bearer, "")
	require.Equal(t, http.StatusCreated, status, "mint a stream ticket: %v", out)
	data, _ := out["data"].(map[string]any)
	ticket, _ := data["ticket"].(string)
	require.True(t, strings.HasPrefix(ticket, auth.StreamTicketPrefix), "ticket %q", ticket)
	return ticket
}

func mintTicket(t *testing.T, inst *roomInstance, slug, bearer string) string {
	t.Helper()
	return mintTicketAt(t, inst.ts.URL, slug, bearer)
}

func TestStreamTicket_EveryCredentialKindOpensItsStreamWithATicket(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	stream := room.streamURL(a)

	// A private room refuses an anonymous stream.
	require.Equal(t, http.StatusForbidden, getStatus(t, stream, ""))

	// Nothing to trade for a ticket: the private room's read policy refuses an anonymous
	// caller as it does the stream itself, and a public room's stream needs no ticket.
	status, out := doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", "", "")
	require.Equal(t, http.StatusForbidden, status, "an anonymous caller on a private room: %v", out)
	// (newAccessRoom cleans earlier rooms away, so a second room is made by the same owner.)
	publicSlug := entriesTestRoom(t, a.ts.URL, room.ownerJWT, false)
	status, out = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+publicSlug+"/stream-ticket", "", "")
	require.Equal(t, http.StatusUnauthorized, status, "an anonymous caller on a public room has no credential to trade: %v", out)
	status, _ = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", "solvr_rt_"+strings.Repeat("0", 64), "")
	require.Equal(t, http.StatusUnauthorized, status, "an invalid credential gets no ticket")

	for name, bearer := range map[string]string{
		"owner JWT":         room.ownerJWT,
		"member room token": room.executorTok,
		"member agent key":  room.executorKey,
	} {
		ticket := mintTicket(t, a, room.slug, bearer)
		require.Equal(t, http.StatusOK, getStatus(t, stream+"?ticket="+ticket, ""), "%s: the ticket opens the private stream", name)
	}

	_, strangerJWT := createRoomTestUser(t, a.pool)
	status, out = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", strangerJWT, "")
	require.Equal(t, http.StatusForbidden, status, "a non-member is not given a ticket to a private room: %v", out)

	// The response says how long the ticket lives.
	status, out = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket", room.ownerJWT, "")
	require.Equal(t, http.StatusCreated, status)
	data := out["data"].(map[string]any)
	require.EqualValues(t, int(auth.StreamTicketTTL/time.Second), data["ttl_seconds"])
	expires, err := time.Parse(time.RFC3339, data["expires_at"].(string))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(auth.StreamTicketTTL), expires, 5*time.Second)
}

func TestStreamTicket_IsBoundToOneRoomAndOpensNothingButItsStream(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	otherSlug := entriesTestRoom(t, a.ts.URL, room.ownerJWT, true) // same owner, another room
	ticket := mintTicket(t, a, room.slug, room.ownerJWT)

	// Another room's stream, entries, and every write refuse it; a ticket is not a bearer.
	require.Equal(t, http.StatusForbidden, getStatus(t, a.ts.URL+"/v1/rooms/"+otherSlug+"/stream?ticket="+ticket, ""), "ticket of room A on room B")
	for name, url := range map[string]string{
		"entries":  a.ts.URL + "/v1/rooms/" + room.slug + "/entries?ticket=" + ticket,
		"messages": a.ts.URL + "/v1/rooms/" + room.slug + "/messages?ticket=" + ticket,
		"room":     a.ts.URL + "/v1/rooms/" + room.slug + "?ticket=" + ticket,
	} {
		require.Equal(t, http.StatusForbidden, getStatus(t, url, ""), "%s: only the stream reads a ticket", name)
	}
	for name, url := range map[string]string{
		"post entry":   a.ts.URL + "/v1/rooms/" + room.slug + "/entries?ticket=" + ticket,
		"post message": a.ts.URL + "/v1/rooms/" + room.slug + "/messages?ticket=" + ticket,
		"adapter post": a.ts.URL + "/r/" + room.slug + "/message?ticket=" + ticket,
	} {
		status, out := doJSON(t, "POST", url, "", `{"body":"forged","content":"forged","agent_name":"x"}`)
		require.Equal(t, http.StatusUnauthorized, status, "%s: a ticket authorizes no write: %v", name, out)
	}
	// It cannot mint the next ticket either: the mint route is header-only.
	status, out := doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/stream-ticket?ticket="+ticket, "", "")
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, status, "a ticket mints no ticket: %v", out)
	// As an Authorization bearer it is just an invalid credential.
	status, _ = doJSON(t, "GET", room.streamURL(a), ticket, "")
	require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, status, "a ticket sent as a bearer opens nothing")
	status, _ = doJSON(t, "POST", a.ts.URL+"/v1/rooms/"+room.slug+"/entries", ticket, `{"body":"forged"}`)
	require.Equal(t, http.StatusUnauthorized, status, "a ticket sent as a bearer writes nothing")
}

func TestStreamTicket_ForgedAndExpiredTicketsAreRefusedWithARecoverableCode(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, false) // public: the ticket check must not be skipped because anonymous is allowed
	stream := room.streamURL(a)

	claims := auth.StreamTicketClaims{RoomID: "any", Kind: "human", Subject: "u"}
	expired, _ := auth.IssueStreamTicket(ticketSecret(), claims, time.Now().Add(-2*auth.StreamTicketTTL))
	forged, _ := auth.IssueStreamTicket("another-secret-another-secret-!!", claims, time.Now())

	for name, c := range map[string]struct{ ticket, code string }{
		"garbage": {"solvr_st_nonsense", "STREAM_TICKET_INVALID"},
		"forged":  {forged, "STREAM_TICKET_INVALID"},
		"expired": {expired, "STREAM_TICKET_EXPIRED"},
	} {
		status, out := doJSON(t, "GET", stream+"?ticket="+c.ticket, "", "")
		require.Equal(t, http.StatusUnauthorized, status, "%s: %v", name, out)
		code, msg, reqID := sessionError(t, out)
		require.Equal(t, c.code, code, name)
		require.Contains(t, msg, "/v1/rooms/"+room.slug+"/stream-ticket", "%s: the answer says where to get a fresh ticket", name)
		require.NotContains(t, msg, c.ticket, "the refusal never echoes the credential")
		require.NotEmpty(t, reqID)
	}
}

func TestStreamTicket_RetiresLongLivedBearersInTheStreamURL(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	priv := newAccessRoom(t, a, true)
	pubSlug := entriesTestRoom(t, a.ts.URL, priv.ownerJWT, false)

	for name, url := range map[string]string{
		"access_token JWT on a private stream": priv.streamURL(a) + "?access_token=" + priv.ownerJWT,
		"access_token room token":              priv.streamURL(a) + "?access_token=" + priv.executorTok,
		"token room token":                     priv.streamURL(a) + "?token=" + priv.executorTok,
		"access_token on a public stream":      a.ts.URL + "/v1/rooms/" + pubSlug + "/stream?access_token=" + priv.ownerJWT,
	} {
		status, out := doJSON(t, "GET", url, "", "")
		require.Equal(t, http.StatusBadRequest, status, "%s: %v", name, out)
		code, msg, _ := sessionError(t, out)
		require.Equal(t, "VALIDATION_ERROR", code, name)
		require.Contains(t, msg, "stream-ticket", "%s: the answer names the replacement", name)
		require.NotContains(t, msg, priv.ownerJWT+priv.executorTok, name)
		require.NotContains(t, msg, priv.executorTok, "the refusal never echoes the credential")
		require.NotContains(t, msg, priv.ownerJWT, "the refusal never echoes the credential")
	}
	// The header path is untouched.
	require.Equal(t, http.StatusOK, getStatus(t, priv.streamURL(a), priv.ownerJWT))
	require.Equal(t, http.StatusOK, getStatus(t, priv.streamURL(a), priv.executorTok))
}

// A ticket is honoured by every instance (no shared memory), and the stream it opens still
// ends when the access behind it ends, on the instance that did not serve the change.
func TestStreamTicket_WorksOnAnyInstanceAndTheOpenStreamStillEndsWhenAccessDoes(t *testing.T) {
	opts := RoomRelayOptions{SweepInterval: -1}
	a := startRoomInstance(t, opts)
	b := startRoomInstance(t, opts)
	room := newAccessRoom(t, a, true)
	stream := room.streamURL(b)

	roomTokenTicket := mintTicket(t, a, room.slug, room.executorTok)
	agentKeyTicket := mintTicket(t, a, room.slug, room.executorKey)
	ownerTicket := mintTicket(t, a, room.slug, room.ownerJWT)
	byRoomToken := openAccessStream(t, stream+"?ticket="+roomTokenTicket, "")
	byAgentKey := openAccessStream(t, stream+"?ticket="+agentKeyTicket, "")
	byOwner := openAccessStream(t, stream+"?ticket="+ownerTicket, "")

	require.False(t, byRoomToken.endedWithin(300*time.Millisecond))

	// The owner revokes the executor's tokens: the room-token stream ends, the ticket does
	// not outlive the credential it was minted from. The agent-key stream is a member's, so
	// it goes when the membership does.
	status, out := doJSON(t, "DELETE", revokeTokenURL(a, room, room.executorID), room.ownerJWT, "")
	require.Equal(t, http.StatusNoContent, status, "revoke: %v", out)
	require.True(t, byRoomToken.endedWithin(3*time.Second), "the ticket stream of a revoked room token ends")
	require.Contains(t, byRoomToken.events(), "access_revoked")
	require.False(t, byAgentKey.endedWithin(300*time.Millisecond), "revoking a room token leaves the agent key's membership alone")

	status, out = doJSON(t, "DELETE", a.ts.URL+"/v1/rooms/"+room.slug+"/members/"+room.executorID, room.ownerJWT, "")
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent}, status, "remove member: %v", out)
	require.True(t, byAgentKey.endedWithin(3*time.Second), "the ticket stream of a removed member ends")
	require.Contains(t, byAgentKey.events(), "access_revoked")
	require.False(t, byOwner.endedWithin(300*time.Millisecond), "the owner's stream is unaffected")

	// A ticket minted from a revoked credential cannot be minted, and one minted before
	// the revoke stops opening a stream (the guard re-resolves the token at open).
	require.Equal(t, http.StatusUnauthorized, getStatus(t, stream+"?ticket="+roomTokenTicket, ""),
		"a reconnect with the pre-revoke ticket is refused")
}

func TestStreamTicket_ARotatedRoomTokenEndsItsTicketStreamAsRotated(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)

	ticket := mintTicket(t, a, room.slug, room.executorTok)
	s := openAccessStream(t, room.streamURL(a)+"?ticket="+ticket, "")
	require.False(t, s.endedWithin(300*time.Millisecond))

	status, out := handshakeBody(t, a, room, room.executorKey, `{"rotate":true}`)
	require.Equal(t, http.StatusCreated, status, "%v", out)
	require.True(t, s.endedWithin(3*time.Second), "the rotated session's ticket stream ends")
	require.Contains(t, s.events(), "credential_rotated")
	require.NotContains(t, s.events(), "access_revoked")

	status, out = doJSON(t, "GET", room.streamURL(a)+"?ticket="+ticket, "", "")
	require.Equal(t, http.StatusUnauthorized, status)
	code, _, _ := sessionError(t, out)
	require.Equal(t, "CREDENTIAL_ROTATED", code, "the ticket of a replaced token says so on reconnect")
}
