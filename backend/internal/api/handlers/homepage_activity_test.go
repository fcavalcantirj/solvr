package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The public room activity stream: what agents and humans are ACTUALLY doing.
//
// The API decides all of it — which entries are worth showing, how a burst from
// one room collapses into a single block, what each entry's action reads as,
// how far an excerpt is cut, where the entry links to, and whether there is
// fresh activity waiting. The browser renders the answer.
//
// The honesty rules pinned here:
//   - an action label is the POSTER'S OWN (typed event or explicit metadata);
//     with nothing stated it reads "Posted a message" and says it was not
//     stated, rather than guessing from the text;
//   - an identity is "verified" only when an authenticated token stamped it;
//   - nothing in the payload reads an outcome out of a message count;
//   - fresh activity is REPORTED, never applied: the stream a reader is
//     looking at does not move underneath them.

func feedMessage(room, author, content string, at time.Time) db.PublicRoomFeedEntry {
	seq := 6
	return db.PublicRoomFeedEntry{
		Kind: "message", EntryID: 99, SequenceNum: &seq,
		RoomSlug: room, RoomName: strings.ToUpper(room[:1]) + room[1:],
		AuthorType: "agent", AuthorName: author, Content: content,
		Metadata: []byte(`{}`), CreatedAt: at,
	}
}

func feedRows(room string, n int, base time.Time) []db.PublicRoomFeedEntry {
	rows := make([]db.PublicRoomFeedEntry, 0, n)
	for i := 0; i < n; i++ {
		e := feedMessage(room, "planner_one", "a short line", base.Add(-time.Duration(i)*time.Minute))
		e.EntryID = int64(i + 1)
		rows = append(rows, e)
	}
	return rows
}

func TestOverviewActivity_SixEntriesAndLoadMore(t *testing.T) {
	// The repo is asked for limit+1 rows so the API — not the browser —
	// decides whether there is more to load.
	activity := buildOverviewActivity(feedRows("room-alpha", 7, time.Now()), 6, 0, 0, time.Now())

	assert.Equal(t, 6, activity.EntryCount, "the homepage shows six recent entries")
	assert.Equal(t, 6, activity.Limit)
	assert.Equal(t, 0, activity.Offset)
	assert.True(t, activity.HasMore)
	assert.Equal(t, 6, activity.NextOffset)
	assert.Equal(t, "/v1/homepage/activity?offset=6&limit=6", activity.LoadMoreURL)
	assert.NotEmpty(t, activity.LoadMoreLabel)
	assert.NotEmpty(t, activity.Definition)
}

func TestOverviewActivity_NoMoreWhenTheRepoReturnsExactlyTheLimit(t *testing.T) {
	activity := buildOverviewActivity(feedRows("room-alpha", 6, time.Now()), 6, 12, 0, time.Now())

	assert.Equal(t, 6, activity.EntryCount)
	assert.False(t, activity.HasMore)
	assert.Equal(t, 0, activity.NextOffset)
	assert.Empty(t, activity.LoadMoreURL)
	assert.Equal(t, 12, activity.Offset)
}

func TestOverviewActivity_BurstsFromOneRoomCollapseIntoOneBlock(t *testing.T) {
	now := time.Now()
	rows := []db.PublicRoomFeedEntry{
		feedMessage("room-alpha", "planner_one", "one", now),
		feedMessage("room-alpha", "executor_one", "two", now.Add(-time.Minute)),
		feedMessage("room-alpha", "planner_one", "three", now.Add(-2*time.Minute)),
		feedMessage("room-beta", "planner_two", "elsewhere", now.Add(-3*time.Minute)),
		feedMessage("room-alpha", "planner_one", "back again", now.Add(-4*time.Minute)),
	}

	activity := buildOverviewActivity(rows, 10, 0, 0, now)

	require.Len(t, activity.Groups, 3, "consecutive entries from one room are one block")
	assert.Equal(t, "room-alpha", activity.Groups[0].RoomSlug)
	assert.Equal(t, 3, activity.Groups[0].EntryCount)
	assert.Equal(t, "3 updates", activity.Groups[0].CountLabel)
	assert.NotEmpty(t, activity.Groups[0].BurstNote, "a burst says it is a burst")
	assert.Equal(t, "/rooms/room-alpha", activity.Groups[0].RoomURL)
	assert.NotEmpty(t, activity.Groups[0].TimeLabel)

	assert.Equal(t, "room-beta", activity.Groups[1].RoomSlug)
	assert.Equal(t, 1, activity.Groups[1].EntryCount)
	assert.Equal(t, "1 update", activity.Groups[1].CountLabel)
	assert.Empty(t, activity.Groups[1].BurstNote, "a single entry is not a burst")

	assert.Equal(t, "room-alpha", activity.Groups[2].RoomSlug,
		"a later return to the same room is its own block, not merged out of order")
	assert.Equal(t, 5, activity.EntryCount)
}

func TestOverviewActivity_AnUnlabelledMessageStaysNeutral(t *testing.T) {
	activity := buildOverviewActivity(
		[]db.PublicRoomFeedEntry{feedMessage("room-alpha", "planner_one", "just talking", time.Now())},
		6, 0, 0, time.Now())

	item := activity.Groups[0].Items[0]
	assert.Equal(t, "Posted a message", item.Action)
	assert.False(t, item.ActionStated, "nothing was stated, so nothing is claimed")
	assert.Equal(t, "just talking", item.Excerpt)
	assert.Equal(t, "message", item.Kind)
}

func TestOverviewActivity_ExplicitMetadataNamesTheAction(t *testing.T) {
	cases := []struct {
		metadata string
		action   string
		stated   bool
	}{
		{`{"type":"directive"}`, "Posted a directive", true},
		{`{"type":"plan"}`, "Shared a plan", true},
		{`{"kind":"evidence"}`, "Submitted evidence", true},
		{`{"action":"review"}`, "Left a review", true},
		{`{"type":"progress"}`, "Reported progress", true},
		{`{"type":"xyzzy"}`, "Posted a message", false},
		{`{}`, "Posted a message", false},
		{`not json at all`, "Posted a message", false},
	}

	for _, tc := range cases {
		row := feedMessage("room-alpha", "planner_one", "body", time.Now())
		row.Metadata = []byte(tc.metadata)

		activity := buildOverviewActivity([]db.PublicRoomFeedEntry{row}, 6, 0, 0, time.Now())
		item := activity.Groups[0].Items[0]

		assert.Equal(t, tc.action, item.Action, "metadata %s", tc.metadata)
		assert.Equal(t, tc.stated, item.ActionStated, "metadata %s", tc.metadata)
	}
}

func TestOverviewActivity_TypedEventReadsAsASummary(t *testing.T) {
	now := time.Now()
	rows := []db.PublicRoomFeedEntry{{
		Kind: "event", EntryID: 12, RoomSlug: "room-alpha", RoomName: "Room Alpha",
		AuthorType: "agent", AuthorName: "executor_one",
		EventType: "CLAIM", Issue: "board render", CreatedAt: now,
	}, {
		Kind: "event", EntryID: 13, RoomSlug: "room-alpha", RoomName: "Room Alpha",
		AuthorType: "agent", AuthorName: "executor_one",
		EventType: "BUILDING", CreatedAt: now.Add(-time.Minute),
	}}

	activity := buildOverviewActivity(rows, 6, 0, 0, now)
	items := activity.Groups[0].Items

	assert.Equal(t, "event", items[0].Kind)
	assert.Equal(t, "Claimed board render", items[0].Action)
	assert.True(t, items[0].ActionStated)
	assert.Empty(t, items[0].Excerpt, "an event is a summary, not a message body")
	assert.Equal(t, "/rooms/room-alpha", items[0].LinkURL)
	assert.Equal(t, "Open the room", items[0].LinkLabel,
		"a typed event has no message anchor to open")

	assert.Equal(t, "Started building", items[1].Action, "no issue, no invented subject")
}

func TestOverviewActivity_HumanAndAgentAreLabelledAndUnverifiedSaysSo(t *testing.T) {
	now := time.Now()
	human := feedMessage("room-alpha", "felipe", "a person wrote this", now)
	human.AuthorType = "human"
	human.AuthorVerified = true

	claimed := feedMessage("room-alpha", "executor_one", "an agent wrote this", now.Add(-time.Minute))
	claimed.AuthorVerified = false

	proven := feedMessage("room-alpha", "planner_one", "a proven agent wrote this", now.Add(-2*time.Minute))
	proven.AuthorVerified = true

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{human, claimed, proven}, 6, 0, 0, now)
	items := activity.Groups[0].Items

	assert.Equal(t, "Human", items[0].AuthorLabel)
	assert.Equal(t, "human", items[0].AuthorRole)
	assert.Empty(t, items[0].AuthorNote)

	assert.Equal(t, "Agent", items[1].AuthorLabel)
	assert.Equal(t, "agent", items[1].AuthorRole)
	assert.NotEmpty(t, items[1].AuthorNote, "a name nobody authenticated says so")
	assert.Contains(t, strings.ToLower(items[1].AuthorNote), "unverified")

	assert.Equal(t, "Agent", items[2].AuthorLabel)
	assert.Empty(t, items[2].AuthorNote)
}

func TestOverviewActivity_EntriesLinkToTheExactMessage(t *testing.T) {
	now := time.Now()
	anchored := feedMessage("tictactoe-human-vs-computer-20260920", "felipe", "short", now.Add(-3*time.Hour))
	anchored.AuthorType = "human"

	loose := feedMessage("room-alpha", "planner_one", "no anchor", now)
	loose.SequenceNum = nil

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{anchored, loose}, 6, 0, 0, now)

	first := activity.Groups[0].Items[0]
	assert.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920#message-6", first.LinkURL)
	assert.Equal(t, "Open the original message", first.LinkLabel)
	assert.Equal(t, "3 hours ago", first.TimeLabel, "the API words the relative time")
	assert.Equal(t, anchored.CreatedAt.UTC(), first.Timestamp, "and carries the machine timestamp too")

	second := activity.Groups[1].Items[0]
	assert.Equal(t, "/rooms/room-alpha", second.LinkURL, "no sequence_num, no anchor to promise")
	assert.Equal(t, "Open the room", second.LinkLabel)
}

func TestOverviewActivity_LongMessageIsExcerptedAndSaysSo(t *testing.T) {
	long := strings.Repeat("alpha beta ", 60) // ~660 chars
	row := feedMessage("room-alpha", "executor_one", long, time.Now())

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{row}, 6, 0, 0, time.Now())
	item := activity.Groups[0].Items[0]

	assert.True(t, item.IsExcerpt)
	assert.NotEmpty(t, item.ExcerptNote)
	assert.Less(t, len([]rune(item.Excerpt)), len([]rune(long)))
	assert.True(t, strings.HasSuffix(item.Excerpt, "…"), "excerpt: %q", item.Excerpt)
}

func TestOverviewActivity_CarriesACursorAndTheURLThatChecksForMore(t *testing.T) {
	now := time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC)
	newest := feedMessage("room-alpha", "planner_one", "newest", now.Add(-time.Minute))
	older := feedMessage("room-alpha", "planner_one", "older", now.Add(-time.Hour))

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{newest, older}, 6, 0, 0, now)

	assert.Equal(t, newest.CreatedAt.UTC().Format(time.RFC3339Nano), activity.Cursor)
	assert.Contains(t, activity.RefreshURL, "/v1/homepage/activity?since=")
	assert.Contains(t, activity.RefreshURL, "limit=6")
	assert.NotEmpty(t, activity.RefreshNote)
}

func TestOverviewActivity_AnEmptyStreamStillSaysWhereToLookNext(t *testing.T) {
	now := time.Date(2026, 9, 21, 10, 30, 0, 0, time.UTC)

	activity := buildOverviewActivity(nil, 6, 0, 0, now)

	assert.Empty(t, activity.Groups)
	assert.Equal(t, 0, activity.EntryCount)
	assert.NotEmpty(t, activity.EmptyNote)
	assert.Equal(t, now.Format(time.RFC3339Nano), activity.Cursor,
		"with nothing to point at, the cursor is the moment we looked")
	assert.False(t, activity.HasNew)
}

func TestOverviewActivity_FreshEntriesAreReportedNotApplied(t *testing.T) {
	now := time.Now()
	rows := feedRows("room-alpha", 3, now)

	quiet := buildOverviewActivity(rows, 6, 0, 0, now)
	assert.False(t, quiet.HasNew)
	assert.Empty(t, quiet.NewLabel)

	busy := buildOverviewActivity(rows, 6, 0, 3, now)
	assert.True(t, busy.HasNew)
	assert.Equal(t, 3, busy.NewCount)
	assert.Equal(t, "3 new updates", busy.NewLabel)
	assert.Equal(t, quiet.Groups[0].Items[0].ID, busy.Groups[0].Items[0].ID,
		"the reader's stream is unchanged: the API reports fresh activity, it does not apply it")

	one := buildOverviewActivity(rows, 6, 0, 1, now)
	assert.Equal(t, "1 new update", one.NewLabel)

	flood := buildOverviewActivity(rows, 6, 0, newActivityCountCap, now)
	assert.Contains(t, flood.NewLabel, "+", "a capped count never pretends to be exact: %q", flood.NewLabel)
}

func TestOverviewActivity_NeverReadsAnOutcomeOutOfMessageVolume(t *testing.T) {
	// A loud room is not a finished room. Nothing the API words may turn a
	// message count into a result.
	activity := buildOverviewActivity(feedRows("room-alpha", 24, time.Now()), 24, 0, 0, time.Now())

	payload, err := json.Marshal(activity)
	require.NoError(t, err)
	lowered := strings.ToLower(string(payload))

	for _, claim := range []string{"solved", "completed", "succeeded", "finished", "shipped"} {
		assert.NotContains(t, lowered, claim,
			"a stream of %d messages must not read as an outcome", activity.EntryCount)
	}
	assert.NotEmpty(t, activity.OutcomeNote, "and the page says so out loud")
}

func TestOverviewActivity_EntryIDsAreStableAcrossKinds(t *testing.T) {
	now := time.Now()
	msg := feedMessage("room-alpha", "planner_one", "body", now)
	msg.EntryID = 41
	evt := db.PublicRoomFeedEntry{
		Kind: "event", EntryID: 41, RoomSlug: "room-alpha", RoomName: "Room Alpha",
		AuthorType: "agent", AuthorName: "executor_one", EventType: "DONE", CreatedAt: now.Add(-time.Minute),
	}

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{msg, evt}, 6, 0, 0, now)
	items := activity.Groups[0].Items

	assert.Equal(t, "message-41", items[0].ID)
	assert.Equal(t, "event-41", items[1].ID, "a message and an event may share a number, never an identity")
}

func TestOverviewActivity_AnUnnamedHumanIsSomeoneNotAnIdentifier(t *testing.T) {
	now := time.Now()
	anonymous := feedMessage("room-alpha", "", "a departed account commented", now)
	anonymous.AuthorType = "human"
	anonymous.AuthorVerified = true

	activity := buildOverviewActivity([]db.PublicRoomFeedEntry{anonymous}, 6, 0, 0, now)
	item := activity.Groups[0].Items[0]

	assert.Equal(t, "Someone", item.Author)
	assert.Equal(t, "Human", item.AuthorLabel)
}
