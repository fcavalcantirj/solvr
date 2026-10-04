package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// roomServer records every request and answers with answer(r) (nil answers 404).
type roomServer struct {
	*httptest.Server
	mu   sync.Mutex
	sent []recordedRequest
}

func newRoomServer(t *testing.T, answer func(w http.ResponseWriter, r *http.Request)) *roomServer {
	t.Helper()
	s := &roomServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.sent = append(s.sent, recordedRequest{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.Query(), header: r.Header.Clone(), body: body})
		s.mu.Unlock()
		if answer == nil {
			http.NotFound(w, r)
			return
		}
		answer(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *roomServer) requests() []recordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]recordedRequest(nil), s.sent...)
}

func answerJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func readConfigFile(t *testing.T, home string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".solvr", "config"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

const handshakeAnswer = `{"data":{"agent_id":"agent_exec","room_slug":"plan-room","room_token":"solvr_rt_issued_one","rotated":false}}`

func TestRoomJoin_SavesTheRoomTokenAndSendPresentsIt(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/handshake"):
			answerJSON(w, 201, handshakeAnswer)
		case strings.HasSuffix(r.URL.Path, "/entries"):
			answerJSON(w, 201, `{"data":{"id":7,"sequence":1,"kind":"message","actor_label":"agent_exec","body":"hi"},"meta":{"idempotent_replay":false}}`)
		}
	})
	home := writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	joined := runCLI(t, "room", "join", "plan-room")
	if joined.code != 0 {
		t.Fatalf("join exited %d: %s", joined.code, joined.stderr)
	}
	for _, want := range []string{"Joined plan-room as agent_exec", "solvr_rt_issued_one"} {
		if !strings.Contains(joined.stdout, want) {
			t.Errorf("join output %q, want %q", joined.stdout, want)
		}
	}
	if cfg := readConfigFile(t, home); !strings.Contains(cfg, "room-token.plan-room=solvr_rt_issued_one") {
		t.Errorf("config %q does not hold the room token", cfg)
	}

	send := runCLI(t, "room", "send", "plan-room", "--body", "hi")
	if send.code != 0 || !strings.Contains(send.stdout, "Message 7 sent") {
		t.Fatalf("send exited %d: %q %q", send.code, send.stdout, send.stderr)
	}
	sent := srv.requests()
	if len(sent) != 2 {
		t.Fatalf("sent %d requests, want 2", len(sent))
	}
	if got := sent[0].header.Get("Authorization"); got != "Bearer solvr_agent_key" {
		t.Errorf("join presented %q, want the API key", got)
	}
	if string(sent[0].body) != "{}" {
		t.Errorf("join sent body %q, want {}", sent[0].body)
	}
	if got := sent[1].header.Get("Authorization"); got != "Bearer solvr_rt_issued_one" {
		t.Errorf("send presented %q, want the saved room token", got)
	}
}

func TestRoomJoin_TokensArePerRoomAndALaterJoinReplacesThem(t *testing.T) {
	token := "solvr_rt_first"
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		slug := strings.Split(r.URL.Path, "/")[3]
		answerJSON(w, 201, `{"data":{"agent_id":"agent_a","room_slug":"`+slug+`","room_token":"`+token+`","rotated":true}}`)
	})
	home := writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	runCLI(t, "room", "join", "room-a")
	token = "solvr_rt_second"
	runCLI(t, "room", "join", "room-b")
	token = "solvr_rt_third"
	rotated := runCLI(t, "room", "join", "room-a", "--rotate", "--ttl", "3600")
	if !strings.Contains(rotated.stdout, "other tokens rotated") {
		t.Errorf("a rotating join should say so, got %q", rotated.stdout)
	}

	cfg := readConfigFile(t, home)
	for _, want := range []string{"room-token.room-a=solvr_rt_third", "room-token.room-b=solvr_rt_second"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config %q, want %q", cfg, want)
		}
	}
	if strings.Contains(cfg, "solvr_rt_first") {
		t.Errorf("config %q still holds the replaced token", cfg)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(srv.requests()[2].body, &body)
	if body["rotate"] != true || body["ttl_seconds"] != float64(3600) {
		t.Errorf("join --rotate --ttl 3600 sent %v", body)
	}
}

func TestRoomCommands_WithoutARoomTokenFailBeforeAnyRequest(t *testing.T) {
	srv := newRoomServer(t, nil)
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")
	for _, args := range [][]string{
		{"room", "send", "plan-room", "--body", "hi"},
		{"room", "ticket", "plan-room"},
		{"room", "watch", "plan-room"},
	} {
		res := runCLI(t, args...)
		if res.code != 1 || !strings.Contains(res.stderr, "No room token for plan-room. Run: solvr room join plan-room") {
			t.Errorf("solvr %s: exit %d, stderr %q", strings.Join(args, " "), res.code, res.stderr)
		}
		if strings.Contains(res.stderr, "Usage:") || strings.Contains(res.stdout, "Usage:") || strings.Contains(res.stderr, "--help") {
			t.Errorf("solvr %s pointed at the usage for a missing room token: %q", strings.Join(args, " "), res.stderr)
		}
	}
	if n := len(srv.requests()); n != 0 {
		t.Errorf("sent %d requests without a room token, want 0 (the API key must not stand in)", n)
	}
}

func TestRoomCommands_ARoomTokenFlagBeatsTheSavedOneAndSlugsAreEscaped(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":[],"meta":{"limit":50,"has_more":false,"next_cursor":null}}`)
	})
	home := writeConfig(t, srv.URL+"/v1", "solvr_agent_key")
	cfg := readConfigFile(t, home) + "room-token.a b/c=solvr_rt_saved\n"
	if err := os.WriteFile(filepath.Join(home, ".solvr", "config"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	saved := runCLI(t, "room", "read", "a b/c")
	given := runCLI(t, "room", "read", "a b/c", "--room-token", "solvr_rt_given")
	if saved.code != 0 || given.code != 0 {
		t.Fatalf("exit %d/%d: %s %s", saved.code, given.code, saved.stderr, given.stderr)
	}
	sent := srv.requests()
	if sent[0].path != "/v1/rooms/a%20b%2Fc/entries" {
		t.Errorf("path %s, want the slug escaped", sent[0].path)
	}
	if got := sent[0].header.Get("Authorization"); got != "Bearer solvr_rt_saved" {
		t.Errorf("saved token: presented %q", got)
	}
	if got := sent[1].header.Get("Authorization"); got != "Bearer solvr_rt_given" {
		t.Errorf("--room-token: presented %q", got)
	}
	if !strings.Contains(saved.stdout, "No entries yet") {
		t.Errorf("empty timeline output %q", saved.stdout)
	}
}

func TestRoomCreate_SendsOnlyTheFieldsGiven(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 201, `{"data":{"id":"r1","slug":"secret-room","display_name":"Secret","is_private":true}}`)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	res := runCLI(t, "room", "create", "--display-name", "Secret", "--private")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if body := string(srv.requests()[0].body); body != `{"display_name":"Secret","is_private":true}` {
		t.Errorf("create sent %s", body)
	}
	for _, want := range []string{"Room secret-room created (private)", "solvr room join secret-room"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
}

// A public room is readable by anyone: without a room token, room read reads it anonymously and
// never presents the API key in its place.
func TestRoomRead_WithoutARoomTokenReadsAPublicRoomAnonymously(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":[{"id":1,"sequence":1,"kind":"message","actor_label":"agent_planner","body":"Plan: build it"}],"meta":{"has_more":false}}`)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	res := runCLI(t, "room", "read", "plan-room")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	sent := srv.requests()
	if len(sent) != 1 || sent[0].path != "/v1/rooms/plan-room/entries" {
		t.Fatalf("read sent %v, want one GET of the entries", sent)
	}
	if got := sent[0].header.Get("Authorization"); got != "" {
		t.Errorf("an anonymous read sent Authorization %q", got)
	}
	if !strings.Contains(res.stdout, "#1 agent_planner: Plan: build it") {
		t.Errorf("output %q, want the entry", res.stdout)
	}
}

// A closed room answers an anonymous read with 403: the error says how to get a room token.
func TestRoomRead_AClosedRoomWithoutATokenSaysToJoin(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 403, `{"error":{"code":"FORBIDDEN","message":"this room is closed to non-members"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_agent_key")

	res := runCLI(t, "room", "read", "plan-room")
	if res.code != 1 || !strings.Contains(res.stderr, "solvr room join plan-room") {
		t.Errorf("exit %d, stderr %q; want exit 1 and the join command", res.code, res.stderr)
	}
}

func TestRoomRead_SendsItsFiltersAndShowsTheNextCursor(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":[
			{"id":1,"sequence":1,"kind":"message","actor_label":"agent_planner","body":"Plan: build it"},
			{"id":2,"sequence":2,"kind":"event","actor_label":"agent_exec","event_type":"task_claimed"}
		],"meta":{"limit":2,"has_more":true,"next_cursor":"c2"}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "room", "read", "plan-room", "--room-token", "solvr_rt_x", "--limit", "2", "--cursor", "c1", "--kind", "message", "--issue", "7")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	q := srv.requests()[0].query
	if q.Get("limit") != "2" || q.Get("cursor") != "c1" || q.Get("kind") != "message" || q.Get("issue") != "7" {
		t.Errorf("read sent query %v", q)
	}
	for _, want := range []string{"#1 agent_planner: Plan: build it", "#2 agent_exec: [task_claimed]", "More: solvr room read plan-room --cursor c2"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
}

func TestRoomSend_RepliesToAndAddressesMembersAndSaysWhenItWasAlreadySent(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, `{"data":{"id":9,"sequence":3,"kind":"message"},"meta":{"idempotent_replay":true}}`)
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "room", "send", "plan-room", "--room-token", "solvr_rt_x", "--body", "Done",
		"--reply-to", "1042", "--to", "agent_a, agent_b,", "--client-entry-id", "done-1")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(srv.requests()[0].body, &body)
	members, _ := body["addressed_member_ids"].([]interface{})
	if body["reply_to_entry_id"] != float64(1042) || len(members) != 2 || members[0] != "agent_a" || members[1] != "agent_b" || body["client_entry_id"] != "done-1" {
		t.Errorf("send sent %s", srv.requests()[0].body)
	}
	if !strings.Contains(res.stdout, "Message 9 was already sent (same client entry id)") {
		t.Errorf("output %q", res.stdout)
	}
}

func TestRoomWatch_SkipsHeartbeatsAndStopsAfterMaxOnAStreamTheServerKeepsOpen(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, ": heartbeat\n\n"+
			"event: presence_join\ndata: {\"type\":\"presence_join\",\"room_id\":\"r1\",\"agent_name\":\"agent_rev\"}\n\n"+
			"id: 5\nevent: message\ndata: {\"id\":5,\"sequence\":2,\"type\":\"message\",\"room_id\":\"r1\",\"payload\":{\"id\":5,\"agent_name\":\"agent_planner\",\"content\":\"Plan ready\",\"sequence_num\":2}}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "room", "watch", "plan-room", "--room-token", "solvr_rt_x", "--max", "2", "--type", "message", "--last-event-id", "4")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	for _, want := range []string{"[presence_join] agent_rev", "#2 agent_planner: Plan ready"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
	if strings.Contains(res.stdout, "heartbeat") {
		t.Errorf("printed a heartbeat: %q", res.stdout)
	}
	req := srv.requests()[0]
	if req.header.Get("Accept") != "text/event-stream" || req.header.Get("Last-Event-ID") != "4" || req.query.Get("type") != "message" {
		t.Errorf("watch sent Accept %q Last-Event-ID %q query %v", req.header.Get("Accept"), req.header.Get("Last-Event-ID"), req.query)
	}
}

func TestRoomWatch_JSONPrintsOneLinePerEvent(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: presence_leave\ndata: {\"type\":\"presence_leave\",\"room_id\":\"r1\"}\n\n"+
			"id: 6\nevent: message\ndata: {\"id\":6,\"type\":\"message\",\"room_id\":\"r1\",\"payload\":{\"content\":\"x\"}}\n\n")
	})
	writeConfig(t, srv.URL+"/v1", "")

	res := runCLI(t, "room", "watch", "plan-room", "--room-token", "solvr_rt_x", "--json")
	lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if res.code != 0 || len(lines) != 2 {
		t.Fatalf("exit %d, lines %q, stderr %q", res.code, res.stdout, res.stderr)
	}
	if lines[0] != `{"event":"presence_leave","frame":{"type":"presence_leave","room_id":"r1"}}` {
		t.Errorf("line 1 %s", lines[0])
	}
	if lines[1] != `{"id":"6","event":"message","frame":{"id":6,"type":"message","room_id":"r1","payload":{"content":"x"}}}` {
		t.Errorf("line 2 %s", lines[1])
	}
}

func TestRoomWatch_WithATicketPresentsNoCredential(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
	})
	home := writeConfig(t, srv.URL+"/v1", "solvr_agent_key")
	cfg := readConfigFile(t, home) + "room-token.plan-room=solvr_rt_saved\n"
	if err := os.WriteFile(filepath.Join(home, ".solvr", "config"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	res := runCLI(t, "room", "watch", "plan-room", "--ticket", "solvr_st_abc")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	req := srv.requests()[0]
	if req.header.Get("Authorization") != "" || req.query.Get("ticket") != "solvr_st_abc" {
		t.Errorf("ticket watch sent Authorization %q query %v", req.header.Get("Authorization"), req.query)
	}
}

func TestRoomWatch_AStreamEndedByRotationExitsWithItsCode(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: credential_rotated\ndata: {\"code\":\"CREDENTIAL_ROTATED\",\"message\":\"handshake again\"}\n\n")
	})
	writeConfig(t, srv.URL+"/v1", "")

	human := runCLI(t, "room", "watch", "plan-room", "--room-token", "solvr_rt_old")
	if human.code != 1 || !strings.Contains(human.stderr, "CREDENTIAL_ROTATED: handshake again") {
		t.Errorf("exit %d, stderr %q", human.code, human.stderr)
	}
	asJSON := runCLI(t, "room", "watch", "plan-room", "--room-token", "solvr_rt_old", "--json")
	if asJSON.code != 1 || jsonDiff([]byte(`{"error":{"code":"CREDENTIAL_ROTATED","message":"handshake again"}}`), []byte(asJSON.stderr), true) != "" {
		t.Errorf("--json exit %d, stderr %q", asJSON.code, asJSON.stderr)
	}
}

func TestRoomWatch_StopsWithinTheGuardWhenMaxIsReached(t *testing.T) {
	// A stream that never ends by itself: --max is the only way out.
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for i := 0; i < 3; i++ {
			_, _ = io.WriteString(w, "event: presence_join\ndata: {\"type\":\"presence_join\",\"room_id\":\"r1\"}\n\n")
		}
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(30 * time.Second):
		}
	})
	writeConfig(t, srv.URL+"/v1", "")

	start := time.Now()
	res := runCLI(t, "room", "watch", "plan-room", "--room-token", "solvr_rt_x", "--max", "1")
	if res.code != 0 || strings.Count(res.stdout, "presence_join") != 1 {
		t.Errorf("exit %d, output %q", res.code, res.stdout)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("watch --max 1 took %s", time.Since(start))
	}
}
