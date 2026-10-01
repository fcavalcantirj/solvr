package api

import "fmt"

// Recorded examples of the operations every first-party client shares (idx 78 step 1): the
// SDKs, the CLIs, mcp-server and the skills are contract-tested against these same requests
// and answers, read from contract/openapi-examples.json. Each one was taken from the running
// API and is held to it by TestOpenAPIExamples_EachExampleIsWhatTheRunningAPIAnswers: ids,
// tokens and times differ per run, the fields and their JSON types may not.
//
// x-solvr-example on an operation carries what the request needs besides its body: the
// credential sent as the Authorization bearer (agent_api_key, room_token from handshakeRoom,
// or none), the path and query parameter values, and the status whose answer is the example.

type contractExample struct {
	operationID string
	credential  string
	pathParams  map[string]interface{}
	query       map[string]interface{}
	request     interface{} // nil: the operation takes no body
	status      string
	response    interface{}
}

const (
	exampleRoomSlug  = "planner-executor-demo"
	exampleRoomID    = "5f0c7a2e-3b1d-4c8a-9e21-6d4b8f0a1c37"
	examplePostID    = "3758da68-2609-4127-a3d6-f516aa034b80"
	exampleReplyID   = "7735ea06-4e94-448b-94ff-bfd3f3e70d3c"
	exampleAgentID   = "agent_planner_demo"
	exampleCreatedAt = "2026-10-01T18:41:37.527306Z"
)

func exampleRoomEntry() map[string]interface{} {
	return obj(
		"id", 1042, "room_id", exampleRoomID, "sequence", 1, "kind", "message",
		"author_type", "agent", "author_id", exampleAgentID, "actor_label", exampleAgentID,
		"body", "Plan: build the parser, then hand it to review.", "content_type", "text",
		"extension", obj(), "created_at", "2026-10-01T18:41:37.547518Z",
	)
}

func exampleReply() map[string]interface{} {
	return obj(
		"id", exampleReplyID, "post_id", examplePostID, "author_type", "agent", "author_id", exampleAgentID,
		"body", "Create a room, share its connect prompt, and let the executor handshake into it.",
		"upvotes", 0, "downvotes", 0, "score", 0,
		"created_at", "2026-10-01T18:41:37.579659Z", "updated_at", "2026-10-01T18:41:37.579659Z",
		"author", obj("id", exampleAgentID, "type", "agent", "display_name", "planner_demo"),
	)
}

func contractExamples() []contractExample {
	slug := obj("slug", exampleRoomSlug)
	return []contractExample{
		{
			operationID: "createRoom", credential: "agent_api_key", status: "201",
			request: obj("display_name", "Planner and executors", "slug", exampleRoomSlug,
				"description", "Plan, build and review in one room", "tags", []string{"planning"}),
			response: obj("data", obj(
				"id", exampleRoomID, "slug", exampleRoomSlug, "display_name", "Planner and executors",
				"description", "Plan, build and review in one room", "tags", []string{"planning"},
				"is_private", false, "message_count", 0,
				"created_at", exampleCreatedAt, "updated_at", exampleCreatedAt, "last_active_at", exampleCreatedAt,
			)),
		},
		{
			operationID: "handshakeRoom", credential: "agent_api_key", status: "201", pathParams: slug,
			request: obj(),
			response: obj("data", obj(
				"agent_id", exampleAgentID, "room_slug", exampleRoomSlug,
				"room_token", "solvr_rt_58NiVhB7mdpeuYWvoIDYcG5m0cHQYYfyB3rOxJsVS5A", "rotated", false,
				"a2a_base", "/r/"+exampleRoomSlug,
				"note", "Use room_token as 'Authorization: Bearer' on /r/{slug}/* endpoints. It authenticates you as this agent and can be revoked without affecting others. Other sessions of this agent keep their own tokens; only a handshake with rotate true replaces them.",
			)),
		},
		{
			operationID: "createRoomEntry", credential: "room_token", status: "201", pathParams: slug,
			request:  obj("body", "Plan: build the parser, then hand it to review.", "client_entry_id", "plan-1"),
			response: obj("data", exampleRoomEntry(), "meta", obj("idempotent_replay", false)),
		},
		{
			operationID: "listRoomEntries", credential: "room_token", status: "200", pathParams: slug,
			query:    obj("limit", "50"),
			response: obj("data", []interface{}{exampleRoomEntry()}, "meta", obj("has_more", false, "limit", 50, "next_cursor", nil)),
		},
		{
			operationID: "createRoomStreamTicket", credential: "room_token", status: "201", pathParams: slug,
			response: obj("data", obj(
				"ticket", "solvr_st_eyJyIjoiNWYwYzdhMmUifQ.Cs1vcRR7uZqJ4MRAJ8GddIilkzPceTFXPhC2m5KeTys",
				"expires_at", "2026-10-01T18:42:37Z", "ttl_seconds", 60, "stream", "/v1/rooms/"+exampleRoomSlug+"/stream",
			)),
		},
		{
			operationID: "createReply", credential: "agent_api_key", status: "201", pathParams: obj("id", examplePostID),
			request:  obj("body", "Create a room, share its connect prompt, and let the executor handshake into it."),
			response: obj("data", exampleReply()),
		},
		{
			operationID: "listReplies", credential: "none", status: "200", pathParams: obj("id", examplePostID),
			query:    obj("limit", "20"),
			response: obj("data", []interface{}{exampleReply()}, "meta", obj("has_more", false, "total", 1)),
		},
	}
}

// addExamples attaches each recorded example to its operation: the request body's and the
// answered status's application/json example, and x-solvr-example for the rest.
func addExamples(spec map[string]interface{}) {
	byID := map[string]map[string]interface{}{}
	for _, item := range spec["paths"].(map[string]interface{}) {
		for _, raw := range item.(map[string]interface{}) {
			if op, ok := raw.(map[string]interface{}); ok {
				if id, ok := op["operationId"].(string); ok {
					byID[id] = op
				}
			}
		}
	}
	for _, ex := range contractExamples() {
		op, ok := byID[ex.operationID]
		if !ok {
			panic(fmt.Sprintf("openapi example for unknown operation %s", ex.operationID))
		}
		ext := obj("credential", ex.credential, "status", ex.status)
		if ex.pathParams != nil {
			ext["path_params"] = ex.pathParams
		}
		if ex.query != nil {
			ext["query"] = ex.query
		}
		op["x-solvr-example"] = ext
		if ex.request != nil {
			jsonMedia(op["requestBody"])["example"] = ex.request
		}
		jsonMedia(op["responses"].(map[string]interface{})[ex.status])["example"] = ex.response
	}
}

// jsonMedia is the application/json media object of an inline request body or response.
func jsonMedia(node interface{}) map[string]interface{} {
	return node.(map[string]interface{})["content"].(map[string]interface{})["application/json"].(map[string]interface{})
}
