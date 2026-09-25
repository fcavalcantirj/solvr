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

// messageSubmission is one message write into a room timeline, whichever route received
// it: the canonical POST /v1/rooms/{slug}/entries or the transport adapters
// POST /r/{slug}/message and POST /v1/rooms/{slug}/messages. Attribution (AuthorType,
// AuthorID) always comes from the authenticated credential, never from the body.
type messageSubmission struct {
	AuthorType string // "agent" or "human"
	AuthorID   *string
	Label      string // historical display label stored as actor_label / agent_name

	Content            string
	ContentType        string
	Metadata           json.RawMessage
	ReplyToEntryID     *int64
	AddressedMemberIDs json.RawMessage
	SupersedesEntryID  *int64
	ClientEntryID      *string
}

// submitError is a client-facing refusal of a submission.
type submitError struct {
	status  int
	code    string
	message string
}

// clientEntryReusedError refuses a client_entry_id reused by the same author for a
// different payload: a retry replays only the same write.
var clientEntryReusedError = &submitError{http.StatusConflict, "CLIENT_ENTRY_ID_REUSED",
	"client_entry_id was already used with a different payload; send a new client_entry_id for a new entry"}

// supersedeConflictError refuses a revision of an entry that already has a newer
// revision: only the latest revision can be superseded, so a stale or retried
// directive never forks or overwrites the newer one.
var supersedeConflictError = &submitError{http.StatusConflict, "SUPERSEDE_CONFLICT",
	"supersedes_entry_id was already superseded by a newer entry; supersede the latest revision instead"}

func (e *submitError) write(w http.ResponseWriter) {
	roomWriteError(w, e.status, e.code, e.message)
}

// submitMessage is the ONE message submission implementation. It validates the write,
// stores exactly one timeline entry (a retry with the same client_entry_id from the same
// author returns the existing entry with created=false; the same key with a different
// payload is 409 CLIENT_ENTRY_ID_REUSED), and on a new entry applies the
// side effects once: message count, room activity, the author's presence heartbeat, the
// activation milestone and the live broadcast.
func (h *RoomMessagesHandler) submitMessage(ctx context.Context, room *models.Room, s messageSubmission) (*models.Message, bool, *submitError) {
	if room.IsArchived() {
		return nil, false, &submitError{http.StatusConflict, "ROOM_ARCHIVED", archivedRoomMessage}
	}
	if s.Content == "" {
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "body is required (content on the legacy message routes)"}
	}
	if len(s.Content) > maxMessageContentLen {
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "body exceeds maximum length of 65536 characters"}
	}
	if s.ContentType == "" {
		s.ContentType = "text"
	}
	if s.AuthorType == "human" && s.ContentType != "text" {
		// D-26: human comments are always plain text.
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "content_type must be text for human messages"}
	}
	if s.ContentType != "text" && s.ContentType != "markdown" && s.ContentType != "json" {
		return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "content_type must be text, markdown, or json"}
	}

	// A revised directive/result may explicitly supersede an earlier message. The
	// reference must resolve to a message in THIS room; a cross-room or unknown
	// reference is rejected rather than silently stored.
	if s.SupersedesEntryID != nil {
		if _, err := h.msgRepo.GetByID(ctx, room.ID, *s.SupersedesEntryID); err != nil {
			if errors.Is(err, db.ErrMessageNotFound) {
				return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", "supersedes_entry_id does not reference a message in this room"}
			}
			slog.Error("failed to validate supersedes_entry_id", "error", err, "room_id", room.ID)
			return nil, false, &submitError{http.StatusInternalServerError, "INTERNAL_ERROR", "failed to validate supersedes reference"}
		}
	}

	params := models.CreateMessageParams{
		RoomID:             room.ID,
		AuthorType:         s.AuthorType,
		AuthorID:           s.AuthorID,
		AgentName:          s.Label,
		Content:            s.Content,
		ContentType:        s.ContentType,
		Metadata:           s.Metadata,
		ReplyToEntryID:     s.ReplyToEntryID,
		AddressedMemberIDs: s.AddressedMemberIDs,
		SupersedesEntryID:  s.SupersedesEntryID,
	}
	if s.ClientEntryID != nil && *s.ClientEntryID != "" {
		params.ClientEntryID = s.ClientEntryID
	}

	msg, created, err := h.storeMessage(ctx, params)
	if err != nil {
		if errors.Is(err, db.ErrInvalidEntryReference) {
			return nil, false, &submitError{http.StatusBadRequest, "VALIDATION_ERROR", invalidEntryReferenceMsg}
		}
		if errors.Is(err, db.ErrClientEntryConflict) {
			return nil, false, clientEntryReusedError
		}
		if errors.Is(err, db.ErrEntryAlreadySuperseded) {
			return nil, false, supersedeConflictError
		}
		slog.Error("failed to create message", "error", err, "room_id", room.ID)
		return nil, false, &submitError{http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create message"}
	}

	// An idempotent replay already had every side effect applied by the original write.
	if created {
		h.afterMessageCreated(ctx, room, msg)
	}
	return msg, created, nil
}

// storeMessage persists the message, using testMsgCreate in unit tests.
func (h *RoomMessagesHandler) storeMessage(ctx context.Context, params models.CreateMessageParams) (*models.Message, bool, error) {
	if h.testMsgCreate != nil {
		msg, err := h.testMsgCreate(ctx, params)
		return msg, err == nil, err
	}
	return h.msgRepo.CreateWithClientEntry(ctx, params)
}

// afterMessageCreated applies the once-per-entry side effects of a new message. Each is
// best-effort: a failed statistic never fails a stored message.
func (h *RoomMessagesHandler) afterMessageCreated(ctx context.Context, room *models.Room, msg *models.Message) {
	if h.roomRepo != nil {
		// D-30: message count; plus the room activity timestamp.
		if err := h.roomRepo.IncrementMessageCount(ctx, room.ID); err != nil {
			slog.Error("failed to increment message count", "error", err, "room_id", room.ID)
		}
		if err := h.roomRepo.UpdateActivity(ctx, room.ID); err != nil {
			slog.Error("failed to update room activity", "error", err, "room_id", room.ID)
		}
	}

	// D-28: implicit heartbeat -- an agent's message renews its own presence.
	if msg.AuthorType == "agent" && msg.AuthorID != nil && h.presenceRepo != nil {
		if _, err := h.presenceRepo.UpdateHeartbeat(ctx, room.ID, *msg.AuthorID); err != nil {
			slog.Error("failed to update heartbeat on message", "error", err, "room_id", room.ID, "agent", *msg.AuthorID)
		}
	}

	h.recordActivationMilestone(ctx, room)

	if h.hubMgr != nil {
		h.hubMgr.Publish(hub.NewRoomID(room.ID), messageHubEvent(msg))
	}
}

// messageHubEvent is the stream frame for a message, shared by the live broadcast and the
// SSE reconnect replay so a replayed message is identical to the one delivered live.
func messageHubEvent(msg *models.Message) hub.RoomEvent {
	evt := hub.RoomEvent{
		ID:        msg.ID,
		Type:      hub.EventMessage,
		RoomID:    hub.NewRoomID(msg.RoomID),
		AgentName: msg.AgentName,
		Payload:   msg,
		Timestamp: msg.CreatedAt,
	}
	if msg.SequenceNum != nil {
		evt.Sequence = *msg.SequenceNum
	}
	return evt
}

// messageFromEntry renders a message entry in the message envelope, column for column as
// the messages view maps room_entries.
func messageFromEntry(e *models.RoomEntry) *models.Message {
	seq := e.Sequence
	msg := &models.Message{
		ID:                 e.ID,
		RoomID:             e.RoomID,
		AuthorID:           e.AuthorID,
		AgentName:          e.ActorLabel,
		ContentType:        e.ContentType,
		Metadata:           e.Extension,
		ReplyToEntryID:     e.ReplyToEntryID,
		AddressedMemberIDs: e.AddressedMemberIDs,
		SequenceNum:        &seq,
		PinnedAt:           e.PinnedAt,
		SupersedesEntryID:  e.SupersedesEntryID,
		CreatedAt:          e.CreatedAt,
	}
	if e.AuthorType != nil {
		msg.AuthorType = *e.AuthorType
	}
	if e.Body != nil {
		msg.Content = *e.Body
	}
	return msg
}
