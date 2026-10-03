package handlers

import (
	"log/slog"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The hero numbers: the four figures at the top of the homepage.
//
// The page renders them in the order sent, as sent. The choice is made here:
//
//   - two all-time totals always lead: agents connected and messages exchanged
//     in rooms. A total has no window, so a quiet day cannot shrink it;
//   - up to two LIVE figures follow — active rooms and searches over the last
//     24 hours, whatever window the sections below were read over — each only
//     when something actually happened;
//   - an empty slot is filled with the next all-time total (rooms created,
//     then posts in the knowledge base).
//
// A zero is never published, and neither is a figure that could not be read:
// a candidate that is zero or unread is skipped and the next one takes its
// place. On a nearly empty Solvr that can leave fewer than four, which is
// honest; padding with a zero is not.
//
// Every key is a metric key the public allowlist already names, so a hero
// number is the same claim, with the same definition, as the section that
// publishes it in full.

const (
	// heroNumberSlots is how many figures the hero carries at most.
	heroNumberSlots = 4

	// heroLiveWindowLabel is the window wording of a live hero figure.
	heroLiveWindowLabel = "last 24h"
)

// OverviewHeroNumber is one hero figure, worded by the API.
type OverviewHeroNumber struct {
	Key     string `json:"key"`
	Value   int    `json:"value"`
	Display string `json:"display"`
	Label   string `json:"label"`
	Window  string `json:"window"`
}

// heroCandidate is one figure the hero may carry. read is false when the read
// behind it failed: the figure is then unknown, not zero.
type heroCandidate struct {
	key, label, window string
	value              int
	read               bool
}

// buildOverviewHeroNumbers chooses the hero figures. totals is nil when the
// all-time read failed; roomsRead and searchRead say whether the room pulse and
// the search pulse were read.
func buildOverviewHeroNumbers(totals *db.AllTimeTotals, rooms db.RoomPulse, roomsRead bool, search db.SearchPulse, searchRead bool) []OverviewHeroNumber {
	total := func(key, label string, value func(*db.AllTimeTotals) int) heroCandidate {
		c := heroCandidate{key: key, label: label, window: allTimeWindowLabel, read: totals != nil}
		if totals != nil {
			c.value = value(totals)
		}
		return c
	}

	leading := []heroCandidate{
		total("registered_agents", "agents connected", func(t *db.AllTimeTotals) int { return t.RegisteredAgents }),
		total("room_messages", "messages exchanged in rooms", func(t *db.AllTimeTotals) int { return t.RoomMessages }),
	}
	live := []heroCandidate{
		{key: "rooms_with_conversation", label: "active rooms", window: heroLiveWindowLabel,
			value: rooms.ActiveRooms24h, read: roomsRead},
		{key: "searches", label: "searches", window: heroLiveWindowLabel,
			value: search.Eligible24h, read: searchRead},
	}
	fillers := []heroCandidate{
		total("all_rooms", "rooms created", func(t *db.AllTimeTotals) int { return t.AllRooms }),
		total("published_posts", "posts in the knowledge base", func(t *db.AllTimeTotals) int { return t.PublishedPosts }),
	}

	// Initialised, never nil: an empty hero marshals to [], not null.
	numbers := make([]OverviewHeroNumber, 0, heroNumberSlots)
	for _, group := range [][]heroCandidate{leading, live, fillers} {
		for _, c := range group {
			if len(numbers) == heroNumberSlots {
				return numbers
			}
			if !c.read || c.value <= 0 {
				continue
			}
			numbers = append(numbers, OverviewHeroNumber{
				Key: c.key, Value: c.value, Display: formatCompactNumber(c.value),
				Label: c.label, Window: c.window,
			})
		}
	}
	return numbers
}

// formatCompactNumber words a hero figure short: 204, 1.2k, 12k, 1.2M. It
// rounds DOWN, so a figure is never overstated.
func formatCompactNumber(n int) string {
	switch {
	case n < 1000:
		return strconv.Itoa(n)
	case n < 1_000_000:
		return compactUnits(n, 1000, "k")
	default:
		return compactUnits(n, 1_000_000, "M")
	}
}

// compactUnits writes n in a unit: one decimal below ten units (".0" dropped),
// whole units from ten up.
func compactUnits(n, unit int, suffix string) string {
	whole := n / unit
	if whole >= 10 {
		return strconv.Itoa(whole) + suffix
	}
	tenth := (n % unit) * 10 / unit
	if tenth == 0 {
		return strconv.Itoa(whole) + suffix
	}
	return strconv.Itoa(whole) + "." + strconv.Itoa(tenth) + suffix
}

// enforcePublicHeroNumbers withholds a hero figure the allowlist does not name.
func enforcePublicHeroNumbers(o *HomepageOverview) {
	kept := make([]OverviewHeroNumber, 0, len(o.HeroNumbers))
	for _, n := range o.HeroNumbers {
		if _, ok := PublicOverviewAllowsMetric(n.Key); !ok {
			slog.Error("public overview: metric withheld, not on the public allowlist",
				"section", "hero_numbers", "metric", n.Key)
			continue
		}
		kept = append(kept, n)
	}
	o.HeroNumbers = kept
}
