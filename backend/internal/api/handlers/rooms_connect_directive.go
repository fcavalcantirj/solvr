package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The directive in force, in a room-bound prompt (idx 92 step 1: resume instructions).
// An agent that joins or resumes reads the instruction the room pinned — followed to its
// newest revision — instead of acting on the first message alone.

// directiveCharsInPrompt bounds how much of the directive a prompt carries; the agent
// re-reads the whole entry at its URL.
const directiveCharsInPrompt = 600

type directiveLookup interface {
	LatestDirective(ctx context.Context, roomID uuid.UUID) (*models.Message, error)
}

// roomConnectDirective names the directive in force in the envelope.
type roomConnectDirective struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	URL  string `json:"url"`
}

// SetDirectiveLookup enables the CURRENT DIRECTIVE section. Optional.
func (h *RoomConnectHandler) SetDirectiveLookup(d directiveLookup) {
	h.directives = d
}

// addCurrentDirective puts the directive in force into the envelope and into the prompt,
// before the completion step. A room without a pinned directive is left unchanged.
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
	cd := &roomConnectDirective{
		ID:   d.ID,
		Body: d.Content,
		URL:  connectEntriesURL(room.Slug) + "/" + strconv.FormatInt(d.ID, 10),
	}
	env.CurrentDirective = cd
	section := currentDirectiveSection(cd)
	env.Prompt = insertBeforeCompletion(env.Prompt, section)
	if env.ExecutorPrompt != "" {
		env.ExecutorPrompt = insertBeforeCompletion(env.ExecutorPrompt, section)
	}
}

func currentDirectiveSection(d *roomConnectDirective) string {
	return strings.Join([]string{
		"",
		"CURRENT DIRECTIVE",
		"The directive in force in this room right now (entry " + strconv.FormatInt(d.ID, 10) + "):",
		truncateRunes(d.Body, directiveCharsInPrompt),
		"A newer revision replaces it. Re-read it any time with GET " + d.URL + ",",
		"and when you resume, follow the newest directive.",
	}, "\n")
}

// insertBeforeCompletion places a section right before WHEN THE WORK IS DONE, or at the
// end of a prompt that has no completion step.
func insertBeforeCompletion(prompt, section string) string {
	marker := "\n\nWHEN THE WORK IS DONE"
	if i := strings.Index(prompt, marker); i >= 0 {
		return prompt[:i] + section + prompt[i:]
	}
	return prompt + "\n" + section
}
