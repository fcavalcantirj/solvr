package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Typed-event field bounds (match the room_entries CHECK constraints and the former
// room_events view trigger).
const (
	maxEventTypeLen  = 50
	maxEventIssueLen = 200
	maxEventLabelLen = 200
)

// eventSubmission is one typed coordination event written into a room timeline, whichever
// route received it: the canonical POST /v1/rooms/{slug}/entries (kind=event) or the
// transport adapter POST /r/{slug}/events. Attribution (AuthorType, AuthorID) always comes
// from the authenticated credential, never from the body.
type eventSubmission struct {
	AuthorType    string // "agent" or "human"
	AuthorID      *string
	Label         string // historical display label stored as actor_label (legacy "actor")
	EventType     string
	Issue         string
	Payload       json.RawMessage
	ClientEntryID *string
}

// submitEvent is the ONE typed-event submission implementation. It validates the event,
// stores exactly one timeline entry (a retry with the same client_entry_id from the same
// author returns the existing entry with created=false) and broadcasts a new event once.
func (h *RoomEventsHandler) submitEvent(ctx context.Context, room *models.Room, s eventSubmission) (*models.RoomEntry, bool, *submitError) {
	switch {
	case s.EventType == "":
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "event_type is required (type on the legacy events route)"}
	case len(s.EventType) > maxEventTypeLen:
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "event_type exceeds maximum length of 50 characters"}
	case len(s.Issue) > maxEventIssueLen:
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "issue exceeds maximum length of 200 characters"}
	case s.Label == "":
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "actor is required"}
	case len(s.Label) > maxEventLabelLen:
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "actor exceeds maximum length of 200 characters"}
	case len(s.Payload) > maxEventPayloadBytes:
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "payload exceeds maximum size of 16384 bytes"}
	}

	eventType := s.EventType
	authorType := s.AuthorType
	params := models.CreateRoomEntryParams{
		RoomID:     room.ID,
		Kind:       models.RoomEntryKindEvent,
		AuthorType: &authorType,
		AuthorID:   s.AuthorID,
		ActorLabel: s.Label,
		EventType:  &eventType,
		Issue:      s.Issue,
		Extension:  s.Payload,
	}
	if s.ClientEntryID != nil && *s.ClientEntryID != "" {
		params.ClientEntryID = s.ClientEntryID
	}

	entry, created, err := h.entryRepo.Create(ctx, params)
	if err != nil {
		if errors.Is(err, db.ErrInvalidEntryReference) {
			return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", invalidEntryReferenceMsg}
		}
		if errors.Is(err, db.ErrClientEntryConflict) {
			return nil, false, clientEntryReusedError
		}
		slog.Error("failed to create room event", "error", err, "room_id", room.ID, "type", s.EventType)
		return nil, false, &submitError{http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create event"}
	}

	// A replay was already broadcast by the original write.
	if created && h.hubMgr != nil {
		roomHub := h.hubMgr.GetOrCreate(ctx, hub.NewRoomID(room.ID))
		roomHub.Broadcast(typedHubEvent(entry))
	}
	return entry, created, nil
}

// typedHubEvent is the stream frame for an event entry, shared by the live broadcast and
// the SSE reconnect replay so a replayed event is identical to the one delivered live.
func typedHubEvent(e *models.RoomEntry) hub.RoomEvent {
	event := roomEventFromEntry(e)
	return hub.RoomEvent{
		ID:        event.ID,
		Sequence:  event.Sequence,
		Type:      hub.EventTyped,
		RoomID:    hub.NewRoomID(e.RoomID),
		AgentName: event.Actor,
		EventName: event.EventType,
		Issue:     event.Issue,
		Payload:   event,
		Timestamp: event.CreatedAt,
	}
}

// roomEventFromEntry renders an event entry in the legacy /r/{slug}/events shape.
func roomEventFromEntry(e *models.RoomEntry) models.RoomEvent {
	event := models.RoomEvent{
		ID:        e.ID,
		RoomID:    e.RoomID,
		Sequence:  e.Sequence,
		Issue:     e.Issue,
		Actor:     e.ActorLabel,
		Payload:   e.Extension,
		CreatedAt: e.CreatedAt,
	}
	if e.EventType != nil {
		event.EventType = *e.EventType
	}
	if event.Payload == nil {
		event.Payload = json.RawMessage(`{}`)
	}
	return event
}
