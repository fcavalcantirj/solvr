package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The contract tests hold POST /v1/mcp to contract/openapi-examples.json (the recorded
// examples of the served OpenAPI document, each held to the running API by
// openapi_examples_test.go), as the npm mcp-server's contract test does: each operation's tool
// is called through the handler's JSON-RPC entry against a dispatcher serving the example, and
// the request it dispatches (method, escaped path, query, headers, credential, body), what its
// result shows of the answer, and how it reports each recorded error are checked.

const (
	contractAgentKey  = "solvr_contract_agent_key"
	contractRoomToken = "solvr_rt_contract_room_token"
	contractDeadToken = "solvr_rt_contract_not_live"
)

type contractRequest struct {
	Case        string            `json:"case"`
	Credential  string            `json:"credential"`
	PathParams  map[string]string `json:"path_params"`
	Query       map[string]string `json:"query"`
	Headers     map[string]string `json:"headers"`
	RequestBody json.RawMessage   `json:"request_body"`
	Status      int               `json:"status"`
	Response    json.RawMessage   `json:"response_body"`
}

type contractOperation struct {
	contractRequest
	OperationID string            `json:"operation_id"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	MediaType   string            `json:"response_media_type"`
	Errors      []contractRequest `json:"errors"`
}

func loadMCPContract(t *testing.T) []contractOperation {
	t.Helper()
	raw, err := os.ReadFile("../../../../contract/openapi-examples.json")
	if err != nil {
		t.Fatalf("read the client contract: %v", err)
	}
	var fixture struct {
		Operations []contractOperation `json:"operations"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil || len(fixture.Operations) == 0 {
		t.Fatalf("the client contract lists no operation (%v)", err)
	}
	return fixture.Operations
}

func contractOp(ops []contractOperation, id string) contractOperation {
	for _, op := range ops {
		if op.OperationID == id {
			return op
		}
	}
	panic("the contract has no " + id + " operation")
}

// mcpContractCall is how an operation's tool takes the example: which argument carries each
// path, query and header value (request body fields go to the argument of the same name).
type mcpContractCall struct {
	path, query, headers map[string]string
	shows                func(ops []contractOperation, data interface{}) []string
}

func field(data interface{}, path ...interface{}) string {
	for _, step := range path {
		switch key := step.(type) {
		case string:
			data = data.(map[string]interface{})[key]
		case int:
			data = data.([]interface{})[key]
		}
	}
	if n, ok := data.(float64); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return fmt.Sprint(data)
}

func contractETag(ops []contractOperation) string {
	return contractOp(ops, "updateReply").Headers["If-Match"]
}

var slugArg = map[string]string{"slug": "slug"}

var mcpContractCalls = map[string]mcpContractCall{
	"createPost": {shows: func(_ []contractOperation, d interface{}) []string {
		return []string{field(d, "id"), field(d, "title")}
	}},
	"getPost": {path: map[string]string{"id": "id"}, shows: func(ops []contractOperation, d interface{}) []string {
		var replies struct{ Data []interface{} }
		_ = json.Unmarshal(contractOp(ops, "listReplies").Response, &replies)
		return []string{field(d, "id"), field(d, "title"), field(replies.Data, 0, "id")}
	}},
	"search": {query: map[string]string{"q": "query", "per_page": "limit", "page": "page", "sort": "sort"},
		shows: func(_ []contractOperation, d interface{}) []string {
			return []string{field(d, 0, "id"), field(d, 0, "title")}
		}},
	"createReply": {path: map[string]string{"id": "post_id"}, shows: func(_ []contractOperation, d interface{}) []string { return []string{field(d, "id")} }},
	"listReplies": {path: map[string]string{"id": "post_id"}, query: map[string]string{"limit": "limit", "cursor": "cursor"},
		shows: func(_ []contractOperation, d interface{}) []string {
			return []string{field(d, 0, "id"), field(d, 0, "body")}
		}},
	"getReply": {path: map[string]string{"id": "id"}, shows: func(ops []contractOperation, d interface{}) []string {
		return []string{field(d, "id"), field(d, "body"), contractETag(ops)}
	}},
	"updateReply": {path: map[string]string{"id": "id"}, headers: map[string]string{"If-Match": "if_match"},
		shows: func(ops []contractOperation, d interface{}) []string {
			return []string{field(d, "id"), contractETag(ops)}
		}},
	"createRoom":    {shows: func(_ []contractOperation, d interface{}) []string { return []string{field(d, "slug")} }},
	"handshakeRoom": {path: slugArg, shows: func(_ []contractOperation, d interface{}) []string { return []string{field(d, "room_token")} }},
	"listRoomEntries": {path: slugArg, query: map[string]string{"limit": "limit", "cursor": "cursor", "kind": "kind", "issue": "issue"},
		shows: func(_ []contractOperation, d interface{}) []string {
			return []string{field(d, 0, "body"), field(d, 0, "actor_label")}
		}},
	"createRoomEntry":        {path: slugArg, shows: func(_ []contractOperation, d interface{}) []string { return []string{field(d, "id")} }},
	"createRoomStreamTicket": {path: slugArg, shows: func(_ []contractOperation, d interface{}) []string { return []string{field(d, "ticket")} }},
	"streamRoom": {path: slugArg, query: map[string]string{"ticket": "ticket", "type": "event_type", "issue": "issue"},
		headers: map[string]string{"Last-Event-ID": "last_event_id"}, shows: nil},
}

func mcpToolSchema(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	for _, tool := range mcpTools {
		if tool["name"] == name {
			return tool["inputSchema"].(map[string]interface{})["properties"].(map[string]interface{})
		}
	}
	t.Fatalf("tools/list has no %s", name)
	return nil
}

// contractArgs maps the example to the tool's arguments; a value no declared argument carries
// fails the test. A number argument takes the example's string as a number.
func contractArgs(t *testing.T, op contractOperation, req contractRequest) map[string]interface{} {
	t.Helper()
	call := mcpContractCalls[op.OperationID]
	tool := MCPOperationTools[op.OperationID]
	props := mcpToolSchema(t, tool)
	args := map[string]interface{}{}
	put := func(name string, value interface{}) {
		declared, ok := props[name].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: %s declares no argument %s", op.OperationID, tool, name)
		}
		if s, isString := value.(string); isString && declared["type"] == "number" {
			n, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("%s: %s=%q is not a number", op.OperationID, name, s)
			}
			value = n
		}
		args[name] = value
	}
	mapValues := func(kind string, values, names map[string]string) {
		for name, value := range values {
			arg, ok := names[name]
			if !ok {
				t.Fatalf("%s: no argument carries the %s %s", op.OperationID, kind, name)
			}
			put(arg, value)
		}
	}
	mapValues("path parameter", req.PathParams, call.path)
	mapValues("query parameter", req.Query, call.query)
	mapValues("header", req.Headers, call.headers)
	var body map[string]interface{}
	_ = json.Unmarshal(req.RequestBody, &body)
	for name, value := range body {
		put(name, value)
	}
	if op.OperationID == "search" && args["query"] == nil {
		args["query"] = "" // the tool requires query; the VALIDATION_ERROR example sends none
	}
	return args
}

// contractCredential returns the Authorization of the MCP call, the credential arguments, and
// the Authorization the dispatched request must carry. Room operations present the room_token
// argument, never the caller's API key; a ticket watch presents nothing.
func contractCredential(t *testing.T, op contractOperation, credential string) (string, map[string]interface{}, string) {
	key := "Bearer " + contractAgentKey
	switch {
	case credential == "agent_api_key":
		return key, nil, key
	case credential == "room_token":
		return key, map[string]interface{}{"room_token": contractRoomToken}, "Bearer " + contractRoomToken
	case credential == "invalid" && op.Credential == "room_token":
		return key, map[string]interface{}{"room_token": contractDeadToken}, "Bearer " + contractDeadToken
	case credential == "none" && op.Credential == "room_token":
		return key, nil, "" // a ticket watch is anonymous even when the caller sent a key
	case credential == "none":
		return "", nil, ""
	}
	t.Fatalf("%s: credential %s has no MCP case", op.OperationID, credential)
	return "", nil, ""
}

func contractPath(op contractOperation, params map[string]string) string {
	p := op.Path
	for name, value := range params {
		p = strings.ReplaceAll(p, "{"+name+"}", url.PathEscape(value))
	}
	return p
}

// serveContract answers the example: getPost's replies read gets the listReplies example; the
// reply reads carry the ETag the updateReply example sends back.
func serveContract(ops []contractOperation, op contractOperation, req contractRequest) http.HandlerFunc {
	path := contractPath(op, req.PathParams)
	return func(w http.ResponseWriter, r *http.Request) {
		if req.Status < 400 && op.OperationID == "getPost" && r.URL.EscapedPath() == path+"/replies" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(contractOp(ops, "listReplies").Response)
			return
		}
		if req.Status < 400 && op.MediaType == "text/event-stream" {
			var stream string
			_ = json.Unmarshal(req.Response, &stream)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(req.Status)
			_, _ = w.Write([]byte(stream))
			return
		}
		if op.OperationID == "getReply" || op.OperationID == "updateReply" {
			w.Header().Set("ETag", contractETag(ops))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(req.Status)
		_, _ = w.Write(req.Response)
	}
}

// checkContractRequest holds the dispatched request of the operation to the example.
func checkContractRequest(t *testing.T, op contractOperation, req contractRequest, sent []mcpSent, auth string) {
	t.Helper()
	id := op.OperationID
	want := 1
	if id == "getPost" && req.Status < 400 {
		want = 2 // the post, then its replies; a failed post read sends no replies read
	}
	if len(sent) != want {
		t.Fatalf("%s: %d requests dispatched, want %d: %+v", id, len(sent), want, sent)
	}
	got := sent[0]
	if got.method != op.Method || got.path != contractPath(op, req.PathParams) {
		t.Fatalf("%s: dispatched %s %s, want %s %s", id, got.method, got.path, op.Method, contractPath(op, req.PathParams))
	}
	for _, s := range sent {
		if s.auth != auth {
			t.Errorf("%s: Authorization of %s %s = %q, want %q", id, s.method, s.path, s.auth, auth)
		}
		if !s.inProcess {
			t.Errorf("%s: the dispatched request is not marked in-process (it would be counted twice)", id)
		}
	}
	wantQuery := map[string]string{}
	for k, v := range req.Query {
		wantQuery[k] = v
	}
	if id == "search" && wantQuery["per_page"] == "" {
		wantQuery["per_page"] = "5" // solvr_search's documented default page size
	}
	gotQuery := map[string]string{}
	for k, v := range got.query {
		if len(v) != 1 {
			t.Errorf("%s: query parameter %s sent %d times", id, k, len(v))
		}
		gotQuery[k] = v[0]
	}
	if !reflect.DeepEqual(gotQuery, wantQuery) {
		t.Errorf("%s: query %v, want %v", id, gotQuery, wantQuery)
	}
	for _, name := range []string{"If-Match", "Last-Event-ID"} {
		if got.header.Get(name) != req.Headers[name] {
			t.Errorf("%s: header %s = %q, want %q", id, name, got.header.Get(name), req.Headers[name])
		}
	}
	if string(req.RequestBody) == "null" || len(req.RequestBody) == 0 {
		if got.body != "" {
			t.Errorf("%s: sent a body the example does not: %s", id, got.body)
		}
		return
	}
	var wantBody, gotBody interface{}
	_ = json.Unmarshal(req.RequestBody, &wantBody)
	if err := json.Unmarshal([]byte(got.body), &gotBody); err != nil || !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("%s: body %s, want exactly %s", id, got.body, req.RequestBody)
	}
}

func TestMCPContract_HasAToolForEveryOperation(t *testing.T) {
	ops := loadMCPContract(t)
	listed := map[string]bool{}
	for _, name := range MCPToolNames() {
		listed[name] = true
	}
	for _, op := range ops {
		tool, ok := MCPOperationTools[op.OperationID]
		if !ok || !listed[tool] {
			t.Errorf("operation %s has no served tool (%q)", op.OperationID, tool)
		}
		if _, ok := mcpContractCalls[op.OperationID]; !ok {
			t.Errorf("operation %s has no contract case", op.OperationID)
		}
	}
	if len(MCPOperationTools) != len(ops) {
		t.Errorf("MCPOperationTools names %d operations, the contract has %d", len(MCPOperationTools), len(ops))
	}
}

// Both first-party MCP servers name each operation's tool alike.
func TestMCPContract_ToolNamesMatchTheNpmServer(t *testing.T) {
	src, err := os.ReadFile("../../../../mcp-server/src/tools.ts")
	if err != nil {
		t.Fatalf("read mcp-server/src/tools.ts: %v", err)
	}
	block := regexp.MustCompile(`(?s)export const OPERATION_TOOLS[^{]*\{(.*?)\n\};`).FindSubmatch(src)
	if block == nil {
		t.Fatal("mcp-server/src/tools.ts has no OPERATION_TOOLS")
	}
	npm := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+): '(solvr_\w+)'`).FindAllStringSubmatch(string(block[1]), -1) {
		npm[m[1]] = m[2]
	}
	if !reflect.DeepEqual(npm, MCPOperationTools) {
		t.Errorf("npm OPERATION_TOOLS %v, /v1/mcp %v", npm, MCPOperationTools)
	}
}

func TestMCPContract_EachOperationSendsTheExampleAndShowsTheAnswer(t *testing.T) {
	ops := loadMCPContract(t)
	for _, op := range ops {
		op := op
		t.Run(op.OperationID, func(t *testing.T) {
			mcpAuth, credArgs, wantAuth := contractCredential(t, op, op.Credential)
			args := contractArgs(t, op, op.contractRequest)
			for k, v := range credArgs {
				args[k] = v
			}
			h, rec := recordMCP(serveContract(ops, op, op.contractRequest))
			text, isError := callMCP(t, h, MCPOperationTools[op.OperationID], args, mcpAuth)
			if isError {
				t.Fatalf("%s failed: %s", op.OperationID, text)
			}
			checkContractRequest(t, op, op.contractRequest, rec.requests(), wantAuth)

			var answer struct{ Data interface{} }
			_ = json.Unmarshal(op.Response, &answer)
			var shows []string
			if op.OperationID == "streamRoom" {
				shows = streamExampleShows(t, op)
			} else {
				shows = mcpContractCalls[op.OperationID].shows(ops, answer.Data)
			}
			for _, want := range shows {
				if !strings.Contains(text, want) {
					t.Errorf("%s result lacks %q:\n%s", op.OperationID, want, text)
				}
			}
		})
	}
}

// streamExampleShows: the first frame's message content and its event id.
func streamExampleShows(t *testing.T, op contractOperation) []string {
	var stream string
	_ = json.Unmarshal(op.Response, &stream)
	id := regexp.MustCompile(`(?m)^id: (\S+)$`).FindStringSubmatch(stream)
	data := regexp.MustCompile(`(?m)^data: (.+)$`).FindStringSubmatch(stream)
	if id == nil || data == nil {
		t.Fatalf("the streamRoom example has no frame: %q", stream)
	}
	var frame struct{ Payload struct{ Content string } }
	if err := json.Unmarshal([]byte(data[1]), &frame); err != nil {
		t.Fatalf("decode the streamRoom frame: %v", err)
	}
	return []string{frame.Payload.Content, "(id " + id[1] + ")"}
}

func TestMCPContract_EachRecordedErrorIsReportedWithItsCodeAndRequestID(t *testing.T) {
	ops := loadMCPContract(t)
	errorCases := 0
	for _, op := range ops {
		for _, example := range op.Errors {
			op, example := op, example
			errorCases++
			t.Run(op.OperationID+"/"+example.Case, func(t *testing.T) {
				mcpAuth, credArgs, wantAuth := contractCredential(t, op, example.Credential)
				args := contractArgs(t, op, example)
				for k, v := range credArgs {
					args[k] = v
				}
				h, rec := recordMCP(serveContract(ops, op, example))
				text, isError := callMCP(t, h, MCPOperationTools[op.OperationID], args, mcpAuth)
				if !isError {
					t.Fatalf("%s: want isError, got %s", op.OperationID, text)
				}
				checkContractRequest(t, op, example, rec.requests(), wantAuth)
				var body struct {
					Error struct {
						Code, Message string
						RequestID     string `json:"request_id"`
					}
				}
				_ = json.Unmarshal(example.Response, &body)
				for _, want := range []string{
					"Error executing " + MCPOperationTools[op.OperationID],
					body.Error.Code + ": " + body.Error.Message,
					"request id: " + body.Error.RequestID,
				} {
					if !strings.Contains(text, want) {
						t.Errorf("%s error text lacks %q:\n%s", op.OperationID, want, text)
					}
				}
			})
		}
	}
	if errorCases == 0 {
		t.Fatal("the contract records no error example")
	}
}
