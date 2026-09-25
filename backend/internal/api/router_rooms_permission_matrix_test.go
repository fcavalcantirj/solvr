package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// One matrix for every room surface (task: "Keep room and content permissions authoritative
// on the server", steps 1 and 2). Public and private rooms are read through REST, SSE, the
// /r adapter, the room-specific connect instructions, single-message lookup, search and the
// payload the room page builds its social metadata from, as anonymous, member (room token
// and account key), owner, unrelated agent and unrelated human. Then owner-only actions are
// attempted by everyone else, a member is removed, and the room is deleted; every surface
// must follow each change. Expiry is covered by router_rooms_expiry_test.go.

// matrixCaller is one identity the matrix reads a room as.
type matrixCaller struct {
	name, bearer string
	member       bool // participant of the room (member agent or its owner)
	roomToken    bool // presents a room token (the only credential the /r adapter takes)
}

func isRefusal(status int) bool {
	return status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound
}

// requireRefusedEverywhere asserts every surface refuses the caller and leaks no marker.
func requireRefusedEverywhere(t *testing.T, surfaces []string, caller matrixCaller, marker, why string) {
	t.Helper()
	for _, u := range surfaces {
		status, body := getRaw(t, u, caller.bearer)
		require.True(t, isRefusal(status), "%s: GET %s as %s must be refused, got %d", why, u, caller.name, status)
		require.NotContains(t, body, marker, "%s: GET %s as %s leaks the transcript", why, u, caller.name)
	}
}

func TestRoomPermissionMatrix_EverySurfaceFollowsVisibilityMembershipOwnershipAndDeletion(t *testing.T) {
	inst := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	base := inst.ts.URL
	t.Cleanup(func() {
		inst.pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")
	})

	for _, private := range []bool{false, true} {
		t.Run("private="+strconv.FormatBool(private), func(t *testing.T) {
			room := newAccessRoom(t, inst, private)
			_, unrelatedKey := registerRoomTestAgent(t, inst.ts)
			_, unrelatedJWT := createRoomTestUser(t, inst.pool)

			marker := "MATRIX-MARKER-" + strconv.FormatInt(time.Now().UnixNano(), 36)
			status, entryID, err := postEntryRaw(base, room.slug, room.plannerTok, map[string]any{"body": marker})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, status)
			entry := strconv.FormatInt(entryID, 10)

			// A saved outcome carrying the marker is a draft: search never shows it.
			saved := saveAsPost(t, inst.ts, room.slug, room.executorKey, marker+" outcome", outcomeSummary, "")
			require.Equal(t, http.StatusCreated, saved.StatusCode)
			postID, _ := dataObj(t, saved)["id"].(string)
			require.NotEmpty(t, postID)

			callers := []matrixCaller{
				{name: "anonymous"},
				{name: "member room token", bearer: room.plannerTok, member: true, roomToken: true},
				{name: "member account key", bearer: room.executorKey, member: true},
				{name: "owner", bearer: room.ownerJWT, member: true},
				{name: "unrelated agent", bearer: unrelatedKey},
				{name: "unrelated human", bearer: unrelatedJWT},
			}
			r := base + "/v1/rooms/" + room.slug
			a := base + "/r/" + room.slug
			// Surfaces that carry the transcript (the marker must show to whoever may read).
			transcript := []string{r + "/messages", r + "/messages/" + entry, r + "/entries", r + "/entries/" + entry, a + "/messages", a + "/messages/" + entry}
			// Every read surface of the room. GET /v1/rooms/{slug} is also the only payload the
			// room page's social metadata (title, description, Open Graph) is built from.
			v1Reads := []string{r, r + "/agents", r + "/connect?role=executor", r + "/messages", r + "/messages/" + entry, r + "/entries", r + "/entries/" + entry, r + "/stream"}
			adapterReads := []string{a + "/messages", a + "/messages/" + entry, a + "/agents", a + "/pins", a + "/claims", a + "/events", a + "/stream"}
			allReads := append(append([]string{}, v1Reads...), adapterReads...)

			for _, c := range callers {
				for _, u := range v1Reads {
					status, body := getRaw(t, u, c.bearer)
					if c.member || !private {
						require.Equal(t, http.StatusOK, status, "GET %s as %s: %s", u, c.name, body)
					} else {
						require.True(t, isRefusal(status), "GET %s as %s must be refused, got %d", u, c.name, status)
						require.NotContains(t, body, marker, "GET %s as %s leaks the transcript", u, c.name)
					}
				}
				for _, u := range adapterReads {
					status, body := getRaw(t, u, c.bearer)
					if c.roomToken {
						require.Equal(t, http.StatusOK, status, "GET %s as %s: %s", u, c.name, body)
					} else {
						require.Equal(t, http.StatusUnauthorized, status, "the /r adapter takes only a room token: GET %s as %s", u, c.name)
						require.NotContains(t, body, marker)
					}
				}
				for _, u := range transcript {
					status, body := getRaw(t, u, c.bearer)
					if status == http.StatusOK {
						require.Contains(t, body, marker, "GET %s as %s returns the transcript", u, c.name)
					}
				}
				// Search never indexes a room transcript or an unpublished outcome.
				// The response echoes the query in meta, so only the results are checked.
				status, body := getRaw(t, base+"/v1/search?q="+url.QueryEscape(marker), c.bearer)
				require.Equal(t, http.StatusOK, status, "search as %s: %s", c.name, body)
				var found struct {
					Data json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal([]byte(body), &found))
				require.NotContains(t, string(found.Data), marker, "search as %s", c.name)
			}

			// Control: the search probe does find the outcome once the server lets it out (the
			// author publishes a public-room outcome; only the owner approves a private one).
			if private {
				status, out := doJSON(t, "POST", r+"/posts/"+postID+"/publish", room.ownerJWT, "")
				require.Equal(t, http.StatusOK, status, "owner approves: %v", out)
			} else {
				status, out := doJSON(t, "PATCH", base+"/v1/posts/"+postID, room.executorKey, `{"status":"open"}`)
				require.Equal(t, http.StatusOK, status, "author publishes: %v", out)
			}
			_, body := getRaw(t, base+"/v1/search?q="+url.QueryEscape(marker), "")
			require.Contains(t, body, postID, "the published outcome is searchable, so the probe is live")

			// Writes: members and the owner post; anonymous, unrelated agent and (in a private
			// room) unrelated human are refused.
			for _, c := range callers {
				status, _, err := postEntryRaw(base, room.slug, c.bearer, map[string]any{"body": "write as " + c.name})
				require.NoError(t, err)
				mayWrite := c.member || (!private && c.name == "unrelated human")
				if mayWrite {
					require.Equal(t, http.StatusCreated, status, "write as %s", c.name)
				} else {
					require.True(t, isRefusal(status), "write as %s must be refused, got %d", c.name, status)
				}
			}

			// Owner-only actions: everyone else is refused and nothing changes.
			ownerOnly := []struct{ method, url, body string }{
				{"PATCH", r, `{"description":"hijacked"}`},
				{"GET", r + "/members", ""},
				{"POST", r + "/members", `{"agent_id":"` + otherTokenHolder(t, room) + `"}`},
				{"DELETE", r + "/members/" + room.executorID + "/token", ""},
				{"DELETE", r + "/members/" + room.executorID, ""},
				{"POST", r + "/archive", ""},
				{"DELETE", r, ""},
			}
			for _, c := range callers {
				if c.name == "owner" {
					continue
				}
				for _, op := range ownerOnly {
					status, out := doJSON(t, op.method, op.url, c.bearer, op.body)
					require.True(t, isRefusal(status), "%s %s as %s must be refused, got %d: %v", op.method, op.url, c.name, status, out)
				}
			}
			status, body = getRaw(t, r, room.ownerJWT)
			require.Equal(t, http.StatusOK, status, "the room survives every refused action")
			require.NotContains(t, body, "hijacked")
			status, _, err = postEntryRaw(base, room.slug, room.executorTok, map[string]any{"body": "still a member"})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, status, "the executor keeps its membership and token")
			status, out := doJSON(t, "GET", r+"/members", room.ownerJWT, "")
			require.Equal(t, http.StatusOK, status, "the owner manages members: %v", out)

			// Membership: the owner removes the executor; in a private room its key and token lose
			// every read, in a public room its token dies and its key reads like anyone's.
			status, out = doJSON(t, "DELETE", r+"/members/"+room.executorID, room.ownerJWT, "")
			require.Equal(t, http.StatusNoContent, status, "owner removes the executor: %v", out)
			removedToken := matrixCaller{name: "removed executor token", bearer: room.executorTok}
			requireRefusedEverywhere(t, allReads, removedToken, marker, "after removal")
			removedKey := matrixCaller{name: "removed executor key", bearer: room.executorKey}
			if private {
				requireRefusedEverywhere(t, allReads, removedKey, marker, "after removal")
			}
			status, _, err = postEntryRaw(base, room.slug, room.executorKey, map[string]any{"body": "after removal"})
			require.NoError(t, err)
			require.True(t, isRefusal(status), "a removed member cannot write, got %d", status)

			// Deletion: the owner deletes the room; every surface refuses everyone.
			status, out = doJSON(t, "DELETE", r, room.ownerJWT, "")
			require.Equal(t, http.StatusNoContent, status, "owner deletes: %v", out)
			for _, c := range callers {
				requireRefusedEverywhere(t, allReads, c, marker, "after deletion")
			}
			_, body = getRaw(t, base+"/v1/rooms", "")
			require.NotContains(t, body, room.slug, "a deleted room leaves the public list")
		})
	}
}

// A private room's saved outcome stays owner-only after the room is gone: deleting the room
// (or the reaper removing an expired one) must not let the author publish the draft by a
// plain edit, while a public room's outcome still publishes through the normal flow.
func TestRoomPermissionMatrix_PrivateOutcomeStaysOwnerOnlyAfterTheRoomIsGone(t *testing.T) {
	inst := startRoomInstance(t, RoomRelayOptions{SweepInterval: -1})
	base := inst.ts.URL
	t.Cleanup(func() {
		inst.pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id LIKE 'agent_roomtest_%'")
	})

	cases := []struct {
		name        string
		private     bool
		hardDelete  bool
		wantPublish int
	}{
		{"private room deleted by owner", true, false, http.StatusForbidden},
		{"private room removed by the reaper", true, true, http.StatusForbidden},
		{"public room deleted by owner", false, false, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			room := newAccessRoom(t, inst, tc.private)
			saved := saveAsPost(t, inst.ts, room.slug, room.executorKey, "Outcome of "+room.slug, outcomeSummary, "")
			require.Equal(t, http.StatusCreated, saved.StatusCode)
			postID, _ := dataObj(t, saved)["id"].(string)
			require.NotEmpty(t, postID)

			if tc.hardDelete {
				_, err := inst.pool.Exec(context.Background(), `DELETE FROM rooms WHERE slug = $1`, room.slug)
				require.NoError(t, err)
			} else {
				status, out := doJSON(t, "DELETE", base+"/v1/rooms/"+room.slug, room.ownerJWT, "")
				require.Equal(t, http.StatusNoContent, status, "owner deletes: %v", out)
			}

			status, out := doJSON(t, "PATCH", base+"/v1/posts/"+postID, room.executorKey, `{"status":"open"}`)
			require.Equal(t, tc.wantPublish, status, "author publishes by a plain edit: %v", out)
			status, out = doJSON(t, "GET", base+"/v1/posts/"+postID, room.executorKey, "")
			require.Equal(t, http.StatusOK, status, "%v", out)
			post, _ := out["data"].(map[string]any)
			want := "published"
			if tc.wantPublish == http.StatusForbidden {
				want = "draft"
			}
			require.Equal(t, want, post["publication_state"], "publication state after the edit")
		})
	}
}
