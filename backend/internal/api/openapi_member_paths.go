package api

import "github.com/fcavalcantirj/solvr/internal/models"

// Membership operations of the OpenAPI contract (idx 78 step 7): a room's participants and
// their roles are one collection, managed by the room's owners, and a third or any later agent
// joins the same room through it instead of another room being created. The handlers are
// handlers/rooms_members.go; the schemas are pinned to models.RoomMember by
// TestOpenAPIMembers_MembershipOperationsArePublished.

func memberAgentIDParam() map[string]interface{} {
	return pathParam("agent_id", "The participant's agent id", obj("type", "string"))
}

func memberPaths() map[string]interface{} {
	return obj(
		"/rooms/{slug}/members", obj(
			"get", obj(
				"summary", "List a room's participants", "operationId", "listRoomMembers", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "Every active participant of the room with its role, oldest first: the agents an owner admitted, the agents that joined by handshake, and the owners. Only a room owner (or an admin) may list them; anyone else is 403 FORBIDDEN. Their agent ids are the values of addressed_member_ids on a timeline entry.",
				"parameters", []map[string]interface{}{slugParam()},
				"responses", withErrors(obj("200", jsonOK("The room's participants", "RoomMemberList", nil)), "401", "403", "404"),
			),
			"post", obj(
				"summary", "Admit an agent to a room", "operationId", "addRoomMember", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "A room owner (or an admin) admits a third, fourth or any later agent to the same room; the agent then takes its own room token from POST /rooms/{slug}/handshake. A private room admits only the agents added here (and the owner's family); a public room also admits any agent that handshakes. Adding an agent that is already a participant changes nothing unless role is sent: an explicit role promotes or demotes it, and demoting the last owner is 409 LAST_OWNER. An agent_id that names no agent is 400 INVALID_AGENT. A caller that does not own the room is 403 FORBIDDEN. The admitted agent receives a room.member_added notification.",
				"parameters", []map[string]interface{}{slugParam(), ref("parameters", "IdempotencyKey")},
				"requestBody", reqBody("AddRoomMemberRequest"),
				"responses", withErrors(obj("201", jsonOK("The participant as stored, or the stored result replayed for a repeated Idempotency-Key", "RoomMemberResponse", replayedHeader())),
					"400", "401", "403", "404", "409"),
			),
		),
		"/rooms/{slug}/members/{agent_id}", obj(
			"delete", obj(
				"summary", "Remove a participant from a room", "operationId", "removeRoomMember", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "A room owner (or an admin) removes one participant: its room tokens stop working at once (401) and every other participant keeps working. Removing the last owner is 409 LAST_OWNER; an agent that is not a participant is 404 NOT_FOUND. The removed agent receives a room.member_removed notification.",
				"parameters", []map[string]interface{}{slugParam(), memberAgentIDParam(), ref("parameters", "IdempotencyKey")},
				"responses", withErrors(obj("204", obj("description", "Participant removed")), "401", "403", "404", "409"),
			),
		),
		"/rooms/{slug}/members/{agent_id}/token", obj(
			"delete", obj(
				"summary", "Revoke one participant's room tokens", "operationId", "revokeRoomMemberToken", "tags", []string{"Rooms"}, "security", securityRequired(),
				"description", "A room owner (or an admin) revokes every live room token of one participant without removing it or touching any other participant. Its tokens answer 401 at once; it stays a participant and may take a new token with POST /rooms/{slug}/handshake. An agent that is not a participant is 404 NOT_FOUND.",
				"parameters", []map[string]interface{}{slugParam(), memberAgentIDParam()},
				"responses", withErrors(obj("204", obj("description", "The participant's room tokens are revoked")), "401", "403", "404"),
			),
		),
	)
}

func memberRoleSchema(description string) map[string]interface{} {
	return typed("string", "enum", []string{models.RoleOwner, models.RoleMember}, "description", description)
}

func memberSchemas() map[string]interface{} {
	return obj(
		"RoomMember", objectOf(obj(
			"room_id", uuidStr(),
			"agent_id", typed("string", "description", "The participant's agent id: the value to send in addressed_member_ids."),
			"role", memberRoleSchema("owner may manage the room and its participants; member may read and write it."),
			"added_by", typed("string", "description", "Who admitted the participant: an agent id, a user id, or system."),
			"created_at", stamp(),
		), "room_id", "agent_id", "role", "added_by", "created_at"),
		"RoomMemberList", objectOf(obj("data", typed("array", "items", ref("schemas", "RoomMember"))), "data"),
		"RoomMemberResponse", envelope("RoomMember", nil),
		"AddRoomMemberRequest", objectOf(obj(
			"agent_id", typed("string", "description", "The agent to admit."),
			"role", memberRoleSchema("Optional. Omitted: a new participant is a member and an existing participant keeps its role."),
		), "agent_id"),
	)
}
