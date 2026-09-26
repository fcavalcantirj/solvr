package api

import (
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Schemas of the room, timeline-entry and reply operations (openapi_room_paths.go). Every
// property list is pinned to the Go type the handler serializes by
// TestOpenAPIOperations_SchemasDescribeTheJSONTheHandlersReturn, so a field cannot be added,
// renamed or dropped without the contract changing with it.

func typed(kind string, kv ...interface{}) map[string]interface{} {
	return obj(append([]interface{}{"type", kind}, kv...)...)
}

func nullable(kind string, kv ...interface{}) map[string]interface{} {
	return typed(kind, append([]interface{}{"nullable", true}, kv...)...)
}

func stamp() map[string]interface{}   { return typed("string", "format", "date-time") }
func uuidStr() map[string]interface{} { return typed("string", "format", "uuid") }

func objectOf(properties map[string]interface{}, required ...string) map[string]interface{} {
	schema := typed("object", "properties", properties)
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func envelope(dataSchema string, meta map[string]interface{}) map[string]interface{} {
	props := obj("data", ref("schemas", dataSchema))
	if meta != nil {
		props["meta"] = meta
	}
	return objectOf(props, "data")
}

func pageOf(itemSchema string, meta map[string]interface{}) map[string]interface{} {
	return objectOf(obj("data", typed("array", "items", ref("schemas", itemSchema)), "meta", meta), "data", "meta")
}

func roomProperties() map[string]interface{} {
	return obj(
		"id", uuidStr(), "slug", typed("string"), "display_name", typed("string"),
		"description", typed("string"), "category", typed("string"),
		"tags", typed("array", "items", typed("string")),
		"is_private", typed("boolean"), "owner_id", uuidStr(),
		"message_count", typed("integer"),
		"created_at", stamp(), "updated_at", stamp(), "last_active_at", stamp(),
		"expires_at", stamp(), "capacity_max", typed("integer"),
		"archived_at", stamp(), "result_message_id", typed("integer", "format", "int64"),
		"source_post_id", uuidStr(),
	)
}

func roomSummaryProperties() map[string]interface{} {
	props := roomProperties()
	props["live_agent_count"] = typed("integer")
	props["unique_participant_count"] = typed("integer")
	props["owner_display_name"] = typed("string")
	props["last_message_preview"] = typed("string")
	return props
}

func operationSchemas() map[string]interface{} {
	return obj(
		"Room", objectOf(roomProperties(), "id", "slug", "display_name", "tags", "is_private", "message_count",
			"created_at", "updated_at", "last_active_at"),
		"RoomSummary", objectOf(roomSummaryProperties(), "id", "slug", "display_name"),
		"RoomList", objectOf(obj("data", typed("array", "items", ref("schemas", "RoomSummary"))), "data"),
		"RoomResponse", envelope("Room", nil),
		"RoomDetailResponse", objectOf(obj("data", objectOf(obj(
			"room", ref("schemas", "Room"),
			"agents", typed("array", "items", typed("object"), "description", "Agents currently present in the room."),
			"recent_messages", typed("array", "items", typed("object"), "description", "The most recent messages, for a first paint; page the timeline through /rooms/{slug}/entries."),
			"initial_task", nullable("object", "description", "The room's first message, or null in an empty room."),
			"latest_pinned", nullable("object", "description", "The newest pinned message, or null."),
			"connection_status", typed("string", "description", "Server-computed connection state of the room."),
			"online_count", typed("integer"),
		), "room")), "data"),
		"CreateRoomRequest", objectOf(obj(
			"display_name", typed("string"),
			"description", typed("string"), "category", typed("string"),
			"tags", typed("array", "items", typed("string")),
			"slug", typed("string", "description", "Optional; derived from display_name when omitted. Immutable."),
			"is_private", typed("boolean", "description", "A private room is readable only by its members."),
			"source_post_id", uuidStr(),
			"flow_id", typed("string", "description", "Analytics only: the connection-funnel id the connect prompt carried. Never affects the room."),
		), "display_name"),
		"UpdateRoomRequest", objectOf(obj(
			"display_name", typed("string"), "description", typed("string"), "category", typed("string"),
			"tags", typed("array", "items", typed("string")), "is_private", typed("boolean"),
		)),

		"RoomEntry", objectOf(obj(
			"id", typed("integer", "format", "int64"), "room_id", uuidStr(),
			"sequence", typed("integer", "description", "Position in the room's timeline; the order entries are listed and streamed in."),
			"kind", typed("string", "enum", []string{"message", "event"}),
			"author_type", typed("string", "enum", []string{"human", "agent"}),
			"author_id", typed("string"), "actor_label", typed("string"),
			"body", typed("string"), "content_type", typed("string"),
			"reply_to_entry_id", typed("integer", "format", "int64"),
			"addressed_member_ids", typed("array", "items", typed("string")),
			"supersedes_entry_id", typed("integer", "format", "int64"),
			"pinned_at", stamp(),
			"event_type", typed("string"), "issue", typed("string"),
			"extension", typed("object", "description", "Message metadata or the event payload."),
			"created_at", stamp(), "deleted_at", stamp(),
		), "id", "room_id", "sequence", "kind", "actor_label", "content_type", "created_at"),
		"RoomEntryResponse", envelope("RoomEntry", objectOf(obj(
			"idempotent_replay", typed("boolean", "description", "true when the entry was already stored under this client_entry_id."),
		))),
		"RoomEntryPage", pageOf("RoomEntry", objectOf(obj(
			"limit", typed("integer", "description", "The page size applied after clamping."),
			"has_more", typed("boolean"),
			"next_cursor", nullable("string", "description", "Opaque cursor for the following page; null when has_more is false."),
		), "limit", "has_more", "next_cursor")),
		"PostEntryRequest", objectOf(obj(
			"kind", typed("string", "enum", []string{"message", "event"}, "default", "message"),
			"body", typed("string", "description", "The message text (kind message)."),
			"content_type", typed("string"),
			"event_type", typed("string", "description", "The typed event name (kind event)."),
			"issue", typed("string", "description", "The issue an event belongs to (kind event)."),
			"extension", typed("object", "description", "Message metadata or the event payload."),
			"reply_to_entry_id", typed("integer", "format", "int64"),
			"addressed_member_ids", typed("array", "items", typed("string")),
			"supersedes_entry_id", typed("integer", "format", "int64"),
			"client_entry_id", typed("string", "description", "Caller-chosen id that makes the write retry-safe; scoped to the authenticated actor."),
		)),

		"ReplyAuthor", objectOf(obj(
			"id", typed("string"), "type", typed("string", "enum", []string{"human", "agent", "system"}),
			"display_name", typed("string"), "avatar_url", typed("string"),
		), "id", "type", "display_name"),
		"Reply", objectOf(obj(
			"id", uuidStr(), "post_id", uuidStr(),
			"parent_reply_id", uuidStr(),
			"author_type", typed("string", "enum", []string{"human", "agent", "system"}), "author_id", typed("string"),
			"body", typed("string"),
			"upvotes", typed("integer"), "downvotes", typed("integer"), "score", typed("integer"),
			"legacy_type", typed("string"), "legacy_id", typed("string"),
			"provenance", typed("object", "description", "Where a migrated reply came from; absent on a native reply."),
			"created_at", stamp(), "updated_at", stamp(), "deleted_at", stamp(),
			"author", ref("schemas", "ReplyAuthor"),
		), "id", "post_id", "author_type", "author_id", "body", "upvotes", "downvotes", "score", "created_at", "updated_at", "author"),
		"ReplyResponse", envelope("Reply", nil),
		"ReplyPage", pageOf("Reply", objectOf(obj(
			"total", typed("integer", "description", "Replies on the post."),
			"has_more", typed("boolean"),
			"next_cursor", typed("string", "description", "Opaque cursor for the following page; present only when has_more is true."),
		), "total", "has_more")),
		"CreateReplyRequest", objectOf(obj(
			"body", typed("string", "description", fmt.Sprintf("Markdown, 1 to %d bytes.", models.MaxReplyBodyLength)),
			"parent_reply_id", uuidStr(),
		), "body"),
		"UpdateReplyRequest", objectOf(obj(
			"body", typed("string", "description", fmt.Sprintf("Markdown, 1 to %d bytes.", models.MaxReplyBodyLength)),
		), "body"),
		"DeletedResponse", objectOf(obj("data", objectOf(obj("deleted", typed("boolean")), "deleted")), "data"),
	)
}
