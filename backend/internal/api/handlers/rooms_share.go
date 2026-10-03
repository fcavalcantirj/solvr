package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Sharing a public collaboration (idx 88 step 3).
//
// GET /v1/rooms/{slug}/share answers what a person may copy to share a PUBLIC room:
// the clean room link, a share link (the same page with ?via=share, which the room page
// counts once per tab as a share_visit and then removes), a "Try this workflow" link that
// reuses the room's public task in a fresh room, and an optional short outcome excerpt.
// Every link is absolute and carries only the public slug — never a token or flow id.
// Solvr never posts any of it anywhere. A private room is never excerpted.

const (
	shareExcerptMaxChars = 280
	shareNote            = "Copy only — Solvr never posts this anywhere. Sharing it, and where, is your decision."
	shareViaParam        = "via=share"
)

// roomOutcomePosts is the published-outcome lookup the share excerpt prefers.
type roomOutcomePosts interface {
	FindPublishedBySourceRoom(ctx context.Context, roomID string) ([]*models.PostWithAuthor, error)
}

// SetOutcomePosts lets the share excerpt prefer a room's published outcome post.
// Optional: without it the excerpt starts from the room's result.
func (h *RoomHandler) SetOutcomePosts(posts roomOutcomePosts) {
	h.outcomePosts = posts
}

// roomTryWorkflowURL is the relative "Try this workflow" link of a public room, or nil:
// a private room's task never seeds a public start flow.
func roomTryWorkflowURL(room *models.Room) *string {
	if room == nil || room.IsPrivate {
		return nil
	}
	u := connectPageURL + "?from_room=" + url.QueryEscape(room.Slug)
	return &u
}

type roomShareExcerpt struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	// Source names what the text came from: outcome_post | result | pinned | initial_task | none.
	Source string `json:"source"`
}

type roomShare struct {
	RoomURL  string           `json:"room_url"`
	ShareURL string           `json:"share_url"`
	TryURL   string           `json:"try_url"`
	Excerpt  roomShareExcerpt `json:"excerpt"`
	CopyText string           `json:"copy_text"`
	Note     string           `json:"note"`
}

// GetRoomShare handles GET /v1/rooms/{slug}/share (room policy: read).
func (h *RoomHandler) GetRoomShare(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		found, err := h.roomRepo.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
		if err != nil {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		room = found
	}
	if room.IsPrivate {
		roomWriteError(w, http.StatusConflict, "ROOM_PRIVATE",
			"a private room cannot be shared; make it public first, or publish an outcome post")
		return
	}

	roomURL := connectAppBaseURL + "/rooms/" + room.Slug
	share := roomShare{
		RoomURL:  roomURL,
		ShareURL: roomURL + "?" + shareViaParam,
		TryURL:   connectAppBaseURL + *roomTryWorkflowURL(room),
		Excerpt:  h.shareExcerpt(r.Context(), room),
		Note:     shareNote,
	}
	parts := []string{share.Excerpt.Title}
	if share.Excerpt.Text != "" {
		parts = append(parts, share.Excerpt.Text)
	}
	share.CopyText = strings.Join(append(parts, share.ShareURL), "\n")

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": share})
}

// shareExcerpt picks the most finished public statement of the room's outcome: its
// published outcome post, else its recorded result, else its pinned directive, else its
// initial task. The text is scrubbed like any copied room text and cut to 280 characters.
func (h *RoomHandler) shareExcerpt(ctx context.Context, room *models.Room) roomShareExcerpt {
	ex := roomShareExcerpt{Title: room.DisplayName, Source: "none"}

	switch {
	case h.excerptFromOutcomePost(ctx, room, &ex):
	case h.excerptFromMessage(ctx, room, &ex):
	}
	ex.Title = truncateRunes(publicTemplateText(ctx, ex.Title, h.roomRepo), shareExcerptMaxChars)
	ex.Text = truncateRunes(strings.Join(strings.Fields(publicTemplateText(ctx, ex.Text, h.roomRepo)), " "), shareExcerptMaxChars)
	return ex
}

func (h *RoomHandler) excerptFromOutcomePost(ctx context.Context, room *models.Room, ex *roomShareExcerpt) bool {
	if h.outcomePosts == nil {
		return false
	}
	posts, err := h.outcomePosts.FindPublishedBySourceRoom(ctx, room.ID.String())
	if err != nil {
		slog.Warn("share excerpt: outcome posts unavailable", "error", err, "room_id", room.ID)
		return false
	}
	if len(posts) == 0 || posts[0] == nil {
		return false
	}
	ex.Title, ex.Text, ex.Source = posts[0].Title, posts[0].Description, "outcome_post"
	return true
}

func (h *RoomHandler) excerptFromMessage(ctx context.Context, room *models.Room, ex *roomShareExcerpt) bool {
	if room.ResultMessageID != nil {
		if msg, err := h.msgRepo.GetByID(ctx, room.ID, *room.ResultMessageID); err == nil && msg != nil {
			ex.Text, ex.Source = msg.Content, "result"
			return true
		}
	}
	if pinned, err := h.msgRepo.LatestDirective(ctx, room.ID); err == nil && pinned != nil {
		ex.Text, ex.Source = pinned.Content, "pinned"
		return true
	}
	if first, err := h.msgRepo.GetFirstMessage(ctx, room.ID); err == nil && first != nil {
		ex.Text, ex.Source = first.Content, "initial_task"
		return true
	}
	return false
}
