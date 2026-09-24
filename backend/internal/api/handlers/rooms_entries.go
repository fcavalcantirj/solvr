package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// Entry page bounds for GET /v1/rooms/{slug}/entries.
const (
	defaultEntryPageLimit = 50
	maxEntryPageLimit     = 100
)

// entryCursorPrefix versions the opaque cursor so its encoding can change later.
const entryCursorPrefix = "seq:"

// RoomEntriesHandler serves the canonical room timeline:
//
//	GET  /v1/rooms/{slug}/entries             ordered messages and events, cursor paged
//	POST /v1/rooms/{slug}/entries             submit a message or typed event entry
//	GET  /v1/rooms/{slug}/entries/{entry_id}  one entry of this room
//
// RoomPolicyGuard runs first, so the room and the authenticated RoomActor are in context.
// Message writes go through RoomMessagesHandler.submitMessage, the same implementation
// the /r/{slug}/message and /v1/rooms/{slug}/messages adapters use; event writes go
// through RoomEventsHandler.submitEvent, shared with the /r/{slug}/events adapter.
type RoomEntriesHandler struct {
	entryRepo *db.RoomEntryRepository
	messages  *RoomMessagesHandler
	events    *RoomEventsHandler
}

// NewRoomEntriesHandler creates the canonical entries handler over the shared
// submission paths.
func NewRoomEntriesHandler(entryRepo *db.RoomEntryRepository, messages *RoomMessagesHandler, events *RoomEventsHandler) *RoomEntriesHandler {
	return &RoomEntriesHandler{entryRepo: entryRepo, messages: messages, events: events}
}

// postEntryRequest is the JSON body for POST /v1/rooms/{slug}/entries. kind is
// "message" (the default: body, content_type, references) or "event" (event_type,
// issue); extension carries message metadata or the event payload.
type postEntryRequest struct {
	Kind               string          `json:"kind,omitempty"`
	EventType          string          `json:"event_type,omitempty"`
	Issue              string          `json:"issue,omitempty"`
	Body               string          `json:"body"`
	ContentType        string          `json:"content_type,omitempty"`
	Extension          json.RawMessage `json:"extension,omitempty"`
	ReplyToEntryID     *int64          `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs json.RawMessage `json:"addressed_member_ids,omitempty"`
	SupersedesEntryID  *int64          `json:"supersedes_entry_id,omitempty"`
	ClientEntryID      *string         `json:"client_entry_id,omitempty"`
}

// PostEntry handles POST /v1/rooms/{slug}/entries. Returns 201 with the stored entry,
// or 200 with meta.idempotent_replay=true when client_entry_id matched an entry this
// actor already stored (through any route).
func (h *RoomEntriesHandler) PostEntry(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	actor := apimiddleware.RoomActorFromContext(r.Context())
	if room == nil || actor == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req postEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}
	authorID := actor.ID
	var entryID int64
	var created bool
	switch req.Kind {
	case "", models.RoomEntryKindMessage:
		msg, ok, serr := h.messages.submitMessage(r.Context(), room, messageSubmission{
			AuthorType:         actor.Type,
			AuthorID:           &authorID,
			Label:              actor.Label,
			Content:            req.Body,
			ContentType:        req.ContentType,
			Metadata:           req.Extension,
			ReplyToEntryID:     req.ReplyToEntryID,
			AddressedMemberIDs: req.AddressedMemberIDs,
			SupersedesEntryID:  req.SupersedesEntryID,
			ClientEntryID:      req.ClientEntryID,
		})
		if serr != nil {
			serr.write(w)
			return
		}
		entryID, created = msg.ID, ok
	case models.RoomEntryKindEvent:
		entry, ok, serr := h.events.submitEvent(r.Context(), room, eventSubmission{
			AuthorType:    actor.Type,
			AuthorID:      &authorID,
			Label:         actor.Label,
			EventType:     req.EventType,
			Issue:         req.Issue,
			Payload:       req.Extension,
			ClientEntryID: req.ClientEntryID,
		})
		if serr != nil {
			serr.write(w)
			return
		}
		entryID, created = entry.ID, ok
	default:
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "kind must be message or event")
		return
	}

	entry, err := h.entryRepo.GetByID(r.Context(), room.ID, entryID)
	if err != nil {
		slog.Error("failed to read stored entry", "error", err, "room_id", room.ID, "entry_id", entryID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read stored entry")
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	roomWriteJSON(w, status, map[string]any{
		"data": entry,
		"meta": map[string]any{"idempotent_replay": !created},
	})
}

// ListEntries handles GET /v1/rooms/{slug}/entries?cursor=&limit=&kind=. Entries are in
// ascending timeline (sequence) order; limit defaults to 50 and is clamped to 100.
// meta.next_cursor is an opaque cursor for the following page, null when has_more=false.
func (h *RoomEntriesHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	q := r.URL.Query()

	kind := q.Get("kind")
	if kind != "" && kind != models.RoomEntryKindMessage && kind != models.RoomEntryKindEvent {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "kind must be message or event")
		return
	}
	after := 0
	if c := q.Get("cursor"); c != "" {
		seq, ok := decodeEntryCursor(c)
		if !ok {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid cursor")
			return
		}
		after = seq
	}
	limit := defaultEntryPageLimit
	if l := q.Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed <= 0 {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be a positive integer")
			return
		}
		limit = min(parsed, maxEntryPageLimit)
	}

	entries, err := h.entryRepo.ListPage(r.Context(), room.ID, after, kind, limit+1)
	if err != nil {
		slog.Error("failed to list room entries", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list entries")
		return
	}
	hasMore := len(entries) > limit
	var nextCursor any
	if hasMore {
		entries = entries[:limit]
		nextCursor = encodeEntryCursor(entries[len(entries)-1].Sequence)
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": entries,
		"meta": map[string]any{"limit": limit, "has_more": hasMore, "next_cursor": nextCursor},
	})
}

// GetEntry handles GET /v1/rooms/{slug}/entries/{entry_id}. The lookup is room-scoped,
// so another room's entry id is 404.
func (h *RoomEntriesHandler) GetEntry(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "entry_id"), 10, 64)
	if err != nil || id <= 0 {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid entry id")
		return
	}
	entry, err := h.entryRepo.GetByID(r.Context(), room.ID, id)
	if err != nil {
		if errors.Is(err, db.ErrRoomEntryNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "entry not found")
			return
		}
		slog.Error("failed to get room entry", "error", err, "room_id", room.ID, "entry_id", id)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get entry")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": entry})
}

func encodeEntryCursor(sequence int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(entryCursorPrefix + strconv.Itoa(sequence)))
}

func decodeEntryCursor(cursor string) (int, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || !strings.HasPrefix(string(raw), entryCursorPrefix) {
		return 0, false
	}
	seq, err := strconv.Atoi(strings.TrimPrefix(string(raw), entryCursorPrefix))
	if err != nil || seq < 0 {
		return 0, false
	}
	return seq, true
}
