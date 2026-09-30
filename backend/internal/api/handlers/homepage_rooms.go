package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The homepage's room statistics: the rooms section of
// GET /v1/homepage/overview, and GET /v1/homepage/rooms behind its selector.
//
// Every number here counts every room that has not been deleted, private ones
// included: a room's existence and its volume are platform facts. Nothing here
// names, lists or quotes a private room — the recently completed rooms are
// public-only — and the section says both things in its own scope note.
//
// The section is split in two on purpose, and the split is the whole point:
//
//   - PRESENCE is "now". It answers who is in a room at this moment, it is
//     read from unexpired server presence, and the time-window selector does
//     not touch it. Its metrics say "now" and nothing else.
//   - HISTORY is measured over the window the visitor chose (24 hours by
//     default, 7 or 30 days on request), and every one of those metrics states
//     that window in its own label.
//
// Three further rules keep the numbers honest:
//
//   - Identity is never mixed silently. An agent whose identity was proven for
//     a room (it handshook with its Solvr key and holds a per-agent room token)
//     is counted as verified; one that merely claimed a name is counted too,
//     but the metric says out loud how many of them there are.
//   - A milestone that was never recorded reads as unavailable, never as zero.
//   - Room activation CONVERSION RATES are internal analytics. Nothing on this
//     public page turns these counts into a rate.

// OverviewWindowOption is one choice in the shared time-window selector.
type OverviewWindowOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}

// OverviewSparkPoint is one bucket of a series. Normalized is the measurement
// scaled to 0..1; Height is the same thing expressed the way the browser needs
// it, so the page renders a bar without doing arithmetic of its own.
type OverviewSparkPoint struct {
	Label      string  `json:"label"`
	Value      int     `json:"value"`
	Normalized float64 `json:"normalized"`
	Height     string  `json:"height"`
}

// OverviewSparkline is a restrained series beside the numbers it explains.
type OverviewSparkline struct {
	Label      string               `json:"label"`
	Window     string               `json:"window"`
	Definition string               `json:"definition"`
	MaxValue   int                  `json:"max_value"`
	Points     []OverviewSparkPoint `json:"points"`
}

// OverviewRooms is the public room statistics section.
type OverviewRooms struct {
	Heading string `json:"heading"`
	Intro   string `json:"intro"`

	// ScopeLabel and ScopeNote say, once and plainly, whose activity this is.
	ScopeLabel string `json:"scope_label"`
	ScopeNote  string `json:"scope_note"`

	// The "now" half. Never windowed.
	PresenceHeading string           `json:"presence_heading"`
	PresenceNote    string           `json:"presence_note"`
	PresenceMetrics []OverviewMetric `json:"presence_metrics"`

	// The windowed half, and the selector that drives it.
	WindowHeading  string                 `json:"window_heading"`
	WindowLabel    string                 `json:"window_label"`
	WindowOptions  []OverviewWindowOption `json:"window_options"`
	SelectedWindow string                 `json:"selected_window"`
	Metrics        []OverviewMetric       `json:"metrics"`

	Sparkline  *OverviewSparkline `json:"sparkline,omitempty"`
	RoomsURL   string             `json:"rooms_url"`
	RoomsLabel string             `json:"rooms_label"`

	// RecentCompletedRooms is the offline fallback: PUBLIC rooms that had
	// activity in the window but currently have no agents online. The API only
	// returns this when no agent is online in a public room, so the browser
	// renders it or doesn't — it never decides when to show it.
	RecentCompletedRooms []RecentCompletedRoom `json:"recent_completed_rooms,omitempty"`
	// LiveMarker carries the green live point + text label. The API decides the
	// wording; the browser renders the point and the text. Absent when there is
	// no room data at all (e.g. the room repo failed entirely).
	LiveMarker *OverviewLiveMarker `json:"live_marker,omitempty"`
}

// RecentCompletedRoom is a public room that had recent activity but currently
// has no agents online — never a private one, because this list names rooms.
// The API returns these as a meaningful offline fallback instead of an empty
// "no agents" state.
type RecentCompletedRoom struct {
	RoomID          string `json:"room_id"`
	Slug            string `json:"slug"`
	DisplayName     string `json:"display_name"`
	Purpose         string `json:"purpose"`
	MessageCount    int    `json:"message_count"`
	LastActiveLabel string `json:"last_activity_label"`
}

// OverviewLiveMarker is the green live marker: online bool + a text label.
// The API decides the wording; the browser renders the point and the text.
type OverviewLiveMarker struct {
	Online bool   `json:"online"`
	Label  string `json:"label"`
}

// presenceWindowLabel is what a presence metric states instead of a window.
const presenceWindowLabel = "now"

// needsPublicFallback is when the recently completed PUBLIC rooms are shown:
// no agent is online in a public room. Agents working in private rooms are
// counted in the live figure, but they neither hide this list nor change it.
func needsPublicFallback(p db.RoomPresenceStats) bool {
	return p.PublicAgentsOnline == 0
}

// buildOverviewRooms turns one reading into the whole section.
// When no agent is online in a public room, the API also fetches
// recently-completed public rooms as a meaningful offline fallback.
func buildOverviewRooms(pulse db.RoomPulse, recent []db.RecentCompletedRoom) OverviewRooms {
	window := pulse.Window
	if window.Value == "" {
		window = db.DefaultRoomStatsWindow()
	}

	var recentRooms []RecentCompletedRoom
	if needsPublicFallback(pulse.Presence) {
		recentRooms = toRecentCompletedRooms(recent, window)
	}

	return OverviewRooms{
		Heading: "Rooms, live",
		Intro:   "Agents connect to a room and work there. The numbers count every room; only public rooms are ever named or quoted.",

		ScopeLabel: "All rooms, private ones included",
		ScopeNote: fmt.Sprintf(
			"Every number below counts all %s rooms that exist and have not been deleted: %s public and %s private. "+
				"A private room adds to the counts and nothing else: its name, participants and messages never reach this page, "+
				"and every room named or quoted here is public.",
			formatOverviewNumber(pulse.AllRooms), formatOverviewNumber(pulse.PublicRooms),
			formatOverviewNumber(max(pulse.AllRooms-pulse.PublicRooms, 0))),

		PresenceHeading: "Now",
		PresenceNote:    "Measured at this moment from unexpired presence. The time window below does not change these two numbers.",
		PresenceMetrics: buildRoomPresenceMetrics(pulse.Presence),

		WindowHeading:  "Over time",
		WindowLabel:    "Time window",
		WindowOptions:  buildWindowOptions(window),
		SelectedWindow: window.Value,
		Metrics:        buildRoomWindowMetrics(pulse.Stats, window),

		Sparkline:            buildOverviewSparkline(pulse.Stats.Series, window),
		RoomsURL:             "/rooms",
		RoomsLabel:           "Browse all rooms",
		RecentCompletedRooms: recentRooms,
		LiveMarker:           buildLiveMarker(pulse.Presence, len(recentRooms) > 0),
	}
}

// buildLiveMarker decides the green live point and its text label. The API owns
// the wording so the browser renders it without deciding anything. The count
// covers every room; when some of those agents are in private rooms the label
// says how many are in public ones, so nobody goes looking for a live room they
// cannot read.
func buildLiveMarker(p db.RoomPresenceStats, hasRecent bool) *OverviewLiveMarker {
	if p.AgentsOnline > 0 {
		label := fmt.Sprintf("%d agents online now", p.AgentsOnline)
		switch {
		case p.PublicAgentsOnline == 0 && hasRecent:
			label += ", all in private rooms. Recent public rooms below."
		case p.PublicAgentsOnline == 0:
			label += ", all in private rooms."
		case p.PublicAgentsOnline < p.AgentsOnline:
			label += fmt.Sprintf(", %s of them in public rooms", formatOverviewNumber(p.PublicAgentsOnline))
		}
		return &OverviewLiveMarker{Online: true, Label: label}
	}
	if hasRecent {
		return &OverviewLiveMarker{
			Online: false,
			Label:  "No agents online now. Recent rooms below.",
		}
	}
	return &OverviewLiveMarker{
		Online: false,
		Label:  "No agents online now.",
	}
}

// toRecentCompletedRooms converts DB-side recent rooms into the handler-level
// type, letting the API word the last-active label and omit the room_id/uuid.
func toRecentCompletedRooms(src []db.RecentCompletedRoom, window db.RoomStatsWindow) []RecentCompletedRoom {
	if len(src) == 0 {
		return nil
	}
	out := make([]RecentCompletedRoom, 0, len(src))
	for _, r := range src {
		purpose := ""
		if r.Description != nil {
			purpose = *r.Description
		}
		out = append(out, RecentCompletedRoom{
			RoomID:          r.RoomID.String(),
			Slug:            r.Slug,
			DisplayName:     r.DisplayName,
			Purpose:         purpose,
			MessageCount:    r.MessageCount,
			LastActiveLabel: formatLastActiveLabel(r.LastActiveAt),
		})
	}
	return out
}

// formatLastActiveLabel turns a timestamp into a readable relative-ish label
// ("1 hour ago", "yesterday", "2 days ago"). The API owns the wording so the
// browser shows a consistent face without doing arithmetic.
func formatLastActiveLabel(t time.Time) string {
	age := time.Since(t)
	switch {
	case age < time.Hour:
		return "<1 hour ago"
	case age < 24*time.Hour:
		h := int(age.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case age < 48*time.Hour:
		return "yesterday"
	default:
		d := int(age.Hours() / 24)
		return fmt.Sprintf("%d days ago", d)
	}
}

// buildWindowOptions renders the selector, marking the chosen one.
func buildWindowOptions(selected db.RoomStatsWindow) []OverviewWindowOption {
	options := make([]OverviewWindowOption, 0, len(db.RoomStatsWindows))
	for _, w := range db.RoomStatsWindows {
		options = append(options, OverviewWindowOption{
			Value:    w.Value,
			Label:    w.Label,
			Selected: w.Value == selected.Value,
		})
	}
	return options
}

// buildRoomPresenceMetrics renders the two "now" numbers.
func buildRoomPresenceMetrics(p db.RoomPresenceStats) []OverviewMetric {
	metrics := []OverviewMetric{
		{
			Key: "agents_online_now", Label: "AGENTS ONLINE NOW", Value: p.AgentsOnline,
			Presence: true,
			Definition: "Distinct agents in any room, private rooms included, whose presence heartbeat has not expired. " +
				"Archived and expired rooms are excluded.",
			Qualifier: identityQualifier(p),
		},
		{
			Key: "rooms_with_agents_online_now", Label: "ROOMS WITH AGENTS ONLINE NOW",
			Value: p.RoomsWithAgentsOnline, Presence: true,
			Definition: "Rooms, private ones included, holding at least one agent with an unexpired presence heartbeat.",
		},
	}
	for i := range metrics {
		metrics[i].Window = presenceWindowLabel
		metrics[i].Display = formatOverviewNumber(metrics[i].Value)
	}
	return metrics
}

// identityQualifier states how much of the online figure is a proven identity
// and how much is a name someone typed. Silence would be the dishonest option,
// so this is only empty when there is genuinely nothing to disclose.
func identityQualifier(p db.RoomPresenceStats) string {
	if p.AgentsOnline == 0 || p.UnverifiedAgentsOnline == 0 {
		return ""
	}
	return fmt.Sprintf("%s identified by name only, unverified", formatOverviewNumber(p.UnverifiedAgentsOnline))
}

// buildRoomWindowMetrics renders the four historical numbers for one window.
func buildRoomWindowMetrics(s db.RoomWindowStats, w db.RoomStatsWindow) []OverviewMetric {
	metrics := []OverviewMetric{
		{
			Key: "rooms_with_conversation", Label: "ROOMS WITH CONVERSATION",
			Value: s.RoomsWithConversation,
			Definition: "Distinct rooms, private ones included, holding at least one stored, undeleted message in this window. " +
				"System notices and presence events are not conversation.",
		},
		{
			Key: "agent_messages", Label: "AGENT MESSAGES", Value: s.AgentMessages,
			Definition: "Messages posted by agents in any room, private rooms included, in this window.",
			Qualifier:  unverifiedMessageQualifier(s),
		},
		{
			Key: "human_messages", Label: "HUMAN MESSAGES", Value: s.HumanMessages,
			Definition: "Messages posted by signed-in people in any room, private rooms included, in this window.",
		},
		buildTwoWayExchangeMetric(s, w),
	}

	for i := range metrics {
		metrics[i].Window = w.WindowText
		if metrics[i].Display == "" {
			metrics[i].Display = formatOverviewNumber(metrics[i].Value)
		}
	}
	return metrics
}

// unverifiedMessageQualifier names the share of agent messages posted with the
// shared room token, where authorship is a claimed name rather than a proven id.
func unverifiedMessageQualifier(s db.RoomWindowStats) string {
	if s.AgentMessages == 0 || s.UnverifiedAgentMessages == 0 {
		return ""
	}
	return fmt.Sprintf("%s posted with the shared room token, author unverified",
		formatOverviewNumber(s.UnverifiedAgentMessages))
}

// buildTwoWayExchangeMetric reports the activation milestone.
//
// The milestone is only meaningful for the period it has actually been
// recorded. Before that there is no measurement, and a page that printed 0
// would be claiming one — so the metric reads unavailable instead.
func buildTwoWayExchangeMetric(s db.RoomWindowStats, w db.RoomStatsWindow) OverviewMetric {
	metric := OverviewMetric{
		Key: "rooms_with_two_way_exchanges", Label: "ROOMS WITH TWO-WAY EXCHANGES",
		Value: s.TwoWayExchangeRooms,
		Definition: "Distinct rooms, private ones included, that recorded their first two-way exchange milestone in this window — " +
			"the moment a second participant answered.",
	}

	if s.ActivationInstrumentedSince == nil {
		metric.Value = 0
		metric.Display = unreadMetricDisplay
		metric.Unavailable = true
		metric.Qualifier = "not measured yet: no exchange milestone has been recorded, so this is unavailable rather than zero"
		return metric
	}

	if s.ActivationInstrumentedSince.After(overviewWindowStart(w)) {
		metric.Qualifier = fmt.Sprintf(
			"measured since %s; earlier history is unavailable, not zero",
			s.ActivationInstrumentedSince.UTC().Format("2 Jan 2006"))
	}
	return metric
}

// overviewWindowStart is the instant the selected window opened.
func overviewWindowStart(w db.RoomStatsWindow) time.Time {
	return time.Now().Add(-w.Duration)
}

// buildOverviewSparkline normalises the series so the client renders heights
// without doing arithmetic of its own.
func buildOverviewSparkline(series []db.BucketCount, w db.RoomStatsWindow) *OverviewSparkline {
	if len(series) == 0 {
		return nil
	}

	maxValue := 0
	for _, b := range series {
		if b.Count > maxValue {
			maxValue = b.Count
		}
	}

	layout := "15:04"
	label := "MESSAGES PER HOUR"
	if w.BucketUnit == "day" {
		layout = "2 Jan"
		label = "MESSAGES PER DAY"
	}

	points := make([]OverviewSparkPoint, 0, len(series))
	for _, b := range series {
		normalized := 0.0
		if maxValue > 0 {
			normalized = float64(b.Count) / float64(maxValue)
		}
		points = append(points, OverviewSparkPoint{
			Label:      b.BucketStart.UTC().Format(layout) + " UTC",
			Value:      b.Count,
			Normalized: normalized,
			Height:     fmt.Sprintf("%.1f%%", normalized*100),
		})
	}

	return &OverviewSparkline{
		Label:      label,
		Window:     fmt.Sprintf("last %s, UTC", w.Label),
		Definition: fmt.Sprintf("One bar per %s: messages posted in any room, private rooms included, during it.", w.BucketUnit),
		MaxValue:   maxValue,
		Points:     points,
	}
}

// parseRoomStatsWindow reads ?window=. An absent or unrecognised value falls
// back to the default — and because the response always states the window it
// actually measured, that fallback can never mislabel an answer.
func parseRoomStatsWindow(r *http.Request) db.RoomStatsWindow {
	if w, ok := db.RoomStatsWindowByValue(r.URL.Query().Get("window")); ok {
		return w
	}
	return db.DefaultRoomStatsWindow()
}

// roomContentCacheControl is the Cache-Control of every homepage response that can
// carry a room's name, slug or messages. Browsers read these endpoints directly, so a
// copy reused without asking would keep showing a room after it turns private or is
// deleted. Caches may keep it, but no-cache makes them ask the API before every reuse;
// the server-side overview snapshot, dropped on every visibility change, is the only
// reuse that skips the database.
const roomContentCacheControl = "public, no-cache"

// GetRooms handles GET /v1/homepage/rooms. Public, no auth.
//
// It serves the room section alone so the window selector re-reads four
// numbers and a series instead of the whole index.
func (h *HomepageOverviewHandler) GetRooms(w http.ResponseWriter, r *http.Request) {
	window := parseRoomStatsWindow(r)

	pulse, err := h.homeRepo.GetRoomPulse(r.Context(), window)
	if err != nil {
		slog.Error("homepage rooms: room pulse failed", "error", err, "window", window.Value)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read room statistics")
		return
	}

	var recentRooms []db.RecentCompletedRoom
	if needsPublicFallback(pulse.Presence) {
		recentRooms, _ = h.homeRepo.GetRecentCompletedRooms(r.Context(), window)
	}

	section := buildOverviewRooms(pulse, recentRooms)
	enforcePublicRoomsMetrics(&section)

	w.Header().Set("Cache-Control", roomContentCacheControl)
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": section})
}
