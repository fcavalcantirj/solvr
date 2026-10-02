package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// A room's participants are a collection its owner reads (solvr room members) and adds
// to (solvr room add-member): a third and any later agent is admitted to the SAME room,
// then joins it with its own API key. Both commands present the API key, never a room
// token, and leave every decision (who may, which roles exist) to the API.

const membersAnswer = `{"data":[
	{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"agent_planner","role":"owner","added_by":"system","created_at":"2026-10-02T11:00:00Z"},
	{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"zeta_executor","role":"member","added_by":"agent_planner","created_at":"2026-10-02T11:30:00Z"},
	{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"alpha_reviewer","role":"member","added_by":"agent_planner","created_at":"2026-10-02T12:00:00Z"}
]}`

const addedAnswer = `{"data":{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"alpha_reviewer","role":"member","added_by":"agent_planner","created_at":"2026-10-02T12:00:00Z"}}`

func sameJSON(t *testing.T, got, want string) bool {
	t.Helper()
	var g, w interface{}
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Errorf("output is not JSON: %v: %q", err, got)
		return false
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(g, w)
}

func TestRoomMembers_ListsTheParticipantsWithTheAPIKeyInTheAPIsOrder(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, membersAnswer)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")

	res := runCLI(t, "room", "members", "a b/c")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	sent := srv.requests()
	if len(sent) != 1 {
		t.Fatalf("sent %d requests, want 1", len(sent))
	}
	if sent[0].method != "GET" || sent[0].path != "/v1/rooms/a%20b%2Fc/members" {
		t.Errorf("sent %s %s, want GET /v1/rooms/a%%20b%%2Fc/members", sent[0].method, sent[0].path)
	}
	if got := sent[0].header.Get("Authorization"); got != "Bearer solvr_planner_key" {
		t.Errorf("presented %q, want the API key", got)
	}
	if len(sent[0].query) != 0 || len(sent[0].body) != 0 {
		t.Errorf("sent query %v body %q, want neither", sent[0].query, sent[0].body)
	}
	for _, want := range []string{"3 participants of a b/c", "AGENT", "ROLE", "ADDED BY", "SINCE",
		"agent_planner", "owner", "system", "zeta_executor", "alpha_reviewer", "2026-10-02T12:00:00Z"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
	planner, zeta, alpha := strings.Index(res.stdout, "agent_planner "), strings.Index(res.stdout, "zeta_executor"), strings.Index(res.stdout, "alpha_reviewer")
	if !(planner < zeta && zeta < alpha) {
		t.Errorf("rows are not in the API's order (planner %d, zeta %d, alpha %d): %q", planner, zeta, alpha, res.stdout)
	}
}

func TestRoomMembers_JSONPrintsTheAPIsAnswer(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, membersAnswer)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")

	res := runCLI(t, "room", "members", "plan-room", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if !sameJSON(t, res.stdout, membersAnswer) {
		t.Errorf("--json printed %q, want the API's answer", res.stdout)
	}
}

func TestRoomAddMember_SendsTheAgentAloneWhenNoRoleIsGiven(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 201, addedAnswer)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")

	res := runCLI(t, "room", "add-member", "plan-room", "alpha_reviewer")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	sent := srv.requests()
	if len(sent) != 1 || sent[0].method != "POST" || sent[0].path != "/v1/rooms/plan-room/members" {
		t.Fatalf("sent %d requests, want one POST /v1/rooms/plan-room/members: %+v", len(sent), sent)
	}
	if got := sent[0].header.Get("Authorization"); got != "Bearer solvr_planner_key" {
		t.Errorf("presented %q, want the API key", got)
	}
	if body := string(sent[0].body); body != `{"agent_id":"alpha_reviewer"}` {
		t.Errorf("sent %s, want the agent id alone (the API decides the role)", body)
	}
	for _, want := range []string{"alpha_reviewer is in plan-room as member (added by agent_planner)", "solvr room join plan-room"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output %q, want %q", res.stdout, want)
		}
	}
}

func TestRoomAddMember_SendsTheRoleItIsGivenAndLeavesItToTheAPI(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 201, strings.Replace(addedAnswer, `"role":"member"`, `"role":"owner"`, 1))
	})
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")

	res := runCLI(t, "room", "add-member", "plan-room", "alpha_reviewer", "--role", "owner")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if body := string(srv.requests()[0].body); body != `{"agent_id":"alpha_reviewer","role":"owner"}` {
		t.Errorf("sent %s", body)
	}
	if !strings.Contains(res.stdout, "alpha_reviewer is in plan-room as owner (added by agent_planner)") {
		t.Errorf("output %q does not name the role the API answered", res.stdout)
	}

	refused := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 400, `{"error":{"code":"VALIDATION_ERROR","message":"role must be owner or member","request_id":"req_role"}}`)
	})
	writeConfig(t, refused.URL+"/v1", "solvr_planner_key")
	bad := runCLI(t, "room", "add-member", "plan-room", "alpha_reviewer", "--role", "admin")
	if n := len(refused.requests()); n != 1 {
		t.Fatalf("--role admin sent %d requests, want 1: the CLI does not validate the role", n)
	}
	if body := string(refused.requests()[0].body); body != `{"agent_id":"alpha_reviewer","role":"admin"}` {
		t.Errorf("sent %s", body)
	}
	if bad.code != 1 || !strings.Contains(bad.stderr, "VALIDATION_ERROR: role must be owner or member") {
		t.Errorf("exit %d stderr %q, want the API's refusal", bad.code, bad.stderr)
	}
}

func TestRoomAddMember_JSONPrintsTheAPIsAnswer(t *testing.T) {
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 201, addedAnswer)
	})
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")

	res := runCLI(t, "room", "add-member", "plan-room", "alpha_reviewer", "--json")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	if !sameJSON(t, res.stdout, addedAnswer) {
		t.Errorf("--json printed %q, want the API's answer", res.stdout)
	}
}

func TestRoomMemberCommands_PresentTheAPIKeyNeverASavedRoomToken(t *testing.T) {
	answers := map[string]string{"GET": membersAnswer, "POST": addedAnswer}
	srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
		answerJSON(w, 200, answers[r.Method])
	})
	for _, tc := range []struct {
		name, configKey string
		extra           []string
		want            string
	}{
		{"the configured key", "solvr_config_key", nil, "Bearer solvr_config_key"},
		{"--api-key beats the configured key", "solvr_config_key", []string{"--api-key", "solvr_flag_key"}, "Bearer solvr_flag_key"},
		{"no key: no credential, not the room token", "", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := writeConfig(t, srv.URL+"/v1", tc.configKey)
			cfg := readConfigFile(t, home) + "room-token.plan-room=solvr_rt_saved\n"
			if err := os.WriteFile(filepath.Join(home, ".solvr", "config"), []byte(cfg), 0600); err != nil {
				t.Fatal(err)
			}
			before := len(srv.requests())
			for _, args := range [][]string{
				{"room", "members", "plan-room"},
				{"room", "add-member", "plan-room", "alpha_reviewer"},
			} {
				res := runCLI(t, append(args, tc.extra...)...)
				if res.code != 0 {
					t.Errorf("solvr %s: exit %d: %s", strings.Join(args, " "), res.code, res.stderr)
				}
			}
			sent := srv.requests()[before:]
			if len(sent) != 2 {
				t.Fatalf("sent %d requests, want 2", len(sent))
			}
			for _, req := range sent {
				if got := req.header.Get("Authorization"); got != tc.want {
					t.Errorf("%s %s presented %q, want %q", req.method, req.path, got, tc.want)
				}
			}
		})
	}
}

func TestRoomMemberCommands_ReportTheAPIsRefusalOnceWithItsCodeAndRequestID(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		status int
		code   string
	}{
		{"a participant who does not own the room lists", []string{"room", "members", "plan-room"}, 403, "FORBIDDEN"},
		{"an agent that does not exist", []string{"room", "add-member", "plan-room", "nobody"}, 400, "INVALID_AGENT"},
		{"the last owner is demoted", []string{"room", "add-member", "plan-room", "agent_planner", "--role", "member"}, 409, "LAST_OWNER"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer := `{"error":{"code":"` + tc.code + `","message":"refused by the API","request_id":"req_` + tc.code + `"}}`
			srv := newRoomServer(t, func(w http.ResponseWriter, r *http.Request) {
				answerJSON(w, tc.status, answer)
			})
			writeConfig(t, srv.URL+"/v1", "solvr_executor_key")

			human := runCLI(t, tc.args...)
			if human.code != 1 || !strings.Contains(human.stderr, tc.code+": refused by the API") || !strings.Contains(human.stderr, "request id: req_"+tc.code) {
				t.Errorf("exit %d stderr %q, want %s with its request id", human.code, human.stderr, tc.code)
			}
			if strings.Contains(human.stderr, "Usage:") || human.stdout != "" {
				t.Errorf("a refusal printed usage or output: stdout %q stderr %q", human.stdout, human.stderr)
			}

			asJSON := runCLI(t, append(tc.args, "--json")...)
			if asJSON.code != 1 || asJSON.stdout != "" || !sameJSON(t, asJSON.stderr, answer) {
				t.Errorf("--json: exit %d stdout %q stderr %q, want the API's error answer on stderr", asJSON.code, asJSON.stdout, asJSON.stderr)
			}
			if n := len(srv.requests()); n != 2 {
				t.Errorf("sent %d requests for two runs, want each sent once", n)
			}
		})
	}
}

func TestRoomMemberCommands_RefuseMissingArgumentsBeforeAnyRequest(t *testing.T) {
	srv := newRoomServer(t, nil)
	writeConfig(t, srv.URL+"/v1", "solvr_planner_key")
	for _, args := range [][]string{
		{"room", "members"},
		{"room", "add-member", "plan-room"},
		{"room", "add-member"},
	} {
		if res := runCLI(t, args...); res.code != 1 {
			t.Errorf("solvr %s: exit %d, want 1", strings.Join(args, " "), res.code)
		}
	}
	if n := len(srv.requests()); n != 0 {
		t.Errorf("sent %d requests, want 0", n)
	}
}

func TestRoomHelp_NamesTheMemberCommands(t *testing.T) {
	res := runCLI(t, "room", "--help")
	for _, want := range []string{"members     List a room's participants", "add-member  Admit an agent to a room"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("room --help %q, want %q", res.stdout, want)
		}
	}
}

// TestRoomMemberTypes_CarryEveryFieldOfThePublishedSchemas holds the types to the
// RoomMember and AddRoomMemberRequest schemas the API publishes (backend
// openapi_member_paths.go, pinned there to the stored model).
func TestRoomMemberTypes_CarryEveryFieldOfThePublishedSchemas(t *testing.T) {
	jsonFields := func(typ reflect.Type) []string {
		var names []string
		for i := 0; i < typ.NumField(); i++ {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			names = append(names, name)
		}
		sort.Strings(names)
		return names
	}
	if got, want := jsonFields(reflect.TypeOf(RoomMember{})), []string{"added_by", "agent_id", "created_at", "role", "room_id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("RoomMember fields %v, want %v", got, want)
	}
	if got, want := jsonFields(reflect.TypeOf(AddRoomMemberRequest{})), []string{"agent_id", "role"}; !reflect.DeepEqual(got, want) {
		t.Errorf("AddRoomMemberRequest fields %v, want %v", got, want)
	}
}
