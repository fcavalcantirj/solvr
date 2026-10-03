package solvr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
)

// The contract tests serve contract/openapi-examples.json (the recorded examples of the
// served OpenAPI document, each held to the running API by the backend) from a local
// server and hold the SDK to it: the request it sends (method, path, query, headers,
// credential, body) and what it surfaces (the typed result, losing no field the API
// answered, or the API's error code).

const contractFixture = "../../contract/openapi-examples.json"

const (
	contractAgentKey  = "solvr_contract_agent_key"
	contractRoomToken = "solvr_rt_contract_room_token"
	contractDeadToken = "solvr_rt_contract_not_live"
)

type contractRequest struct {
	Credential   string            `json:"credential"`
	PathParams   map[string]string `json:"path_params"`
	Query        map[string]string `json:"query"`
	Headers      map[string]string `json:"headers"`
	RequestBody  json.RawMessage   `json:"request_body"`
	Status       int               `json:"status"`
	ResponseBody json.RawMessage   `json:"response_body"`
}

type contractError struct {
	Case string `json:"case"`
	contractRequest
}

type contractOperation struct {
	OperationID       string `json:"operation_id"`
	Method            string `json:"method"`
	Path              string `json:"path"`
	ResponseMediaType string `json:"response_media_type"`
	contractRequest
	Errors []contractError `json:"errors"`
}

func loadContract(t *testing.T) []contractOperation {
	t.Helper()
	raw, err := os.ReadFile(contractFixture)
	if err != nil {
		t.Fatalf("read the client contract: %v", err)
	}
	var fixture struct {
		Operations []contractOperation `json:"operations"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse the client contract: %v", err)
	}
	if len(fixture.Operations) == 0 {
		t.Fatal("the client contract lists no operation")
	}
	return fixture.Operations
}

// contractCall is one example as the SDK caller sees it.
type contractCall struct {
	ctx context.Context
	t   *testing.T
	op  contractOperation
	req contractRequest
}

func (x contractCall) path(name string) string {
	v, ok := x.req.PathParams[name]
	if !ok {
		x.t.Fatalf("%s: the example has no path parameter %q", x.op.OperationID, name)
	}
	return v
}

func (x contractCall) query(name string) string { return x.req.Query[name] }

func (x contractCall) queryInt(name string) int {
	v, ok := x.req.Query[name]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		x.t.Fatalf("%s: query %s=%q is not an integer", x.op.OperationID, name, v)
	}
	return n
}

func (x contractCall) header(name string) string { return x.req.Headers[name] }

// body decodes the example's request body into the SDK's request type, failing on a
// field the type does not have (the SDK could not send it).
func (x contractCall) body(into interface{}) {
	dec := json.NewDecoder(strings.NewReader(string(x.req.RequestBody)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		x.t.Fatalf("%s: the SDK request type cannot carry the example's body: %v", x.op.OperationID, err)
	}
}

// contractCallers call each operation through the SDK with the example's inputs and
// return what the SDK surfaces. The key is the operationId; the SDK method has its name.
var contractCallers = map[string]func(c *Client, x contractCall) (interface{}, error){
	"createPost": func(c *Client, x contractCall) (interface{}, error) {
		var req CreatePostRequest
		x.body(&req)
		return c.CreatePost(x.ctx, req)
	},
	"getPost": func(c *Client, x contractCall) (interface{}, error) {
		return c.GetPost(x.ctx, x.path("id"))
	},
	"search": func(c *Client, x contractCall) (interface{}, error) {
		return c.Search(x.ctx, x.query("q"), &SearchOptions{
			PerPage: x.queryInt("per_page"), Page: x.queryInt("page"), Sort: x.query("sort"),
		})
	},
	"createReply": func(c *Client, x contractCall) (interface{}, error) {
		var req CreateReplyRequest
		x.body(&req)
		return c.CreateReply(x.ctx, x.path("id"), req)
	},
	"listReplies": func(c *Client, x contractCall) (interface{}, error) {
		return c.ListReplies(x.ctx, x.path("id"), &ListRepliesOptions{Cursor: x.query("cursor"), Limit: x.queryInt("limit")})
	},
	"getReply": func(c *Client, x contractCall) (interface{}, error) {
		return c.GetReply(x.ctx, x.path("id"))
	},
	"updateReply": func(c *Client, x contractCall) (interface{}, error) {
		var req UpdateReplyRequest
		x.body(&req)
		return c.UpdateReply(x.ctx, x.path("id"), x.header("If-Match"), req)
	},
	"createRoom": func(c *Client, x contractCall) (interface{}, error) {
		var req CreateRoomRequest
		x.body(&req)
		return c.CreateRoom(x.ctx, req)
	},
	"handshakeRoom": func(c *Client, x contractCall) (interface{}, error) {
		var req HandshakeRoomRequest
		x.body(&req)
		return c.HandshakeRoom(x.ctx, x.path("slug"), req)
	},
	"addRoomMember": func(c *Client, x contractCall) (interface{}, error) {
		var req AddRoomMemberRequest
		x.body(&req)
		return c.AddRoomMember(x.ctx, x.path("slug"), req)
	},
	"listRoomMembers": func(c *Client, x contractCall) (interface{}, error) {
		return c.ListRoomMembers(x.ctx, x.path("slug"))
	},
	"listRoomEntries": func(c *Client, x contractCall) (interface{}, error) {
		return c.ListRoomEntries(x.ctx, x.path("slug"), &ListRoomEntriesOptions{
			Cursor: x.query("cursor"), Limit: x.queryInt("limit"), Kind: x.query("kind"), Issue: x.query("issue"),
		})
	},
	"createRoomEntry": func(c *Client, x contractCall) (interface{}, error) {
		var req CreateRoomEntryRequest
		x.body(&req)
		return c.CreateRoomEntry(x.ctx, x.path("slug"), req)
	},
	"createRoomStreamTicket": func(c *Client, x contractCall) (interface{}, error) {
		return c.CreateRoomStreamTicket(x.ctx, x.path("slug"))
	},
	"streamRoom": func(c *Client, x contractCall) (interface{}, error) {
		stream, err := c.StreamRoom(x.ctx, x.path("slug"), &StreamRoomOptions{
			LastEventID: x.header("Last-Event-ID"), Ticket: x.query("ticket"),
			Type: x.query("type"), Issue: x.query("issue"),
		})
		if err != nil {
			return nil, err
		}
		defer stream.Close()
		return stream.Next()
	},
}

// methodName is the SDK method of an operationId: the same words, exported.
func methodName(operationID string) string {
	r := []rune(operationID)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func TestContract_EveryOperationIsAClientMethodNamedAfterItsOperationID(t *testing.T) {
	client := reflect.TypeOf(&Client{})
	for _, op := range loadContract(t) {
		if _, ok := contractCallers[op.OperationID]; !ok {
			t.Errorf("%s: the contract publishes it and no SDK caller exercises it", op.OperationID)
		}
		if _, ok := client.MethodByName(methodName(op.OperationID)); !ok {
			t.Errorf("%s: the SDK has no method %s", op.OperationID, methodName(op.OperationID))
		}
	}
}

// recordedRequest is what the SDK sent.
type recordedRequest struct {
	method, path, auth string
	query              url.Values
	header             http.Header
	body               []byte
}

// serve answers one recorded request with the example's status and body; getReply also
// answers the ETag the updateReply example sends back as If-Match (the read before the edit).
func serve(t *testing.T, op contractOperation, req contractRequest, etag string) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{method: r.Method, path: r.URL.EscapedPath(), auth: r.Header.Get("Authorization"),
			query: r.URL.Query(), header: r.Header.Clone(), body: body})
		if op.OperationID == "getReply" && etag != "" {
			w.Header().Set("ETag", etag)
		}
		if req.Status < 400 && op.ResponseMediaType == "text/event-stream" {
			var text string
			if err := json.Unmarshal(req.ResponseBody, &text); err != nil {
				t.Errorf("%s: the event-stream example is not a string: %v", op.OperationID, err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(req.Status)
			_, _ = io.WriteString(w, text)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(req.Status)
		_, _ = w.Write(req.ResponseBody)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// clientFor is the SDK client that presents the example's credential, and the
// Authorization header it must send.
func clientFor(t *testing.T, op contractOperation, credential, baseURL string) (*Client, string) {
	t.Helper()
	opts := []ClientOption{WithBaseURL(baseURL), WithMaxRetries(0)}
	if credential == "invalid" {
		credential = op.Credential // a dead credential of the kind the operation takes
		if credential != "room_token" {
			t.Fatalf("%s: an invalid %s credential has no SDK case yet", op.OperationID, credential)
		}
		return NewClient(contractAgentKey, opts...).WithRoomToken(contractDeadToken), "Bearer " + contractDeadToken
	}
	switch credential {
	case "none":
		return NewClient("", opts...), ""
	case "agent_api_key":
		return NewClient(contractAgentKey, opts...), "Bearer " + contractAgentKey
	case "room_token":
		return NewClient(contractAgentKey, opts...).WithRoomToken(contractRoomToken), "Bearer " + contractRoomToken
	}
	t.Fatalf("%s: unknown credential %q", op.OperationID, credential)
	return nil, ""
}

func expectedPath(t *testing.T, op contractOperation, params map[string]string) string {
	t.Helper()
	path := op.Path
	for name, value := range params {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if strings.Contains(path, "{") {
		t.Fatalf("%s: path %s keeps a variable the example gives no value", op.OperationID, path)
	}
	return path
}

// checkRequest holds what the SDK sent to the example.
func checkRequest(t *testing.T, op contractOperation, req contractRequest, sent []recordedRequest, wantAuth string) {
	t.Helper()
	if len(sent) != 1 {
		t.Fatalf("%s: the SDK sent %d requests, want 1", op.OperationID, len(sent))
	}
	got := sent[0]
	if got.method != op.Method {
		t.Errorf("%s: method %s, want %s", op.OperationID, got.method, op.Method)
	}
	if want := expectedPath(t, op, req.PathParams); got.path != want {
		t.Errorf("%s: path %s, want %s", op.OperationID, got.path, want)
	}
	if got.auth != wantAuth {
		t.Errorf("%s: Authorization %q, want %q", op.OperationID, got.auth, wantAuth)
	}
	wantQuery := url.Values{}
	for k, v := range req.Query {
		wantQuery.Set(k, v)
	}
	if !reflect.DeepEqual(got.query, wantQuery) {
		t.Errorf("%s: query %v, want %v", op.OperationID, got.query, wantQuery)
	}
	for name, value := range req.Headers {
		if got.header.Get(name) != value {
			t.Errorf("%s: header %s %q, want %q", op.OperationID, name, got.header.Get(name), value)
		}
	}
	for _, name := range []string{"If-Match", "Last-Event-ID"} {
		if _, want := req.Headers[name]; !want && got.header.Get(name) != "" {
			t.Errorf("%s: sent %s the example does not", op.OperationID, name)
		}
	}
	if string(req.RequestBody) == "null" || len(req.RequestBody) == 0 {
		if len(got.body) != 0 {
			t.Errorf("%s: sent body %s, the example sends none", op.OperationID, got.body)
		}
		return
	}
	if diff := jsonDiff(req.RequestBody, got.body, true); diff != "" {
		t.Errorf("%s: request body %s; example %s: %s", op.OperationID, got.body, req.RequestBody, diff)
	}
}

// jsonDiff compares two JSON documents. exact also fails on a field only got has; either
// way a field the example has must be in got with the same value (null and absent agree).
func jsonDiff(want, got []byte, exact bool) string {
	var w, g interface{}
	if err := json.Unmarshal(want, &w); err != nil {
		return "example: " + err.Error()
	}
	if err := json.Unmarshal(got, &g); err != nil {
		return "got: " + err.Error()
	}
	return valueDiff("$", w, g, exact)
}

func valueDiff(at string, want, got interface{}, exact bool) string {
	switch w := want.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("%s: want an object, got %v", at, got)
		}
		keys := make([]string, 0, len(w))
		for k := range w {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if w[k] == nil && g[k] == nil {
				continue
			}
			gv, ok := g[k]
			if !ok {
				return fmt.Sprintf("%s.%s: the example has it, the SDK lost it", at, k)
			}
			if d := valueDiff(at+"."+k, w[k], gv, exact); d != "" {
				return d
			}
		}
		if exact {
			for k, gv := range g {
				if _, ok := w[k]; !ok && gv != nil {
					return fmt.Sprintf("%s.%s: the SDK sent it, the example does not", at, k)
				}
			}
		}
		return ""
	case []interface{}:
		g, ok := got.([]interface{})
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: want %d items, got %v", at, len(w), got)
		}
		for i := range w {
			if d := valueDiff(fmt.Sprintf("%s[%d]", at, i), w[i], g[i], exact); d != "" {
				return d
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(want, got) {
			return fmt.Sprintf("%s: want %v, got %v", at, want, got)
		}
		return ""
	}
}

func TestContract_EachExampleIsWhatTheSDKSendsAndSurfaces(t *testing.T) {
	ops := loadContract(t)
	etag := ""
	for _, op := range ops {
		if op.OperationID == "updateReply" {
			etag = op.Headers["If-Match"]
		}
	}
	for _, op := range ops {
		op := op
		t.Run(op.OperationID, func(t *testing.T) {
			call, ok := contractCallers[op.OperationID]
			if !ok {
				t.Fatalf("no SDK caller for %s", op.OperationID)
			}
			srv, sent := serve(t, op, op.contractRequest, etag)
			client, wantAuth := clientFor(t, op, op.Credential, srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result, err := call(client, contractCall{ctx: ctx, t: t, op: op, req: op.contractRequest})
			if err != nil {
				t.Fatalf("%s: the SDK failed on the example's answer: %v", op.OperationID, err)
			}
			checkRequest(t, op, op.contractRequest, *sent, wantAuth)
			checkResult(t, op, result, etag)
		})
	}
}

// checkResult holds the typed result to the example's answer: re-encoded, it carries every
// field the API answered, with the same value.
func checkResult(t *testing.T, op contractOperation, result interface{}, etag string) {
	t.Helper()
	if op.ResponseMediaType == "text/event-stream" {
		checkStreamResult(t, op, result)
		return
	}
	got, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("%s: encode the SDK result: %v", op.OperationID, err)
	}
	if diff := jsonDiff(op.ResponseBody, got, false); diff != "" {
		t.Errorf("%s: the SDK result %s: %s", op.OperationID, got, diff)
	}
	if op.OperationID == "getReply" {
		resp, ok := result.(*ReplyResponse)
		if !ok {
			t.Fatalf("getReply: the SDK surfaced %T, want *ReplyResponse", result)
		}
		if resp.ETag != etag {
			t.Errorf("getReply: the SDK surfaced ETag %q, want the %q the edit sends back", resp.ETag, etag)
		}
	}
}

// checkStreamResult holds the first event the SDK read to the example's first frame.
func checkStreamResult(t *testing.T, op contractOperation, result interface{}) {
	t.Helper()
	var text string
	if err := json.Unmarshal(op.ResponseBody, &text); err != nil {
		t.Fatalf("%s: the event-stream example is not a string: %v", op.OperationID, err)
	}
	var id, event, data string
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	ev, ok := result.(*RoomStreamEvent)
	if !ok || ev == nil {
		t.Fatalf("%s: the SDK surfaced %T, want *RoomStreamEvent", op.OperationID, result)
	}
	if ev.ID != id || ev.Event != event {
		t.Errorf("%s: event id %q name %q, want %q %q", op.OperationID, ev.ID, ev.Event, id, event)
	}
	if ev.Frame == nil {
		t.Fatalf("%s: the SDK did not decode the frame", op.OperationID)
	}
	got, err := json.Marshal(ev.Frame)
	if err != nil {
		t.Fatalf("%s: encode the frame: %v", op.OperationID, err)
	}
	if diff := jsonDiff([]byte(data), got, false); diff != "" {
		t.Errorf("%s: the SDK frame %s: %s", op.OperationID, got, diff)
	}
	msg, err := ev.Frame.Message()
	if err != nil {
		t.Fatalf("%s: decode the message payload: %v", op.OperationID, err)
	}
	var frame struct {
		Payload json.RawMessage `json:"payload"`
	}
	_ = json.Unmarshal([]byte(data), &frame)
	gotMsg, _ := json.Marshal(msg)
	if diff := jsonDiff(frame.Payload, gotMsg, false); diff != "" {
		t.Errorf("%s: the SDK message %s: %s", op.OperationID, gotMsg, diff)
	}
}

func TestContract_EachErrorExampleSurfacesTheAPIErrorCode(t *testing.T) {
	errorsSeen := 0
	for _, op := range loadContract(t) {
		for _, e := range op.Errors {
			op, e := op, e
			errorsSeen++
			t.Run(op.OperationID+"/"+strconv.Itoa(e.Status), func(t *testing.T) {
				call := contractCallers[op.OperationID]
				srv, sent := serve(t, op, e.contractRequest, "")
				client, wantAuth := clientFor(t, op, e.Credential, srv.URL)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				_, err := call(client, contractCall{ctx: ctx, t: t, op: op, req: e.contractRequest})
				checkRequest(t, op, e.contractRequest, *sent, wantAuth)
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("%s (%s): the SDK surfaced %v, want an *APIError", op.OperationID, e.Case, err)
				}
				var want struct {
					Error json.RawMessage `json:"error"`
				}
				_ = json.Unmarshal(e.ResponseBody, &want)
				got, _ := json.Marshal(apiErr)
				if diff := jsonDiff(want.Error, got, false); diff != "" {
					t.Errorf("%s (%s): the SDK error %s: %s", op.OperationID, e.Case, got, diff)
				}
				if apiErr.Status != e.Status {
					t.Errorf("%s (%s): the SDK error status %d, want %d", op.OperationID, e.Case, apiErr.Status, e.Status)
				}
			})
		}
	}
	if errorsSeen == 0 {
		t.Fatal("the client contract carries no error example")
	}
}

// The comparison the contract tests rely on must catch what it claims.
func TestContract_JSONDiffCatchesALostFieldAValueAndAnExtraRequestField(t *testing.T) {
	cases := []struct {
		want, got string
		exact     bool
		fails     bool
	}{
		{`{"a":1,"b":{"c":"x"}}`, `{"a":1,"b":{"c":"x"},"d":2}`, false, false},
		{`{"a":1,"b":null}`, `{"a":1}`, true, false},
		{`{"a":1,"b":{"c":"x"}}`, `{"a":1,"b":{}}`, false, true},
		{`{"a":[1,2]}`, `{"a":[1]}`, false, true},
		{`{"a":"2026-10-01T18:41:37.518736Z"}`, `{"a":"2026-10-01T18:41:37.518737Z"}`, false, true},
		{`{"a":1}`, `{"a":1,"rotate":false}`, true, true},
		{`{"a":{}}`, `{}`, false, true},
	}
	for i, c := range cases {
		if d := jsonDiff([]byte(c.want), []byte(c.got), c.exact); (d != "") != c.fails {
			t.Errorf("case %d: diff %q, want failing=%v", i, d, c.fails)
		}
	}
}
