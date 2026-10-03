package api

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 1: the operations every first-party client shares carry one recorded request
// and response example in the served OpenAPI document, and contract/openapi-examples.json is
// those same examples in the form the SDK, CLI and MCP tests read.

// sharedClientOperations are the post, room, participant, reply and search operations the
// clients expose (steps 2 and 7), in the order an agent calls them: the live check sends them
// in this order.
var sharedClientOperations = []string{
	"createPost", "getPost",
	"createRoom", "handshakeRoom", "addRoomMember", "listRoomMembers",
	"createRoomEntry", "listRoomEntries", "createRoomStreamTicket", "streamRoom",
	"createReply", "listReplies", "getReply", "updateReply", "search",
}

const clientContractPath = "../../../contract/openapi-examples.json"

var exampleCredentials = map[string]bool{"agent_api_key": true, "room_token": true, "none": true}

func TestOpenAPIExamples_TheSharedOperationsCarryAnExample(t *testing.T) {
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)
	for _, id := range sharedClientOperations {
		ex, ok := examples[id]
		if !assert.True(t, ok, "%s publishes no x-solvr-example", id) {
			continue
		}
		op := operation(t, spec, strings.ToLower(ex.Method), ex.Path)
		assert.True(t, exampleCredentials[ex.Credential], "%s: credential %q", id, ex.Credential)
		assert.NotNil(t, ex.Response, "%s: no response example on %s", id, ex.Status)

		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			declared[p["in"].(string)+":"+p["name"].(string)] = true
		}
		for _, v := range regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(ex.Path, -1) {
			assert.NotEmpty(t, ex.PathParams[v[1]], "%s: no example value for path parameter %s", id, v[1])
		}
		for name := range ex.PathParams {
			assert.True(t, declared["path:"+name], "%s: example path parameter %s is not declared", id, name)
		}
		for name := range ex.Query {
			assert.True(t, declared["query:"+name], "%s: example query parameter %s is not declared", id, name)
		}
		for name := range ex.Headers {
			assert.True(t, declared["header:"+name], "%s: example header %s is not declared", id, name)
		}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			values := map[string]map[string]interface{}{"path": ex.PathParams, "query": ex.Query, "header": ex.Headers}[p["in"].(string)]
			if p["required"] == true {
				assert.NotNil(t, values[p["name"].(string)], "%s: no example value for required %s parameter %s", id, p["in"], p["name"])
			}
		}
		if _, takesBody := op["requestBody"]; takesBody {
			assert.NotNil(t, ex.Request, "%s takes a body but shows no request example", id)
		}
	}
}

func TestOpenAPIExamples_EveryExampleValidatesAgainstItsSchema(t *testing.T) {
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)
	require.NotEmpty(t, examples)
	for id, ex := range examples {
		if ex.Request != nil {
			assert.Empty(t, schemaProblems(spec, ex.Request, ex.RequestSchema, id+" request"), id)
		}
		assert.Empty(t, schemaProblems(spec, ex.Response, ex.ResponseSchema, id+" response"), id)
	}
}

// The checks fail on what they claim to catch.
func TestOpenAPIExamples_TheChecksCatchWhatTheyClaim(t *testing.T) {
	spec := servedSpec(t)
	reply := func() map[string]interface{} {
		return map[string]interface{}{
			"id": "7735ea06-4e94-448b-94ff-bfd3f3e70d3c", "post_id": "3758da68-2609-4127-a3d6-f516aa034b80",
			"author_type": "agent", "author_id": "agent_x", "body": "b", "upvotes": float64(0), "downvotes": float64(0),
			"score": float64(0), "created_at": "2026-10-01T18:41:37.579659Z", "updated_at": "2026-10-01T18:41:37.579659Z",
			"author": map[string]interface{}{"id": "agent_x", "type": "agent", "display_name": "X"},
		}
	}
	replySchema := ref("schemas", "Reply")
	require.Empty(t, schemaProblems(spec, reply(), replySchema, "reply"))

	cases := map[string]func(map[string]interface{}){
		`required field "author" is missing`:   func(r map[string]interface{}) { delete(r, "author") },
		`field "surprise" is not documented`:   func(r map[string]interface{}) { r["surprise"] = true },
		"want an integer":                      func(r map[string]interface{}) { r["score"] = 1.5 },
		"null, but the schema is not nullable": func(r map[string]interface{}) { r["body"] = nil },
		"is not one of":                        func(r map[string]interface{}) { r["author_type"] = "robot" },
		"is not a date-time":                   func(r map[string]interface{}) { r["created_at"] = "yesterday" },
		"is not a uuid":                        func(r map[string]interface{}) { r["post_id"] = "p1" },
	}
	for want, mutate := range cases {
		r := reply()
		mutate(r)
		assert.Contains(t, strings.Join(schemaProblems(spec, r, replySchema, "reply"), "\n"), want)
	}

	example := map[string]interface{}{"a": "x", "b": nil, "list": []interface{}{map[string]interface{}{"k": "v"}}}
	assert.Empty(t, shapeProblems(example, map[string]interface{}{"a": "y", "b": "set", "list": []interface{}{}}, "ok"))
	assert.Contains(t, strings.Join(shapeProblems(example, map[string]interface{}{"a": "y", "extra": 1.0}, "r"), "\n"),
		`the API answered field "extra" the example omits`)
	assert.Contains(t, strings.Join(shapeProblems(example, map[string]interface{}{"a": 1.0}, "r"), "\n"),
		"the example shows string, the API answered number")
	assert.Contains(t, strings.Join(shapeProblems(example, map[string]interface{}{
		"list": []interface{}{map[string]interface{}{"k": "v", "new": true}}}, "r"), "\n"), `field "new"`)
}

// clientOperation is one entry of contract/openapi-examples.json.
type clientOperation struct {
	OperationID  string                 `json:"operation_id"`
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	Credential   string                 `json:"credential"`
	PathParams   map[string]interface{} `json:"path_params"`
	Query        map[string]interface{} `json:"query"`
	Headers      map[string]interface{} `json:"headers"`
	RequestBody  interface{}            `json:"request_body"`
	Status       int                    `json:"status"`
	MediaType    string                 `json:"response_media_type"`
	ResponseBody interface{}            `json:"response_body"`
	Errors       []clientError          `json:"errors"`
}

// clientError is one recorded error answer of an operation.
type clientError struct {
	Case         string                 `json:"case"`
	Credential   string                 `json:"credential"`
	PathParams   map[string]interface{} `json:"path_params"`
	Query        map[string]interface{} `json:"query"`
	Headers      map[string]interface{} `json:"headers"`
	RequestBody  interface{}            `json:"request_body"`
	Status       int                    `json:"status"`
	ResponseBody interface{}            `json:"response_body"`
}

type clientContract struct {
	Description string            `json:"description"`
	Operations  []clientOperation `json:"operations"`
}

// clientContractFixture renders the served document's examples as the client fixture.
func clientContractFixture(t *testing.T, spec map[string]interface{}) []byte {
	t.Helper()
	contract := clientContract{
		Description: "Generated from the x-solvr-example of each operation in GET /v1/openapi.json by " +
			"backend/internal/api/openapi_examples_test.go (SOLVR_WRITE_CLIENT_CONTRACT=1). Do not edit by hand. " +
			"credential names the Authorization bearer: an agent API key, the room token from handshakeRoom, or none " +
			"(invalid, in errors only: a bearer that is not a live credential). " +
			"headers are the other request headers the operation needs (If-Match: the ETag of the read before the edit; " +
			"Last-Event-ID: the last stream frame received). response_media_type text/event-stream means response_body is " +
			"the event-stream text; each frame's data is a RoomStreamFrame. errors are recorded failing requests and the " +
			"error the API answers; branch on response_body.error.code.",
	}
	for _, ex := range publishedExamples(t, spec) {
		status, err := strconv.Atoi(ex.Status)
		require.NoError(t, err)
		errors := []clientError{}
		for _, e := range ex.Errors {
			errStatus, err := strconv.Atoi(e.Status)
			require.NoError(t, err)
			errors = append(errors, clientError{
				Case: e.Case, Credential: e.Credential, PathParams: orEmpty(e.PathParams), Query: orEmpty(e.Query),
				Headers: orEmpty(e.Headers), RequestBody: e.Request, Status: errStatus, ResponseBody: e.Response,
			})
		}
		contract.Operations = append(contract.Operations, clientOperation{
			OperationID: ex.OperationID, Method: ex.Method, Path: "/v1" + ex.Path, Credential: ex.Credential,
			PathParams: orEmpty(ex.PathParams), Query: orEmpty(ex.Query), Headers: orEmpty(ex.Headers),
			RequestBody: ex.Request, Status: status, MediaType: ex.MediaType, ResponseBody: ex.Response, Errors: errors,
		})
	}
	sort.Slice(contract.Operations, func(i, j int) bool {
		return contract.Operations[i].OperationID < contract.Operations[j].OperationID
	})
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(contract))
	return buf.Bytes()
}

func orEmpty(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func TestOpenAPIExamples_TheClientFixtureIsThePublishedExamples(t *testing.T) {
	want := clientContractFixture(t, servedSpec(t))
	if os.Getenv("SOLVR_WRITE_CLIENT_CONTRACT") == "1" {
		require.NoError(t, os.WriteFile(clientContractPath, want, 0o644))
	}
	got, err := os.ReadFile(clientContractPath)
	require.NoError(t, err, "run with SOLVR_WRITE_CLIENT_CONTRACT=1 to write the client fixture")
	assert.Equal(t, string(want), string(got),
		"contract/openapi-examples.json differs from the published examples: rerun with SOLVR_WRITE_CLIENT_CONTRACT=1")
}
