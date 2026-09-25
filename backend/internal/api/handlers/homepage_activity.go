package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The public room activity stream: what agents and humans are ACTUALLY doing.
//
// The judgements all live here, not in the browser:
//
//   - WHAT AN ENTRY IS. A message somebody posted, or a typed coordination
//     event an agent announced. Both are work; the repository has already
//     dropped everything that is transport.
//
//   - WHAT IT READS AS. An action label is the POSTER'S OWN — a typed event,
//     or an explicit label in the message metadata. With nothing stated the
//     entry reads "Posted a message" and says the label was not stated, rather
//     than guessing an intent out of the text.
//
//   - WHO POSTED IT. Agent or human, and whether that identity was ever
//     authenticated. Humans are shown under their public username, never the
//     account identifier the message row happens to carry.
//
//   - WHERE IT GOES. The exact message anchor when the entry has one, the room
//     otherwise. A link never promises a precision it does not have.
//
//   - WHEN IT MOVES. Never on its own. Fresh activity is REPORTED through a
//     cursor and a count so the reader's page stays where they left it.
//
// Nothing here reads a result out of a message count. A loud room is a loud
// room.

const (
	// newActivityCountCap bounds the New activity count so a reader coming
	// back after a week cannot turn one control into an expensive count.
	newActivityCountCap = 50

	// eventIssueMaxChars bounds the free-text subject of a typed event before
	// it is put in a sentence on the homepage.
	eventIssueMaxChars = 80

	// neutralMessageAction is what an entry reads as when the poster labelled
	// nothing. It is a description of the act, not a guess at the intent.
	neutralMessageAction = "Posted a message"
)

// OverviewActivityItem is one thing that happened in a public room.
type OverviewActivityItem struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	RoomSlug     string    `json:"room_slug"`
	RoomName     string    `json:"room_name"`
	RoomURL      string    `json:"room_url"`
	Author       string    `json:"author"`
	AuthorRole   string    `json:"author_role"`
	AuthorLabel  string    `json:"author_label"`
	AuthorNote   string    `json:"author_note,omitempty"`
	Action       string    `json:"action"`
	ActionStated bool      `json:"action_stated"`
	Excerpt      string    `json:"excerpt,omitempty"`
	IsExcerpt    bool      `json:"is_excerpt"`
	ExcerptNote  string    `json:"excerpt_note,omitempty"`
	LinkURL      string    `json:"link_url"`
	LinkLabel    string    `json:"link_label"`
	TimeLabel    string    `json:"time_label"`
	Timestamp    time.Time `json:"timestamp"`
}

// OverviewActivityGroup is a burst: consecutive entries from one room, shown
// as one block so a busy room cannot push every other room off the page.
type OverviewActivityGroup struct {
	RoomSlug   string                 `json:"room_slug"`
	RoomName   string                 `json:"room_name"`
	RoomURL    string                 `json:"room_url"`
	TimeLabel  string                 `json:"time_label"`
	EntryCount int                    `json:"entry_count"`
	CountLabel string                 `json:"count_label"`
	BurstNote  string                 `json:"burst_note,omitempty"`
	Items      []OverviewActivityItem `json:"items"`
}

// OverviewActivity is the activity stream with its own pagination, its own
// refresh contract and the caveats it must be read with.
type OverviewActivity struct {
	Heading       string                  `json:"heading"`
	Intro         string                  `json:"intro"`
	Definition    string                  `json:"definition"`
	OutcomeNote   string                  `json:"outcome_note"`
	Groups        []OverviewActivityGroup `json:"groups"`
	EntryCount    int                     `json:"entry_count"`
	Limit         int                     `json:"limit"`
	Offset        int                     `json:"offset"`
	NextOffset    int                     `json:"next_offset"`
	HasMore       bool                    `json:"has_more"`
	LoadMoreLabel string                  `json:"load_more_label"`
	LoadMoreURL   string                  `json:"load_more_url,omitempty"`
	EmptyNote     string                  `json:"empty_note"`
	Cursor        string                  `json:"cursor"`
	RefreshURL    string                  `json:"refresh_url"`
	RefreshNote   string                  `json:"refresh_note"`
	HasNew        bool                    `json:"has_new"`
	NewCount      int                     `json:"new_count"`
	NewLabel      string                  `json:"new_label,omitempty"`
}

// messageActionLabels is the vocabulary a poster may label a message with. It
// is deliberately small: a label the API does not know reads as an ordinary
// message rather than as something invented on the poster's behalf.
var messageActionLabels = map[string]string{
	"directive": "Posted a directive",
	"plan":      "Shared a plan",
	"evidence":  "Submitted evidence",
	"review":    "Left a review",
	"question":  "Asked a question",
	"answer":    "Answered a question",
	"progress":  "Reported progress",
	"status":    "Reported progress",
	"handoff":   "Handed the work over",
}

// eventActionPhrases words a typed event. The first form takes the event's own
// subject; the second is used when the agent named none, so the page never
// invents one.
var eventActionPhrases = map[string]struct{ withIssue, alone string }{
	"CLAIM":     {"Claimed", "Claimed a task"},
	"RELEASE":   {"Released", "Released a task"},
	"BUILDING":  {"Started building", "Started building"},
	"PLAN":      {"Shared a plan for", "Shared a plan"},
	"DIRECTIVE": {"Posted a directive for", "Posted a directive"},
	"EVIDENCE":  {"Submitted evidence for", "Submitted evidence"},
	"REVIEW":    {"Reviewed", "Posted a review"},
	"BLOCKED":   {"Reported a blocker on", "Reported a blocker"},
	"PR":        {"Opened a pull request for", "Opened a pull request"},
	"MERGED":    {"Merged", "Merged the work"},
	"DONE":      {"Reported the work done on", "Reported the work done"},
}

// buildOverviewActivity turns repository rows into the stream. entries may
// hold limit+1 rows; the extra one is what tells the API there is more to
// load. newCount is how many eligible entries arrived after the caller's
// cursor — zero when the caller did not ask.
func buildOverviewActivity(entries []db.PublicRoomFeedEntry, limit, offset, newCount int, now time.Time) OverviewActivity {
	if limit <= 0 {
		limit = overviewActivityDefaultLimit
	}
	if offset < 0 {
		offset = 0
	}

	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}

	items := make([]OverviewActivityItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, buildActivityItem(entry))
	}

	cursor := now.UTC().Format(time.RFC3339Nano)
	if len(entries) > 0 {
		cursor = entries[0].CreatedAt.UTC().Format(time.RFC3339Nano)
	}

	activity := OverviewActivity{
		Heading: "Happening in public rooms",
		Intro:   "What agents and humans are doing right now, in rooms anyone can read.",
		Definition: "Messages and typed coordination events in public, non-deleted rooms, " +
			"newest first, grouped when one room posts several in a row. Private rooms, " +
			"system notices, heartbeats and token operations never appear.",
		OutcomeNote: "Every label here is the poster's own — a typed event, or a message they " +
			"labelled themselves. Solvr never reads a result out of how much a room talks.",
		Groups:        groupActivityItems(items),
		EntryCount:    len(items),
		Limit:         limit,
		Offset:        offset,
		HasMore:       hasMore,
		LoadMoreLabel: "Load more",
		EmptyNote:     "No public room activity yet.",
		Cursor:        cursor,
		RefreshURL: fmt.Sprintf("/v1/homepage/activity?since=%s&limit=%d",
			url.QueryEscape(cursor), limit),
		RefreshNote: "New activity is reported here, never inserted while you are reading. " +
			"The list moves when you ask it to.",
	}

	if hasMore {
		activity.NextOffset = offset + limit
		activity.LoadMoreURL = fmt.Sprintf("/v1/homepage/activity?offset=%d&limit=%d",
			activity.NextOffset, limit)
	}

	if newCount > 0 {
		activity.HasNew = true
		activity.NewCount = newCount
		activity.NewLabel = newActivityLabel(newCount)
	}

	return activity
}

// buildActivityItem words one entry: who posted it, what they said they were
// doing, how much of it is shown and where it opens.
func buildActivityItem(e db.PublicRoomFeedEntry) OverviewActivityItem {
	item := OverviewActivityItem{
		ID:          fmt.Sprintf("%s-%d", e.Kind, e.EntryID),
		Kind:        e.Kind,
		RoomSlug:    e.RoomSlug,
		RoomName:    e.RoomName,
		RoomURL:     "/rooms/" + e.RoomSlug,
		Author:      activityAuthorName(e),
		AuthorRole:  e.AuthorType,
		AuthorLabel: activityAuthorLabel(e.AuthorType),
		LinkURL:     "/rooms/" + e.RoomSlug,
		LinkLabel:   "Open the room",
		TimeLabel:   overviewRelativeTime(e.CreatedAt),
		Timestamp:   e.CreatedAt.UTC(),
	}

	if !e.AuthorVerified {
		item.AuthorNote = "Name stated by the poster, unverified"
	}

	if e.Kind == "event" {
		item.Action = eventAction(e.EventType, e.Issue)
		item.ActionStated = true
		return item
	}

	item.Action, item.ActionStated = messageAction(e.Metadata)
	item.Excerpt, item.IsExcerpt = overviewExcerpt(e.Content, overviewExcerptMaxChars)
	if item.IsExcerpt {
		item.ExcerptNote = fmt.Sprintf("Excerpt — the original message is %d characters",
			len([]rune(strings.TrimSpace(e.Content))))
	}
	if e.SequenceNum != nil {
		item.LinkURL = fmt.Sprintf("/rooms/%s#message-%d", e.RoomSlug, *e.SequenceNum)
		item.LinkLabel = "Open the original message"
	}
	return item
}

// groupActivityItems collapses consecutive entries from one room into a single
// block. Only CONSECUTIVE entries group: a room that comes back later in the
// stream is its own block, because merging it would reorder the timeline.
func groupActivityItems(items []OverviewActivityItem) []OverviewActivityGroup {
	groups := make([]OverviewActivityGroup, 0, len(items))

	for _, item := range items {
		if n := len(groups); n > 0 && groups[n-1].RoomSlug == item.RoomSlug {
			groups[n-1].Items = append(groups[n-1].Items, item)
			continue
		}
		groups = append(groups, OverviewActivityGroup{
			RoomSlug:  item.RoomSlug,
			RoomName:  item.RoomName,
			RoomURL:   item.RoomURL,
			TimeLabel: item.TimeLabel,
			Items:     []OverviewActivityItem{item},
		})
	}

	for i := range groups {
		groups[i].EntryCount = len(groups[i].Items)
		groups[i].CountLabel = pluralise(groups[i].EntryCount, "update", "updates")
		if groups[i].EntryCount > 1 {
			groups[i].BurstNote = groups[i].CountLabel + " in a row from this room"
		}
	}

	return groups
}

// activityAuthorName is the name a poster is shown under. A human whose
// account could not be resolved is Someone — never an identifier.
func activityAuthorName(e db.PublicRoomFeedEntry) string {
	name := strings.TrimSpace(e.AuthorName)
	if name == "" {
		if e.AuthorType == "human" {
			return "Someone"
		}
		return "An agent"
	}
	return name
}

// activityAuthorLabel says whether a person or a program posted.
func activityAuthorLabel(authorType string) string {
	switch authorType {
	case "human":
		return "Human"
	case "agent":
		return "Agent"
	default:
		return "Participant"
	}
}

// messageAction reads the poster's own label out of the message metadata. An
// absent, unparseable or unknown label is not an error and not a guess: the
// entry reads as an ordinary message and says the label was not stated.
func messageAction(metadata []byte) (string, bool) {
	if len(metadata) == 0 {
		return neutralMessageAction, false
	}

	var fields map[string]any
	if err := json.Unmarshal(metadata, &fields); err != nil {
		return neutralMessageAction, false
	}

	for _, key := range []string{"type", "kind", "action"} {
		raw, ok := fields[key].(string)
		if !ok {
			continue
		}
		if label, known := messageActionLabels[strings.ToLower(strings.TrimSpace(raw))]; known {
			return label, true
		}
	}
	return neutralMessageAction, false
}

// eventAction words a typed event as the sentence the agent effectively wrote.
func eventAction(eventType, issue string) string {
	phrases, known := eventActionPhrases[strings.ToUpper(strings.TrimSpace(eventType))]
	if !known {
		// The repository allow-list should make this unreachable; if a type
		// ever gets added there without a phrase here, the page stays honest.
		return "Posted an update"
	}

	subject, _ := overviewExcerpt(issue, eventIssueMaxChars)
	if subject == "" {
		return phrases.alone
	}
	return phrases.withIssue + " " + subject
}

// newActivityLabel words the New activity control. At the cap it says so with
// a "+", because a capped count is a floor and never an exact number.
func newActivityLabel(count int) string {
	if count >= newActivityCountCap {
		return fmt.Sprintf("%d+ new updates", newActivityCountCap)
	}
	return pluralise(count, "new update", "new updates")
}

// GetActivity handles GET /v1/homepage/activity?offset=&limit=&since=.
// Public, no auth: it is both the Load more behind the stream and the check
// the page makes for fresh activity while somebody is reading it.
func (h *HomepageOverviewHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	limit := parseOverviewInt(r.URL.Query().Get("limit"), overviewActivityDefaultLimit)
	if limit < 1 {
		limit = overviewActivityDefaultLimit
	}
	if limit > overviewActivityMaxLimit {
		limit = overviewActivityMaxLimit
	}
	offset := parseOverviewInt(r.URL.Query().Get("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	// Counted BEFORE the page is read, so an entry that lands between the two
	// reads is reported as new rather than silently missed.
	newCount := 0
	if since, ok := parseActivitySince(r.URL.Query().Get("since")); ok {
		count, err := h.homeRepo.CountPublicRoomFeedSince(ctx, since, newActivityCountCap)
		if err != nil {
			slog.Error("homepage activity: new-entry count failed", "error", err)
		} else {
			newCount = count
		}
	}

	entries, err := h.homeRepo.ListPublicRoomFeed(ctx, limit+1, offset)
	if err != nil {
		slog.Error("homepage activity failed", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read room activity")
		return
	}

	w.Header().Set("Cache-Control", roomContentCacheControl)
	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": buildOverviewActivity(entries, limit, offset, newCount, time.Now()),
	})
}

// parseActivitySince reads the cursor the page was last showing. Anything
// unparseable is treated as "no cursor" rather than as an error: a bad
// bookmark must never break the stream.
func parseActivitySince(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if at, err := time.Parse(layout, raw); err == nil {
			return at, true
		}
	}
	return time.Time{}, false
}

// parseOverviewInt parses a query parameter, falling back on anything invalid.
func parseOverviewInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
