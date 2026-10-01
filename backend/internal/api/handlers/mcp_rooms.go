package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// The room tools: create, join, read, send, ticket and watch. create and join present the
// caller's agent API key; read, send, ticket and watch present the room_token argument (the
// token join returned), never the caller's key. /v1/mcp keeps no state between calls.

// mcpRoomAuth is the Authorization of a room-token call.
func mcpRoomAuth(slug string, args map[string]interface{}) (string, error) {
	token := mcpOptionalString(args, "room_token")
	if token == "" {
		return "", &ValidationError{Message: "No room token for " + slug + ". Call solvr_room_join with slug \"" + slug +
			"\" and pass the room_token it returns."}
	}
	return "Bearer " + token, nil
}

func mcpRoomCreate(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	name, err := mcpRequireString(args, "display_name")
	if err != nil {
		return mcpResult{}, err
	}
	body := map[string]interface{}{"display_name": name}
	for _, field := range []string{"slug", "description"} {
		if value := mcpOptionalString(args, field); value != "" {
			body[field] = value
		}
	}
	if tags, ok := mcpOptionalList(args, "tags"); ok {
		body["tags"] = tags
	}
	if args["is_private"] != nil {
		body["is_private"] = args["is_private"] == true
	}
	var answer struct {
		Data struct {
			ID          string `json:"id"`
			Slug        string `json:"slug"`
			DisplayName string `json:"display_name"`
			IsPrivate   bool   `json:"is_private"`
		} `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: "/v1/rooms", auth: c.callerAuth(), body: body}, &answer); err != nil {
		return mcpResult{}, err
	}
	room := answer.Data
	private := ""
	if room.IsPrivate {
		private = " (private)"
	}
	return mcpLines("Room "+room.Slug+" created"+private+": "+room.DisplayName, "ID: "+room.ID,
		"Join it with solvr_room_join (slug "+room.Slug+"); every agent that joins the same slug works in this room."), nil
}

func mcpRoomJoin(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	slug, err := mcpRequireString(args, "slug")
	if err != nil {
		return mcpResult{}, err
	}
	body := map[string]interface{}{}
	if args["rotate"] != nil {
		body["rotate"] = args["rotate"] == true
	}
	ttl, ok, err := mcpOptionalNumber(args, "ttl_seconds")
	if err != nil {
		return mcpResult{}, err
	}
	if ok {
		body["ttl_seconds"] = int64(ttl)
	}
	var answer struct {
		Data struct {
			AgentID   string `json:"agent_id"`
			RoomSlug  string `json:"room_slug"`
			RoomToken string `json:"room_token"`
			Rotated   bool   `json:"rotated"`
		} `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: apiPath("/v1/rooms/%s/handshake", slug), auth: c.callerAuth(), body: body}, &answer); err != nil {
		return mcpResult{}, err
	}
	joined := answer.Data
	rotated := ""
	if joined.Rotated {
		rotated = " (your other tokens for this room were rotated)"
	}
	return mcpLines("Joined "+joined.RoomSlug+" as "+joined.AgentID+rotated, "Room token: "+joined.RoomToken,
		"Pass it as room_token to solvr_room_read, solvr_room_send, solvr_room_ticket and solvr_room_watch on "+slug+"."), nil
}

type mcpRoomEntry struct {
	ID         int64    `json:"id"`
	Sequence   int      `json:"sequence"`
	Kind       string   `json:"kind"`
	ActorLabel string   `json:"actor_label"`
	Body       string   `json:"body"`
	ReplyTo    *int64   `json:"reply_to_entry_id"`
	Addressed  []string `json:"addressed_member_ids"`
	EventType  string   `json:"event_type"`
	Issue      string   `json:"issue"`
}

// line is one timeline entry: its sequence and id, who wrote it, and its body or event.
func (e mcpRoomEntry) line() string {
	head := fmt.Sprintf("#%d (id %d) %s", e.Sequence, e.ID, e.ActorLabel)
	var notes []string
	if e.ReplyTo != nil && *e.ReplyTo != 0 {
		notes = append(notes, "reply to "+strconv.FormatInt(*e.ReplyTo, 10))
	}
	if len(e.Addressed) > 0 {
		notes = append(notes, "to "+strings.Join(e.Addressed, ", "))
	}
	note := ""
	if len(notes) > 0 {
		note = " (" + strings.Join(notes, "; ") + ")"
	}
	if e.Kind == "event" {
		name := e.EventType
		if name == "" {
			name = "event"
		}
		issue := ""
		if e.Issue != "" {
			issue = " issue " + e.Issue
		}
		return head + " [" + name + "]" + issue + note
	}
	return head + note + ": " + e.Body
}

func mcpRoomRead(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	slug, err := mcpRequireString(args, "slug")
	if err != nil {
		return mcpResult{}, err
	}
	auth, err := mcpRoomAuth(slug, args)
	if err != nil {
		return mcpResult{}, err
	}
	q := url.Values{}
	if err := setNumber(q, "limit", args, "limit"); err != nil {
		return mcpResult{}, err
	}
	for _, name := range []string{"cursor", "kind", "issue"} {
		setString(q, name, args, name)
	}
	var page struct {
		Data []mcpRoomEntry `json:"data"`
		Meta struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if _, err := c.do(apiRequest{method: http.MethodGet, path: apiPath("/v1/rooms/%s/entries", slug), query: q, auth: auth}, &page); err != nil {
		return mcpResult{}, err
	}
	if len(page.Data) == 0 {
		return mcpLines("No entries yet."), nil
	}
	var lines []string
	for _, entry := range page.Data {
		lines = append(lines, entry.line())
	}
	if page.Meta.HasMore && page.Meta.NextCursor != "" {
		lines = append(lines, "", "More: call solvr_room_read with slug "+slug+" and cursor "+page.Meta.NextCursor)
	}
	return mcpLines(lines...), nil
}

func mcpRoomSend(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	slug, err := mcpRequireString(args, "slug")
	if err != nil {
		return mcpResult{}, err
	}
	text, err := mcpRequireString(args, "body")
	if err != nil {
		return mcpResult{}, err
	}
	auth, err := mcpRoomAuth(slug, args)
	if err != nil {
		return mcpResult{}, err
	}
	body := map[string]interface{}{"body": text}
	if id := mcpOptionalString(args, "client_entry_id"); id != "" {
		body["client_entry_id"] = id
	}
	replyTo, ok, err := mcpOptionalNumber(args, "reply_to_entry_id")
	if err != nil {
		return mcpResult{}, err
	}
	if ok {
		body["reply_to_entry_id"] = int64(replyTo)
	}
	if addressed, ok := mcpOptionalList(args, "addressed_member_ids"); ok {
		body["addressed_member_ids"] = addressed
	}
	var answer struct {
		Data mcpRoomEntry `json:"data"`
		Meta struct {
			IdempotentReplay bool `json:"idempotent_replay"`
		} `json:"meta"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: apiPath("/v1/rooms/%s/entries", slug), auth: auth, body: body}, &answer); err != nil {
		return mcpResult{}, err
	}
	id := strconv.FormatInt(answer.Data.ID, 10)
	if answer.Meta.IdempotentReplay {
		return mcpLines("Message " + id + " was already sent (same client_entry_id); not sent again."), nil
	}
	return mcpLines("Message " + id + " sent to " + slug + " (#" + itoa(answer.Data.Sequence) + ")."), nil
}

func mcpRoomTicket(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	slug, err := mcpRequireString(args, "slug")
	if err != nil {
		return mcpResult{}, err
	}
	auth, err := mcpRoomAuth(slug, args)
	if err != nil {
		return mcpResult{}, err
	}
	var answer struct {
		Data struct {
			Ticket     string `json:"ticket"`
			ExpiresAt  string `json:"expires_at"`
			TTLSeconds int    `json:"ttl_seconds"`
			Stream     string `json:"stream"`
		} `json:"data"`
	}
	if _, err := c.do(apiRequest{method: http.MethodPost, path: apiPath("/v1/rooms/%s/stream-ticket", slug), auth: auth}, &answer); err != nil {
		return mcpResult{}, err
	}
	t := answer.Data
	return mcpLines("Ticket: "+t.Ticket, "Opens "+t.Stream+" until "+t.ExpiresAt+" ("+itoa(t.TTLSeconds)+
		" s) without a credential: solvr_room_watch with slug "+slug+" and this ticket."), nil
}
