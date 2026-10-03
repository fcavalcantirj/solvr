package main

import (
	"bufio"
	"io"
	"strings"
)

// sseEvent is one dispatched server-sent event.
type sseEvent struct {
	ID    string
	Event string
	Data  string
}

// readSSE parses a text/event-stream and calls onEvent for every dispatched event (a frame
// with data). Comments (": heartbeat") and retry-only frames dispatch nothing. It returns
// when the stream ends.
func readSSE(r io.Reader, onEvent func(sseEvent)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var cur sseEvent
	var data []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(data) > 0 {
				cur.Data = strings.Join(data, "\n")
				onEvent(cur)
			}
			cur, data = sseEvent{}, nil
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			cur.ID = value
		case "event":
			cur.Event = value
		case "data":
			data = append(data, value)
		}
	}
	return sc.Err()
}
