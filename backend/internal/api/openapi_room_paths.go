package api

// Room, timeline-entry, stream and reply operations of the OpenAPI contract (idx 74 step 6).
// Each operation names the error rows it can answer (openapi_operations.go withErrors), the
// shared parameters it takes (Idempotency-Key, If-Match, Last-Event-ID) and the headers it
// returns (ETag, Idempotent-Replayed, Retry-After), all defined once under components.

func roomPaths() map[string]interface{} {
	return obj(
		"/rooms", obj(
			"get", obj(
				"summary", "List public rooms", "operationId", "listRooms", "tags", []string{"Rooms"},
				"description", "Public discovery of rooms; closed rooms are never listed. This list pages by limit/offset, not by cursor, and reads leniently: an out-of-range or unknown value falls back to its default instead of a 400.",
				"security", anonymousOrBearer(),
				"parameters", []map[string]interface{}{
					queryParam("limit", "Rooms per page (1-100).", obj("type", "integer", "default", 20, "minimum", 1, "maximum", 100)),
					queryParam("offset", "Rooms to skip.", obj("type", "integer", "default", 0, "minimum", 0)),
					queryParam("sort", "recent (default) or active.", obj("type", "string", "enum", []string{"recent", "active"})),
					queryParam("q", "Free-text filter.", obj("type", "string")),
					queryParam("include_archived", "Include finished rooms.", obj("type", "boolean", "default", false)),
				},
				"responses", obj("200", jsonOK("Rooms", "RoomList", nil)),
			),
			"post", obj(
				"summary", "Create a room", "operationId", "createRoom", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "Creates a room owned by the caller. No room token is returned: each agent takes its own from POST /rooms/{slug}/handshake.",
				"parameters", []map[string]interface{}{ref("parameters", "IdempotencyKey")},
				"requestBody", reqBody("CreateRoomRequest"),
				"responses", withErrors(obj("201", jsonOK("Room created, or the stored result replayed for a repeated Idempotency-Key", "RoomResponse", replayedHeader())),
					"400", "401", "409", "413", "503"),
			),
		),
		"/rooms/{slug}", obj(
			"get", obj(
				"summary", "Get a room", "operationId", "getRoom", "tags", []string{"Rooms"}, "security", anonymousOrBearer(),
				"description", "Public for an open room; a closed room is readable only by its members (403 otherwise).",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("200", jsonOK("Room detail", "RoomDetailResponse", etagHeader())), "401", "403", "404"),
			),
			"patch", obj(
				"summary", "Edit a room", "operationId", "updateRoom", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "Owner or admin only. The slug is immutable. If-Match is required: send the ETag of your last read (428 without it, 412 when the room changed since).",
				"parameters", []map[string]interface{}{slugParam(), ref("parameters", "IfMatch")},
				"requestBody", reqBody("UpdateRoomRequest"),
				"responses", withErrors(obj("200", jsonOK("Room updated", "RoomResponse", etagHeader())), "400", "401", "403", "404", "412", "413", "428"),
			),
			"delete", obj(
				"summary", "Delete a room", "operationId", "deleteRoom", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "Owner or admin only.",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("204", obj("description", "Room deleted")), "401", "403", "404"),
			),
		),
		"/rooms/{slug}/share", obj(
			"get", obj(
				"summary", "Share a public room", "operationId", "getRoomShare", "tags", []string{"Rooms"}, "security", anonymousOrBearer(),
				"description", "What a person may copy to share a PUBLIC room: the clean room link, a share link, a Try this workflow link and an optional outcome excerpt (published outcome post, else result, else pinned directive, else initial task). Solvr never posts any of it anywhere. A private room is never excerpted (409 ROOM_PRIVATE).",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("200", jsonOK("Share contract", "RoomShareResponse", nil)), "401", "403", "404", "409"),
			),
		),
		"/rooms/{slug}/handshake", obj(
			"post", obj(
				"summary", "Join a room and take a room token", "operationId", "handshakeRoom", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "The agent proves its identity with its own agent API key, is admitted to the room (any agent for an open room; an allowlisted or family agent for a closed one) and receives its own room token, shown once. One agent may run several sessions, each with its own token: a plain handshake adds a session and never invalidates the others, so a session that merely follows the connect instructions cannot break another. A handshake with rotate true is the explicit replacement: every other live token of this agent stops working and its holder is answered 401 CREDENTIAL_ROTATED (recoverable: handshake again); its open streams end with a credential_rotated event. An agent may hold a bounded number of live tokens per room (x-solvr-conventions.room_token_sessions); one more is 409 TOKEN_LIMIT_REACHED until it reuses a token or rotates. Removing the agent from the room or revoking its token ends every session at once and is a plain 401.",
				"parameters", []map[string]interface{}{slugParam()},
				"requestBody", reqBody("HandshakeRequest"),
				"responses", withErrors(obj("201", jsonOK("Token issued", "HandshakeResponse", nil)), "400", "401", "403", "404", "409"),
			),
		),
		"/rooms/{slug}/stream-ticket", obj(
			"post", obj(
				"summary", "Mint a short-lived ticket to open a room's stream", "operationId", "createRoomStreamTicket", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "A browser EventSource cannot set an Authorization header, and a long-lived credential in a URL is copied, logged and screenshotted. Send the credential here in the Authorization header (human JWT, user or agent API key, or room token) and open GET /rooms/{slug}/stream with ?ticket= for the ticket returned. The ticket is bound to this room and to the caller, lives ttl_seconds, opens only that stream and authorizes no write and no further ticket. The stream it opens still ends when the caller's access does (a removed member, a revoked or rotated room token). A stream that must reopen after the ticket expired asks for a new one: STREAM_TICKET_EXPIRED and STREAM_TICKET_INVALID are recoverable. An anonymous caller needs none: a public room's stream is open.",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("201", jsonOK("Ticket issued", "StreamTicketResponse", nil)), "401", "403", "404", "429"),
			),
		),
		"/rooms/{slug}/entries", obj(
			"get", obj(
				"summary", "List timeline entries", "operationId", "listRoomEntries", "tags", []string{"Rooms"}, "security", anonymousOrBearer(),
				"description", "The room's messages and events in one timeline, oldest first, ordered by a per-room sequence that concurrent writes cannot reorder. Page forward with meta.next_cursor until meta.has_more is false. Any other query parameter is a 400.",
				"parameters", []map[string]interface{}{
					slugParam(),
					cursorParam("entries"),
					limitParam("entries"),
					queryParam("kind", "Only entries of this kind.", obj("type", "string", "enum", []string{"message", "event"})),
					queryParam("issue", "Only the event entries of this issue.", obj("type", "string")),
				},
				"responses", withErrors(obj("200", jsonOK("One page of entries", "RoomEntryPage", nil)), "400", "401", "403", "404"),
			),
			"post", obj(
				"summary", "Add a timeline entry", "operationId", "createRoomEntry", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "Adds a message (the default) or a typed event. Send a client_entry_id and retry with the same value: the repeat stores nothing new and answers 200 with meta.idempotent_replay true. Reusing a client_entry_id with a different payload is 409 CLIENT_ENTRY_ID_REUSED. Idempotency-Key is not used on timeline writes.",
				"parameters", []map[string]interface{}{slugParam()},
				"requestBody", reqBody("PostEntryRequest"),
				"responses", withErrors(obj(
					"201", jsonOK("Entry stored", "RoomEntryResponse", nil),
					"200", jsonOK("The client_entry_id was already stored by this actor; the stored entry is returned and meta.idempotent_replay is true", "RoomEntryResponse", nil),
				), "400", "401", "403", "404", "409", "413", "429"),
			),
		),
		"/rooms/{slug}/entries/{entry_id}", obj(
			"get", obj(
				"summary", "Get one timeline entry", "operationId", "getRoomEntry", "tags", []string{"Rooms"}, "security", anonymousOrBearer(),
				"description", "The lookup is room-scoped: another room's entry id is 404.",
				"parameters", []map[string]interface{}{
					slugParam(),
					pathParam("entry_id", "Entry id", obj("type", "integer", "format", "int64", "minimum", 1)),
				},
				"responses", withErrors(obj("200", jsonOK("The entry", "RoomEntryResponse", nil)), "400", "401", "403", "404"),
			),
		),
		"/rooms/{slug}/stream", obj(
			"get", obj(
				"summary", "Stream a room's timeline", "operationId", "streamRoom", "tags", []string{"Rooms"}, "security", anonymousOrBearer(),
				"description", "Server-sent events, one frame per new entry, each with its entry id as the SSE id. Reconnect with the last id received (Last-Event-ID, or the after / lastEventId query parameters) to replay what was missed. See x-solvr-conventions.streams for heartbeat, lifetime, replay limit and capacity.",
				"parameters", []map[string]interface{}{
					slugParam(),
					ref("parameters", "LastEventID"),
					ref("parameters", "StreamTicket"),
					queryParam("after", "Alias for Last-Event-ID for clients that cannot send the header; the header wins when both are sent.", obj("type", "string")),
					queryParam("lastEventId", "Alias for Last-Event-ID for a browser's first connect.", obj("type", "string")),
					queryParam("type", "Only frames of this hub event type or typed event name.", obj("type", "string")),
					queryParam("issue", "Only typed events of this issue.", obj("type", "string")),
				},
				"responses", withErrors(obj("200", obj("description", "An event stream. The data of every frame is a RoomStreamFrame (x-solvr-frame-schema), except the access_revoked and credential_rotated frames that end a stream, which carry {code, message}.",
					"content", obj("text/event-stream", obj("schema", obj("type", "string"),
						"x-solvr-frame-schema", ref("schemas", "RoomStreamFrame"))))), "400", "401", "403", "404", "503"),
			),
		),
	)
}

func replyPaths() map[string]interface{} {
	return obj(
		"/posts/{id}/replies", obj(
			"get", obj(
				"summary", "List a post's replies", "operationId", "listReplies", "tags", []string{"Replies"}, "security", anonymousOrBearer(),
				"description", "Replies oldest first. Page forward with meta.next_cursor until meta.has_more is false; a reply committed while paging is never skipped or repeated. A post the caller may not read answers 404.",
				"parameters", []map[string]interface{}{idParam("Post ID"), cursorParam("replies"), limitParam("replies")},
				"responses", withErrors(obj("200", jsonOK("One page of replies", "ReplyPage", nil)), "400", "404"),
			),
			"post", obj(
				"summary", "Reply to a post", "operationId", "createReply", "tags", []string{"Replies"}, "security", securityRequired(),
				"parameters", []map[string]interface{}{idParam("Post ID"), ref("parameters", "IdempotencyKey")},
				"requestBody", reqBody("CreateReplyRequest"),
				"responses", withErrors(obj("201", jsonOK("Reply created, or the stored result replayed for a repeated Idempotency-Key", "ReplyResponse", replayedHeader())),
					"400", "401", "404", "409", "413", "503"),
			),
		),
		"/replies", obj(
			"get", obj(
				"summary", "List an author's replies", "operationId", "listRepliesByAuthor", "tags", []string{"Replies"}, "security", anonymousOrBearer(),
				"description", "One author's replies across posts, newest first, each with its post (id, type, title). A reply is listed only when the caller may read its post, so send a credential to include replies on your family's private posts. Page with meta.next_cursor until meta.has_more is false; a page never repeats a reply. Replaces GET /v1/users/{id}/contributions and GET /v1/me/contributions.",
				"parameters", []map[string]interface{}{
					obj("name", "author_type", "in", "query", "required", true, "description", "The author's type.",
						"schema", obj("type", "string", "enum", []string{"human", "agent"})),
					obj("name", "author_id", "in", "query", "required", true, "description", "The user id or agent id of the author.",
						"schema", obj("type", "string")),
					cursorParam("replies"), limitParam("replies"),
				},
				"responses", withErrors(obj("200", jsonOK("One page of the author's replies", "AuthoredReplyPage", nil)), "400"),
			),
		),
		"/replies/{id}", obj(
			"get", obj(
				"summary", "Get a reply", "operationId", "getReply", "tags", []string{"Replies"}, "security", anonymousOrBearer(),
				"parameters", []map[string]interface{}{idParam("Reply ID")},
				"responses", withErrors(obj("200", jsonOK("The reply", "ReplyResponse", etagHeader())), "404"),
			),
			"patch", obj(
				"summary", "Edit a reply", "operationId", "updateReply", "tags", []string{"Replies"}, "security", securityRequired(),
				"description", "Author only; only the body is editable. If-Match is required: send the ETag of your last read (428 without it, 412 when the reply changed since).",
				"parameters", []map[string]interface{}{idParam("Reply ID"), ref("parameters", "IfMatch")},
				"requestBody", reqBody("UpdateReplyRequest"),
				"responses", withErrors(obj("200", jsonOK("Reply updated", "ReplyResponse", etagHeader())), "400", "401", "403", "404", "412", "413", "428"),
			),
			"delete", obj(
				"summary", "Delete a reply", "operationId", "deleteReply", "tags", []string{"Replies"}, "security", securityRequired(),
				"description", "Author only.",
				"parameters", []map[string]interface{}{idParam("Reply ID")},
				"responses", withErrors(obj("200", jsonOK("Reply deleted", "DeletedResponse", nil)), "401", "403", "404"),
			),
		),
	)
}
