package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Pinned directives on the canonical room API (idx 92 step 1).
//
//	POST|DELETE /v1/rooms/{slug}/entries/{entry_id}/pin   pin / unpin a message entry
//	GET         /v1/rooms/{slug}/viewer                   what this caller may do here
//
// Pinning marks the instruction in force. It is open to the room's participants — a
// room token of this room, a member agent or family owner, a human admin or member — and
// NOT to every signed-in human a public room lets write. Every pin change, on this route
// or the /r/{slug} adapter, publishes a room_update frame naming the directive now in
// force, so open pages and agents see it without polling.

// RoomPinHandler serves the canonical pin routes and the viewer capabilities.
type RoomPinHandler struct {
	msgRepo    *db.MessageRepository
	memberRepo *db.RoomMemberRepository
	hubMgr     *hub.HubManager
}

// NewRoomPinHandler wires the pin routes.
func NewRoomPinHandler(msgRepo *db.MessageRepository, memberRepo *db.RoomMemberRepository, hubMgr *hub.HubManager) *RoomPinHandler {
	return &RoomPinHandler{msgRepo: msgRepo, memberRepo: memberRepo, hubMgr: hubMgr}
}

// PinEntry handles POST /v1/rooms/{slug}/entries/{entry_id}/pin. Idempotent.
func (h *RoomPinHandler) PinEntry(w http.ResponseWriter, r *http.Request) { h.setPin(w, r, true) }

// UnpinEntry handles DELETE /v1/rooms/{slug}/entries/{entry_id}/pin. Idempotent.
func (h *RoomPinHandler) UnpinEntry(w http.ResponseWriter, r *http.Request) { h.setPin(w, r, false) }

func (h *RoomPinHandler) setPin(w http.ResponseWriter, r *http.Request, pin bool) {
	room := apimiddleware.RoomFromContext(r.Context())
	actor := apimiddleware.RoomActorFromContext(r.Context())
	if room == nil || actor == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	allowed, err := canPinRoom(r.Context(), room, actor, h.memberRepo)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check membership")
		return
	}
	if !allowed {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room's participants can pin a directive")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "entry_id"), 10, 64)
	if err != nil || id <= 0 {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "entry_id must be a positive integer")
		return
	}

	var msg *models.Message
	if pin {
		msg, err = h.msgRepo.Pin(r.Context(), room.ID, id)
	} else {
		msg, err = h.msgRepo.Unpin(r.Context(), room.ID, id)
	}
	if err != nil {
		if errors.Is(err, db.ErrMessageNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "no message entry with this id in this room")
			return
		}
		slog.Error("failed to set entry pin", "error", err, "room_id", room.ID, "entry_id", id)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update pin")
		return
	}
	latest := publishPinChange(r.Context(), h.hubMgr, h.msgRepo, room, msg)
	// meta.latest_pinned is the directive now in force (a revision may stand in for the
	// pinned entry), so a page updates its context without another read.
	roomWriteJSON(w, http.StatusOK, map[string]interface{}{
		"data": msg,
		"meta": map[string]interface{}{"latest_pinned": latest},
	})
}

// GetViewer handles GET /v1/rooms/{slug}/viewer (room policy: read): what the calling
// person or agent may do in this room, decided here so the page only renders controls.
func (h *RoomPinHandler) GetViewer(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}
	canPin := false
	if actor := apimiddleware.RoomActorFromContext(r.Context()); actor != nil {
		ok, err := canPinRoom(r.Context(), room, actor, h.memberRepo)
		if err != nil {
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check membership")
			return
		}
		canPin = ok
	}
	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"can_pin": canPin}})
}

// canPinRoom: a room token of this room, a member agent or family owner, a human admin, or
// a human member (owner included). Writing to a public room is not enough.
func canPinRoom(ctx context.Context, room *models.Room, actor *apimiddleware.RoomActor, members *db.RoomMemberRepository) (bool, error) {
	if actor.Credential == apimiddleware.RoomCredentialRoomToken {
		return true, nil
	}
	if members == nil {
		return false, nil
	}
	switch actor.Type {
	case apimiddleware.RoomActorAgent:
		if ok, err := members.IsMember(ctx, room.ID, actor.ID); err != nil || ok {
			return ok, err
		}
		return members.IsFamilyOwner(ctx, room.ID, actor.ID)
	case apimiddleware.RoomActorHuman:
		if actor.Admin {
			return true, nil
		}
		return members.IsUserMember(ctx, room.ID, actor.ID)
	}
	return false, nil
}

// publishPinChange announces a pin change as a room_update frame: which entry changed,
// whether it is now pinned, and the directive now in force (LatestDirective), or null —
// and returns that directive. The frame is instance-local (see below).
func publishPinChange(ctx context.Context, hubMgr *hub.HubManager, msgs *db.MessageRepository, room *models.Room, changed *models.Message) *models.Message {
	var latest *models.Message
	var latestID interface{}
	if d, err := msgs.LatestDirective(ctx, room.ID); err == nil {
		latest, latestID = d, d.ID
	} else if !errors.Is(err, db.ErrMessageNotFound) {
		slog.Warn("pin change: latest directive unavailable", "error", err, "room_id", room.ID)
	}
	if hubMgr == nil || changed == nil {
		return latest
	}
	// Not hubMgr.Publish: with the relay on, Publish replays committed timeline entries
	// only, and a pin change is not an entry. The frame goes to this instance's streams;
	// another instance's streams see the new directive on their next room read.
	roomHub := hubMgr.Get(hub.NewRoomID(room.ID))
	if roomHub == nil {
		return latest // nobody is streaming this room here
	}
	roomHub.Broadcast(hub.RoomEvent{
		Type:   hub.EventRoomUpdate,
		RoomID: hub.NewRoomID(room.ID),
		Payload: map[string]interface{}{
			"change":           "pin",
			"pinned_entry_id":  changed.ID,
			"pinned":           changed.PinnedAt != nil,
			"latest_pinned_id": latestID,
		},
		Timestamp: time.Now().UTC(),
	})
	return latest
}
