package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 2 (errors): each family of shared operations publishes a recorded error
// answer, so every client is tested on surfacing the stable error code it branches on.

// errorExampleFamilies are the operations that carry the error example of their family and
// the code each one answers.
var errorExampleFamilies = map[string]string{
	"getPost":         "NOT_FOUND",             // posts
	"updateReply":     "PRECONDITION_FAILED",   // replies: a stale If-Match
	"createRoomEntry": "UNAUTHORIZED",          // rooms: a room token that is not live
	"streamRoom":      "STREAM_TICKET_INVALID", // watch
	"search":          "VALIDATION_ERROR",      // search
}

// errorCredentials adds "invalid" to the success credentials: a bearer that is not a live
// credential (a revoked, rotated-away or mistyped token).
var errorCredentials = map[string]bool{"agent_api_key": true, "room_token": true, "none": true, "invalid": true}

func TestOpenAPIExamples_EachFamilyPublishesAnErrorExample(t *testing.T) {
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)
	for id, code := range errorExampleFamilies {
		ex, ok := examples[id]
		if !assert.True(t, ok, "%s publishes no example", id) {
			continue
		}
		codes := []string{}
		for _, e := range ex.Errors {
			codes = append(codes, errorCode(e.Response))
		}
		assert.Contains(t, codes, code, "%s: no error example answering %s", id, code)
	}
}

// Every error example names a status the operation declares, answers the documented error
// envelope, and its code is one the declared response row names; its parameters are declared.
func TestOpenAPIExamples_ErrorExamplesAreDeclaredAndValid(t *testing.T) {
	spec := servedSpec(t)
	total := 0
	for id, ex := range publishedExamples(t, spec) {
		op := operation(t, spec, strings.ToLower(ex.Method), ex.Path)
		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			declared[p["in"].(string)+":"+p["name"].(string)] = true
		}
		for _, e := range ex.Errors {
			total++
			where := id + " error " + e.Status + " (" + e.Case + ")"
			assert.NotEmpty(t, e.Case, "%s: says what goes wrong", where)
			assert.True(t, errorCredentials[e.Credential], "%s: credential %q", where, e.Credential)
			row, ok := op["responses"].(map[string]interface{})[e.Status]
			if !assert.True(t, ok, "%s: the operation does not declare %s", where, e.Status) {
				continue
			}
			resp := deref(t, spec, row).(map[string]interface{})
			code := errorCode(e.Response)
			require.NotEmpty(t, code, "%s: the example answers no error.code", where)
			assert.Contains(t, resp["description"], code, "%s: the %s row does not name %s", where, e.Status, code)
			assert.Empty(t, schemaProblems(spec, e.Response, e.ResponseSchema, where), where)
			for name := range e.PathParams {
				assert.True(t, declared["path:"+name], "%s: path parameter %s is not declared", where, name)
			}
			for name := range e.Query {
				assert.True(t, declared["query:"+name], "%s: query parameter %s is not declared", where, name)
			}
			for name := range e.Headers {
				assert.True(t, declared["header:"+name], "%s: header %s is not declared", where, name)
			}
			if e.Request != nil {
				assert.Empty(t, schemaProblems(spec, e.Request, ex.RequestSchema, where+" request"), where)
			}
		}
	}
	assert.GreaterOrEqual(t, total, len(errorExampleFamilies))
}

func errorCode(response interface{}) string {
	m, _ := response.(map[string]interface{})
	e, _ := m["error"].(map[string]interface{})
	code, _ := e["code"].(string)
	return code
}
