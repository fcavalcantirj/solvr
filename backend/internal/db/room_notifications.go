package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Opt-in room notifications (idx 92 step 3; SPEC.md Part 5.6, schema version 3).
//
// A person or agent opts in per room (room_notification_subscriptions); a global pause
// (users/agents.room_notifications_paused_at) silences every room at once. When an entry is
// stored, RecordForEntry tells only the recipients the entry is actually for:
//
//   - room.reply: the author of the entry a message replied to, and every member a message
//     addressed (addressed_member_ids);
//   - room.review_requested: every subscriber, for a review.requested event.
//
// and only those who opted in to the room, are not paused, can read the room and are not the
// entry's author. Nothing else — a heartbeat, a join, a pin, any other event — notifies.

// NotificationSubscriber is the person (UserID) or agent (AgentID) a subscription belongs to.
type NotificationSubscriber struct {
	UserID  *uuid.UUID
	AgentID string
}

func (s NotificationSubscriber) columns() (any, any) {
	var agent any
	if s.AgentID != "" {
		agent = s.AgentID
	}
	return s.UserID, agent
}

func (s NotificationSubscriber) valid() bool { return (s.UserID != nil) != (s.AgentID != "") }

// RoomNotificationRepository stores room opt-ins and records room events for them.
type RoomNotificationRepository struct {
	pool *Pool
}

// NewRoomNotificationRepository creates the room notification store.
func NewRoomNotificationRepository(pool *Pool) *RoomNotificationRepository {
	return &RoomNotificationRepository{pool: pool}
}

// ErrInvalidSubscriber is returned for a subscriber that names both or neither identity.
var ErrInvalidSubscriber = fmt.Errorf("a room notification subscriber is one person or one agent")

// Subscribe opts the subscriber in to the room. Idempotent.
func (r *RoomNotificationRepository) Subscribe(ctx context.Context, roomID uuid.UUID, s NotificationSubscriber) error {
	if !s.valid() {
		return ErrInvalidSubscriber
	}
	user, agent := s.columns()
	_, err := r.pool.Exec(ctx, `INSERT INTO room_notification_subscriptions (room_id, user_id, agent_id)
		VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, roomID, user, agent)
	if err != nil {
		LogQueryError(ctx, "Subscribe", "room_notification_subscriptions", err)
	}
	return err
}

// Unsubscribe is the per-room off. Idempotent.
func (r *RoomNotificationRepository) Unsubscribe(ctx context.Context, roomID uuid.UUID, s NotificationSubscriber) error {
	if !s.valid() {
		return ErrInvalidSubscriber
	}
	user, agent := s.columns()
	_, err := r.pool.Exec(ctx, `DELETE FROM room_notification_subscriptions
		WHERE room_id = $1 AND (user_id = $2::uuid OR agent_id = $3::text)`, roomID, user, agent)
	if err != nil {
		LogQueryError(ctx, "Unsubscribe", "room_notification_subscriptions", err)
	}
	return err
}

// IsSubscribed reports whether the subscriber opted in to the room.
func (r *RoomNotificationRepository) IsSubscribed(ctx context.Context, roomID uuid.UUID, s NotificationSubscriber) (bool, error) {
	if !s.valid() {
		return false, ErrInvalidSubscriber
	}
	user, agent := s.columns()
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM room_notification_subscriptions
		WHERE room_id = $1 AND (user_id = $2::uuid OR agent_id = $3::text))`, roomID, user, agent).Scan(&ok)
	return ok, err
}

// SetPaused sets or lifts the global pause of every room notification for the subscriber.
func (r *RoomNotificationRepository) SetPaused(ctx context.Context, s NotificationSubscriber, paused bool) error {
	if !s.valid() {
		return ErrInvalidSubscriber
	}
	expr := "NULL"
	if paused {
		expr = "COALESCE(room_notifications_paused_at, NOW())"
	}
	var err error
	if s.UserID != nil {
		_, err = r.pool.Exec(ctx, `UPDATE users SET room_notifications_paused_at = `+expr+` WHERE id = $1`, *s.UserID)
	} else {
		_, err = r.pool.Exec(ctx, `UPDATE agents SET room_notifications_paused_at = `+expr+` WHERE id = $1`, s.AgentID)
	}
	if err != nil {
		LogQueryError(ctx, "SetPaused", "room_notifications_paused_at", err)
	}
	return err
}

// IsPaused reports whether the subscriber paused every room notification.
func (r *RoomNotificationRepository) IsPaused(ctx context.Context, s NotificationSubscriber) (bool, error) {
	if !s.valid() {
		return false, ErrInvalidSubscriber
	}
	var paused bool
	var err error
	if s.UserID != nil {
		err = r.pool.QueryRow(ctx, `SELECT room_notifications_paused_at IS NOT NULL FROM users WHERE id = $1`, *s.UserID).Scan(&paused)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT room_notifications_paused_at IS NOT NULL FROM agents WHERE id = $1`, s.AgentID).Scan(&paused)
	}
	return paused, err
}

// RoomEntryNotice describes a just-stored room entry for RecordForEntry.
type RoomEntryNotice struct {
	Room           *models.Room
	EntryID        int64
	Kind           string // models.RoomEntryKindMessage | models.RoomEntryKindEvent
	EventType      string
	AuthorType     string // "agent" | "human"
	AuthorID       string
	Label          string // the author's display label
	ReplyToEntryID *int64
	Addressed      []string // addressed_member_ids: agent ids or user UUIDs
}

// ReviewRequestedEventType is the typed event that asks the room for a review.
const ReviewRequestedEventType = "review.requested"

type noticeRecipient struct {
	typ, id string // "human" (user UUID) | "agent" (agent id)
}

// RecordForEntry records the schema version 3 events the entry calls for and returns how many
// were recorded. Each is conditional on the recipient's opt-in, pause, room readability and
// on not being the author, and is recorded once per (entry, type, recipient).
func (r *RoomNotificationRepository) RecordForEntry(ctx context.Context, n RoomEntryNotice) (int, error) {
	if n.Room == nil {
		return 0, nil
	}
	eventType, recipients, err := r.recipientsFor(ctx, n)
	if err != nil || eventType == "" {
		return 0, err
	}
	recorded := 0
	for _, rc := range recipients {
		if rc.typ == n.AuthorType && rc.id == n.AuthorID {
			continue
		}
		ok, err := r.recordOne(ctx, n, eventType, rc)
		if err != nil {
			return recorded, err
		}
		if ok {
			recorded++
		}
	}
	return recorded, nil
}

func (r *RoomNotificationRepository) recipientsFor(ctx context.Context, n RoomEntryNotice) (string, []noticeRecipient, error) {
	seen := map[noticeRecipient]bool{}
	var out []noticeRecipient
	add := func(rc noticeRecipient) {
		if rc.id != "" && !seen[rc] {
			seen[rc] = true
			out = append(out, rc)
		}
	}
	switch {
	case n.Kind == models.RoomEntryKindMessage:
		if n.ReplyToEntryID != nil {
			var typ, id string
			err := r.pool.QueryRow(ctx, `SELECT COALESCE(author_type, ''), COALESCE(author_id, '')
				FROM room_entries WHERE id = $1 AND room_id = $2`, *n.ReplyToEntryID, n.Room.ID).Scan(&typ, &id)
			if err == nil && (typ == "human" || typ == "agent") {
				add(noticeRecipient{typ, id})
			}
		}
		for _, a := range n.Addressed {
			if _, err := uuid.Parse(a); err == nil {
				add(noticeRecipient{"human", a})
			} else {
				add(noticeRecipient{"agent", a})
			}
		}
		return models.NotificationRoomReply, out, nil
	case n.Kind == models.RoomEntryKindEvent && strings.EqualFold(n.EventType, ReviewRequestedEventType):
		rows, err := r.pool.Query(ctx, `SELECT COALESCE(user_id::text, ''), COALESCE(agent_id, '')
			FROM room_notification_subscriptions WHERE room_id = $1 ORDER BY id`, n.Room.ID)
		if err != nil {
			return "", nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var user, agent string
			if err := rows.Scan(&user, &agent); err != nil {
				return "", nil, err
			}
			if user != "" {
				add(noticeRecipient{"human", user})
			} else {
				add(noticeRecipient{"agent", agent})
			}
		}
		return models.NotificationRoomReviewRequested, out, rows.Err()
	}
	return "", nil, nil
}

// recordOne inserts one event for one recipient when every condition holds, queuing the
// agent's subscribed webhooks in the same statement.
func (r *RoomNotificationRepository) recordOne(ctx context.Context, n RoomEntryNotice, eventType string, rc noticeRecipient) (bool, error) {
	var user, agent any
	if rc.typ == "human" {
		id, err := uuid.Parse(rc.id)
		if err != nil {
			return false, nil
		}
		user = id
	} else {
		agent = rc.id
	}
	title, body := roomNoticeText(n, eventType)
	link := "/rooms/" + n.Room.Slug
	if n.Kind == models.RoomEntryKindMessage {
		link += "?message=" + strconv.FormatInt(n.EntryID, 10)
	}
	query := `
		WITH created AS (
			INSERT INTO notifications (user_id, agent_id, type, title, body, link, schema_version, room_id, entry_id)
			SELECT $1::uuid, $2::text, $3, $4, $5, $6, 3, $7, $8
			 WHERE EXISTS (SELECT 1 FROM room_notification_subscriptions s
			                WHERE s.room_id = $7 AND (s.user_id = $1::uuid OR s.agent_id = $2::text))
			   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = $1::uuid AND u.room_notifications_paused_at IS NOT NULL)
			   AND NOT EXISTS (SELECT 1 FROM agents a WHERE a.id = $2::text AND a.room_notifications_paused_at IS NOT NULL)
			   AND EXISTS (SELECT 1 FROM rooms rm WHERE rm.id = $7 AND rm.deleted_at IS NULL AND (
			         NOT rm.is_private
			      OR EXISTS (SELECT 1 FROM room_members m WHERE m.room_id = rm.id AND m.revoked_at IS NULL
			                  AND (m.user_id = $1::uuid OR m.agent_id = $2::text))
			      OR EXISTS (SELECT 1 FROM agents fa JOIN room_members fm ON fm.user_id = fa.human_id
			                  WHERE fa.id = $2::text AND fm.room_id = rm.id AND fm.role = 'owner' AND fm.revoked_at IS NULL)
			      OR EXISTS (SELECT 1 FROM users au WHERE au.id = $1::uuid AND au.role = 'admin')))
			ON CONFLICT DO NOTHING
			RETURNING *
		), queued AS (` + queueWebhookDeliveries + `)
		SELECT COUNT(*) FROM created`
	var count int
	if err := r.pool.QueryRow(ctx, query, user, agent, eventType, title, body, link, n.Room.ID, n.EntryID).Scan(&count); err != nil {
		LogQueryError(ctx, "RecordForEntry", "notifications", err)
		return false, fmt.Errorf("record room notification: %w", err)
	}
	return count > 0, nil
}

// roomNoticeText names the room and who acted — never the entry's body — and the per-room off.
func roomNoticeText(n RoomEntryNotice, eventType string) (string, string) {
	off := " Stop these: turn off notifications for this room (DELETE /v1/rooms/" + n.Room.Slug +
		"/notifications), or pause every room notification in your notification settings."
	who := n.Label
	if who == "" {
		who = "A participant"
	}
	if eventType == models.NotificationRoomReviewRequested {
		return "Review requested in " + n.Room.DisplayName,
			fmt.Sprintf("%s asked for a review in %q. Open the room to resume the work.", who, n.Room.DisplayName) + off
	}
	return "New reply in " + n.Room.DisplayName,
		fmt.Sprintf("%s replied to you in %q. Open the room to resume the work.", who, n.Room.DisplayName) + off
}
