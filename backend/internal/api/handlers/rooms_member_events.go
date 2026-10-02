package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// memberNotifyTimeout bounds recording one membership event after the change committed.
const memberNotifyTimeout = 5 * time.Second

// SetMemberNotifier makes room membership changes notify the agent they concern (SPEC.md
// Part 5.6, schema version 2): room.member_added when a room owner admits it (a new or
// readmitted membership), room.member_removed when an owner removes it. Each names the room
// in subject.room_id and reaches the agent's webhooks subscribed to it. Optional: with no
// notifier wired, membership changes notify nobody.
func (h *RoomHandler) SetMemberNotifier(notify ContributionNotifier) {
	h.memberNotify = notify
}

// notifyMember records eventType for agentID about room, unless the agent made the change
// itself. Best-effort: the membership change has committed; a lost event is logged.
func (h *RoomHandler) notifyMember(r *http.Request, room *models.Room, agentID, eventType, role string) {
	if h.memberNotify == nil || managerIdentity(r) == agentID {
		return
	}
	roomID := room.ID.String()
	n := &models.Notification{AgentID: &agentID, Type: eventType, Link: "/rooms/" + room.Slug,
		SchemaVersion: models.NotificationRoomSchemaVersion, Subject: models.NotificationSubject{RoomID: &roomID}}
	if eventType == models.NotificationRoomMemberAdded {
		n.Title = "You were added to a room"
		n.Body = fmt.Sprintf("You were added to %q with the %s role. Get your room token with POST /v1/rooms/%s/handshake.",
			room.DisplayName, role, room.Slug)
	} else {
		n.Title = "You were removed from a room"
		n.Body = fmt.Sprintf("You were removed from %q; your room token for it no longer works.", room.DisplayName)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), memberNotifyTimeout)
	defer cancel()
	if _, err := h.memberNotify(ctx, n); err != nil {
		slog.Error("room membership event not recorded", "error", err, "type", eventType, "room_id", roomID, "agent", agentID)
	}
}
