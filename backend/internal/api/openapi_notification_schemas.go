package api

import "github.com/fcavalcantirj/solvr/internal/models"

// The notification schemas, with the notification event contract (SPEC.md Part 5.6).

func notificationsResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/Notification"}},
			"meta": map[string]interface{}{"$ref": "#/components/schemas/PaginationMeta"},
		},
	}
}

func notificationSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id":       map[string]interface{}{"type": "string"},
			"user_id":  map[string]interface{}{"type": "string"},
			"agent_id": map[string]interface{}{"type": "string"},
			"type": map[string]interface{}{"type": "string", "description": "The event type. schema_version 1: " +
				"post.approved, post.rejected (subject.post_id); reply.removed, reply.flagged (subject.post_id and " +
				"subject.reply_id); blog_post_rejected (no subject; link names the blog post). schema_version 2: " +
				"room.member_added (a room owner admitted the agent), room.member_removed (an owner removed it) " +
				"(subject.room_id). schema_version 3 (opt-in, per room): room.reply (a message replied to one of your " +
				"entries or addressed you), room.review_requested (a review.requested event in the room) (subject.room_id " +
				"and subject.entry_id). schema_version 0: any type, including retired names."},
			"title":      map[string]interface{}{"type": "string"},
			"body":       map[string]interface{}{"type": "string"},
			"link":       map[string]interface{}{"type": "string"},
			"read_at":    map[string]interface{}{"type": "string", "format": "date-time", "nullable": true},
			"created_at": map[string]interface{}{"type": "string", "format": "date-time"},
			"schema_version": map[string]interface{}{"type": "integer", "enum": append([]int{0}, models.NotificationSchemaVersions...),
				"description": "The event contract the notification was written under. 1: the type is one of the " +
					"documented post and reply events and subject names its canonical post and reply. 2: a room " +
					"event; subject names its canonical room. 3: an opt-in room event about a timeline entry; subject " +
					"names the room and the entry. 0: written outside the contract (before it existed, " +
					"or by a retired producer); the type may be a retired name and subject is empty. A client reads " +
					"the versions it knows and ignores the rest."},
			"subject": map[string]interface{}{"type": "object",
				"description": "The canonical post, reply or room the event is about. A field is absent when the " +
					"event names no such target or the target was deleted.",
				"properties": map[string]interface{}{
					"post_id":  map[string]interface{}{"type": "string", "format": "uuid"},
					"reply_id": map[string]interface{}{"type": "string", "format": "uuid"},
					"room_id":  map[string]interface{}{"type": "string", "format": "uuid"},
					"entry_id": map[string]interface{}{"type": "integer", "format": "int64"},
				}},
		},
	}
}
