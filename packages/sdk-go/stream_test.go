package solvr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func streamServer(t *testing.T, status int, text string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
		} else {
			w.Header().Set("Content-Type", "text/event-stream")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, text)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func openStream(t *testing.T, srv *httptest.Server, opts *StreamRoomOptions) *RoomStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	stream, err := NewClient("", WithBaseURL(srv.URL)).StreamRoom(ctx, "demo", opts)
	if err != nil {
		t.Fatalf("StreamRoom: %v", err)
	}
	t.Cleanup(func() { stream.Close() })
	return stream
}

func TestStreamRoom_SkipsCommentsAndRetriesAndJoinsMultiLineData(t *testing.T) {
	srv := streamServer(t, http.StatusOK, ": heartbeat\n\nretry: 1000\n\n"+
		"id: 7\nevent: event\ndata: {\"id\":7,\"type\":\"event\",\n"+
		"data: \"room_id\":\"r1\",\"event\":\"handoff\",\"issue\":\"parser\",\"timestamp\":\"2026-10-01T18:41:41Z\"}\n\n"+
		"event: presence_join\ndata: {\"type\":\"presence_join\",\"room_id\":\"r1\",\"agent_name\":\"executor\",\"timestamp\":\"2026-10-01T18:41:42Z\"}\n\n")
	stream := openStream(t, srv, nil)

	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("first event: %v", err)
	}
	if ev.ID != "7" || ev.Event != "event" || ev.Frame == nil || ev.Frame.Event != "handoff" || ev.Frame.Issue != "parser" {
		t.Fatalf("first event = %+v frame %+v", ev, ev.Frame)
	}
	if _, err := ev.Frame.Message(); err == nil {
		t.Error("an event frame is not a message")
	}
	ev, err = stream.Next()
	if err != nil || ev.ID != "" || ev.Frame.Type != "presence_join" || ev.Frame.AgentName != "executor" {
		t.Fatalf("second event = %+v, %v", ev, err)
	}
	if stream.LastEventID() != "7" {
		t.Errorf("LastEventID = %q, want 7 (a frame without an id keeps the last one)", stream.LastEventID())
	}
	if _, err := stream.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("after the server closed: %v, want io.EOF", err)
	}
}

func TestStreamRoom_EndFramesSurfaceTheirCodeAsAnAPIError(t *testing.T) {
	for event, code := range map[string]string{"credential_rotated": "CREDENTIAL_ROTATED", "access_revoked": "ACCESS_REVOKED"} {
		srv := streamServer(t, http.StatusOK, "event: "+event+"\ndata: {\"code\":\""+code+"\",\"message\":\"ended\"}\n\n")
		stream := openStream(t, srv, nil)
		_, err := stream.Next()
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Code != code || apiErr.Message != "ended" {
			t.Errorf("%s: Next = %v, want APIError %s", event, err, code)
		}
	}
}

func TestStreamRoom_ARefusedStreamIsTheAPIError(t *testing.T) {
	srv := streamServer(t, http.StatusUnauthorized, `{"error":{"code":"STREAM_TICKET_EXPIRED","message":"expired","request_id":"rq-1"}}`)
	_, err := NewClient("", WithBaseURL(srv.URL)).StreamRoom(context.Background(), "demo", &StreamRoomOptions{Ticket: "solvr_st_x"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "STREAM_TICKET_EXPIRED" || apiErr.Status != http.StatusUnauthorized || apiErr.RequestID != "rq-1" {
		t.Fatalf("StreamRoom = %v, want the 401 STREAM_TICKET_EXPIRED APIError", err)
	}
}

func TestStreamRoom_AFrameWithBadJSONIsAnError(t *testing.T) {
	srv := streamServer(t, http.StatusOK, "id: 1\nevent: message\ndata: {not json\n\n")
	if _, err := openStream(t, srv, nil).Next(); err == nil {
		t.Fatal("a frame whose data is not JSON must fail")
	}
}

// The client's request timeout bounds a request, not a stream that stays open: the
// stream ends with its context.
func TestStreamRoom_OutlivesTheClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "id: 1\nevent: message\ndata: {\"id\":1,\"type\":\"message\",\"room_id\":\"r1\",\"timestamp\":\"2026-10-01T18:41:41Z\"}\n\n")
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := NewClient("", WithBaseURL(srv.URL), WithTimeout(100*time.Millisecond))
	stream, err := client.StreamRoom(ctx, "demo", nil)
	if err != nil {
		t.Fatalf("StreamRoom: %v", err)
	}
	defer stream.Close()
	if ev, err := stream.Next(); err != nil || ev.ID != "1" {
		t.Fatalf("Next = %+v, %v; the stream must outlive the 100ms request timeout", ev, err)
	}
}

func TestStreamRoom_ResumesWithLastEventIDAndAFilter(t *testing.T) {
	var lastEventID, query, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastEventID, query, auth = r.Header.Get("Last-Event-ID"), r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
	}))
	defer srv.Close()
	stream, err := NewClient("agent-key", WithBaseURL(srv.URL)).WithRoomToken("solvr_rt_x").
		StreamRoom(context.Background(), "demo room", &StreamRoomOptions{LastEventID: "41", Type: "event", Issue: "parser"})
	if err != nil {
		t.Fatalf("StreamRoom: %v", err)
	}
	stream.Close()
	if lastEventID != "41" || auth != "Bearer solvr_rt_x" || !strings.Contains(query, "type=event") || !strings.Contains(query, "issue=parser") {
		t.Errorf("sent Last-Event-ID %q, Authorization %q, query %q", lastEventID, auth, query)
	}
	if stream.LastEventID() != "41" {
		t.Errorf("LastEventID before any frame = %q, want the one resumed from", stream.LastEventID())
	}
}

func TestWithRoomTokenLeavesTheAgentClientAlone(t *testing.T) {
	agent := NewClient("agent-key")
	room := agent.WithRoomToken("solvr_rt_x")
	if agent.apiKey != "agent-key" || room.apiKey != "solvr_rt_x" {
		t.Errorf("agent key %q, room client key %q", agent.apiKey, room.apiKey)
	}
}
