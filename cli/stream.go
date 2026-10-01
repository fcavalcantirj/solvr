package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// roomStreamEvent is one event of a room stream: its id (the entry id; empty on
// presence and room-update frames), its name, and its data as the API sent it
// (a RoomStreamFrame).
type roomStreamEvent struct {
	ID    string          `json:"id,omitempty"`
	Event string          `json:"event"`
	Frame json.RawMessage `json:"frame"`
}

// roomStream is an open room stream.
type roomStream struct {
	body  io.ReadCloser
	lines *bufio.Reader
}

// streamEnds are the events that end a stream; their data is {code, message}.
var streamEnds = map[string]bool{"access_revoked": true, "credential_rotated": true}

// openRoomStream opens a room's stream. No request timeout applies: the stream
// lasts until the server closes it or ends the caller's access.
func openRoomStream(streamURL, credential, lastEventID string) (*roomStream, error) {
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, apiErrorFrom(resp.StatusCode, body)
	}
	return &roomStream{body: resp.Body, lines: bufio.NewReader(resp.Body)}, nil
}

// next reads the next event, skipping heartbeats. A closed stream answers
// io.EOF; a stream the server ended because the caller's access did answers
// an *APIFailure of that code (CREDENTIAL_ROTATED: join again; ACCESS_REVOKED).
func (s *roomStream) next() (*roomStreamEvent, error) {
	var id, event string
	var data []string
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
				id = value
			case "event":
				event = value
			case "data":
				data = append(data, value)
			}
			continue
		}
		if len(data) == 0 {
			id, event = "", ""
			continue
		}
		return dispatchStreamEvent(id, event, strings.Join(data, "\n"))
	}
}

func dispatchStreamEvent(id, event, data string) (*roomStreamEvent, error) {
	if event == "" {
		event = "message"
	}
	if streamEnds[event] {
		var end struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &end); err != nil || end.Code == "" {
			return nil, fmt.Errorf("the stream ended with %s: %s", event, data)
		}
		answer, _ := json.Marshal(map[string]json.RawMessage{"error": json.RawMessage(data)})
		return nil, &APIFailure{Code: end.Code, Message: end.Message, Answer: answer}
	}
	if !json.Valid([]byte(data)) {
		return nil, fmt.Errorf("failed to decode the %s frame: %s", event, data)
	}
	return &roomStreamEvent{ID: id, Event: event, Frame: json.RawMessage(data)}, nil
}

// close closes the stream.
func (s *roomStream) close() error { return s.body.Close() }
