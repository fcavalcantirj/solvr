package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// maxEventPayloadBytes bounds a typed-event payload (matches the DB CHECK).
const maxEventPayloadBytes = 16384

// RoomEventsHandler serves typed room events (mission #4) on the A2A namespace. Its
// routes are transport adapters over the canonical timeline: writes go through
// submitEvent (shared with POST /v1/rooms/{slug}/entries kind=event) and reads come from
// the same room_entries rows. All handlers run behind BearerGuard, so the room is in
// context.
type RoomEventsHandler struct {
	entryRepo *db.RoomEntryRepository
	hubMgr    *hub.HubManager
}

// NewRoomEventsHandler creates a new RoomEventsHandler.
func NewRoomEventsHandler(entryRepo *db.RoomEntryRepository, hubMgr *hub.HubManager) *RoomEventsHandler {
	return &RoomEventsHandler{entryRepo: entryRepo, hubMgr: hubMgr}
}

type postEventRequest struct {
	Type          string          `json:"type"`
	Issue         string          `json:"issue,omitempty"`
	Actor         string          `json:"actor"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	ClientEntryID *string         `json:"client_entry_id,omitempty"`
}

// PostEvent handles POST /r/{slug}/events — the legacy adapter into submitEvent.
// Body: {type, issue?, actor, payload?, client_entry_id?}. The per-agent room token's
// agent is the authoritative author; actor is kept as the display label. Returns 201
// with the event in the legacy shape, or 200 with idempotent_replay=true on a retry.
func (h *RoomEventsHandler) PostEvent(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	var req postEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	sub := eventSubmission{
		AuthorType:    "agent",
		Label:         req.Actor,
		EventType:     req.Type,
		Issue:         req.Issue,
		Payload:       req.Payload,
		ClientEntryID: req.ClientEntryID,
	}
	if authAgentID := apimiddleware.RoomAgentIDFromContext(r.Context()); authAgentID != "" {
		sub.AuthorID = &authAgentID
	}

	entry, created, serr := h.submitEvent(r.Context(), room, sub)
	if serr != nil {
		serr.write(w)
		return
	}
	event := roomEventFromEntry(entry)
	if !created {
		roomWriteJSON(w, http.StatusOK, map[string]any{"data": event, "idempotent_replay": true})
		return
	}
	roomWriteJSON(w, http.StatusCreated, map[string]any{"data": event})
}

// ListEvents handles GET /r/{slug}/events?type=&issue=&cursor=&limit= — an adapter over
// the event entries of the canonical timeline. Events page oldest to newest from the
// opaque cursor with the canonical default 50 / maximum 100 limits and meta.next_cursor /
// has_more; every event matching the filters within a page is returned. Items keep the
// legacy shape (id, type, issue, actor, payload, created_at) plus the room sequence. Any
// other query parameter is a 400.
func (h *RoomEventsHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}
	q := r.URL.Query()
	if !rejectUnknownQuery(w, q, "type", "issue", "cursor", "limit") {
		return
	}
	after, limit, ok := parseEntryPage(w, q)
	if !ok {
		return
	}
	entries, hasMore, nextCursor, err := listEntryPage(r.Context(), h.entryRepo, models.RoomEntryPageParams{
		RoomID:        room.ID,
		AfterSequence: after,
		Kind:          models.RoomEntryKindEvent,
		EventType:     q.Get("type"),
		Issue:         q.Get("issue"),
		Limit:         limit,
	})
	if err != nil {
		slog.Error("failed to query room events", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to query events")
		return
	}
	events := make([]models.RoomEvent, 0, len(entries))
	for i := range entries {
		events = append(events, roomEventFromEntry(&entries[i]))
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": events,
		"meta": map[string]any{"limit": limit, "has_more": hasMore, "next_cursor": nextCursor},
	})
}
