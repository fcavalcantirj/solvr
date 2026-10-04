package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// CreateRoomRequest is the request body for creating a room: only the fields
// given are sent.
type CreateRoomRequest struct {
	DisplayName string   `json:"display_name"`
	Slug        string   `json:"slug,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	IsPrivate   bool     `json:"is_private,omitempty"`
}

// Room is the part of a room the CLI shows.
type Room struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	IsPrivate   bool   `json:"is_private"`
}

// HandshakeRoomRequest is the request body for joining a room.
type HandshakeRoomRequest struct {
	TTLSeconds int  `json:"ttl_seconds,omitempty"`
	Rotate     bool `json:"rotate,omitempty"`
}

// RoomHandshake is the room token a join issued, shown once.
type RoomHandshake struct {
	AgentID   string `json:"agent_id"`
	RoomSlug  string `json:"room_slug"`
	RoomToken string `json:"room_token"`
	Rotated   bool   `json:"rotated"`
}

// HandshakeRoomResponse is the answer of a join.
type HandshakeRoomResponse struct {
	Data RoomHandshake `json:"data"`
}

// CreateRoomEntryRequest is the request body for sending a message.
type CreateRoomEntryRequest struct {
	Body               string   `json:"body"`
	ClientEntryID      string   `json:"client_entry_id,omitempty"`
	ReplyToEntryID     *int64   `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs []string `json:"addressed_member_ids,omitempty"`
}

// RoomEntry is one message or typed event of a room's timeline.
type RoomEntry struct {
	ID         int64  `json:"id"`
	Sequence   int64  `json:"sequence"`
	Kind       string `json:"kind"`
	ActorLabel string `json:"actor_label"`
	Body       string `json:"body"`
	EventType  string `json:"event_type"`
}

func displayRoom(out io.Writer, body []byte) error {
	var resp struct {
		Data Room `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	private := ""
	if resp.Data.IsPrivate {
		private = " (private)"
	}
	fmt.Fprintf(out, "Room %s created%s\n", resp.Data.Slug, private)
	fmt.Fprintf(out, "Join it with: solvr room join %s\n", resp.Data.Slug)
	return nil
}

func displayJoined(out io.Writer, slug string, handshake RoomHandshake) {
	rotated := ""
	if handshake.Rotated {
		rotated = " (other tokens rotated)"
	}
	fmt.Fprintf(out, "Joined %s as %s%s\n", handshake.RoomSlug, handshake.AgentID, rotated)
	fmt.Fprintf(out, "Room token: %s\n", handshake.RoomToken)
	fmt.Fprintf(out, "Saved for: solvr room read|send|ticket|watch %s\n", slug)
}

func displayRoomEntries(out io.Writer, slug string, body []byte) error {
	var page struct {
		Data []RoomEntry `json:"data"`
		Meta struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if len(page.Data) == 0 {
		fmt.Fprintln(out, "No entries yet.")
		return nil
	}
	for _, entry := range page.Data {
		text := entry.Body
		if entry.Kind == "event" {
			eventType := entry.EventType
			if eventType == "" {
				eventType = "event"
			}
			text = "[" + eventType + "]"
		}
		fmt.Fprintf(out, "#%d %s: %s\n", entry.Sequence, entry.ActorLabel, text)
	}
	if page.Meta.HasMore && page.Meta.NextCursor != "" {
		fmt.Fprintf(out, "More: solvr room read %s --cursor %s\n", slug, page.Meta.NextCursor)
	}
	return nil
}

func displayRoomEntry(out io.Writer, body []byte) error {
	var resp struct {
		Data RoomEntry `json:"data"`
		Meta struct {
			IdempotentReplay bool `json:"idempotent_replay"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if resp.Meta.IdempotentReplay {
		fmt.Fprintf(out, "Message %d was already sent (same client entry id)\n", resp.Data.ID)
		return nil
	}
	fmt.Fprintf(out, "Message %d sent\n", resp.Data.ID)
	return nil
}

func displayTicket(out io.Writer, body []byte) error {
	var resp struct {
		Data struct {
			Ticket    string `json:"ticket"`
			ExpiresAt string `json:"expires_at"`
			Stream    string `json:"stream"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	fmt.Fprintf(out, "Ticket: %s\n", resp.Data.Ticket)
	fmt.Fprintf(out, "Opens %s until %s\n", resp.Data.Stream, resp.Data.ExpiresAt)
	return nil
}

// displayStreamEvent prints one stream event: a JSON line with --json, else a
// message as "#sequence author: content" and any other frame as its name.
func displayStreamEvent(out io.Writer, event *roomStreamEvent, jsonOutput bool) error {
	if jsonOutput {
		encoder := json.NewEncoder(out)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(event)
	}
	var frame struct {
		Type      string `json:"type"`
		Event     string `json:"event"`
		AgentName string `json:"agent_name"`
		Payload   *struct {
			AgentName   string `json:"agent_name"`
			Content     string `json:"content"`
			SequenceNum int64  `json:"sequence_num"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(event.Frame, &frame); err != nil {
		return fmt.Errorf("failed to decode the %s frame: %w", event.Event, err)
	}
	if frame.Type == "message" && frame.Payload != nil {
		fmt.Fprintf(out, "#%d %s: %s\n", frame.Payload.SequenceNum, frame.Payload.AgentName, frame.Payload.Content)
		return nil
	}
	name := frame.Event
	if name == "" {
		name = frame.Type
	}
	if frame.AgentName != "" {
		fmt.Fprintf(out, "[%s] %s\n", name, frame.AgentName)
		return nil
	}
	fmt.Fprintf(out, "[%s]\n", name)
	return nil
}
