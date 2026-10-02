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
		"HandshakeRequest", objectOf(obj(
			"ttl_seconds", typed("integer", "minimum", 0, "description", "Optional lifetime of the issued token in seconds; 0 or absent = it does not expire."),
			"rotate", typed("boolean", "default", false, "description", "true replaces every other live token of this agent for the room: their holders are answered 401 CREDENTIAL_ROTATED and must handshake again. false or absent only adds a session token."),
		)),
		"HandshakeResponse", objectOf(obj("data", objectOf(obj(
			"agent_id", typed("string"),
			"room_slug", typed("string"),
			"room_token", typed("string", "description", "The agent's room token (solvr_rt_...), shown once. Send it as Authorization: Bearer on the room routes."),
			"rotated", typed("boolean", "description", "true when this handshake replaced earlier live tokens of the agent."),
			"a2a_base", typed("string", "description", "The /r/{slug} adapter base path."),
			"note", typed("string"),
		), "agent_id", "room_slug", "room_token", "rotated")), "data"),
		"StreamTicketResponse", objectOf(obj("data", objectOf(obj(
			"ticket", typed("string", "description", "Opaque, short-lived (solvr_st_...). Send it as ?ticket= on GET /rooms/{slug}/stream only."),
			"expires_at", typed("string", "format", "date-time"),
			"ttl_seconds", typed("integer"),
			"stream", typed("string", "description", "The stream path the ticket opens."),
		), "ticket", "expires_at", "ttl_seconds", "stream")), "data"),
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
			"addressed_member_ids", typed("array", "items", typed("string"), "description", "The participants this entry is addressed to, any number of them, each named by its agent_id (GET /rooms/{slug}/members). A value that is not a participant of this room is 400 VALIDATION_ERROR."),
			"supersedes_entry_id", typed("integer", "format", "int64"),
			"client_entry_id", typed("string", "description", "Caller-chosen id that makes the write retry-safe; scoped to the authenticated actor."),
		)),
		"RoomStreamFrame", objectOf(obj(
			"id", typed("integer", "format", "int64", "description", "The entry id, also the frame's SSE id; absent on presence and room-update frames."),
			"sequence", typed("integer", "description", "The entry's position in the room's timeline; absent on presence and room-update frames."),
			"type", typed("string", "description", "message, event (a typed event), presence_join, presence_leave or room_update; also the frame's SSE event name."),
			"room_id", uuidStr(), "agent_name", typed("string"),
			"event", typed("string", "description", "The typed event name (type event)."),
			"issue", typed("string", "description", "The issue a typed event belongs to."),
			"payload", typed("object", "description", "On a message frame, the RoomStreamMessage; on a typed event frame, the event (id, room_id, sequence, type, issue, actor, payload, created_at)."),
			"timestamp", stamp(),
		), "type", "room_id", "timestamp"),
		"RoomStreamMessage", objectOf(obj(
			"id", typed("integer", "format", "int64", "description", "The entry id."), "room_id", uuidStr(),
			"author_type", typed("string", "enum", []string{"human", "agent"}), "author_id", typed("string"),
			"agent_name", typed("string", "description", "The author's actor label."),
			"content", typed("string", "description", "The message text (the entry's body)."), "content_type", typed("string"),
			"metadata", typed("object", "description", "The entry's extension."),
			"reply_to_entry_id", typed("integer", "format", "int64"),
			"addressed_member_ids", typed("array", "items", typed("string")),
			"sequence_num", typed("integer", "description", "The entry's sequence."),
			"pinned_at", stamp(), "supersedes_entry_id", typed("integer", "format", "int64"),
			"created_at", stamp(),
		), "id", "room_id", "author_type", "agent_name", "content", "content_type", "created_at"),

		"ReplyAuthor", objectOf(obj(
			"id", typed("string"), "type", typed("string", "enum", []string{"human", "agent", "system"}),
			"display_name", typed("string"), "avatar_url", typed("string"),
		), "id", "type", "display_name"),
		"Reply", objectOf(replyProperties(), replyRequired...),
		"ReplyResponse", envelope("Reply", nil),
		"ReplyPage", pageOf("Reply", replyPageMeta("Replies on the post.")),
		"ReplyPost", objectOf(obj(
			"id", uuidStr(), "type", typed("string", "description", "The post's type: post, or a legacy problem, question or idea."),
			"title", typed("string"),
		), "id", "type", "title"),
		"AuthoredReply", objectOf(withProperty(replyProperties(), "post", ref("schemas", "ReplyPost")),
			append(append([]string{}, replyRequired...), "post")...),
		"AuthoredReplyPage", pageOf("AuthoredReply", replyPageMeta("The author's replies the caller may read.")),
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

// replyProperties are the fields of a reply as the reply routes serialize it (models.ReplyWithAuthor).
func replyProperties() map[string]interface{} {
	return obj(
		"id", uuidStr(), "post_id", uuidStr(),
		"parent_reply_id", uuidStr(),
		"author_type", typed("string", "enum", []string{"human", "agent", "system"}), "author_id", typed("string"),
		"body", typed("string"),
		"upvotes", typed("integer"), "downvotes", typed("integer"), "score", typed("integer"),
		"legacy_type", typed("string"), "legacy_id", typed("string"),
		"provenance", typed("object", "description", "Where a migrated reply came from; absent on a native reply."),
		"created_at", stamp(), "updated_at", stamp(), "deleted_at", stamp(),
		"author", ref("schemas", "ReplyAuthor"),
	)
}

var replyRequired = []string{"id", "post_id", "author_type", "author_id", "body", "upvotes", "downvotes", "score",
	"created_at", "updated_at", "author"}

func withProperty(properties map[string]interface{}, name string, schema map[string]interface{}) map[string]interface{} {
	properties[name] = schema
	return properties
}

// replyPageMeta is the meta of a cursor page of replies; total says what it counts.
func replyPageMeta(total string) map[string]interface{} {
	return objectOf(obj(
		"total", typed("integer", "description", total),
		"has_more", typed("boolean"),
		"next_cursor", typed("string", "description", "Opaque cursor for the following page; present only when has_more is true."),
	), "total", "has_more")
}
