package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// No public or participant surface of a room reveals a credential (task: "Keep room and
// content permissions authoritative on the server", step 5). Every read surface of a room
// is fetched as anonymous, member (room token and account key), owner and an unrelated
// agent; no body may contain any agent API key, room token, owner session, or the stored
// hashes behind them (agents.api_key_hash/key_sha256, room_agent_tokens.token_hash). The
// default connect prompts are among those surfaces, and they must tell each agent to take
// its OWN room token by handshake.

// getRaw returns the status and the raw body of a GET. A stream answers with its first
// frames: the read stops when the context expires.
func getRaw(t *testing.T, url, bearer string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "GET %s", url)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body) // a stream ends with a context error; its frames are kept
	return resp.StatusCode, string(body)
}

// roomStoredSecrets reads, from the database, every hash stored for the room's tokens and
// for the given agents' keys.
func roomStoredSecrets(t *testing.T, room *accessRoom, agentIDs []string) map[string]string {
	t.Helper()
	ctx := context.Background()
	out := map[string]string{}
	rows, err := room.pool.Query(ctx, `SELECT t.agent_id, t.token_hash FROM room_agent_tokens t
		JOIN rooms r ON r.id = t.room_id WHERE r.slug = $1`, room.slug)
	require.NoError(t, err)
	for rows.Next() {
		var agentID, hash string
		require.NoError(t, rows.Scan(&agentID, &hash))
		out["room token hash of "+agentID] = hash
	}
	rows.Close()
	require.GreaterOrEqual(t, len(out), 2, "both participants hold a stored room token")
	for _, id := range agentIDs {
		var bcryptHash, sha *string
		require.NoError(t, room.pool.QueryRow(ctx,
			`SELECT api_key_hash, key_sha256 FROM agents WHERE id = $1`, id).Scan(&bcryptHash, &sha))
		if bcryptHash != nil && *bcryptHash != "" {
			out["api_key_hash of "+id] = *bcryptHash
		}
		if sha != nil && *sha != "" {
			out["key_sha256 of "+id] = *sha
		}
	}
	return out
}

// otherTokenHolder is the planner: the room's token holder that is not the executor.
func otherTokenHolder(t *testing.T, room *accessRoom) string {
	t.Helper()
	var id string
	require.NoError(t, room.pool.QueryRow(context.Background(), `SELECT t.agent_id FROM room_agent_tokens t
		JOIN rooms r ON r.id = t.room_id WHERE r.slug = $1 AND t.agent_id <> $2`, room.slug, room.executorID).Scan(&id))
	return id
}

func TestRoomSecrets_NoRoomSurfaceRevealsCredentials(t *testing.T) {
	inst := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})

	for _, private := range []bool{false, true} {
		t.Run("private="+strconv.FormatBool(private), func(t *testing.T) {
			room := newAccessRoom(t, inst, private)
			plannerID := otherTokenHolder(t, room)
			unrelatedID, unrelatedKey := registerRoomTestAgent(t, inst.ts)

			base := inst.ts.URL
			status, entryID, err := postEntryRaw(base, room.slug, room.plannerTok, map[string]any{"body": "secrets sweep entry"})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, status)
			for i, tok := range []string{room.plannerTok, room.executorTok} {
				status, out := doJSON(t, "POST", base+"/r/"+room.slug+"/join", tok, `{"agent_name":"sweep-agent-`+strconv.Itoa(i)+`"}`)
				require.Equal(t, http.StatusOK, status, "join marks the agent present: %v", out)
			}

			secrets := map[string]string{
				"planner API key":     room.plannerKey,
				"executor API key":    room.executorKey,
				"unrelated API key":   unrelatedKey,
				"planner room token":  room.plannerTok,
				"executor room token": room.executorTok,
				"owner session":       room.ownerJWT,
			}
			for name, v := range roomStoredSecrets(t, room, []string{plannerID, room.executorID, unrelatedID}) {
				secrets[name] = v
			}

			r := base + "/v1/rooms/" + room.slug
			entry := strconv.FormatInt(entryID, 10)
			surfaces := []string{
				r, r + "/agents", r + "/messages", r + "/messages/" + entry, r + "/entries",
				r + "/entries/" + entry, r + "/posts", r + "/members", r + "/stream",
				r + "/connect?role=executor", r + "/connect?role=planner", r + "/connect?role=reviewer",
				base + "/v1/connect", base + "/v1/rooms", base + "/v1/search?q=" + room.slug,
				base + "/v1/sitemap/urls", base + "/v1/agents/" + plannerID, base + "/v1/agents/" + room.executorID,
				base + "/r/" + room.slug + "/messages", base + "/r/" + room.slug + "/messages/" + entry,
				base + "/r/" + room.slug + "/agents", base + "/r/" + room.slug + "/pins",
				base + "/r/" + room.slug + "/claims", base + "/r/" + room.slug + "/events",
				base + "/r/" + room.slug + "/stream",
			}
			callers := map[string]string{
				"anonymous":           "",
				"member room token":   room.plannerTok,
				"member account key":  room.executorKey,
				"owner":               room.ownerJWT,
				"unrelated agent key": unrelatedKey,
			}

			served := 0
			for _, url := range surfaces {
				for caller, bearer := range callers {
					status, body := getRaw(t, url, bearer)
					if status == http.StatusOK {
						served++
					}
					for name, secret := range secrets {
						require.NotContains(t, body, secret, "GET %s as %s (status %d) reveals the %s", url, caller, status, name)
					}
				}
			}
			require.Greater(t, served, len(surfaces), "the sweep read real bodies, not only refusals")

			// A connect prompt served to the owner is a default prompt: it names the per-agent
			// handshake and never embeds a credential of its own.
			status, body := getRaw(t, r+"/connect?role=executor", room.ownerJWT)
			require.Equal(t, http.StatusOK, status, body)
			require.Contains(t, body, "/handshake", "the prompt tells the agent to take its own room token")
			require.NotRegexp(t, `solvr_(rt|rm|sk)_[A-Za-z0-9]{8,}`, body, "no concrete token in the prompt")
		})
	}
}
