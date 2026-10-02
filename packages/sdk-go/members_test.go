package solvr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// A room's participants are a collection the owner reads (listRoomMembers) and adds to
// (addRoomMember): a third and any later agent is admitted to the SAME room, then joins it
// with its own HandshakeRoom.

// memberRequest is what the SDK sent to the stub.
type memberRequest struct {
	method, path, auth string
	body               []byte
}

func memberServer(t *testing.T, status int, answer string) (*httptest.Server, *[]memberRequest) {
	t.Helper()
	var got []memberRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, memberRequest{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization"), body: body})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

const memberJSON = `{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"reviewer","role":"member",` +
	`"added_by":"planner","created_at":"2026-10-02T12:00:00Z"}`

func TestListRoomMembers_ReadsTheParticipantsWithTheAgentKey(t *testing.T) {
	srv, got := memberServer(t, http.StatusOK, `{"data":[`+
		`{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"planner","role":"owner","added_by":"planner","created_at":"2026-10-02T11:00:00Z"},`+
		`{"room_id":"7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f","agent_id":"executor","role":"member","added_by":"planner","created_at":"2026-10-02T11:30:00Z"},`+
		memberJSON+`]}`)
	client := NewClient("solvr_planner_key", WithBaseURL(srv.URL))

	resp, err := client.ListRoomMembers(context.Background(), "handoff room/1")
	if err != nil {
		t.Fatalf("ListRoomMembers: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("sent %d requests, want 1", len(*got))
	}
	sent := (*got)[0]
	if sent.method != http.MethodGet || sent.path != "/v1/rooms/handoff%20room%2F1/members" {
		t.Errorf("sent %s %s, want GET /v1/rooms/handoff%%20room%%2F1/members", sent.method, sent.path)
	}
	if sent.auth != "Bearer solvr_planner_key" {
		t.Errorf("Authorization %q, want the agent API key", sent.auth)
	}
	if len(sent.body) != 0 {
		t.Errorf("sent body %s, a read sends none", sent.body)
	}
	var ids, roles []string
	for _, m := range resp.Data {
		ids = append(ids, m.AgentID)
		roles = append(roles, m.Role)
	}
	if want := []string{"planner", "executor", "reviewer"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("participants %v, want %v (the API's order)", ids, want)
	}
	if want := []string{RoomRoleOwner, RoomRoleMember, RoomRoleMember}; !reflect.DeepEqual(roles, want) {
		t.Errorf("roles %v, want %v", roles, want)
	}
	third := resp.Data[2]
	if third.RoomID != "7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f" || third.AddedBy != "planner" ||
		!third.CreatedAt.Equal(time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("the third participant lost a field: %+v", third)
	}
}

func TestAddRoomMember_AdmitsAnAgentToTheSameRoomAndSurfacesTheParticipant(t *testing.T) {
	for _, tc := range []struct {
		name     string
		req      AddRoomMemberRequest
		wantBody map[string]any
	}{
		{"role omitted", AddRoomMemberRequest{AgentID: "reviewer"}, map[string]any{"agent_id": "reviewer"}},
		{"role named", AddRoomMemberRequest{AgentID: "reviewer", Role: RoomRoleMember},
			map[string]any{"agent_id": "reviewer", "role": "member"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := memberServer(t, http.StatusCreated, `{"data":`+memberJSON+`}`)
			client := NewClient("solvr_planner_key", WithBaseURL(srv.URL))

			resp, err := client.AddRoomMember(context.Background(), "handoff", tc.req)
			if err != nil {
				t.Fatalf("AddRoomMember: %v", err)
			}
			sent := (*got)[0]
			if len(*got) != 1 || sent.method != http.MethodPost || sent.path != "/v1/rooms/handoff/members" {
				t.Fatalf("sent %d requests, first %s %s; want one POST /v1/rooms/handoff/members", len(*got), sent.method, sent.path)
			}
			if sent.auth != "Bearer solvr_planner_key" {
				t.Errorf("Authorization %q, want the agent API key", sent.auth)
			}
			var body map[string]any
			if err := json.Unmarshal(sent.body, &body); err != nil || !reflect.DeepEqual(body, tc.wantBody) {
				t.Errorf("request body %s (%v), want %v", sent.body, err, tc.wantBody)
			}
			want := RoomMember{RoomID: "7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f", AgentID: "reviewer", Role: "member",
				AddedBy: "planner", CreatedAt: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
			if !reflect.DeepEqual(resp.Data, want) {
				t.Errorf("participant %+v, want %+v", resp.Data, want)
			}
		})
	}
}

func TestRoomMembers_AnErrorIsTheAPIErrorItsCodeNamesAndIsNotRetried(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		code   string
		call   func(c *Client) error
	}{
		{"a participant who does not own the room lists", http.StatusForbidden, "FORBIDDEN", func(c *Client) error {
			_, err := c.ListRoomMembers(context.Background(), "handoff")
			return err
		}},
		{"an agent that does not exist", http.StatusBadRequest, "INVALID_AGENT", func(c *Client) error {
			_, err := c.AddRoomMember(context.Background(), "handoff", AddRoomMemberRequest{AgentID: "nobody"})
			return err
		}},
		{"the last owner is demoted", http.StatusConflict, "LAST_OWNER", func(c *Client) error {
			_, err := c.AddRoomMember(context.Background(), "handoff", AddRoomMemberRequest{AgentID: "planner", Role: RoomRoleMember})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := memberServer(t, tc.status, `{"error":{"code":"`+tc.code+`","message":"refused"}}`)
			err := tc.call(NewClient("solvr_executor_key", WithBaseURL(srv.URL)))
			apiErr, ok := err.(*APIError)
			if !ok {
				t.Fatalf("got %T (%v), want *APIError", err, err)
			}
			if apiErr.Code != tc.code || apiErr.Status != tc.status {
				t.Errorf("error %s %d, want %s %d", apiErr.Code, apiErr.Status, tc.code, tc.status)
			}
			if len(*got) != 1 {
				t.Errorf("a %d was sent %d times, want once", tc.status, len(*got))
			}
		})
	}
}

// TestRoomMember_CarriesEveryFieldOfThePublishedSchema holds the type to the RoomMember
// schema the API publishes (backend openapi_member_paths.go, pinned there to the stored model).
func TestRoomMember_CarriesEveryFieldOfThePublishedSchema(t *testing.T) {
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
	if RoomRoleOwner != "owner" || RoomRoleMember != "member" {
		t.Errorf("roles %q %q, want the API's owner and member", RoomRoleOwner, RoomRoleMember)
	}
}
