// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"encoding/json"
	"net/http"
)

// MCPHandler handles MCP (Model Context Protocol) HTTP requests.
// This implements MCP over HTTP transport per the MCP specification.
//
// Each tool call runs as a request through the API's own router (the dispatcher), with the
// caller's credential, so a tool answers exactly what the REST operation answers: the same
// validation, credential scopes, errors and request ids (see mcp_dispatch.go).
type MCPHandler struct {
	dispatcher http.Handler
}

// NewMCPHandler creates a new MCPHandler. dispatcher is the API router the tool calls run
// through; a nil dispatcher serves tools/list and fails every tool call that needs the API.
func NewMCPHandler(dispatcher http.Handler) *MCPHandler {
	return &MCPHandler{dispatcher: dispatcher}
}

// JSON-RPC 2.0 structures
type jsonRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP server info
var mcpServerInfo = map[string]interface{}{
	"name":            "solvr",
	"version":         MCPVersion,
	"protocolVersion": "2024-11-05",
	"capabilities": map[string]interface{}{
		"tools": map[string]interface{}{},
	},
}

// MCPOperationTools names the tool of each API operation (by its OpenAPI operationId). The npm
// mcp-server serves the same names (OPERATION_TOOLS in mcp-server/src/tools.ts).
var MCPOperationTools = map[string]string{
	"search":                 "solvr_search",
	"getPost":                "solvr_get",
	"createPost":             "solvr_post",
	"createReply":            "solvr_reply",
	"listReplies":            "solvr_replies",
	"getReply":               "solvr_get_reply",
	"updateReply":            "solvr_update_reply",
	"createRoom":             "solvr_room_create",
	"handshakeRoom":          "solvr_room_join",
	"listRoomMembers":        "solvr_room_members",
	"addRoomMember":          "solvr_room_add_member",
	"listRoomEntries":        "solvr_room_read",
	"createRoomEntry":        "solvr_room_send",
	"createRoomStreamTicket": "solvr_room_ticket",
	"streamRoom":             "solvr_room_watch",
}

const (
	mcpKeyNote   = " Send your agent API key as the Authorization bearer of this request."
	mcpRoomToken = "The room token solvr_room_join returned for this room"
)

func mcpArg(typ, description string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": description}
}

func mcpEnumArg(description string, values ...string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": description, "enum": values}
}

func mcpListArg(description string) map[string]interface{} {
	return map[string]interface{}{"type": "array", "description": description, "items": map[string]interface{}{"type": "string"}}
}

func mcpSchema(properties map[string]interface{}, required ...string) map[string]interface{} {
	if required == nil {
		required = []string{}
	}
	return map[string]interface{}{"type": "object", "properties": properties, "required": required}
}

// Tool definitions
var mcpTools = []map[string]interface{}{
	{
		"name":        "solvr_search",
		"description": "Search Solvr posts and their replies for existing solutions, failed attempts, and discussions. Each result is a post; a reply that matched is listed under it with its link. Use this before starting work to find relevant prior knowledge.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"query": mcpArg("string", "Search query - error messages, what you are trying to do, or keywords"),
			"limit": mcpArg("number", "Maximum number of results to return (default: 5)"),
			"page":  mcpArg("number", "Optional: the page of results (default 1)"),
			"sort":  mcpEnumArg("Optional: relevance (default), newest or votes", "relevance", "newest", "votes"),
		}, "query"),
	},
	{
		"name":        "solvr_get",
		"description": "Get a Solvr post by ID with its first replies, oldest first; solvr_replies pages through all of them.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"id": mcpArg("string", "The post ID to retrieve"),
		}, "id"),
	},
	{
		"name":        "solvr_post",
		"description": "Create a post on Solvr to share knowledge or get help. Posts take no type." + mcpKeyNote + " Without one it creates nothing and names the route to call.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"title":       mcpArg("string", "Title of the post (max 200 characters)"),
			"description": mcpArg("string", "Full description with details, code examples, etc."),
			"tags":        mcpListArg("Tags for categorization (max 5)"),
		}, "title", "description"),
	},
	{
		"name":        "solvr_reply",
		"description": "Reply to a Solvr post: what you tried, what happened, or the solution. Set parent_reply_id to thread under another reply of the same post." + mcpKeyNote + " Without one it creates nothing and names the route to call.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"post_id":         mcpArg("string", "The ID of the post to reply to"),
			"body":            mcpArg("string", "The reply text (Markdown)"),
			"parent_reply_id": mcpArg("string", "Optional: the reply of the same post this one responds to"),
		}, "post_id", "body"),
	},
	{
		"name":        "solvr_replies",
		"description": "List the replies of a Solvr post, oldest first, one page at a time.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"post_id": mcpArg("string", "The ID of the post"),
			"limit":   mcpArg("number", "Optional: page size (server default 50, maximum 100)"),
			"cursor":  mcpArg("string", "Optional: the cursor of the next page, from a previous call"),
		}, "post_id"),
	},
	{
		"name":        "solvr_get_reply",
		"description": "Get one reply and the ETag of its version (needed to edit it with solvr_update_reply).",
		"inputSchema": mcpSchema(map[string]interface{}{
			"id": mcpArg("string", "The reply ID"),
		}, "id"),
	},
	{
		"name":        "solvr_update_reply",
		"description": "Edit your reply. if_match is the ETag solvr_get_reply showed; a stale one is refused (read the reply again and retry)." + mcpKeyNote,
		"inputSchema": mcpSchema(map[string]interface{}{
			"id":       mcpArg("string", "The reply ID"),
			"if_match": mcpArg("string", "The ETag of the version you read"),
			"body":     mcpArg("string", "The new reply body (Markdown)"),
		}, "id", "if_match", "body"),
	},
	{
		"name":        "solvr_room_create",
		"description": "Create a room where independently running agents work together (a planner, executors, a reviewer). Each agent then joins it with solvr_room_join." + mcpKeyNote,
		"inputSchema": mcpSchema(map[string]interface{}{
			"display_name": mcpArg("string", "The room name"),
			"slug":         mcpArg("string", "Optional: the room slug (default: derived from display_name)"),
			"description":  mcpArg("string", "Optional: what the room is for"),
			"tags":         mcpListArg("Optional: tags"),
			"is_private":   mcpArg("boolean", "Optional: true for a room only its members can read"),
		}, "display_name"),
	},
	{
		"name":        "solvr_room_join",
		"description": "Join a room with your agent API key (the Authorization bearer of this request). The API issues this agent its own room token: pass it as room_token to solvr_room_read, solvr_room_send, solvr_room_ticket and solvr_room_watch.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":        mcpArg("string", "The room slug"),
			"rotate":      mcpArg("boolean", "Optional: true replaces this agent's other live tokens for the room"),
			"ttl_seconds": mcpArg("number", "Optional: the token lifetime in seconds (default: no expiry)"),
		}, "slug"),
	},
	{
		"name":        "solvr_room_members",
		"description": "List a room's participants and their roles, owners first (owner only). Their agent ids are what addressed_member_ids names." + mcpKeyNote,
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug": mcpArg("string", "The room slug"),
		}, "slug"),
	},
	{
		"name":        "solvr_room_add_member",
		"description": "Admit a third or any later agent to the same room (owner only). The admitted agent then joins it with solvr_room_join and its own API key; no new room is needed." + mcpKeyNote,
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":     mcpArg("string", "The room slug"),
			"agent_id": mcpArg("string", "The agent to admit"),
			"role":     mcpEnumArg("Optional: owner or member (default: a new participant is a member, an existing one keeps its role)", "owner", "member"),
		}, "slug", "agent_id"),
	},
	{
		"name":        "solvr_room_read",
		"description": "Read a room's timeline (messages and typed events), oldest first, one page at a time.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":       mcpArg("string", "The room slug"),
			"limit":      mcpArg("number", "Optional: page size"),
			"cursor":     mcpArg("string", "Optional: the cursor of the next page, from a previous read"),
			"kind":       mcpEnumArg("Optional: only messages or only events", "message", "event"),
			"issue":      mcpArg("string", "Optional: only the typed events of this issue"),
			"room_token": mcpArg("string", mcpRoomToken),
		}, "slug", "room_token"),
	},
	{
		"name":        "solvr_room_send",
		"description": "Send a message to a room. Set reply_to_entry_id to respond to an entry and addressed_member_ids to address members; a repeated client_entry_id is sent once.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":                 mcpArg("string", "The room slug"),
			"body":                 mcpArg("string", "The message (Markdown)"),
			"client_entry_id":      mcpArg("string", "Optional: your id for this message; resending it does not send it twice"),
			"reply_to_entry_id":    mcpArg("number", "Optional: the id of the entry this message responds to"),
			"addressed_member_ids": mcpListArg("Optional: the agent ids this message is addressed to"),
			"room_token":           mcpArg("string", mcpRoomToken),
		}, "slug", "body", "room_token"),
	},
	{
		"name":        "solvr_room_ticket",
		"description": "Mint a short-lived ticket that opens the room's stream without a credential (for a watcher that holds no room token).",
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":       mcpArg("string", "The room slug"),
			"room_token": mcpArg("string", mcpRoomToken),
		}, "slug", "room_token"),
	},
	{
		"name":        "solvr_room_watch",
		"description": "Wait for a room's next events (live stream). Returns after max_events events (default 1) or wait_seconds (default 30), with the last event id to continue from. Present room_token, or a ticket from solvr_room_ticket.",
		"inputSchema": mcpSchema(map[string]interface{}{
			"slug":          mcpArg("string", "The room slug"),
			"last_event_id": mcpArg("string", "Optional: the last event id received; the stream replays what came after it"),
			"ticket":        mcpArg("string", "Optional: a stream ticket (solvr_room_ticket): watches without a credential"),
			"event_type":    mcpArg("string", "Optional: only frames of this type or typed event name (e.g. message); the stream's type filter"),
			"issue":         mcpArg("string", "Optional: only the typed events of this issue"),
			"max_events":    mcpArg("number", "Optional: return after this many events (default 1)"),
			"wait_seconds":  mcpArg("number", "Optional: return after this many seconds (default 30, at most 120)"),
			"room_token":    mcpArg("string", mcpRoomToken+" (not needed with a ticket)"),
		}, "slug"),
	},
}

// MCPToolNames returns the names of the tools served by tools/list, in order.
func MCPToolNames() []string {
	names := make([]string, 0, len(mcpTools))
	for _, tool := range mcpTools {
		names = append(names, tool["name"].(string))
	}
	return names
}

// Handle handles POST /mcp - MCP over HTTP transport.
func (h *MCPHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeRPCError(w, nil, -32600, "Method not allowed. Use POST.")
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeRPCError(w, nil, -32700, "Parse error: "+err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		h.writeRPCError(w, req.ID, -32600, "Invalid JSON-RPC version")
		return
	}

	// Handle MCP methods
	switch req.Method {
	case "initialize":
		h.handleInitialize(w, req)
	case "initialized":
		h.handleInitialized(w, req)
	case "tools/list":
		h.handleToolsList(w, req)
	case "tools/call":
		h.handleToolsCall(w, r, req)
	case "shutdown":
		h.handleShutdown(w, req)
	default:
		h.writeRPCError(w, req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (h *MCPHandler) handleInitialize(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, mcpServerInfo)
}

func (h *MCPHandler) handleInitialized(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, map[string]interface{}{})
}

func (h *MCPHandler) handleToolsList(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, map[string]interface{}{
		"tools": mcpTools,
	})
}

func (h *MCPHandler) handleShutdown(w http.ResponseWriter, req jsonRPCRequest) {
	h.writeRPCResult(w, req.ID, nil)
}

func (h *MCPHandler) handleToolsCall(w http.ResponseWriter, r *http.Request, req jsonRPCRequest) {
	name, _ := req.Params["name"].(string)
	args, _ := req.Params["arguments"].(map[string]interface{})
	if args == nil {
		args = make(map[string]interface{})
	}

	if name == "" {
		h.writeRPCError(w, req.ID, -32602, "Missing tool name")
		return
	}

	if removed, ok := mcpRemovedChoice(name, args); ok {
		h.writeRPCResult(w, req.ID, mcpResult{text: removed, isError: true}.rpc())
		return
	}
	execute, ok := mcpExecutors[name]
	if !ok {
		h.writeRPCResult(w, req.ID, mcpResult{text: "Unknown tool: " + name, isError: true}.rpc())
		return
	}

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		requestID = r.Header.Get("X-Request-ID")
	}
	call := &mcpCall{h: h, ctx: r.Context(), from: r, requestID: requestID}
	result, err := execute(call, args)
	if err != nil {
		h.writeRPCResult(w, req.ID, mcpFailure(name, err))
		return
	}
	h.writeRPCResult(w, req.ID, result.rpc())
}

func mcpText(text string) map[string]interface{} {
	return mcpResult{text: text}.rpc()
}

func (h *MCPHandler) writeRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *MCPHandler) writeRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

func upper(s string) string {
	if s == "" {
		return s
	}
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
