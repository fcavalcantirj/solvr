package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The hero numbers: four labeled figures at the top of the homepage, chosen
// here. Two all-time totals always lead; up to two live last-24h figures follow,
// each only when something actually happened; an empty slot is filled with the
// next all-time total. A zero is never published, and neither is a number that
// could not be read.

func heroTotals() *db.AllTimeTotals {
	return &db.AllTimeTotals{
		AllRooms: 60, PublicRooms: 27, PublishedPosts: 467,
		RegisteredAgents: 204, RegisteredHumans: 353, RoomMessages: 1234,
	}
}

func heroKeys(numbers []OverviewHeroNumber) []string {
	keys := make([]string, 0, len(numbers))
	for _, n := range numbers {
		keys = append(keys, n.Key)
	}
	return keys
}

func TestHeroNumbers_QuietDayShowsFourAllTimeTotals(t *testing.T) {
	numbers := buildOverviewHeroNumbers(heroTotals(),
		db.RoomPulse{ActiveRooms24h: 0}, true,
		db.SearchPulse{Eligible24h: 0}, true)

	assert.Equal(t, []string{"registered_agents", "room_messages", "all_rooms", "published_posts"}, heroKeys(numbers))
	for _, n := range numbers {
		assert.Equal(t, "all time", n.Window, "%s", n.Key)
		assert.Positive(t, n.Value, "%s is never a zero", n.Key)
		assert.NotEmpty(t, n.Label, "%s", n.Key)
		assert.NotEmpty(t, n.Display, "%s", n.Key)
	}
	assert.Equal(t, "agents connected", numbers[0].Label)
	assert.Equal(t, "204", numbers[0].Display)
	assert.Equal(t, "messages exchanged in rooms", numbers[1].Label)
	assert.Equal(t, "1.2k", numbers[1].Display)
	assert.Equal(t, "rooms created", numbers[2].Label)
	assert.Equal(t, "posts in the knowledge base", numbers[3].Label)
}

func TestHeroNumbers_BusyDayShowsTwoAllTimeThenTwoLive(t *testing.T) {
	numbers := buildOverviewHeroNumbers(heroTotals(),
		db.RoomPulse{ActiveRooms24h: 4}, true,
		db.SearchPulse{Eligible24h: 91}, true)

	assert.Equal(t, []string{"registered_agents", "room_messages", "rooms_with_conversation", "searches"}, heroKeys(numbers))
	assert.Equal(t, "all time", numbers[0].Window)
	assert.Equal(t, "all time", numbers[1].Window)

	assert.Equal(t, 4, numbers[2].Value)
	assert.Equal(t, "active rooms", numbers[2].Label)
	assert.Equal(t, "last 24h", numbers[2].Window)
	assert.Equal(t, 91, numbers[3].Value)
	assert.Equal(t, "91", numbers[3].Display)
	assert.Equal(t, "searches", numbers[3].Label)
	assert.Equal(t, "last 24h", numbers[3].Window)
}

func TestHeroNumbers_PartlyBusyDayFillsTheEmptyLiveSlotWithATotal(t *testing.T) {
	onlySearches := buildOverviewHeroNumbers(heroTotals(),
		db.RoomPulse{ActiveRooms24h: 0}, true,
		db.SearchPulse{Eligible24h: 12}, true)
	assert.Equal(t, []string{"registered_agents", "room_messages", "searches", "all_rooms"}, heroKeys(onlySearches))

	onlyRooms := buildOverviewHeroNumbers(heroTotals(),
		db.RoomPulse{ActiveRooms24h: 3}, true,
		db.SearchPulse{Eligible24h: 0}, true)
	assert.Equal(t, []string{"registered_agents", "room_messages", "rooms_with_conversation", "all_rooms"}, heroKeys(onlyRooms))
}

func TestHeroNumbers_OrderIsDeterministic(t *testing.T) {
	first := buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{ActiveRooms24h: 2}, true, db.SearchPulse{Eligible24h: 5}, true)
	for i := 0; i < 20; i++ {
		again := buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{ActiveRooms24h: 2}, true, db.SearchPulse{Eligible24h: 5}, true)
		require.Equal(t, first, again, "run %d", i)
	}
}

// A live figure whose read failed is not "nothing happened" and not a zero: it
// is left out. An all-time total that is zero or unread is left out too, and the
// next candidate takes its place.
func TestHeroNumbers_NeverPublishesAZeroOrAnUnreadValue(t *testing.T) {
	t.Run("live reads failed", func(t *testing.T) {
		numbers := buildOverviewHeroNumbers(heroTotals(),
			db.RoomPulse{ActiveRooms24h: 9}, false,
			db.SearchPulse{Eligible24h: 9}, false)
		assert.Equal(t, []string{"registered_agents", "room_messages", "all_rooms", "published_posts"}, heroKeys(numbers))
	})

	t.Run("a zero total is skipped", func(t *testing.T) {
		totals := heroTotals()
		totals.RoomMessages = 0
		numbers := buildOverviewHeroNumbers(totals, db.RoomPulse{ActiveRooms24h: 1}, true, db.SearchPulse{}, true)
		assert.Equal(t, []string{"registered_agents", "rooms_with_conversation", "all_rooms", "published_posts"}, heroKeys(numbers))
	})

	t.Run("totals unread", func(t *testing.T) {
		numbers := buildOverviewHeroNumbers(nil, db.RoomPulse{ActiveRooms24h: 1}, true, db.SearchPulse{Eligible24h: 2}, true)
		assert.Equal(t, []string{"rooms_with_conversation", "searches"}, heroKeys(numbers),
			"fewer than four is honest; padding with an unread total is not")
	})

	t.Run("an empty Solvr", func(t *testing.T) {
		numbers := buildOverviewHeroNumbers(&db.AllTimeTotals{}, db.RoomPulse{}, true, db.SearchPulse{}, true)
		assert.Empty(t, numbers)
		assert.NotNil(t, numbers, "an empty list, never null")
	})

	for _, n := range buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{}, true, db.SearchPulse{}, true) {
		assert.Positive(t, n.Value, "%s", n.Key)
	}
	assert.Len(t, buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{ActiveRooms24h: 1}, true, db.SearchPulse{Eligible24h: 1}, true), 4,
		"never more than four")
}

// The compact display rounds DOWN, so a figure is never overstated.
func TestFormatCompactNumber_RoundsDown(t *testing.T) {
	cases := map[int]string{
		1: "1", 204: "204", 999: "999",
		1000: "1k", 1049: "1k", 1234: "1.2k", 1999: "1.9k", 9999: "9.9k",
		10000: "10k", 12345: "12k", 999999: "999k",
		1000000: "1M", 1250000: "1.2M", 12500000: "12M",
	}
	for n, want := range cases {
		assert.Equal(t, want, formatCompactNumber(n), "%d", n)
	}
}

// Every hero number is a number the public allowlist names; anything else is
// withheld on the way out, like every other section.
func TestHeroNumbers_AreAllowlistedAndAnUnlistedKeyIsWithheld(t *testing.T) {
	for _, n := range buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{ActiveRooms24h: 1}, true, db.SearchPulse{Eligible24h: 1}, true) {
		_, ok := PublicOverviewAllowsMetric(n.Key)
		assert.True(t, ok, "hero key %q must be on the public allowlist", n.Key)
	}
	for _, n := range buildOverviewHeroNumbers(heroTotals(), db.RoomPulse{}, true, db.SearchPulse{}, true) {
		_, ok := PublicOverviewAllowsMetric(n.Key)
		assert.True(t, ok, "hero key %q must be on the public allowlist", n.Key)
	}

	overview := buildPublicOverviewFixture()
	overview.HeroNumbers = append(overview.HeroNumbers, OverviewHeroNumber{
		Key: "daily_visitors", Value: 10, Display: "10", Label: "visitors", Window: "last 24h",
	})
	enforcePublicOverviewMetrics(&overview)
	assert.NotContains(t, heroKeys(overview.HeroNumbers), "daily_visitors")
	assert.NotEmpty(t, overview.HeroNumbers, "the allowlisted hero numbers stay")
}

// The meta envelope no longer carries a clock-time label: "Updated 7:59 PM" was
// server UTC read as local time. The stale state stays.
func TestOverviewMeta_CarriesNoLastUpdatedLabel(t *testing.T) {
	for _, partialErrors := range [][]string{nil, {"search statistics unavailable: simulated"}} {
		raw, err := json.Marshal(buildOverviewMeta(db.DefaultRoomStatsWindow(), partialErrors))
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		assert.NotContains(t, fields, "last_updated_label")
		assert.Contains(t, fields, "stale")
		assert.Contains(t, fields, "partial_errors")
	}
}

// The all-time section carries the room-message total the hero leads with.
func TestOverviewAllTimeSection_CountsMessagesInRooms(t *testing.T) {
	section := buildOverviewCommunity(heroTotals(), &db.AllStatsResult{})
	var found *OverviewMetric
	for i := range section.Metrics {
		if section.Metrics[i].Key == "room_messages" {
			found = &section.Metrics[i]
		}
	}
	require.NotNil(t, found, "the all-time section must carry room_messages")
	assert.Equal(t, 1234, found.Value)
	assert.Equal(t, "1,234", found.Display)
	assert.Equal(t, allTimeWindowLabel, found.Window)
	assert.NotEmpty(t, found.Definition)

	unread := buildOverviewCommunity(nil, nil)
	for _, m := range unread.Metrics {
		if m.Key == "room_messages" {
			assert.True(t, m.Unavailable)
			assert.Equal(t, "—", m.Display)
		}
	}
}
