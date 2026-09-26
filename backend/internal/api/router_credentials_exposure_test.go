package api

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/token"
)

// idx 75 step 5: "credential storage, HTTP logging ... cannot expose account or room
// secrets". Run against the real router and the real database: what is stored is a hash
// of every credential, and a credential a client sends (valid or not, in a header or in a
// URL) is never echoed in a response and never reaches the access log.

func rawRequest(t *testing.T, method, url, bearer, body string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestCredentials_AreStoredAsHashesOnly(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	ctx := context.Background()

	// A per-agent room token: only its SHA-256 is stored.
	var stored string
	require.NoError(t, a.pool.QueryRow(ctx,
		`SELECT token_hash FROM room_agent_tokens WHERE agent_id = $1 ORDER BY created_at DESC LIMIT 1`,
		room.executorID).Scan(&stored))
	require.Equal(t, token.HashToken(room.executorTok), stored, "the room token is stored as its hash")
	require.NotContains(t, stored, room.executorTok)

	// An agent API key: never stored in the clear.
	var keyHash string
	require.NoError(t, a.pool.QueryRow(ctx,
		`SELECT api_key_hash FROM agents WHERE id = $1`, room.executorID).Scan(&keyHash))
	require.NotEmpty(t, keyHash)
	require.NotEqual(t, room.executorKey, keyHash)
	require.NotContains(t, keyHash, room.executorKey)
}

func TestCredentials_AreNeverEchoedInResponsesOrLogs(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	room := newAccessRoom(t, a, true)
	base := a.ts.URL

	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)

	// Credentials that do not exist, as a client would send them wrongly.
	bogus := []string{
		"solvr_rt_b0gus0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab",
		"solvr_b0gusagentkey0123456789abcdef",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJib2d1cyJ9.b0gusSIGNATURE0123456789",
	}
	requests := []struct{ method, path, body string }{
		{"GET", "/v1/rooms/" + room.slug + "/entries", ""},
		{"GET", "/r/" + room.slug + "/messages", ""},
		{"POST", "/r/" + room.slug + "/message", `{"content":"hi"}`},
		{"POST", "/v1/rooms/" + room.slug + "/stream-ticket", ""},
		{"POST", "/v1/rooms/" + room.slug + "/handshake", `{}`},
		{"POST", "/v1/rooms/" + room.slug + "/entries", `{"kind":"message","content":"hi"}`},
		{"POST", "/v1/posts", `{"type":"question","title":"a title long enough","description":"a description that is long enough to pass"}`},
		{"GET", "/v1/me", ""},
	}
	for _, secret := range bogus {
		for _, rq := range requests {
			status, body := rawRequest(t, rq.method, base+rq.path, secret, rq.body)
			require.GreaterOrEqual(t, status, 400, "%s %s must refuse a credential that does not exist", rq.method, rq.path)
			require.NotContains(t, body, secret, "%s %s echoed the credential it refused", rq.method, rq.path)
		}
	}

	// A credential in a URL: the real ones, so the log line is the one that matters.
	tok, jwt := room.executorTok, room.ownerJWT
	for _, url := range []string{
		base + "/v1/rooms/" + room.slug + "/entries?token=" + tok,
		base + "/v1/rooms/" + room.slug + "/entries?access_token=" + jwt,
		base + "/r/" + room.slug + "/messages?token=" + tok,
		base + "/v1/rooms/" + room.slug + "/stream?token=" + tok,
		base + "/v1/rooms/" + room.slug + "/stream?ticket=solvr_st_b0gusticket.b0gussig",
	} {
		status, body := rawRequest(t, "GET", url, "", "")
		require.GreaterOrEqual(t, status, 400, url)
		require.NotContains(t, body, tok, url)
		require.NotContains(t, body, jwt, url)
		require.NotContains(t, body, "b0gusticket", url)
	}

	captured := logs.String()
	require.Contains(t, captured, "Request completed", "the log capture must not be vacuous")
	for _, secret := range append(bogus, tok, jwt, room.executorKey, "b0gusticket", "b0gussig") {
		require.NotContains(t, captured, secret, "a credential reached the log")
	}
}
