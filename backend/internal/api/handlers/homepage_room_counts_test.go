package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// Every room is counted; only public rooms are ever shown (spec.json idx 96).
//
// A room's existence and its volume are platform facts, so the homepage's
// counts cover every non-deleted room, private ones included. Its name, slug,
// description, participants, excerpts and entries are not facts about the
// platform, so every list that names or quotes a room stays public-only. These
// tests hold the wording to that: each count says it includes private rooms,
// the public figure is reported beside the all-room one where a reader could
// confuse them, and nothing on the page still promises the opposite.

func TestOverviewRooms_EveryCountSaysPrivateRoomsAreIncluded(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()), nil)

	counts := append(append([]OverviewMetric{}, section.PresenceMetrics...), section.Metrics...)
	require.Len(t, counts, 6)
	for _, m := range counts {
		assert.Contains(t, m.Definition, "private", "%s must say private rooms are counted", m.Key)
		assert.Contains(t, m.Definition, "included", "%s must say private rooms are counted", m.Key)
		assert.NotContains(t, strings.ToLower(m.Definition), "public",
			"%s counts every room, so its definition may not scope it to public rooms", m.Key)
	}

	require.NotNil(t, section.Sparkline)
	assert.Contains(t, section.Sparkline.Definition, "private rooms included")
	assert.NotContains(t, strings.ToLower(section.Sparkline.Definition), "public")
}

// The live marker counts every room, but the offline fallback it announces is a
// list of PUBLIC rooms, and it appears exactly when it did before: when no agent
// is online in a public room. An agent working in a private room must neither
// hide that list nor make the marker read as if a public room were live.
func TestBuildOverviewRooms_FallbackFollowsPublicPresenceWhileTheMarkerCountsEveryRoom(t *testing.T) {
	recent := []db.RecentCompletedRoom{{
		RoomID: uuid.New(), Slug: "recent-public", DisplayName: "Recent Public",
		MessageCount: 3, LastActiveAt: time.Now().Add(-time.Hour),
	}}

	t.Run("online only in private rooms", func(t *testing.T) {
		pulse := samplePulse(db.DefaultRoomStatsWindow())
		pulse.Presence.AgentsOnline, pulse.Presence.PublicAgentsOnline = 2, 0

		section := buildOverviewRooms(pulse, recent)

		require.NotNil(t, section.LiveMarker)
		assert.True(t, section.LiveMarker.Online, "two agents are online")
		assert.Equal(t, "2 agents online now, all in private rooms. Recent public rooms below.",
			section.LiveMarker.Label)
		require.Len(t, section.RecentCompletedRooms, 1,
			"no agent is in a public room, so the public fallback is shown as before")
		assert.Equal(t, "recent-public", section.RecentCompletedRooms[0].Slug)
	})

	t.Run("online only in private rooms, nothing recent", func(t *testing.T) {
		pulse := samplePulse(db.DefaultRoomStatsWindow())
		pulse.Presence.AgentsOnline, pulse.Presence.PublicAgentsOnline = 2, 0

		section := buildOverviewRooms(pulse, nil)

		require.NotNil(t, section.LiveMarker)
		assert.Equal(t, "2 agents online now, all in private rooms.", section.LiveMarker.Label)
		assert.Nil(t, section.RecentCompletedRooms)
	})

	t.Run("some in public rooms", func(t *testing.T) {
		pulse := samplePulse(db.DefaultRoomStatsWindow())
		pulse.Presence.AgentsOnline, pulse.Presence.PublicAgentsOnline = 3, 1

		section := buildOverviewRooms(pulse, recent)

		require.NotNil(t, section.LiveMarker)
		assert.True(t, section.LiveMarker.Online)
		assert.Equal(t, "3 agents online now, 1 of them in public rooms", section.LiveMarker.Label)
		assert.Nil(t, section.RecentCompletedRooms, "an agent is live in a public room")
	})

	t.Run("all in public rooms", func(t *testing.T) {
		pulse := samplePulse(db.DefaultRoomStatsWindow())
		pulse.Presence.AgentsOnline, pulse.Presence.PublicAgentsOnline = 3, 3

		section := buildOverviewRooms(pulse, nil)

		require.NotNil(t, section.LiveMarker)
		assert.Equal(t, "3 agents online now", section.LiveMarker.Label)
	})
}

// The all-time section reports the all-room total and the public figure side by
// side, so nobody reads a private-inclusive count as a count of readable rooms.
func TestOverviewAllTimeSection_ReportsEveryRoomBesideThePublicRooms(t *testing.T) {
	section := buildOverviewCommunity(
		&db.AllTimeTotals{AllRooms: 230, PublicRooms: 214, PublishedPosts: 2098, RegisteredAgents: 64, RegisteredHumans: 1003},
		&db.AllStatsResult{TotalContributions: 3327},
	)

	byKey := map[string]OverviewMetric{}
	order := make([]string, 0, len(section.Metrics))
	for _, m := range section.Metrics {
		byKey[m.Key] = m
		order = append(order, m.Key)
	}
	require.Contains(t, byKey, "all_rooms")
	require.Contains(t, byKey, "public_rooms")
	require.GreaterOrEqual(t, len(order), 2)
	assert.Equal(t, []string{"all_rooms", "public_rooms"}, order[:2], "the two room figures sit together")

	all := byKey["all_rooms"]
	assert.Equal(t, 230, all.Value)
	assert.Equal(t, "ROOMS", all.Label)
	assert.Contains(t, all.Definition, "private rooms included")
	assert.Contains(t, all.Definition, "never shown", "the definition says what a private room does NOT give away")

	public := byKey["public_rooms"]
	assert.Equal(t, 214, public.Value)
	assert.NotContains(t, strings.ToLower(public.Definition), "never counted")

	unread := buildOverviewCommunity(nil, nil)
	for _, m := range unread.Metrics {
		if m.Key == "all_rooms" {
			assert.True(t, m.Unavailable, "an unread total is unavailable, never zero")
			assert.Equal(t, unreadMetricDisplay, m.Display)
		}
	}
}

// Nothing the API writes on the page may still promise that private rooms are
// not counted: a note that contradicts the numbers beside it is worse than none.
func TestPublicOverview_NoCopyPromisesPrivateRoomsAreNotCounted(t *testing.T) {
	overview := buildPublicOverviewFixture()

	raw, err := json.Marshal(overview)
	require.NoError(t, err)
	body := strings.ToLower(string(raw))

	for _, stale := range []string{
		"private rooms are never counted",
		"never counted",
	} {
		assert.NotContains(t, body, stale, "the overview still carries %q", stale)
	}
	assert.Contains(t, body, "private rooms included")
	// The activity stream really is public-only, so it may still say so; the
	// rooms section's counts are not, so their label may not.
	assert.NotEqual(t, "Public room activity", overview.Rooms.ScopeLabel)
}
