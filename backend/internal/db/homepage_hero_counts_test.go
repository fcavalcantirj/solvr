package db_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The counts behind the homepage hero numbers (handlers/homepage_hero.go).
//
// The hero states two kinds of number, and each must mean exactly what its
// label says: an all-time total that no window can shrink, and a live figure
// that is ALWAYS the last 24 hours, whichever window a caller selected for the
// sections below it.

// Every message ever exchanged in a room counts, private rooms included (a
// count never names a room). System notices, deleted messages and the messages
// of a deleted room do not.
func TestGetAllTimeTotals_CountsRoomMessagesInEveryRoom(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)
	before, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)

	f := newRoomStatsFixture(t, ctx, pool)
	public := f.room("heromsgpub", false, nil, false)
	private := f.room("heromsgpriv", true, nil, false)
	gone := f.room("heromsggone", false, nil, true)

	f.message(public, "agent", "hero-agent"+f.suffix, nil, time.Minute, false)
	f.message(private, "human", "hero-human"+f.suffix, nil, 40*24*time.Hour, false)
	f.message(public, "system", "system", nil, time.Minute, false)
	f.message(public, "agent", "hero-agent"+f.suffix, nil, time.Minute, true)
	f.message(gone, "agent", "hero-agent"+f.suffix, nil, time.Minute, false)

	after, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, after.RoomMessages-before.RoomMessages,
		"the public agent message and the 40-day-old private one count; system, deleted and deleted-room messages do not")
}

// Active rooms in the hero are the last 24 hours whatever window was read, and
// on a 24-hour read they are exactly the rooms with conversation.
func TestGetRoomPulse_ActiveRooms24hIsPinnedToTheLastDay(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)
	day := mustWindow(t, "24h")
	month := mustWindow(t, "30d")

	beforeDay, err := repo.GetRoomPulse(ctx, day)
	require.NoError(t, err)
	beforeMonth, err := repo.GetRoomPulse(ctx, month)
	require.NoError(t, err)

	f := newRoomStatsFixture(t, ctx, pool)
	recent := f.room("heroactive", false, nil, false)
	f.message(recent, "agent", "hero-agent"+f.suffix, nil, time.Minute, false)
	privateRecent := f.room("heroactivepriv", true, nil, false)
	f.message(privateRecent, "agent", "hero-agent"+f.suffix, nil, time.Minute, false)
	old := f.room("heroactiveold", false, nil, false)
	f.message(old, "agent", "hero-agent"+f.suffix, nil, 72*time.Hour, false)

	afterDay, err := repo.GetRoomPulse(ctx, day)
	require.NoError(t, err)
	afterMonth, err := repo.GetRoomPulse(ctx, month)
	require.NoError(t, err)

	assert.Equal(t, 2, afterDay.ActiveRooms24h-beforeDay.ActiveRooms24h,
		"the two rooms with a message in the last day; the 3-day-old room is not active")
	assert.Equal(t, 2, afterMonth.ActiveRooms24h-beforeMonth.ActiveRooms24h,
		"a 30-day read must not widen the hero's last 24 hours")
	assert.Equal(t, 3, afterMonth.Stats.RoomsWithConversation-beforeMonth.Stats.RoomsWithConversation,
		"the 30-day section still counts the 3-day-old room")
	assert.Equal(t, afterDay.Stats.RoomsWithConversation, afterDay.ActiveRooms24h,
		"on a 24-hour read the hero and the room section count the same rooms")
}

// Searches in the hero are the last 24 hours whatever window was read, counted
// exactly like the search section's eligible searches: monitoring is excluded.
func TestSearchPulse_Eligible24hIsPinnedToTheLastDay(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true
	day := mustWindow(t, "24h")
	week := mustWindow(t, "7d")

	beforeDay, err := repo.GetSearchPulse(ctx, day, 5)
	require.NoError(t, err)
	beforeWeek, err := repo.GetSearchPulse(ctx, week, 5)
	require.NoError(t, err)

	f.record("hero pinned", "agent", 1, &yes, "", time.Minute)
	f.record("hero pinned", "anonymous", 1, &yes, "", 2*time.Minute)
	f.record("hero three days", "agent", 1, &yes, "", 72*time.Hour)
	f.record("hero ping", "anonymous", 1, &yes, "UptimeRobot/2.0; http://uptimerobot.com/", time.Minute)

	afterDay, err := repo.GetSearchPulse(ctx, day, 5)
	require.NoError(t, err)
	afterWeek, err := repo.GetSearchPulse(ctx, week, 5)
	require.NoError(t, err)

	assert.Equal(t, 2, afterDay.Eligible24h-beforeDay.Eligible24h,
		"two eligible searches in the last day; the monitor and the 3-day-old search are not")
	assert.Equal(t, 2, afterWeek.Eligible24h-beforeWeek.Eligible24h,
		"a 7-day read must not widen the hero's last 24 hours")
	assert.Equal(t, 3, afterWeek.Eligible-beforeWeek.Eligible,
		"the 7-day section still counts the 3-day-old search")
	assert.Equal(t, afterDay.Eligible, afterDay.Eligible24h,
		"on a 24-hour read the hero and the search section count the same searches")
}
