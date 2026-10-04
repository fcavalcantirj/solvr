package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /v1/homepage/activity end to end, against the real router and a real
// database, called the way a logged-out browser calls it: no credentials at
// all.
//
// What is guarded here is that the stream shows WORK and never TRANSPORT, that
// an entry stops being served the moment it stops being eligible, that a human
// comment never publishes the account id the message row carries, and that
// fresh activity is reported rather than inserted under the reader.

// hpaGetActivity calls the stream with no credentials, optionally with a
// cursor, and returns the decoded payload plus the raw body.
func hpaGetActivity(t *testing.T, baseURL string, offset, limit int, since string) (hpoActivity, string) {
	t.Helper()

	target := fmt.Sprintf("%s/v1/homepage/activity?offset=%d&limit=%d", baseURL, offset, limit)
	if since != "" {
		target += "&since=" + url.QueryEscape(since)
	}

	resp, err := http.Get(target)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hpoActivity `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// hpaEvent appends a typed room event, the way an agent does on the A2A
// namespace.
func hpaEvent(t *testing.T, pool *db.Pool, roomID uuid.UUID, eventType, issue, actor string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO room_events (room_id, event_type, issue, actor)
		VALUES ($1, $2, $3, $4)
	`, roomID, eventType, issue, actor)
	require.NoError(t, err)
}

// hpaCleanupEvents removes the typed events this file writes. The shared
// hpoCleanup does not touch room_events.
func hpaCleanupEvents(t *testing.T, pool *db.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`DELETE FROM room_events WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-hpo-%')`)
	if err != nil {
		t.Logf("hpaCleanupEvents: %v", err)
	}
}

func TestHomepageActivity_ShowsWorkAndHidesTransport(t *testing.T) {
	slug := hpoSlug("work")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)
	defer hpaCleanupEvents(t, pool)

	room := hpoSeedRoom(t, pool, slug, "Work Room", "what agents are doing", false,
		[]string{"planner writes the brief"})

	// Work an agent announced...
	hpaEvent(t, pool, room.ID, "CLAIM", "render the board", "hpo_executor")
	// ...and everything that is plumbing.
	hpaEvent(t, pool, room.ID, "HEARTBEAT", "", "hpo_executor")
	hpaEvent(t, pool, room.ID, "TOKEN_ISSUED", "", "system")
	hpaEvent(t, pool, room.ID, db.RoomActivationEventType, "", "system")

	activity, raw := hpaGetActivity(t, ts.URL, 0, 24, "")

	var mine []hpoActivityItem
	for _, item := range activity.entries() {
		if item.RoomSlug == slug {
			mine = append(mine, item)
		}
	}
	require.Len(t, mine, 2, "one message and one work event, nothing else: %+v", mine)

	assert.Equal(t, "event", mine[0].Kind)
	assert.Equal(t, "Claimed render the board", mine[0].Action)
	assert.True(t, mine[0].ActionStated)
	assert.Equal(t, "/rooms/"+slug, mine[0].LinkURL)
	assert.Equal(t, "Open the room", mine[0].LinkLabel)

	assert.Equal(t, "message", mine[1].Kind)
	assert.Equal(t, "Posted a message", mine[1].Action)
	assert.False(t, mine[1].ActionStated, "nothing was stated, so nothing is claimed")
	assert.Equal(t, "planner writes the brief", mine[1].Excerpt)
	assert.Contains(t, mine[1].LinkURL, "#message-")
	assert.Equal(t, "Open the original message", mine[1].LinkLabel)

	assert.NotContains(t, raw, "HEARTBEAT")
	assert.NotContains(t, raw, "TOKEN_ISSUED")
	assert.NotContains(t, raw, db.RoomActivationEventType)
	assert.NotEmpty(t, activity.OutcomeNote)
}

func TestHomepageActivity_GroupsABurstFromOneRoom(t *testing.T) {
	slug := hpoSlug("burst")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, slug, "Burst Room", "a busy room", false,
		[]string{"one", "two", "three", "four"})

	activity, _ := hpaGetActivity(t, ts.URL, 0, 4, "")

	require.NotEmpty(t, activity.Groups)
	group := activity.Groups[0]
	require.Equal(t, slug, group.RoomSlug, "the newest room leads the stream")
	assert.Equal(t, 4, group.EntryCount, "four messages in a row are one block")
	assert.Equal(t, "4 updates", group.CountLabel)
	assert.NotEmpty(t, group.BurstNote)
	assert.Equal(t, "/rooms/"+slug, group.RoomURL)
	assert.Len(t, group.Items, 4)
}

func TestHomepageActivity_ReportsFreshEntriesWithoutMovingTheStream(t *testing.T) {
	slug := hpoSlug("fresh")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	room := hpoSeedRoom(t, pool, slug, "Fresh Room", "a room somebody is reading", false,
		[]string{"the reader is looking at this"})

	reading, _ := hpaGetActivity(t, ts.URL, 0, 6, "")
	require.NotEmpty(t, reading.Cursor)
	assert.Contains(t, reading.RefreshURL, "/v1/homepage/activity?since=")
	assert.False(t, reading.HasNew, "nothing has arrived yet")

	// Something happens while they read.
	time.Sleep(10 * time.Millisecond)
	hpaEvent(t, pool, room.ID, "BUILDING", "", "hpo_executor")
	defer hpaCleanupEvents(t, pool)

	checked, _ := hpaGetActivity(t, ts.URL, 0, 6, reading.Cursor)

	assert.True(t, checked.HasNew, "the page is told there is something new")
	assert.GreaterOrEqual(t, checked.NewCount, 1)
	assert.NotEmpty(t, checked.NewLabel)

	// The entry the reader already had is still exactly where it was: the API
	// reports fresh activity, the browser decides when to take it.
	before := reading.entries()
	require.NotEmpty(t, before)
	assert.Equal(t, before[0].ID, reading.entries()[0].ID)
	stale, _ := hpaGetActivity(t, ts.URL, 0, 6, "")
	assert.Equal(t, "Started building", stale.entries()[0].Action,
		"and the same read without a cursor simply carries the new entry")
}

func TestHomepageActivity_IneligibleEntriesLeaveOnTheNextRequest(t *testing.T) {
	slug := hpoSlug("elig")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	ctx := context.Background()
	room := hpoSeedRoom(t, pool, slug, "Eligibility Room", "moderation fixture", false,
		[]string{"this one stays", "this one is removed"})

	_, raw := hpaGetActivity(t, ts.URL, 0, 24, "")
	require.Contains(t, raw, "this one is removed")

	_, err := pool.Exec(ctx,
		`UPDATE messages SET deleted_at = NOW() WHERE room_id = $1 AND content = 'this one is removed'`, room.ID)
	require.NoError(t, err)

	_, afterDelete := hpaGetActivity(t, ts.URL, 0, 24, "")
	assert.NotContains(t, afterDelete, "this one is removed", "a removed message is gone from the next response")
	assert.Contains(t, afterDelete, "this one stays")

	_, err = pool.Exec(ctx, `UPDATE rooms SET is_private = TRUE WHERE id = $1`, room.ID)
	require.NoError(t, err)

	_, afterPrivate := hpaGetActivity(t, ts.URL, 0, 24, "")
	assert.NotContains(t, afterPrivate, "this one stays", "a room taken private takes its excerpts with it")
	assert.NotContains(t, afterPrivate, slug, "and no trace of the room survives")
}

func TestHomepageActivity_HumanCommentsNeverPublishAnAccountID(t *testing.T) {
	slug := hpoSlug("human")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	ctx := context.Background()
	room := hpoSeedRoom(t, pool, slug, "Human Room", "a person joins in", false,
		[]string{"an agent wrote this"})

	username := fmt.Sprintf("hpo%d", time.Now().UnixNano()%100000000)
	var userID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, referral_code)
		VALUES ($1, $1, $2, substr(md5(random()::text), 1, 8))
		RETURNING id
	`, username, username+"@example.test").Scan(&userID)
	require.NoError(t, err)
	defer func() {
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	}()

	// Exactly what POST /v1/rooms/{slug}/messages writes for a human comment.
	_, err = pool.Exec(ctx, `
		INSERT INTO messages (room_id, author_type, author_id, agent_name, content, sequence_num)
		VALUES ($1, 'human', $2, $3, 'a person wrote this', 99)
	`, room.ID, userID.String(), "human:"+userID.String())
	require.NoError(t, err)

	activity, raw := hpaGetActivity(t, ts.URL, 0, 24, "")

	assert.NotContains(t, raw, "human:", "the internal author name never reaches the page")
	assert.NotContains(t, raw, userID.String(), "and neither does the account id")

	var human *hpoActivityItem
	for i, item := range activity.entries() {
		if item.RoomSlug == slug && item.AuthorRole == "human" {
			human = &activity.entries()[i]
			break
		}
	}
	require.NotNil(t, human, "the human comment is in the stream")
	assert.Equal(t, username, human.Author, "shown under the public username")
	assert.Equal(t, "Human", human.AuthorLabel)
	assert.Empty(t, human.AuthorNote, "an authenticated account is not flagged unverified")
}

func TestHomepageActivity_BoundsWhatACallerMayAskFor(t *testing.T) {
	slug := hpoSlug("bounds")

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, slug, "Bounds Room", "limits fixture", false, []string{"only message"})

	// An unreasonable page size is clamped, not honoured and not rejected.
	huge, _ := hpaGetActivity(t, ts.URL, 0, 9999, "")
	assert.Equal(t, 24, huge.Limit, "the API caps the page size it will serve")

	// A nonsense page size falls back to the default.
	nonsense, _ := hpaGetActivity(t, ts.URL, -3, 0, "")
	assert.Equal(t, 6, nonsense.Limit)
	assert.Equal(t, 0, nonsense.Offset)

	// A bookmark the API cannot parse is treated as "no cursor" rather than as
	// an error: a stale tab must never break the stream.
	broken, raw := hpaGetActivity(t, ts.URL, 0, 6, "not-a-timestamp")
	assert.False(t, broken.HasNew, "body: %s", raw)
	assert.NotEmpty(t, broken.Cursor)
}
