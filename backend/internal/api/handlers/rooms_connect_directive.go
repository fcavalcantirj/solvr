package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The directive in force, in the room connect envelope (idx 92). A page shows it beside
// the joining agent's sentence; the agent itself reads the room's latest_pinned, as the
// skill teaches, so the sentence never repeats the directive.

type directiveLookup interface {
	LatestDirective(ctx context.Context, roomID uuid.UUID) (*models.Message, error)
}

// roomConnectDirective names the directive in force in the envelope.
type roomConnectDirective struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	URL  string `json:"url"`
}

// SetDirectiveLookup enables the envelope's current_directive. Optional.
func (h *RoomConnectHandler) SetDirectiveLookup(d directiveLookup) {
	h.directives = d
}

// addCurrentDirective puts the directive in force into the envelope. A room without a
// pinned directive is left unchanged.
func (h *RoomConnectHandler) addCurrentDirective(ctx context.Context, room *models.Room, env *roomConnectEnvelope) {
	if h.directives == nil {
		return
	}
	d, err := h.directives.LatestDirective(ctx, room.ID)
	if err != nil {
		if !errors.Is(err, db.ErrMessageNotFound) {
			slog.Warn("room connect: directive unavailable", "error", err, "room_id", room.ID)
		}
		return
	}
	env.CurrentDirective = &roomConnectDirective{
		ID:   d.ID,
		Body: d.Content,
		URL:  connectEntriesURL(room.Slug) + "/" + strconv.FormatInt(d.ID, 10),
	}
}
