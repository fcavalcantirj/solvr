package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Opt-in room notifications (idx 92 step 3; SPEC.md Part 5.6, schema version 3).
//
//	GET|PUT|DELETE /v1/rooms/{slug}/notifications   this caller's opt-in for the room
//	GET|PATCH      /v1/me/notification-settings     the global pause of every room
//
// Off by default. Only actual replies (room.reply) and requested reviews
// (room.review_requested) are ever recorded, in-app and on the agent's subscribed webhooks;
// agent heartbeats, joins and pins never notify. No email is sent.

// roomNotifyTimeout bounds recording one entry's events after the entry committed.
const roomNotifyTimeout = 5 * time.Second

var roomNotificationEvents = []string{models.NotificationRoomReply, models.NotificationRoomReviewRequested}

// RoomNotificationHandler serves the opt-in and the pause.
type RoomNotificationHandler struct {
	repo *db.RoomNotificationRepository
}

// NewRoomNotificationHandler wires the opt-in routes.
func NewRoomNotificationHandler(repo *db.RoomNotificationRepository) *RoomNotificationHandler {
	return &RoomNotificationHandler{repo: repo}
}

// roomNotificationState is what GET/PUT/DELETE /v1/rooms/{slug}/notifications answer.
type roomNotificationState struct {
	Subscribed bool     `json:"subscribed"`
	Paused     bool     `json:"paused"`
	Events     []string `json:"events"`
	Off        string   `json:"off"`
}

// GetRoomState handles GET /v1/rooms/{slug}/notifications.
func (h *RoomNotificationHandler) GetRoomState(w http.ResponseWriter, r *http.Request) {
	h.roomState(w, r, nil)
}

// Subscribe handles PUT /v1/rooms/{slug}/notifications: opt in. Idempotent.
func (h *RoomNotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	on := true
	h.roomState(w, r, &on)
}

// Unsubscribe handles DELETE /v1/rooms/{slug}/notifications: the per-room off. Idempotent.
func (h *RoomNotificationHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	off := false
	h.roomState(w, r, &off)
}

func (h *RoomNotificationHandler) roomState(w http.ResponseWriter, r *http.Request, set *bool) {
	room := apimiddleware.RoomFromContext(r.Context())
	sub, ok := subscriberFromActor(apimiddleware.RoomActorFromContext(r.Context()))
	if room == nil || !ok {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "sign in, or use your agent credential, to manage room notifications")
		return
	}
	ctx := r.Context()
	if set != nil {
		var err error
		if *set {
			err = h.repo.Subscribe(ctx, room.ID, sub)
		} else {
			err = h.repo.Unsubscribe(ctx, room.ID, sub)
		}
		if err != nil {
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update room notifications")
			return
		}
	}
	state, err := h.state(ctx, room, sub)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read room notifications")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": state})
}

func (h *RoomNotificationHandler) state(ctx context.Context, room *models.Room, sub db.NotificationSubscriber) (roomNotificationState, error) {
	subscribed, err := h.repo.IsSubscribed(ctx, room.ID, sub)
	if err != nil {
		return roomNotificationState{}, err
	}
	paused, err := h.repo.IsPaused(ctx, sub)
	if err != nil {
		return roomNotificationState{}, err
	}
	return roomNotificationState{Subscribed: subscribed, Paused: paused, Events: roomNotificationEvents,
		Off: "DELETE /v1/rooms/" + room.Slug + "/notifications"}, nil
}

// notificationSettings is the global room-notification switch.
type notificationSettings struct {
	RoomNotifications string `json:"room_notifications"` // "on" | "paused"
}

// GetSettings handles GET /v1/me/notification-settings.
func (h *RoomNotificationHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	h.settings(w, r, nil)
}

// PatchSettings handles PATCH /v1/me/notification-settings {"room_notifications": "on"|"paused"}.
func (h *RoomNotificationHandler) PatchSettings(w http.ResponseWriter, r *http.Request) {
	var req notificationSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.RoomNotifications != "on" && req.RoomNotifications != "paused" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", `room_notifications must be "on" or "paused"`)
		return
	}
	paused := req.RoomNotifications == "paused"
	h.settings(w, r, &paused)
}

func (h *RoomNotificationHandler) settings(w http.ResponseWriter, r *http.Request, setPaused *bool) {
	sub, ok := subscriberFromAccount(r)
	if !ok {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	ctx := r.Context()
	if setPaused != nil {
		if err := h.repo.SetPaused(ctx, sub, *setPaused); err != nil {
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update notification settings")
			return
		}
	}
	paused, err := h.repo.IsPaused(ctx, sub)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read notification settings")
		return
	}
	value := "on"
	if paused {
		value = "paused"
	}
	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": notificationSettings{RoomNotifications: value}})
}

// subscriberFromActor names the person or agent a room request is from.
func subscriberFromActor(actor *apimiddleware.RoomActor) (db.NotificationSubscriber, bool) {
	if actor == nil || actor.ID == "" {
		return db.NotificationSubscriber{}, false
	}
	if actor.Type == apimiddleware.RoomActorHuman {
		id, err := uuid.Parse(actor.ID)
		if err != nil {
			return db.NotificationSubscriber{}, false
		}
		return db.NotificationSubscriber{UserID: &id}, true
	}
	return db.NotificationSubscriber{AgentID: actor.ID}, true
}

// subscriberFromAccount names the person or agent of an account request.
func subscriberFromAccount(r *http.Request) (db.NotificationSubscriber, bool) {
	if agent := auth.AgentFromContext(r.Context()); agent != nil {
		return db.NotificationSubscriber{AgentID: agent.ID}, true
	}
	if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
		if id, err := uuid.Parse(claims.UserID); err == nil {
			return db.NotificationSubscriber{UserID: &id}, true
		}
	}
	return db.NotificationSubscriber{}, false
}

// roomEntryNotifier records the events a stored entry calls for (RecordForEntry).
type roomEntryNotifier interface {
	RecordForEntry(ctx context.Context, n db.RoomEntryNotice) (int, error)
}

// notifyRoomEntry runs after an entry committed, once (never on an idempotent replay).
// Best-effort: the entry stands; a lost event is logged.
func notifyRoomEntry(ctx context.Context, notifier roomEntryNotifier, n db.RoomEntryNotice) {
	if notifier == nil || n.Room == nil {
		return
	}
	c, cancel := context.WithTimeout(context.WithoutCancel(ctx), roomNotifyTimeout)
	defer cancel()
	if _, err := notifier.RecordForEntry(c, n); err != nil {
		slog.Error("room notification not recorded", "error", err, "room_id", n.Room.ID, "entry_id", n.EntryID)
	}
}

// messageNotice and eventNotice describe a stored message or event for notifyRoomEntry.
func messageNotice(room *models.Room, msg *models.Message) db.RoomEntryNotice {
	n := db.RoomEntryNotice{Room: room, EntryID: msg.ID, Kind: models.RoomEntryKindMessage,
		AuthorType: msg.AuthorType, Label: msg.AgentName, ReplyToEntryID: msg.ReplyToEntryID}
	if msg.AuthorID != nil {
		n.AuthorID = *msg.AuthorID
	}
	if len(msg.AddressedMemberIDs) > 0 {
		_ = json.Unmarshal(msg.AddressedMemberIDs, &n.Addressed)
	}
	return n
}

func eventNotice(room *models.Room, e *models.RoomEntry) db.RoomEntryNotice {
	n := db.RoomEntryNotice{Room: room, EntryID: e.ID, Kind: models.RoomEntryKindEvent, Label: e.ActorLabel}
	if e.AuthorType != nil {
		n.AuthorType = *e.AuthorType
	}
	if e.AuthorID != nil {
		n.AuthorID = *e.AuthorID
	}
	if e.EventType != nil {
		n.EventType = *e.EventType
	}
	return n
}

// SetRoomNotifier makes new messages record the opt-in room notifications they call for.
func (h *RoomMessagesHandler) SetRoomNotifier(n *db.RoomNotificationRepository) { h.roomNotifier = n }

// SetRoomNotifier makes new typed events record the opt-in room notifications they call for.
func (h *RoomEventsHandler) SetRoomNotifier(n *db.RoomNotificationRepository) { h.roomNotifier = n }
