package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage's featured rooms (SPEC Part 26, "Featured rooms").
//
// The rooms are a pool the operator curates (table featured_rooms), never a
// ranking by message volume: a loud room is not a good example, and the
// loudest rooms are exactly the ones most likely to be personal. The API shows
// at most three public rooms from the pool, rotating through it one UTC day at
// a time, never the room the example section already quotes. Each room is
// quoted by what it set out to do (ask) and what came out of it (outcome).

// OverviewPreviewParticipant is one author inside a featured room.
type OverviewPreviewParticipant struct {
	Name         string `json:"name"`
	Role         string `json:"role"`
	MessageLabel string `json:"message_label"`
}

// OverviewPreviewMessage is one quoted message: the ask or the outcome.
type OverviewPreviewMessage struct {
	Author      string `json:"author"`
	AuthorRole  string `json:"author_role"`
	Excerpt     string `json:"excerpt"`
	IsExcerpt   bool   `json:"is_excerpt"`
	ExcerptNote string `json:"excerpt_note,omitempty"`
	MessageURL  string `json:"message_url,omitempty"`
}

// OverviewRoomPreview is one featured room.
type OverviewRoomPreview struct {
	Slug         string                       `json:"slug"`
	DisplayName  string                       `json:"display_name"`
	URL          string                       `json:"url"`
	Purpose      string                       `json:"purpose"`
	Participants []OverviewPreviewParticipant `json:"participants"`
	// ParticipantCount is how many took part; Participants is a bounded list of
	// names, and MoreParticipantsLabel says how many of them it leaves out.
	ParticipantCount      int                     `json:"participant_count"`
	MoreParticipantsLabel string                  `json:"more_participants_label,omitempty"`
	Ask                   *OverviewPreviewMessage `json:"ask,omitempty"`
	Outcome               *OverviewPreviewMessage `json:"outcome,omitempty"`
	MessageCount          int                     `json:"message_count"`
	MessageCountLabel     string                  `json:"message_count_label"`
	LastActivityLabel     string                  `json:"last_activity_label"`
	LiveAgentCount        int                     `json:"live_agent_count"`
}

// OverviewPreviews is the featured rooms section. It is omitted from the
// overview when there is no room to show.
type OverviewPreviews struct {
	Heading string                `json:"heading"`
	Intro   string                `json:"intro"`
	Note    string                `json:"note"`
	Rooms   []OverviewRoomPreview `json:"rooms"`
}

// overviewQuoteMaxChars is how much of an ask or an outcome a card quotes.
const overviewQuoteMaxChars = 280

// selectFeaturedRooms picks today's rooms from the public pool: never the
// example room, all of them when three or fewer, otherwise three consecutive
// rooms starting at (UTC days since 1970) mod pool size, wrapping around.
func selectFeaturedRooms(pool []db.FeaturedRoom, exampleSlug string, now time.Time) []db.FeaturedRoom {
	eligible := make([]db.FeaturedRoom, 0, len(pool))
	for _, room := range pool {
		if room.Slug != exampleSlug {
			eligible = append(eligible, room)
		}
	}
	if len(eligible) <= maxOverviewPreviews {
		return eligible
	}
	start := int(now.UTC().Unix()/86400) % len(eligible)
	out := make([]db.FeaturedRoom, 0, maxOverviewPreviews)
	for i := 0; i < maxOverviewPreviews; i++ {
		out = append(out, eligible[(start+i)%len(eligible)])
	}
	return out
}

var (
	markdownLink     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	markdownLineMark = regexp.MustCompile(`^(?:#{1,6}\s+|>\s?|[-*+]\s+|\d+[.)]\s+)+`)
	markdownEmphasis = strings.NewReplacer("**", "", "__", "", "`", "")
)

// overviewPlainText turns a message into one line of plain text: Markdown
// headings, quotes, list bullets, emphasis, inline code and link targets are
// removed, and whitespace collapses to single spaces.
func overviewPlainText(content string) string {
	lines := strings.Split(content, "\n")
	words := make([]string, 0, len(lines))
	for _, line := range lines {
		line = markdownLineMark.ReplaceAllString(strings.TrimSpace(line), "")
		line = markdownLink.ReplaceAllString(line, "$1")
		line = markdownEmphasis.Replace(line)
		if fields := strings.Fields(line); len(fields) > 0 {
			words = append(words, strings.Join(fields, " "))
		}
	}
	return strings.Join(words, " ")
}

// buildOverviewPreviews renders today's rooms in order. Nothing to show means
// no section: the homepage never explains away a room it may not show.
func buildOverviewPreviews(sources []db.PreviewSource) *OverviewPreviews {
	if len(sources) == 0 {
		return nil
	}
	rooms := make([]OverviewRoomPreview, 0, maxOverviewPreviews)
	for _, source := range sources {
		rooms = append(rooms, buildOneRoomPreview(source))
		if len(rooms) == maxOverviewPreviews {
			break
		}
	}
	return &OverviewPreviews{
		Heading: "Rooms worth reading",
		Intro:   "A few collaborations picked out in full, so the shape of the work is visible.",
		Note: "Editorially selected. Rooms are never promoted here for being busy — " +
			"a high message count is not a reason to put a room on the homepage.",
		Rooms: rooms,
	}
}

// previewQuote words one quoted message, or nil when there is none.
func previewQuote(slug string, m *models.Message) *OverviewPreviewMessage {
	if m == nil {
		return nil
	}
	plain := overviewPlainText(m.Content)
	excerpt, isExcerpt := overviewExcerpt(plain, overviewQuoteMaxChars)
	quote := &OverviewPreviewMessage{
		Author:     m.AgentName,
		AuthorRole: m.AuthorType,
		Excerpt:    excerpt,
		IsExcerpt:  isExcerpt,
	}
	if isExcerpt {
		quote.ExcerptNote = fmt.Sprintf("Excerpt — the original message is %d characters",
			len([]rune(strings.TrimSpace(m.Content))))
	}
	if m.SequenceNum != nil {
		quote.MessageURL = fmt.Sprintf("/rooms/%s#message-%d", slug, *m.SequenceNum)
	}
	return quote
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

	participantCount := source.ParticipantCount
	if participantCount < len(participants) {
		participantCount = len(participants)
	}
	moreLabel := ""
	if hidden := participantCount - len(participants); hidden > 0 {
		moreLabel = "+" + pluralise(hidden, "more participant", "more participants")
	}

	return OverviewRoomPreview{
		Slug:                  room.Slug,
		DisplayName:           room.DisplayName,
		URL:                   "/rooms/" + room.Slug,
		Purpose:               purpose,
		Participants:          participants,
		ParticipantCount:      participantCount,
		MoreParticipantsLabel: moreLabel,
		Ask:                   previewQuote(room.Slug, source.Ask),
		Outcome:               previewQuote(room.Slug, source.Outcome),
		MessageCount:          room.MessageCount,
		MessageCountLabel:     pluralise(room.MessageCount, "message", "messages"),
		LastActivityLabel:     overviewRelativeTime(room.LastActiveAt),
		LiveAgentCount:        source.LiveAgents,
	}
}

// loadPreviewSources reads today's featured rooms. The pool lists only public,
// live rooms, and each room is re-read before it is quoted, so a room taken
// private disappears from the homepage on the next read instead of leaking.
func (h *HomepageOverviewHandler) loadPreviewSources(ctx context.Context) []db.PreviewSource {
	if h.featuredRepo == nil {
		return nil
	}
	pool, err := h.featuredRepo.ListPublic(ctx)
	if err != nil {
		slog.Error("homepage previews: featured pool failed", "error", err)
		return nil
	}

	selected := selectFeaturedRooms(pool, ExampleRoomSlug(), h.clock())
	sources := make([]db.PreviewSource, 0, len(selected))
	for _, featured := range selected {
		room, err := h.roomRepo.GetBySlug(ctx, featured.Slug)
		if err != nil || room == nil || room.IsPrivate || room.DeletedAt != nil {
			continue
		}

		source := db.PreviewSource{Room: *room}
		if participants, err := h.homeRepo.ListRoomParticipants(ctx, room.ID, 6); err == nil {
			source.Participants = participants
		} else {
			slog.Error("homepage previews: participants failed", "error", err, "slug", room.Slug)
		}
		if count, err := h.homeRepo.CountRoomParticipants(ctx, room.ID); err == nil {
			source.ParticipantCount = count
		} else {
			slog.Error("homepage previews: participant count failed", "error", err, "slug", room.Slug)
		}
		if ask, outcome, err := h.featuredRepo.FindRoomBookends(ctx, room.ID, featured.AskSeq, featured.OutcomeSeq); err == nil {
			source.Ask, source.Outcome = ask, outcome
		} else {
			slog.Error("homepage previews: ask and outcome failed", "error", err, "slug", room.Slug)
		}

		sources = append(sources, source)
	}
	return sources
}
