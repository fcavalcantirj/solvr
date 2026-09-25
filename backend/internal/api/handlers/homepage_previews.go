package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The editorial room previews on the homepage.
//
// A room preview is EDITORIAL. It comes from an explicit allow-list
// (HOMEPAGE_PREVIEW_ROOM_SLUGS, defaulting to the coding example), never from
// ranking rooms by message volume. A loud room is not a good example, and the
// loudest rooms are exactly the ones most likely to be personal.

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
	Slug         string                       `json:"slug"`
	DisplayName  string                       `json:"display_name"`
	URL          string                       `json:"url"`
	Purpose      string                       `json:"purpose"`
	Participants []OverviewPreviewParticipant `json:"participants"`
	// ParticipantCount is how many took part; Participants is a bounded list of
	// names, and MoreParticipantsLabel says how many of them it leaves out.
	ParticipantCount      int                      `json:"participant_count"`
	MoreParticipantsLabel string                   `json:"more_participants_label,omitempty"`
	Exchange              []OverviewPreviewMessage `json:"exchange"`
	MessageCount          int                      `json:"message_count"`
	MessageCountLabel     string                   `json:"message_count_label"`
	LastActivityLabel     string                   `json:"last_activity_label"`
	LiveAgentCount        int                      `json:"live_agent_count"`
	SelectedReason        string                   `json:"selected_reason"`
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

	participantCount := source.ParticipantCount
	if participantCount < len(participants) {
		participantCount = len(participants)
	}
	moreLabel := ""
	if hidden := participantCount - len(participants); hidden > 0 {
		moreLabel = "+" + pluralise(hidden, "more participant", "more participants")
	}

	reason := "Selected by the Solvr team"
	if room.Slug == DefaultCollabExampleRoomSlug {
		reason = "The coding example this homepage is built around"
	}

	return OverviewRoomPreview{
		Slug:                  room.Slug,
		DisplayName:           room.DisplayName,
		URL:                   "/rooms/" + room.Slug,
		Purpose:               purpose,
		Participants:          participants,
		ParticipantCount:      participantCount,
		MoreParticipantsLabel: moreLabel,
		Exchange:              exchange,
		MessageCount:          room.MessageCount,
		MessageCountLabel:     pluralise(room.MessageCount, "message", "messages"),
		LastActivityLabel:     overviewRelativeTime(room.LastActiveAt),
		LiveAgentCount:        source.LiveAgents,
		SelectedReason:        reason,
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
		if count, err := h.homeRepo.CountRoomParticipants(ctx, room.ID); err == nil {
			source.ParticipantCount = count
		} else {
			slog.Error("homepage previews: participant count failed", "error", err, "slug", slug)
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
