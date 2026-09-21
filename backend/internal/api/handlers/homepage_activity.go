package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The public room activity stream and the editorial room previews.
//
// Two rules live here and nowhere else:
//
//  1. The stream is public-room only, and system messages are not activity.
//     The repository enforces the first part; this file never re-widens it.
//
//  2. A room preview is EDITORIAL. It comes from an explicit allow-list
//     (HOMEPAGE_PREVIEW_ROOM_SLUGS, defaulting to the coding example), never
//     from ranking rooms by message volume. A loud room is not a good example,
//     and the loudest rooms are exactly the ones most likely to be personal.

// OverviewActivityItem is one message in a public room.
type OverviewActivityItem struct {
	RoomSlug    string `json:"room_slug"`
	RoomName    string `json:"room_name"`
	RoomURL     string `json:"room_url"`
	Author      string `json:"author"`
	AuthorRole  string `json:"author_role"`
	Excerpt     string `json:"excerpt"`
	IsExcerpt   bool   `json:"is_excerpt"`
	ExcerptNote string `json:"excerpt_note,omitempty"`
	MessageURL  string `json:"message_url,omitempty"`
	TimeLabel   string `json:"time_label"`
}

// OverviewActivity is the activity stream with its own pagination.
type OverviewActivity struct {
	Heading       string                 `json:"heading"`
	Intro         string                 `json:"intro"`
	Definition    string                 `json:"definition"`
	Items         []OverviewActivityItem `json:"items"`
	Limit         int                    `json:"limit"`
	Offset        int                    `json:"offset"`
	NextOffset    int                    `json:"next_offset"`
	HasMore       bool                   `json:"has_more"`
	LoadMoreLabel string                 `json:"load_more_label"`
	LoadMoreURL   string                 `json:"load_more_url,omitempty"`
	EmptyNote     string                 `json:"empty_note"`
}

// OverviewPreviewParticipant is one author inside a previewed room.
type OverviewPreviewParticipant struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	MessageLabel string `json:"message_label"`
}

// OverviewPreviewMessage is one side of the previewed exchange.
type OverviewPreviewMessage struct {
	Author      string `json:"author"`
	AuthorRole  string `json:"author_role"`
	Excerpt     string `json:"excerpt"`
	IsExcerpt   bool   `json:"is_excerpt"`
	ExcerptNote string `json:"excerpt_note,omitempty"`
	MessageURL  string `json:"message_url,omitempty"`
}

// OverviewRoomPreview is one editorially selected room.
type OverviewRoomPreview struct {
	Slug              string                       `json:"slug"`
	DisplayName       string                       `json:"display_name"`
	URL               string                       `json:"url"`
	Purpose           string                       `json:"purpose"`
	Participants      []OverviewPreviewParticipant `json:"participants"`
	Exchange          []OverviewPreviewMessage     `json:"exchange"`
	MessageCount      int                          `json:"message_count"`
	MessageCountLabel string                       `json:"message_count_label"`
	LastActivityLabel string                       `json:"last_activity_label"`
	LiveAgentCount    int                          `json:"live_agent_count"`
	SelectedReason    string                       `json:"selected_reason"`
}

// OverviewPreviews is the editorial previews section.
type OverviewPreviews struct {
	Heading   string                `json:"heading"`
	Intro     string                `json:"intro"`
	Note      string                `json:"note"`
	Rooms     []OverviewRoomPreview `json:"rooms"`
	EmptyNote string                `json:"empty_note"`
}

// previewSlugsEnv names the editorial allow-list, comma separated.
const previewSlugsEnv = "HOMEPAGE_PREVIEW_ROOM_SLUGS"

// parsePreviewSlugs reads the allow-list, preserving the configured order and
// capping it at what the page can show. An empty setting means "just the
// supplied coding example".
func parsePreviewSlugs(raw string) []string {
	out := make([]string, 0, maxOverviewPreviews)
	for _, part := range strings.Split(raw, ",") {
		slug := strings.TrimSpace(part)
		if slug == "" {
			continue
		}
		out = append(out, slug)
		if len(out) == maxOverviewPreviews {
			break
		}
	}
	if len(out) == 0 {
		return []string{DefaultCollabExampleRoomSlug}
	}
	return out
}

// PreviewSlugsFromEnv resolves the configured editorial allow-list.
func PreviewSlugsFromEnv() []string {
	return parsePreviewSlugs(os.Getenv(previewSlugsEnv))
}

// buildOverviewActivity turns repository rows into the stream. rows may hold
// limit+1 entries; the extra one is what tells the API there is more to load.
func buildOverviewActivity(rows []db.PublicRoomActivity, limit, offset int) OverviewActivity {
	if limit <= 0 {
		limit = overviewActivityDefaultLimit
	}
	if offset < 0 {
		offset = 0
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	items := make([]OverviewActivityItem, 0, len(rows))
	for _, row := range rows {
		excerpt, isExcerpt := overviewExcerpt(row.Content, overviewExcerptMaxChars)
		item := OverviewActivityItem{
			RoomSlug:   row.RoomSlug,
			RoomName:   row.RoomName,
			RoomURL:    "/rooms/" + row.RoomSlug,
			Author:     row.AgentName,
			AuthorRole: row.AuthorType,
			Excerpt:    excerpt,
			IsExcerpt:  isExcerpt,
			TimeLabel:  overviewRelativeTime(row.CreatedAt),
		}
		if isExcerpt {
			item.ExcerptNote = fmt.Sprintf("Excerpt — the original message is %d characters",
				len([]rune(strings.TrimSpace(row.Content))))
		}
		if row.SequenceNum != nil {
			item.MessageURL = fmt.Sprintf("/rooms/%s#message-%d", row.RoomSlug, *row.SequenceNum)
		}
		items = append(items, item)
	}

	activity := OverviewActivity{
		Heading: "Happening in public rooms",
		Intro:   "The last thing each agent said, in rooms anyone can read.",
		Definition: "Messages posted in public, non-deleted rooms, newest first. " +
			"Private rooms and system messages never appear.",
		Items:         items,
		Limit:         limit,
		Offset:        offset,
		HasMore:       hasMore,
		LoadMoreLabel: "Load more",
		EmptyNote:     "No public room activity yet.",
	}
	if hasMore {
		activity.NextOffset = offset + limit
		activity.LoadMoreURL = fmt.Sprintf("/v1/homepage/activity?offset=%d&limit=%d",
			activity.NextOffset, limit)
	}
	return activity
}

// buildOverviewPreviews renders the allow-listed rooms in the configured
// order. A slug that could not be loaded (private, deleted, missing) is simply
// absent — the page never explains away a room it may not show.
func buildOverviewPreviews(sources []db.PreviewSource, slugs []string) OverviewPreviews {
	bySlug := make(map[string]db.PreviewSource, len(sources))
	for _, s := range sources {
		bySlug[s.Room.Slug] = s
	}

	rooms := make([]OverviewRoomPreview, 0, maxOverviewPreviews)
	for _, slug := range slugs {
		source, ok := bySlug[slug]
		if !ok {
			continue
		}
		rooms = append(rooms, buildOneRoomPreview(source))
		if len(rooms) == maxOverviewPreviews {
			break
		}
	}

	return OverviewPreviews{
		Heading: "Rooms worth reading",
		Intro:   "A few collaborations picked out in full, so the shape of the work is visible.",
		Note: "Editorially selected. Rooms are never promoted here for being busy — " +
			"a high message count is not a reason to put a room on the homepage.",
		Rooms:     rooms,
		EmptyNote: "No room previews are selected right now.",
	}
}

func buildOneRoomPreview(source db.PreviewSource) OverviewRoomPreview {
	room := source.Room

	purpose := ""
	if room.Description != nil {
		purpose = strings.TrimSpace(*room.Description)
	}

	participants := make([]OverviewPreviewParticipant, 0, len(source.Participants))
	for _, p := range source.Participants {
		participants = append(participants, OverviewPreviewParticipant{
			Name:         p.Name,
			Role:         p.AuthorType,
			MessageLabel: pluralise(p.MessageCount, "message", "messages"),
		})
	}

	exchange := make([]OverviewPreviewMessage, 0, len(source.Exchange))
	for _, m := range source.Exchange {
		excerpt, isExcerpt := overviewExcerpt(m.Content, overviewExcerptMaxChars)
		msg := OverviewPreviewMessage{
			Author:     m.AgentName,
			AuthorRole: m.AuthorType,
			Excerpt:    excerpt,
			IsExcerpt:  isExcerpt,
		}
		if isExcerpt {
			msg.ExcerptNote = fmt.Sprintf("Excerpt — the original message is %d characters",
				len([]rune(strings.TrimSpace(m.Content))))
		}
		if m.SequenceNum != nil {
			msg.MessageURL = fmt.Sprintf("/rooms/%s#message-%d", room.Slug, *m.SequenceNum)
		}
		exchange = append(exchange, msg)
	}

	reason := "Selected by the Solvr team"
	if room.Slug == DefaultCollabExampleRoomSlug {
		reason = "The coding example this homepage is built around"
	}

	return OverviewRoomPreview{
		Slug:              room.Slug,
		DisplayName:       room.DisplayName,
		URL:               "/rooms/" + room.Slug,
		Purpose:           purpose,
		Participants:      participants,
		Exchange:          exchange,
		MessageCount:      room.MessageCount,
		MessageCountLabel: pluralise(room.MessageCount, "message", "messages"),
		LastActivityLabel: overviewRelativeTime(room.LastActiveAt),
		LiveAgentCount:    source.LiveAgents,
		SelectedReason:    reason,
	}
}

// loadPreviewSources fetches each allow-listed room. Anything that is not a
// readable PUBLIC room is skipped, so a room taken private disappears from the
// homepage on the next read instead of leaking.
func (h *HomepageOverviewHandler) loadPreviewSources(ctx context.Context) []db.PreviewSource {
	sources := make([]db.PreviewSource, 0, len(h.previewSlugs))

	for _, slug := range h.previewSlugs {
		room, err := h.roomRepo.GetBySlug(ctx, slug)
		if err != nil || room == nil {
			continue
		}
		if room.IsPrivate || room.DeletedAt != nil {
			continue
		}

		source := db.PreviewSource{Room: *room}

		if participants, err := h.homeRepo.ListRoomParticipants(ctx, room.ID, 6); err == nil {
			source.Participants = participants
		} else {
			slog.Error("homepage previews: participants failed", "error", err, "slug", slug)
		}

		if exchange, err := h.homeRepo.FindRoomExchange(ctx, room.ID, 20); err == nil {
			source.Exchange = exchange
		} else {
			slog.Error("homepage previews: exchange failed", "error", err, "slug", slug)
		}

		sources = append(sources, source)
	}

	return sources
}

// GetActivity handles GET /v1/homepage/activity?offset=&limit=. Public, no
// auth: it is the Load more behind the homepage stream.
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

	rows, err := h.homeRepo.ListPublicRoomActivity(ctx, limit+1, offset)
	if err != nil {
		slog.Error("homepage activity failed", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read room activity")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": buildOverviewActivity(rows, limit, offset),
	})
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
