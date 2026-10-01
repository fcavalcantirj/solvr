package api

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Checks behind the published examples (idx 78 step 1): a value against an OpenAPI schema of
// the subset this document uses, and a documented example against the JSON the running API
// answered. Both return their problems instead of failing, so the checks themselves are tested.

// publishedExample is one operation's x-solvr-example with the request and response examples
// it carries, read from the served document.
type publishedExample struct {
	OperationID    string
	Method         string
	Path           string
	Credential     string
	PathParams     map[string]interface{}
	Query          map[string]interface{}
	Headers        map[string]interface{}
	Status         string
	Request        interface{}
	RequestSchema  interface{}
	MediaType      string // of the answer: application/json, or text/event-stream for the stream
	Response       interface{}
	ResponseSchema interface{}
	Errors         []publishedError
}

// publishedError is one entry of an operation's x-solvr-error-examples: a request that fails
// and the error the API answers, with the schema of the status it answers.
type publishedError struct {
	Case           string
	Credential     string
	PathParams     map[string]interface{}
	Query          map[string]interface{}
	Headers        map[string]interface{}
	Request        interface{}
	Status         string
	Response       interface{}
	ResponseSchema interface{}
}

func publishedExamples(t *testing.T, spec map[string]interface{}) map[string]publishedExample {
	t.Helper()
	out := map[string]publishedExample{}
	for path, rawItem := range spec["paths"].(map[string]interface{}) {
		for method, rawOp := range rawItem.(map[string]interface{}) {
			op, ok := rawOp.(map[string]interface{})
			if !ok {
				continue
			}
			ext, ok := op["x-solvr-example"].(map[string]interface{})
			if !ok {
				continue
			}
			ex := publishedExample{
				OperationID: op["operationId"].(string), Method: strings.ToUpper(method), Path: path,
				Credential: fmt.Sprint(ext["credential"]), Status: fmt.Sprint(ext["status"]),
			}
			ex.PathParams, _ = ext["path_params"].(map[string]interface{})
			ex.Query, _ = ext["query"].(map[string]interface{})
			ex.Headers, _ = ext["headers"].(map[string]interface{})
			if body, ok := op["requestBody"].(map[string]interface{}); ok {
				media := at(t, deref(t, spec, body), "content", "application/json").(map[string]interface{})
				ex.Request, ex.RequestSchema = media["example"], media["schema"]
			}
			resp := deref(t, spec, at(t, op, "responses", ex.Status)).(map[string]interface{})
			if content, ok := resp["content"].(map[string]interface{}); ok {
				for _, mediaType := range []string{"application/json", "text/event-stream"} {
					if media, ok := content[mediaType].(map[string]interface{}); ok {
						ex.MediaType, ex.Response, ex.ResponseSchema = mediaType, media["example"], media["schema"]
					}
				}
			}
			raw, _ := op["x-solvr-error-examples"].([]interface{})
			for _, item := range raw {
				e := item.(map[string]interface{})
				pe := publishedError{
					Case: fmt.Sprint(e["case"]), Credential: fmt.Sprint(e["credential"]), Status: fmt.Sprint(e["status"]),
					Request: e["request"], Response: e["response"],
				}
				pe.PathParams, _ = e["path_params"].(map[string]interface{})
				pe.Query, _ = e["query"].(map[string]interface{})
				pe.Headers, _ = e["headers"].(map[string]interface{})
				if row, ok := op["responses"].(map[string]interface{})[pe.Status]; ok {
					errResp := deref(t, spec, row).(map[string]interface{})
					pe.ResponseSchema = at(t, errResp, "content", "application/json", "schema")
				}
				ex.Errors = append(ex.Errors, pe)
			}
			out[ex.OperationID] = ex
		}
	}
	return out
}

// resolveRef follows a local $ref without a *testing.T, for the checks below.
func resolveRef(spec map[string]interface{}, node interface{}) (map[string]interface{}, error) {
	for {
		m, ok := node.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("schema is not an object: %T", node)
		}
		ref, ok := m["$ref"].(string)
		if !ok {
			return m, nil
		}
		var cur interface{} = spec
		for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			next, ok := cur.(map[string]interface{})[part]
			if !ok {
				return nil, fmt.Errorf("unresolvable %s", ref)
			}
			cur = next
		}
		node = cur
	}
}

// schemaProblems lists every way value breaks schema: a missing required field, a field the
// schema does not document (where it lists properties), a wrong type, a null where the field
// is not nullable, a value outside its enum, or a malformed date-time or uuid.
func schemaProblems(spec map[string]interface{}, value, schemaNode interface{}, at string) []string {
	schema, err := resolveRef(spec, schemaNode)
	if err != nil {
		return []string{at + ": " + err.Error()}
	}
	if value == nil {
		if schema["nullable"] == true {
			return nil
		}
		return []string{at + ": null, but the schema is not nullable"}
	}
	var problems []string
	switch schema["type"] {
	case "object":
		obj, ok := value.(map[string]interface{})
		if !ok {
			return []string{fmt.Sprintf("%s: want an object, got %s", at, jsonKind(value))}
		}
		for _, r := range asStrings(schema["required"]) {
			if _, ok := obj[r]; !ok {
				problems = append(problems, fmt.Sprintf("%s: required field %q is missing", at, r))
			}
		}
		props, listed := schema["properties"].(map[string]interface{})
		for _, key := range sortedKeys(obj) {
			prop, ok := props[key]
			if !ok {
				if listed {
					problems = append(problems, fmt.Sprintf("%s: field %q is not documented", at, key))
				}
				continue
			}
			problems = append(problems, schemaProblems(spec, obj[key], prop, at+"."+key)...)
		}
	case "array":
		items, ok := value.([]interface{})
		if !ok {
			return []string{fmt.Sprintf("%s: want an array, got %s", at, jsonKind(value))}
		}
		for i, item := range items {
			problems = append(problems, schemaProblems(spec, item, schema["items"], fmt.Sprintf("%s[%d]", at, i))...)
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return []string{fmt.Sprintf("%s: want a string, got %s", at, jsonKind(value))}
		}
		switch schema["format"] {
		case "date-time":
			if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %q is not a date-time", at, s))
			}
		case "uuid":
			if _, err := uuid.Parse(s); err != nil {
				problems = append(problems, fmt.Sprintf("%s: %q is not a uuid", at, s))
			}
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != math.Trunc(n) {
			return []string{fmt.Sprintf("%s: want an integer, got %s %v", at, jsonKind(value), value)}
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return []string{fmt.Sprintf("%s: want a number, got %s", at, jsonKind(value))}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return []string{fmt.Sprintf("%s: want a boolean, got %s", at, jsonKind(value))}
		}
	}
	if enum := schema["enum"]; enum != nil {
		found := false
		for _, allowed := range enum.([]interface{}) {
			if allowed == value {
				found = true
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("%s: %v is not one of %v", at, value, enum))
		}
	}
	return problems
}

// shapeProblems lists where the documented example does not show what the API answered: a
// field the API returned that the example omits, or a field whose JSON type differs (null on
// either side is a nullable or absent value, not a type). Array items are compared with the
// example's first item. A field only the example shows is allowed: the API omits empty
// optional fields, and the schema check already holds the example to the documented fields.
func shapeProblems(example, real interface{}, at string) []string {
	if example == nil || real == nil {
		return nil
	}
	if jsonKind(example) != jsonKind(real) {
		return []string{fmt.Sprintf("%s: the example shows %s, the API answered %s", at, jsonKind(example), jsonKind(real))}
	}
	var problems []string
	switch r := real.(type) {
	case map[string]interface{}:
		e := example.(map[string]interface{})
		for _, key := range sortedKeys(r) {
			ev, ok := e[key]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: the API answered field %q the example omits", at, key))
				continue
			}
			problems = append(problems, shapeProblems(ev, r[key], at+"."+key)...)
		}
	case []interface{}:
		e := example.([]interface{})
		if len(r) > 0 && len(e) == 0 {
			problems = append(problems, at+": the API answered items, the example shows an empty list")
		}
		if len(e) > 0 {
			for i, item := range r {
				problems = append(problems, shapeProblems(e[0], item, fmt.Sprintf("%s[%d]", at, i))...)
			}
		}
	}
	return problems
}

func jsonKind(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	}
	return fmt.Sprintf("%T", v)
}

func asStrings(v interface{}) []string {
	items, _ := v.([]interface{})
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
