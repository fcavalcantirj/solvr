package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// Pinned directives on the canonical room API (idx 92 step 1): a room's owner, an admin or
// a member pins with an account credential (humans included, so the web page can pin), the
// pin change is announced on the live stream, and the room's latest_pinned follows the
// directive's revisions.

type pinFixture struct {
	ts      *httptest.Server
	pool    *db.Pool
	hubMgr  *hub.HubManager
	slug    string
	roomID  string
	owner   string
	token   string
	cleanup func()
}

func newPinFixture(t *testing.T) pinFixture {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	ts := httptest.NewServer(NewRouter(pool, hubMgr, registry))

	_, ownerJWT := createRoomTestUser(t, pool)
	slug := sourceTestSlug("pin")
	code, env := createSourceTestRoom(t, ts, ownerJWT, fmt.Sprintf(`{"display_name":"Pins","slug":%q}`, slug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	f := pinFixture{ts: ts, pool: pool, hubMgr: hubMgr, slug: slug, owner: ownerJWT,
		roomID: env["data"].(map[string]interface{})["id"].(string)}
	f.token = admitRoomAgent(t, ts, slug, "")
	f.cleanup = func() {
		ts.Close()
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-%')")
		pool.Exec(ctx, "DELETE FROM rooms WHERE slug LIKE 'test-src-pin-%'")
		pool.Close()
	}
	return f
}

func (f pinFixture) post(t *testing.T, body string) int64 {
	t.Helper()
	resp := doRoomRequest(t, "POST", f.ts.URL+"/v1/rooms/"+f.slug+"/entries", body, f.token)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", raw)
	var env struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	return env.Data.ID
}

func (f pinFixture) latestPinnedID(t *testing.T) interface{} {
	t.Helper()
	resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/rooms/"+f.slug, "", "")
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var env map[string]map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &env), "%s", raw)
	lp, _ := env["data"]["latest_pinned"].(map[string]interface{})
	if lp == nil {
		return nil
	}
	return int64(lp["id"].(float64))
}

func TestCanonicalPin_TheOwnerPinsAndTheDirectiveFollowsItsRevisions(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()

	v1 := f.post(t, `{"body":"directive v1","client_entry_id":"pin-1"}`)
	resp := doRoomRequest(t, "POST", fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, v1), "", f.owner)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
	require.Contains(t, string(raw), `"pinned_at"`)
	require.Equal(t, v1, f.latestPinnedID(t))

	v2 := f.post(t, fmt.Sprintf(`{"body":"directive v2","client_entry_id":"pin-2","supersedes_entry_id":%d}`, v1))
	require.Equal(t, v2, f.latestPinnedID(t), "the revision is the directive in force")

	// Re-pinning answers with the directive now in force — the revision, not the pinned v1 —
	// so a page updates its context without another read.
	resp = doRoomRequest(t, "POST", fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, v1), "", f.owner)
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	var pinned struct {
		Meta struct {
			LatestPinned struct {
				ID int64 `json:"id"`
			} `json:"latest_pinned"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(raw, &pinned), "%s", raw)
	require.Equal(t, v2, pinned.Meta.LatestPinned.ID)

	resp = doRoomRequest(t, "DELETE", fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, v1), "", f.owner)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Nil(t, f.latestPinnedID(t))
}

func TestCanonicalPin_AnOutsiderOrAnonymousCannotPinAPublicRoom(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	v1 := f.post(t, `{"body":"directive","client_entry_id":"pin-o"}`)
	_, outsider := createRoomTestUser(t, f.pool)

	url := fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, v1)
	resp := doRoomRequest(t, "POST", url, "", outsider)
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "writing to a public room is not membership")
	resp = doRoomRequest(t, "POST", url, "", "")
	resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp = doRoomRequest(t, "POST", fmt.Sprintf("%s/v1/rooms/%s/entries/999999999/pin", f.ts.URL, f.slug), "", f.owner)
	resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Nil(t, f.latestPinnedID(t))
}

func TestCanonicalPin_APinChangeIsAnnouncedOnTheLiveStream(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	v1 := f.post(t, `{"body":"directive","client_entry_id":"pin-s"}`)

	roomHub := f.hubMgr.GetOrCreate(context.Background(), hub.NewRoomID(mustUUID(t, f.roomID)))
	ch, err := roomHub.SubscribeStream("pin-watcher")
	require.NoError(t, err)

	resp := doRoomRequest(t, "POST", fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, v1), "", f.owner)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	deadline := time.After(5 * time.Second)
	for {
		select {
		case evt := <-ch:
			if evt.Type == hub.EventRoomUpdate {
				payload, _ := json.Marshal(evt.Payload)
				require.Contains(t, string(payload), `"pinned_entry_id":`)
				require.Contains(t, string(payload), fmt.Sprintf(`"latest_pinned_id":%d`, v1))
				return
			}
		case <-deadline:
			t.Fatal("no room_update frame after a pin change")
		}
	}
}

func TestRoomViewer_SaysWhetherTheCallerMayPin(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	_, outsider := createRoomTestUser(t, f.pool)

	for name, tc := range map[string]struct {
		bearer string
		want   bool
	}{"owner": {f.owner, true}, "outsider": {outsider, false}, "anonymous": {"", false}, "room token": {f.token, true}} {
		resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/rooms/"+f.slug+"/viewer", "", tc.bearer)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", name, raw)
		var env struct {
			Data struct {
				CanPin bool `json:"can_pin"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env))
		require.Equal(t, tc.want, env.Data.CanPin, name)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

// A room-bound prompt carries the directive in force, so an agent that resumes follows the
// current instruction rather than the first message (idx 92 step 1: resume instructions).
func TestRoomConnect_ThePromptCarriesTheDirectiveInForce(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	f.post(t, `{"body":"Build the board","client_entry_id":"rc-1"}`)

	connect := func() (string, map[string]interface{}) {
		resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/rooms/"+f.slug+"/connect?role=reviewer", "", "")
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
		var env struct {
			Data map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env))
		return env.Data["prompt"].(string), env.Data
	}

	prompt, data := connect()
	require.NotContains(t, prompt, "CURRENT DIRECTIVE", "no pin, no directive section")
	require.Nil(t, data["current_directive"])

	d := f.post(t, `{"body":"Directive: ship the scoreboard first","client_entry_id":"rc-2"}`)
	resp := doRoomRequest(t, "POST", fmt.Sprintf("%s/v1/rooms/%s/entries/%d/pin", f.ts.URL, f.slug, d), "", f.owner)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	prompt, data = connect()
	idx := strings.Index(prompt, "CURRENT DIRECTIVE")
	require.GreaterOrEqual(t, idx, 0)
	section := prompt[idx:]
	require.Contains(t, section, "Directive: ship the scoreboard first")
	require.Contains(t, section, fmt.Sprintf("https://api.solvr.dev/v1/rooms/%s/entries/%d", f.slug, d))
	require.Less(t, idx, strings.Index(prompt, "WHEN THE WORK IS DONE"), "the directive comes before the completion step")
	cd := data["current_directive"].(map[string]interface{})
	require.Equal(t, float64(d), cd["id"])
}
