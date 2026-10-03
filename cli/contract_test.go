package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The contract tests serve contract/openapi-examples.json (the recorded examples of the
// served OpenAPI document, each held to the running API by the backend) from a local
// server and run the CLI command of each operation against it: the request the command
// sends (method, path, query, headers, credential, body), what --json prints (the API's
// answer), what the human output shows, and how the command reports each recorded error.

const contractFixture = "../contract/openapi-examples.json"

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

// contractETag is the ETag getReply answers: the one the updateReply example sends back
// as If-Match (the read before the edit).
func contractETag(ops []contractOperation) string {
	for _, op := range ops {
		if op.OperationID == "updateReply" {
			return op.Headers["If-Match"]
		}
	}
	return ""
}

// contractCall is one example as the CLI user types it.
type contractCall struct {
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

// query is the option with the value of the example's query parameter, or nothing when
// it has none.
func (x contractCall) query(option, name string) []string {
	v, ok := x.req.Query[name]
	if !ok {
		return nil
	}
	return []string{option, v}
}

func (x contractCall) header(option, name string) []string {
	v, ok := x.req.Headers[name]
	if !ok {
		return nil
	}
	return []string{option, v}
}

// field is a string field of the example's request body that the command takes as an
// argument (its option in body is ""). A missing field fails.
func (x contractCall) field(name string) string {
	var fields map[string]interface{}
	_ = json.Unmarshal(x.req.RequestBody, &fields)
	v, ok := fields[name].(string)
	if !ok {
		x.t.Fatalf("%s: the request body has no string field %s", x.op.OperationID, name)
	}
	return v
}

// body is the options that carry the example's request body: each field becomes the
// option the command declares for it ("" for a field the command takes as an argument,
// see field). A field the command has no option for fails.
func (x contractCall) body(options map[string]string) []string {
	if len(x.req.RequestBody) == 0 || string(x.req.RequestBody) == "null" {
		return nil
	}
	var fields map[string]interface{}
	if err := json.Unmarshal(x.req.RequestBody, &fields); err != nil {
		x.t.Fatalf("%s: the request body is not an object: %v", x.op.OperationID, err)
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	var args []string
	for _, name := range names {
		option, ok := options[name]
		if !ok {
			x.t.Fatalf("%s: the command has no option for the request field %s", x.op.OperationID, name)
		}
		if option == "" {
			continue // an argument of the command line
		}
		switch v := fields[name].(type) {
		case bool:
			if !v {
				x.t.Fatalf("%s: no option form for %s=false", x.op.OperationID, name)
			}
			args = append(args, option)
		case string:
			args = append(args, option, v)
		case float64:
			args = append(args, option, strconv.FormatFloat(v, 'f', -1, 64))
		case []interface{}:
			items := make([]string, len(v))
			for i, item := range v {
				items[i] = fmt.Sprint(item)
			}
			args = append(args, option, strings.Join(items, ","))
		default:
			x.t.Fatalf("%s: no option form for %s=%v", x.op.OperationID, name, v)
		}
	}
	return args
}

// contractCommand is the CLI command of one operation.
type contractCommand struct {
	args  func(x contractCall) []string // the command line of the example (credentials excluded)
	shows func(answer map[string]interface{}) []string
}

func dataField(answer map[string]interface{}, name string) string {
	data, _ := answer["data"].(map[string]interface{})
	return fmt.Sprint(data[name])
}

func firstItem(answer map[string]interface{}, name string) string {
	items, _ := answer["data"].([]interface{})
	if len(items) == 0 {
		return "<no item>"
	}
	item, _ := items[0].(map[string]interface{})
	return fmt.Sprint(item[name])
}

func lastItem(answer map[string]interface{}, name string) string {
	items, _ := answer["data"].([]interface{})
	if len(items) == 0 {
		return "<no item>"
	}
	item, _ := items[len(items)-1].(map[string]interface{})
	return fmt.Sprint(item[name])
}

func join(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// contractCommands has one command per operationId. Rooms: create, join, members,
// add-member, read, send, ticket, watch.
func contractCommands(etag, streamContent string) map[string]contractCommand {
	return map[string]contractCommand{
		"createPost": {
			args: func(x contractCall) []string {
				return join([]string{"post"}, x.body(map[string]string{
					"title": "--title", "description": "--description", "tags": "--tags", "visibility": "--visibility",
				}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "id")} },
		},
		"getPost": {
			args:  func(x contractCall) []string { return []string{"get", x.path("id")} },
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "id"), dataField(a, "title")} },
		},
		"search": {
			args: func(x contractCall) []string {
				return join([]string{"search", x.req.Query["q"]},
					x.query("--limit", "per_page"), x.query("--page", "page"), x.query("--sort", "sort"))
			},
			shows: func(a map[string]interface{}) []string { return []string{firstItem(a, "id"), firstItem(a, "title")} },
		},
		"createReply": {
			args: func(x contractCall) []string {
				return join([]string{"reply", x.path("id")}, x.body(map[string]string{"body": "--body", "parent_reply_id": "--parent"}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "id")} },
		},
		"listReplies": {
			args: func(x contractCall) []string {
				return join([]string{"replies", x.path("id")}, x.query("--limit", "limit"), x.query("--cursor", "cursor"))
			},
			shows: func(a map[string]interface{}) []string { return []string{firstItem(a, "id"), firstItem(a, "body")} },
		},
		"getReply": {
			args: func(x contractCall) []string { return []string{"get-reply", x.path("id")} },
			shows: func(a map[string]interface{}) []string {
				return []string{dataField(a, "id"), dataField(a, "body"), etag}
			},
		},
		"updateReply": {
			args: func(x contractCall) []string {
				return join([]string{"update-reply", x.path("id")}, x.header("--if-match", "If-Match"), x.body(map[string]string{"body": "--body"}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "id")} },
		},
		"createRoom": {
			args: func(x contractCall) []string {
				return join([]string{"room", "create"}, x.body(map[string]string{
					"display_name": "--display-name", "slug": "--slug", "description": "--description",
					"tags": "--tags", "is_private": "--private",
				}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "slug")} },
		},
		"handshakeRoom": {
			args: func(x contractCall) []string {
				return join([]string{"room", "join", x.path("slug")}, x.body(map[string]string{"rotate": "--rotate", "ttl_seconds": "--ttl"}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "room_token")} },
		},
		"addRoomMember": {
			args: func(x contractCall) []string {
				return join([]string{"room", "add-member", x.path("slug"), x.field("agent_id")},
					x.body(map[string]string{"agent_id": "", "role": "--role"}))
			},
			shows: func(a map[string]interface{}) []string {
				return []string{dataField(a, "agent_id"), dataField(a, "role"), dataField(a, "added_by")}
			},
		},
		"listRoomMembers": {
			args: func(x contractCall) []string { return []string{"room", "members", x.path("slug")} },
			shows: func(a map[string]interface{}) []string {
				return []string{firstItem(a, "agent_id"), firstItem(a, "role"), lastItem(a, "agent_id"), lastItem(a, "added_by")}
			},
		},
		"listRoomEntries": {
			args: func(x contractCall) []string {
				return join([]string{"room", "read", x.path("slug")},
					x.query("--limit", "limit"), x.query("--cursor", "cursor"), x.query("--kind", "kind"), x.query("--issue", "issue"))
			},
			shows: func(a map[string]interface{}) []string { return []string{firstItem(a, "body")} },
		},
		"createRoomEntry": {
			args: func(x contractCall) []string {
				return join([]string{"room", "send", x.path("slug")}, x.body(map[string]string{
					"body": "--body", "client_entry_id": "--client-entry-id", "reply_to_entry_id": "--reply-to",
					"addressed_member_ids": "--to",
				}))
			},
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "id")} },
		},
		"createRoomStreamTicket": {
			args:  func(x contractCall) []string { return []string{"room", "ticket", x.path("slug")} },
			shows: func(a map[string]interface{}) []string { return []string{dataField(a, "ticket")} },
		},
		"streamRoom": {
			args: func(x contractCall) []string {
				return join([]string{"room", "watch", x.path("slug")},
					x.header("--last-event-id", "Last-Event-ID"), x.query("--ticket", "ticket"),
					x.query("--type", "type"), x.query("--issue", "issue"))
			},
			shows: func(map[string]interface{}) []string { return []string{streamContent} },
		},
	}
}

// contractCredential is the credential options of a run, whether an API key is
// configured, and the Authorization header the CLI must send.
func contractCredential(t *testing.T, op contractOperation, credential string) (args []string, apiKey, auth string) {
	t.Helper()
	switch credential {
	case "none":
		return nil, "", ""
	case "agent_api_key":
		return nil, contractAgentKey, "Bearer " + contractAgentKey
	case "room_token":
		return []string{"--room-token", contractRoomToken}, contractAgentKey, "Bearer " + contractRoomToken
	case "invalid":
		if op.Credential != "room_token" {
			t.Fatalf("%s: an invalid %s credential has no CLI case yet", op.OperationID, op.Credential)
		}
		return []string{"--room-token", contractDeadToken}, contractAgentKey, "Bearer " + contractDeadToken
	}
	t.Fatalf("%s: unknown credential %q", op.OperationID, credential)
	return nil, "", ""
}

// recordedRequest is what the CLI sent.
type recordedRequest struct {
	method, path string
	query        url.Values
	header       http.Header
	body         []byte
}

// serveContract answers every request with the example's status and body; getReply and
// updateReply also answer the ETag.
func serveContract(t *testing.T, op contractOperation, req contractRequest, etag string) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = append(got, recordedRequest{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.Query(), header: r.Header.Clone(), body: body})
		if op.OperationID == "getReply" || op.OperationID == "updateReply" {
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

// writeConfig gives the run a home of its own whose config points at the API (and holds
// the API key when there is one).
func writeConfig(t *testing.T, apiURL, apiKey string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	lines := "api-url=" + apiURL + "\n"
	if apiKey != "" {
		lines += "api-key=" + apiKey + "\n"
	}
	if err := os.MkdirAll(filepath.Join(home, ".solvr"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".solvr", "config"), []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}
	return home
}

type cliResult struct {
	code           int
	stdout, stderr string
}

// runCLI runs the CLI in-process as the bin does, within a time limit.
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- run(args, &stdout, &stderr) }()
	select {
	case code := <-done:
		return cliResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
	case <-time.After(10 * time.Second):
		t.Fatalf("solvr %s did not finish within 10s", strings.Join(args, " "))
		return cliResult{}
	}
}

type contractRun struct {
	result cliResult
	sent   []recordedRequest
	auth   string
	args   []string
}

func runContractExample(t *testing.T, op contractOperation, req contractRequest, command contractCommand, etag string, asJSON bool) contractRun {
	t.Helper()
	credArgs, apiKey, auth := contractCredential(t, op, req.Credential)
	srv, sent := serveContract(t, op, req, etag)
	writeConfig(t, srv.URL+"/v1", apiKey)
	args := join(command.args(contractCall{t: t, op: op, req: req}), credArgs)
	if asJSON {
		args = append(args, "--json")
	}
	result := runCLI(t, args...)
	return contractRun{result: result, sent: *sent, auth: auth, args: args}
}

func contractPath(t *testing.T, op contractOperation, params map[string]string) string {
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

// checkContractRequest holds what the CLI sent to the example.
func checkContractRequest(t *testing.T, op contractOperation, req contractRequest, res contractRun) {
	t.Helper()
	id := op.OperationID
	if len(res.sent) != 1 {
		t.Fatalf("%s: the CLI sent %d requests, want 1 (solvr %s)", id, len(res.sent), strings.Join(res.args, " "))
	}
	got := res.sent[0]
	if got.method != op.Method {
		t.Errorf("%s: method %s, want %s", id, got.method, op.Method)
	}
	if want := contractPath(t, op, req.PathParams); got.path != want {
		t.Errorf("%s: path %s, want %s", id, got.path, want)
	}
	if a := got.header.Get("Authorization"); a != res.auth {
		t.Errorf("%s: Authorization %q, want %q", id, a, res.auth)
	}
	wantQuery := url.Values{}
	for k, v := range req.Query {
		wantQuery.Set(k, v)
	}
	if !reflect.DeepEqual(got.query, wantQuery) {
		t.Errorf("%s: query %v, want %v", id, got.query, wantQuery)
	}
	for name, value := range req.Headers {
		if got.header.Get(name) != value {
			t.Errorf("%s: header %s %q, want %q", id, name, got.header.Get(name), value)
		}
	}
	for _, name := range []string{"If-Match", "Last-Event-ID"} {
		if _, want := req.Headers[name]; !want && got.header.Get(name) != "" {
			t.Errorf("%s: sent %s the example does not", id, name)
		}
	}
	if len(req.RequestBody) == 0 || string(req.RequestBody) == "null" {
		if len(got.body) != 0 {
			t.Errorf("%s: sent body %s, the example sends none", id, got.body)
		}
		return
	}
	if len(got.body) == 0 {
		t.Errorf("%s: sent no body, the example sends %s", id, req.RequestBody)
		return
	}
	if diff := jsonDiff(req.RequestBody, got.body, true); diff != "" {
		t.Errorf("%s: request body %s; example %s: %s", id, got.body, req.RequestBody, diff)
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
				return fmt.Sprintf("%s.%s: the example has it, the CLI lost it", at, k)
			}
			if d := valueDiff(at+"."+k, w[k], gv, exact); d != "" {
				return d
			}
		}
		if exact {
			for k, gv := range g {
				if _, ok := w[k]; !ok && gv != nil {
					return fmt.Sprintf("%s.%s: the CLI sent it, the example does not", at, k)
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

// firstStreamFrame is the id, name and data of the first event of the streamRoom example.
func firstStreamFrame(t *testing.T, op contractOperation) (id, event, data string) {
	t.Helper()
	var text string
	if err := json.Unmarshal(op.ResponseBody, &text); err != nil {
		t.Fatalf("%s: the event-stream example is not a string: %v", op.OperationID, err)
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && data != "":
			return id, event, data
		}
	}
	return id, event, data
}

// streamContent is the message content of the streamRoom example's first frame.
func streamContent(t *testing.T, ops []contractOperation) string {
	t.Helper()
	for _, op := range ops {
		if op.OperationID == "streamRoom" {
			_, _, data := firstStreamFrame(t, op)
			var frame struct {
				Payload struct {
					Content string `json:"content"`
				} `json:"payload"`
			}
			if err := json.Unmarshal([]byte(data), &frame); err != nil || frame.Payload.Content == "" {
				t.Fatalf("streamRoom: the example's first frame carries no message content: %v", err)
			}
			return frame.Payload.Content
		}
	}
	t.Fatal("the contract has no streamRoom example")
	return ""
}

// checkContractJSON holds what --json printed to the example's answer.
func checkContractJSON(t *testing.T, op contractOperation, stdout, etag string) {
	t.Helper()
	id := op.OperationID
	if op.ResponseMediaType == "text/event-stream" {
		var lines []string
		for _, line := range strings.Split(stdout, "\n") {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) != 1 {
			t.Fatalf("%s: want one line per stream event, got %q", id, stdout)
		}
		var event struct {
			ID    string          `json:"id"`
			Event string          `json:"event"`
			Frame json.RawMessage `json:"frame"`
		}
		if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
			t.Fatalf("%s: the event line is not JSON: %v (%s)", id, err, lines[0])
		}
		wantID, wantEvent, wantData := firstStreamFrame(t, op)
		if event.ID != wantID || event.Event != wantEvent {
			t.Errorf("%s: event id %q name %q, want %q %q", id, event.ID, event.Event, wantID, wantEvent)
		}
		if diff := jsonDiff([]byte(wantData), event.Frame, false); diff != "" {
			t.Errorf("%s: the printed frame %s: %s", id, event.Frame, diff)
		}
		return
	}
	if diff := jsonDiff(op.ResponseBody, []byte(stdout), false); diff != "" {
		t.Errorf("%s: --json printed %s: %s", id, stdout, diff)
	}
	if id == "getReply" || id == "updateReply" {
		var printed struct {
			Data struct {
				ETag string `json:"etag"`
			} `json:"data"`
		}
		_ = json.Unmarshal([]byte(stdout), &printed)
		if printed.Data.ETag != etag {
			t.Errorf("%s: --json printed etag %q, want the %q the edit sends back", id, printed.Data.ETag, etag)
		}
	}
}

func TestContract_EveryOperationHasACommand(t *testing.T) {
	ops := loadContract(t)
	commands := contractCommands("", "")
	ids := make([]string, 0, len(ops))
	for _, op := range ops {
		ids = append(ids, op.OperationID)
	}
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(ids)
	sort.Strings(names)
	if !reflect.DeepEqual(ids, names) {
		t.Errorf("the contract's operations %v, the CLI's commands %v", ids, names)
	}
}

func TestContract_JSONSendsTheExampleRequestAndPrintsTheAnswer(t *testing.T) {
	ops := loadContract(t)
	etag := contractETag(ops)
	commands := contractCommands(etag, streamContent(t, ops))
	for _, op := range ops {
		op := op
		t.Run(op.OperationID, func(t *testing.T) {
			res := runContractExample(t, op, op.contractRequest, commands[op.OperationID], etag, true)
			if res.result.code != 0 || res.result.stderr != "" {
				t.Fatalf("%s: solvr %s exited %d, stderr %q", op.OperationID, strings.Join(res.args, " "), res.result.code, res.result.stderr)
			}
			checkContractRequest(t, op, op.contractRequest, res)
			checkContractJSON(t, op, res.result.stdout, etag)
		})
	}
}

func TestContract_HumanOutputShowsTheAnswer(t *testing.T) {
	ops := loadContract(t)
	etag := contractETag(ops)
	commands := contractCommands(etag, streamContent(t, ops))
	for _, op := range ops {
		op := op
		t.Run(op.OperationID, func(t *testing.T) {
			command := commands[op.OperationID]
			res := runContractExample(t, op, op.contractRequest, command, etag, false)
			if res.result.code != 0 || res.result.stderr != "" {
				t.Fatalf("%s: solvr %s exited %d, stderr %q", op.OperationID, strings.Join(res.args, " "), res.result.code, res.result.stderr)
			}
			checkContractRequest(t, op, op.contractRequest, res)
			var answer map[string]interface{}
			_ = json.Unmarshal(op.ResponseBody, &answer)
			for _, shown := range command.shows(answer) {
				if !strings.Contains(res.result.stdout, shown) {
					t.Errorf("%s: the human output does not show %q:\n%s", op.OperationID, shown, res.result.stdout)
				}
			}
		})
	}
}

func TestContract_EachErrorExampleIsReportedAsTheAPIAnswersIt(t *testing.T) {
	ops := loadContract(t)
	commands := contractCommands(contractETag(ops), streamContent(t, ops))
	seen := 0
	for _, op := range ops {
		for _, e := range op.Errors {
			op, e := op, e
			seen++
			t.Run(op.OperationID+"/"+strconv.Itoa(e.Status), func(t *testing.T) {
				var answer struct {
					Error struct {
						Code      string `json:"code"`
						Message   string `json:"message"`
						RequestID string `json:"request_id"`
					} `json:"error"`
				}
				if err := json.Unmarshal(e.ResponseBody, &answer); err != nil || answer.Error.Code == "" {
					t.Fatalf("%s (%s): the error example carries no error code", op.OperationID, e.Case)
				}

				asJSON := runContractExample(t, op, e.contractRequest, commands[op.OperationID], "", true)
				checkContractRequest(t, op, e.contractRequest, asJSON)
				if asJSON.result.code != 1 {
					t.Errorf("%s (%s): --json exit code %d, want 1", op.OperationID, e.Case, asJSON.result.code)
				}
				if asJSON.result.stdout != "" {
					t.Errorf("%s (%s): --json printed %q on stdout, want nothing", op.OperationID, e.Case, asJSON.result.stdout)
				}
				if diff := jsonDiff(e.ResponseBody, []byte(asJSON.result.stderr), true); diff != "" {
					t.Errorf("%s (%s): --json error %q: %s", op.OperationID, e.Case, asJSON.result.stderr, diff)
				}

				human := runContractExample(t, op, e.contractRequest, commands[op.OperationID], "", false)
				if human.result.code != 1 {
					t.Errorf("%s (%s): exit code %d, want 1", op.OperationID, e.Case, human.result.code)
				}
				if want := answer.Error.Code + ": " + answer.Error.Message; !strings.Contains(human.result.stderr, want) {
					t.Errorf("%s (%s): stderr %q, want it to contain %q", op.OperationID, e.Case, human.result.stderr, want)
				}
				if !strings.Contains(human.result.stderr, answer.Error.RequestID) {
					t.Errorf("%s (%s): stderr %q, want the request id %s", op.OperationID, e.Case, human.result.stderr, answer.Error.RequestID)
				}
			})
		}
	}
	if seen == 0 {
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
