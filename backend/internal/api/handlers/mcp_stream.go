package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// solvr_room_watch opens the room's stream through the router like any other tool request,
// reads its events as the stream handler writes them, and closes it (cancels the request)
// once max_events arrived, the stream ended, or wait_seconds passed. A tool answers once.

const (
	mcpWatchWaitSeconds    = 30
	mcpWatchWaitMaxSeconds = 120
)

type mcpStreamFrame struct {
	Type      string          `json:"type"`
	Event     string          `json:"event"`
	AgentName string          `json:"agent_name"`
	Payload   json.RawMessage `json:"payload"`
}

type mcpStreamEvent struct {
	id, event string
	frame     mcpStreamFrame
}

// line is one stream event: a message with its author and content, else the frame's event name.
func (e mcpStreamEvent) line() string {
	id := ""
	if e.id != "" {
		id = " (id " + e.id + ")"
	}
	if e.frame.Type == "message" && len(e.frame.Payload) > 0 {
		var message struct {
			AgentName   string `json:"agent_name"`
			Content     string `json:"content"`
			SequenceNum int    `json:"sequence_num"`
		}
		if json.Unmarshal(e.frame.Payload, &message) == nil {
			return fmt.Sprintf("#%d%s %s: %s", message.SequenceNum, id, message.AgentName, message.Content)
		}
	}
	name := e.frame.Event
	if name == "" {
		name = e.frame.Type
	}
	agent := ""
	if e.frame.AgentName != "" {
		agent = " " + e.frame.AgentName
	}
	return "[" + name + "]" + id + agent
}

// mcpStreamReader parses an event stream as it is written. It skips comments (heartbeats),
// keeps up to max events, and stops the stream at max or at an event that ends it.
type mcpStreamReader struct {
	max       int
	stop      context.CancelFunc
	buf       string
	id, event string
	data      []string
	events    []mcpStreamEvent
	failure   error
	done      bool
}

func (s *mcpStreamReader) write(p []byte) {
	if s.done {
		return
	}
	s.buf += string(p)
	for !s.done {
		end := strings.IndexByte(s.buf, '\n')
		if end < 0 {
			return
		}
		line := strings.TrimSuffix(s.buf[:end], "\r")
		s.buf = s.buf[end+1:]
		s.line(line)
	}
}

func (s *mcpStreamReader) line(line string) {
	if line == "" {
		if len(s.data) > 0 {
			s.dispatch()
		}
		s.id, s.event, s.data = "", "", nil
		return
	}
	if strings.HasPrefix(line, ":") {
		return
	}
	field, value, _ := strings.Cut(line, ":")
	value = strings.TrimPrefix(value, " ")
	switch field {
	case "id":
		s.id = value
	case "event":
		s.event = value
	case "data":
		s.data = append(s.data, value)
	}
}

func (s *mcpStreamReader) dispatch() {
	event := s.event
	if event == "" {
		event = "message"
	}
	data := strings.Join(s.data, "\n")
	if event == "access_revoked" || event == "credential_rotated" {
		var end struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal([]byte(data), &end)
		if end.Code == "" {
			end.Code = strings.ToUpper(event)
		}
		if end.Message == "" {
			end.Message = "the stream ended with " + event
		}
		s.finish(&mcpAPIError{code: end.Code, message: end.Message, requestID: end.RequestID})
		return
	}
	var frame mcpStreamFrame
	if err := json.Unmarshal([]byte(data), &frame); err != nil {
		s.finish(fmt.Errorf("failed to decode the %s frame: %w", event, err))
		return
	}
	s.events = append(s.events, mcpStreamEvent{id: s.id, event: event, frame: frame})
	if len(s.events) >= s.max {
		s.finish(nil)
	}
}

func (s *mcpStreamReader) finish(failure error) {
	s.failure = failure
	s.done = true
	s.stop()
}

func mcpRoomWatch(c *mcpCall, args map[string]interface{}) (mcpResult, error) {
	slug, err := mcpRequireString(args, "slug")
	if err != nil {
		return mcpResult{}, err
	}
	q := url.Values{}
	auth := "" // a ticket watch is anonymous
	if ticket := mcpOptionalString(args, "ticket"); ticket != "" {
		q.Set("ticket", ticket)
	} else if auth, err = mcpRoomAuth(slug, args); err != nil {
		return mcpResult{}, err
	}
	setString(q, "type", args, "event_type")
	setString(q, "issue", args, "issue")
	headers := map[string]string{"Accept": "text/event-stream"}
	lastEventID := mcpOptionalString(args, "last_event_id")
	if lastEventID != "" {
		headers["Last-Event-ID"] = lastEventID
	}
	maxEvents := 1
	if n, ok, err := mcpOptionalNumber(args, "max_events"); err != nil {
		return mcpResult{}, err
	} else if ok && n > 1 {
		maxEvents = int(math.Floor(n))
	}
	wait := float64(mcpWatchWaitSeconds)
	if n, ok, err := mcpOptionalNumber(args, "wait_seconds"); err != nil {
		return mcpResult{}, err
	} else if ok {
		wait = math.Min(mcpWatchWaitMaxSeconds, math.Max(1, n))
	}

	ctx, cancel := context.WithTimeout(c.ctx, time.Duration(wait*float64(time.Second)))
	defer cancel()
	reader := &mcpStreamReader{max: maxEvents, stop: cancel}
	w := newMCPResponseWriter()
	w.stream = reader.write
	if err := c.serve(ctx, apiRequest{method: http.MethodGet, path: apiPath("/v1/rooms/%s/stream", slug),
		query: q, headers: headers, auth: auth}, w); err != nil {
		return mcpResult{}, err
	}
	if w.status >= 400 {
		return mcpResult{}, mcpAPIErrorFrom(w)
	}
	if reader.failure != nil && len(reader.events) == 0 {
		return mcpResult{}, reader.failure
	}
	timedOut := !reader.done && errors.Is(ctx.Err(), context.DeadlineExceeded)

	var lines []string
	lastID := lastEventID
	for _, event := range reader.events {
		lines = append(lines, event.line())
		if event.id != "" {
			lastID = event.id
		}
	}
	if len(reader.events) == 0 {
		if timedOut {
			lines = append(lines, "No events within "+strconv.FormatFloat(wait, 'f', -1, 64)+" s.")
		} else {
			lines = append(lines, "No events: the stream ended.")
		}
	}
	if reader.failure != nil {
		lines = append(lines, "", mcpFailureText("solvr_room_watch", reader.failure))
		return mcpResult{text: strings.Join(lines, "\n"), isError: true}, nil
	}
	if lastID != "" {
		lines = append(lines, "", "To continue: solvr_room_watch with slug "+slug+" and last_event_id "+lastID)
	}
	return mcpLines(lines...), nil
}
