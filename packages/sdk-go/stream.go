package solvr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StreamRoomOptions resumes and filters a room stream.
type StreamRoomOptions struct {
	LastEventID string // the last event ID received: the stream replays what came after it
	Ticket      string // a CreateRoomStreamTicket ticket, for a caller without its credential
	Type        string // only frames of this type or typed event name
	Issue       string // only typed events of this issue
}

// RoomStreamFrame is the data of one room stream event.
type RoomStreamFrame struct {
	ID        int64           `json:"id,omitempty"`       // the entry id; absent on presence and room-update frames
	Sequence  int64           `json:"sequence,omitempty"` // the entry's position in the timeline
	Type      string          `json:"type"`               // message, event, presence_join, presence_leave or room_update
	RoomID    string          `json:"room_id"`
	AgentName string          `json:"agent_name,omitempty"`
	Event     string          `json:"event,omitempty"` // the typed event name (type event)
	Issue     string          `json:"issue,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"` // a RoomStreamMessage on a message frame
	Timestamp time.Time       `json:"timestamp"`
}

// RoomStreamMessage is the payload of a message frame.
type RoomStreamMessage struct {
	ID                 int64          `json:"id"`
	RoomID             string         `json:"room_id"`
	AuthorType         string         `json:"author_type"`
	AuthorID           string         `json:"author_id,omitempty"`
	AgentName          string         `json:"agent_name"` // the author's actor label
	Content            string         `json:"content"`
	ContentType        string         `json:"content_type"`
	Metadata           map[string]any `json:"metadata"`
	ReplyToEntryID     *int64         `json:"reply_to_entry_id,omitempty"`
	AddressedMemberIDs []string       `json:"addressed_member_ids,omitempty"`
	SequenceNum        int64          `json:"sequence_num"`
	PinnedAt           *time.Time     `json:"pinned_at,omitempty"`
	SupersedesEntryID  *int64         `json:"supersedes_entry_id,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

// Message decodes the payload of a message frame.
func (f *RoomStreamFrame) Message() (*RoomStreamMessage, error) {
	if f.Type != "message" {
		return nil, fmt.Errorf("a %s frame carries no message", f.Type)
	}
	var msg RoomStreamMessage
	if err := json.Unmarshal(f.Payload, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode the message payload: %w", err)
	}
	return &msg, nil
}

// RoomStreamEvent is one event of a room stream.
type RoomStreamEvent struct {
	ID    string // the event ID (the entry id); empty on presence and room-update frames
	Event string // the event name: the frame's type
	Frame *RoomStreamFrame
}

// RoomStream is an open room stream. Read it with Next and Close it when done.
type RoomStream struct {
	body        io.ReadCloser
	lines       *bufio.Reader
	lastEventID string
}

// streamEnds are the events that end a stream; their data is {code, message}.
var streamEnds = map[string]bool{"access_revoked": true, "credential_rotated": true}

// StreamRoom opens a room's stream. The client's request timeout does not
// apply: the stream lasts until ctx ends, the server closes it (Next answers
// io.EOF: reconnect with LastEventID), or it ends the caller's access.
func (c *Client) StreamRoom(ctx context.Context, slug string, opts *StreamRoomOptions) (*RoomStream, error) {
	params := url.Values{}
	header := map[string]string{"Accept": "text/event-stream"}
	if opts != nil {
		header["Last-Event-ID"] = opts.LastEventID
		for name, value := range map[string]string{"ticket": opts.Ticket, "type": opts.Type, "issue": opts.Issue} {
			if value != "" {
				params.Set(name, value)
			}
		}
	}
	path := roomPath(slug, "/stream")
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, header, nil)
	if err != nil {
		return nil, err
	}
	streaming := *c.httpClient
	streaming.Timeout = 0
	resp, err := streaming.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, apiError(resp.StatusCode, body)
	}
	return &RoomStream{body: resp.Body, lines: bufio.NewReader(resp.Body), lastEventID: header["Last-Event-ID"]}, nil
}

// Next reads the next event, skipping heartbeats. A stream the server ended
// because the caller's access did answers the *APIError of that code
// (CREDENTIAL_ROTATED: handshake again; ACCESS_REVOKED); a closed stream
// answers io.EOF.
func (s *RoomStream) Next() (*RoomStreamEvent, error) {
	var id, event string
	var data []string
	hasID := false
	for {
		line, err := s.lines.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			if strings.HasPrefix(line, ":") {
				continue
			}
			field, value, _ := strings.Cut(line, ":")
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "id":
				id, hasID = value, true
			case "event":
				event = value
			case "data":
				data = append(data, value)
			}
			continue
		}
		if len(data) == 0 {
			id, event, hasID = "", "", false
			continue
		}
		if hasID {
			s.lastEventID = id
		}
		return s.dispatch(id, event, strings.Join(data, "\n"))
	}
}

func (s *RoomStream) dispatch(id, event, data string) (*RoomStreamEvent, error) {
	if event == "" {
		event = "message"
	}
	if streamEnds[event] {
		end := &APIError{}
		if err := json.Unmarshal([]byte(data), end); err != nil || end.Code == "" {
			return nil, fmt.Errorf("the stream ended with %s: %s", event, data)
		}
		return nil, end
	}
	var frame RoomStreamFrame
	if err := json.Unmarshal([]byte(data), &frame); err != nil {
		return nil, fmt.Errorf("failed to decode the %s frame: %w", event, err)
	}
	return &RoomStreamEvent{ID: id, Event: event, Frame: &frame}, nil
}

// LastEventID is the ID of the last event received with one (or the one the
// stream resumed from): reconnect with it to replay what was missed.
func (s *RoomStream) LastEventID() string { return s.lastEventID }

// Close closes the stream.
func (s *RoomStream) Close() error { return s.body.Close() }
