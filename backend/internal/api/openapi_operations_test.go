package api

import (
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// idx 74 step 6 (slice 23): the room, timeline-entry and reply operations are published in
// the same OpenAPI document as the conventions, with the shared parameters, headers and error
// responses attached to them, so a client generator sees one contract for retries, conditional
// edits, cursor pages and stream resume rather than prose.

var roomEntryReplyOperations = []struct{ method, path string }{
	{"get", "/rooms"},
	{"post", "/rooms"},
	{"get", "/rooms/{slug}"},
	{"patch", "/rooms/{slug}"},
	{"delete", "/rooms/{slug}"},
	{"get", "/rooms/{slug}/entries"},
	{"post", "/rooms/{slug}/entries"},
	{"get", "/rooms/{slug}/entries/{entry_id}"},
	{"get", "/rooms/{slug}/stream"},
	{"get", "/posts/{id}/replies"},
	{"post", "/posts/{id}/replies"},
	{"get", "/replies/{id}"},
	{"patch", "/replies/{id}"},
	{"delete", "/replies/{id}"},
}

func operation(t *testing.T, spec map[string]interface{}, method, path string) map[string]interface{} {
	t.Helper()
	return at(t, spec, "paths", path, method).(map[string]interface{})
}

// deref follows a local $ref so a test can look through a component.
func deref(t *testing.T, spec map[string]interface{}, node interface{}) interface{} {
	t.Helper()
	m, ok := node.(map[string]interface{})
	if !ok {
		return node
	}
	ref, ok := m["$ref"].(string)
	if !ok {
		return node
	}
	cur := interface{}(spec)
	for _, p := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		cur = at(t, cur, p)
	}
	return cur
}

func refName(node interface{}) string {
	m, _ := node.(map[string]interface{})
	ref, _ := m["$ref"].(string)
	return ref
}

// param finds a parameter of an operation by name, looking through component references.
func param(t *testing.T, spec, op map[string]interface{}, name string) map[string]interface{} {
	t.Helper()
	list, _ := op["parameters"].([]interface{})
	for _, raw := range list {
		p, ok := deref(t, spec, raw).(map[string]interface{})
		if ok && p["name"] == name {
			return p
		}
	}
	t.Fatalf("operation has no %q parameter", name)
	return nil
}

func hasParamRef(op map[string]interface{}, component string) bool {
	list, _ := op["parameters"].([]interface{})
	for _, raw := range list {
		if refName(raw) == "#/components/parameters/"+component {
			return true
		}
	}
	return false
}

// jsonFields lists the JSON property names a Go type marshals (embedded structs flattened).
func jsonFields(typ reflect.Type) []string {
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if f.Anonymous && name == "" {
			out = append(out, jsonFields(f.Type)...)
			continue
		}
		if name == "" {
			name = f.Name
		}
		out = append(out, name)
	}
	return out
}

func propertyNames(t *testing.T, spec map[string]interface{}, schema string) []string {
	t.Helper()
	props := at(t, spec, "components", "schemas", schema, "properties").(map[string]interface{})
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestOpenAPIOperations_RoomEntryAndReplyOperationsArePublished(t *testing.T) {
	spec := servedSpec(t)

	seen := map[string]string{}
	for _, o := range roomEntryReplyOperations {
		op := operation(t, spec, o.method, o.path)
		id, _ := op["operationId"].(string)
		assert.NotEmpty(t, id, "%s %s has no operationId", o.method, o.path)
		assert.NotEmpty(t, op["tags"], "%s %s has no tag", o.method, o.path)
		assert.NotEmpty(t, op["responses"], "%s %s has no responses", o.method, o.path)
		assert.NotEmpty(t, op["summary"], "%s %s has no summary", o.method, o.path)
		if prev, dup := seen[id]; dup {
			t.Errorf("operationId %q is used by %s and %s %s", id, prev, o.method, o.path)
		}
		seen[id] = o.method + " " + o.path
	}

	// Path parameters are declared wherever the path template names them.
	for _, o := range roomEntryReplyOperations {
		op := operation(t, spec, o.method, o.path)
		for _, name := range []string{"slug", "id", "entry_id"} {
			if strings.Contains(o.path, "{"+name+"}") {
				p := param(t, spec, op, name)
				assert.Equal(t, "path", p["in"], "%s %s: %s must be a path parameter", o.method, o.path, name)
				assert.Equal(t, true, p["required"], "%s %s: %s must be required", o.method, o.path, name)
			}
		}
	}
}

// The one pagination rule of idx 74 step 2, published where a client reads it and tied to
// the constants the page parsers enforce.
func TestOpenAPIOperations_CursorPaginationRulesMatchTheHandlers(t *testing.T) {
	require.Equal(t, handlers.EntryPageDefaultLimit, handlers.ReplyPageDefaultLimit, "entries and replies must share one default")
	require.Equal(t, handlers.EntryPageMaxLimit, handlers.ReplyPageMaxLimit, "entries and replies must share one maximum")
	spec := servedSpec(t)

	rule := at(t, spec, "x-solvr-conventions", "pagination")
	assert.Equal(t, float64(handlers.EntryPageDefaultLimit), at(t, rule, "default_limit"))
	assert.Equal(t, float64(handlers.EntryPageMaxLimit), at(t, rule, "max_limit"))
	assert.Equal(t, "cursor", at(t, rule, "cursor_parameter"))
	assert.Equal(t, "limit", at(t, rule, "limit_parameter"))
	sameSet(t, "pagination.response_meta", strings_(t, at(t, rule, "response_meta")), []string{"next_cursor", "has_more"})
	sameSet(t, "pagination.applies_to", strings_(t, at(t, rule, "applies_to")),
		[]string{"/v1/rooms/{slug}/entries", "/v1/posts/{id}/replies"})

	// The entry page answers next_cursor: null on its last page; the reply page leaves the
	// field out. Each schema says which, so a generated client does not guess.
	for _, c := range []struct {
		path, page string
		nullCursor bool
	}{
		{"/rooms/{slug}/entries", "RoomEntryPage", true},
		{"/posts/{id}/replies", "ReplyPage", false},
	} {
		get := operation(t, spec, "get", c.path)

		limit := param(t, spec, get, "limit")
		assert.Equal(t, "query", limit["in"], c.path)
		schema := limit["schema"].(map[string]interface{})
		assert.Equal(t, "integer", schema["type"], c.path)
		assert.Equal(t, float64(handlers.EntryPageDefaultLimit), schema["default"], "%s limit default", c.path)
		assert.Equal(t, float64(1), schema["minimum"], "%s limit minimum", c.path)
		assert.Equal(t, float64(handlers.EntryPageMaxLimit), schema["maximum"], "%s limit maximum", c.path)

		cursor := param(t, spec, get, "cursor")
		assert.Equal(t, "query", cursor["in"], c.path)
		assert.Equal(t, "string", cursor["schema"].(map[string]interface{})["type"], c.path)
		assert.Contains(t, cursor["description"], "opaque", "%s cursor must be described as opaque", c.path)

		ok := at(t, get, "responses", "200", "content", "application/json", "schema")
		assert.Equal(t, "#/components/schemas/"+c.page, refName(ok), c.path)
		meta := at(t, spec, "components", "schemas", c.page, "properties", "meta", "properties")
		assert.Equal(t, "boolean", at(t, meta, "has_more", "type"), c.path)
		assert.Equal(t, "string", at(t, meta, "next_cursor", "type"), c.path)
		nextCursor := at(t, meta, "next_cursor").(map[string]interface{})
		if c.nullCursor {
			assert.Equal(t, true, nextCursor["nullable"], "%s: next_cursor is null on the last page", c.path)
		} else {
			assert.Nil(t, nextCursor["nullable"], "%s: next_cursor is absent, not null, on the last page", c.path)
			assert.Contains(t, nextCursor["description"], "only when has_more", c.path)
		}
		assert.Equal(t, "array", at(t, spec, "components", "schemas", c.page, "properties", "data", "type"), c.path)
	}

	// The room entry list is also filterable; the filters are the only other query parameters.
	entries := operation(t, spec, "get", "/rooms/{slug}/entries")
	for _, name := range []string{"kind", "issue"} {
		assert.Equal(t, "query", param(t, spec, entries, name)["in"])
	}
	assert.Equal(t, []interface{}{"message", "event"}, param(t, spec, entries, "kind")["schema"].(map[string]interface{})["enum"])
}

func TestOpenAPIOperations_CreatesAreRetrySafe(t *testing.T) {
	spec := servedSpec(t)

	// Room, post and reply creation take an Idempotency-Key; a replay is the stored 2xx.
	for _, c := range []struct{ path, created string }{
		{"/rooms", "201"},
		{"/posts", "201"},
		{"/posts/{id}/replies", "201"},
	} {
		post := operation(t, spec, "post", c.path)
		assert.True(t, hasParamRef(post, "IdempotencyKey"), "POST %s must accept Idempotency-Key", c.path)
		created := at(t, post, "responses", c.created)
		assert.Equal(t, "#/components/headers/IdempotentReplayed",
			refName(at(t, deref(t, spec, created), "headers", "Idempotent-Replayed")),
			"POST %s: the 201 must document Idempotent-Replayed", c.path)
		assert.Equal(t, "#/components/responses/Conflict", refName(at(t, post, "responses", "409")),
			"POST %s: a reused or in-flight key is 409", c.path)
	}

	// Timeline writes are deduplicated by client_entry_id, not by the header.
	entry := operation(t, spec, "post", "/rooms/{slug}/entries")
	assert.False(t, hasParamRef(entry, "IdempotencyKey"), "timeline writes use client_entry_id")
	assert.Contains(t, propertyNames(t, spec, "PostEntryRequest"), "client_entry_id")
	at(t, entry, "responses", "201")
	replay := at(t, entry, "responses", "200")
	assert.Contains(t, replay.(map[string]interface{})["description"], "idempotent_replay",
		"the 200 of a repeated client_entry_id must be documented")
	assert.Equal(t, "#/components/responses/Conflict", refName(at(t, entry, "responses", "409")))
}

func TestOpenAPIOperations_EditsAreConditionalAndReadsHandBackTheValidator(t *testing.T) {
	spec := servedSpec(t)

	for _, path := range []string{"/posts/{id}", "/rooms/{slug}", "/replies/{id}"} {
		patch := operation(t, spec, "patch", path)
		assert.True(t, hasParamRef(patch, "IfMatch"), "PATCH %s must accept If-Match", path)
		assert.Equal(t, "#/components/responses/PreconditionFailed", refName(at(t, patch, "responses", "412")),
			"PATCH %s: a stale If-Match is 412", path)
		assert.Equal(t, "#/components/headers/ETag", refName(at(t, patch, "responses", "200", "headers", "ETag")),
			"PATCH %s: the edit returns the new validator", path)

		get := operation(t, spec, "get", path)
		assert.Equal(t, "#/components/headers/ETag", refName(at(t, get, "responses", "200", "headers", "ETag")),
			"GET %s: the read returns the validator", path)
	}
}

func TestOpenAPIOperations_StreamPublishesResumeAndCredentialTransport(t *testing.T) {
	spec := servedSpec(t)
	stream := operation(t, spec, "get", "/rooms/{slug}/stream")

	assert.True(t, hasParamRef(stream, "LastEventID"))
	assert.True(t, hasParamRef(stream, "StreamTicket"))
	for _, name := range []string{"after", "lastEventId", "type", "issue"} {
		assert.Equal(t, "query", param(t, spec, stream, name)["in"], name)
	}
	at(t, stream, "responses", "200", "content", "text/event-stream")
	assert.Equal(t, "#/components/responses/ServiceUnavailable", refName(at(t, stream, "responses", "503")),
		"a stream at capacity is 503")

	// No other route documents the query-string credential.
	for _, o := range roomEntryReplyOperations {
		if o.path == "/rooms/{slug}/stream" {
			continue
		}
		assert.False(t, hasParamRef(operation(t, spec, o.method, o.path), "StreamTicket"),
			"%s %s must be header-only", o.method, o.path)
	}
}

// Every operation names the error rows it can answer, as references to the shared responses.
func TestOpenAPIOperations_EveryOperationNamesItsErrorRows(t *testing.T) {
	spec := servedSpec(t)
	names := map[string]string{
		"400": "BadRequest", "401": "Unauthorized", "403": "Forbidden", "404": "NotFound", "409": "Conflict",
		"412": "PreconditionFailed", "413": "PayloadTooLarge", "429": "RateLimited", "503": "ServiceUnavailable",
	}
	want := map[string][]string{
		"post /rooms":                          {"400", "401", "409", "413", "503"},
		"get /rooms/{slug}":                    {"401", "403", "404"},
		"patch /rooms/{slug}":                  {"400", "401", "403", "404", "412", "413"},
		"delete /rooms/{slug}":                 {"401", "403", "404"},
		"get /rooms/{slug}/entries":            {"400", "401", "403", "404"},
		"post /rooms/{slug}/entries":           {"400", "401", "403", "404", "409", "413", "429"},
		"get /rooms/{slug}/entries/{entry_id}": {"400", "401", "403", "404"},
		"get /rooms/{slug}/stream":             {"400", "401", "403", "404", "503"},
		"get /posts/{id}/replies":              {"400", "404"},
		"post /posts/{id}/replies":             {"400", "401", "404", "409", "413", "503"},
		"get /replies/{id}":                    {"404"},
		"patch /replies/{id}":                  {"400", "401", "403", "404", "412", "413"},
		"delete /replies/{id}":                 {"401", "403", "404"},
		"post /posts":                          {"400", "401", "409", "413", "503"},
		"patch /posts/{id}":                    {"400", "401", "403", "404", "412", "413"},
	}
	for key, statuses := range want {
		parts := strings.SplitN(key, " ", 2)
		op := operation(t, spec, parts[0], parts[1])
		for _, status := range statuses {
			got := refName(at(t, op, "responses", status))
			assert.Equal(t, "#/components/responses/"+names[status], got, "%s: %s", key, status)
		}
	}
}

// Reads that a caller may make anonymously say so; writes require a credential.
func TestOpenAPIOperations_SecurityFollowsWhoMayCall(t *testing.T) {
	spec := servedSpec(t)
	public := []string{"get /rooms", "get /rooms/{slug}", "get /rooms/{slug}/entries",
		"get /rooms/{slug}/entries/{entry_id}", "get /rooms/{slug}/stream", "get /posts/{id}/replies", "get /replies/{id}"}
	for _, key := range public {
		parts := strings.SplitN(key, " ", 2)
		sec, _ := operation(t, spec, parts[0], parts[1])["security"].([]interface{})
		require.NotEmpty(t, sec, "%s must state that a credential is optional", key)
		assert.Equal(t, map[string]interface{}{}, sec[0], "%s: the first alternative is anonymous", key)
	}
	for _, key := range []string{"post /rooms", "patch /rooms/{slug}", "delete /rooms/{slug}", "post /posts/{id}/replies",
		"patch /replies/{id}", "delete /replies/{id}", "post /rooms/{slug}/entries"} {
		parts := strings.SplitN(key, " ", 2)
		sec, _ := operation(t, spec, parts[0], parts[1])["security"].([]interface{})
		require.Len(t, sec, 1, "%s requires a credential", key)
		assert.NotEmpty(t, sec[0], "%s requires a credential", key)
	}
}

// A schema that lists a field the server never sends (or omits one it always sends) sends a
// client generator down the wrong path, so the published schemas are pinned to the Go types the
// handlers serialize.
func TestOpenAPIOperations_SchemasDescribeTheJSONTheHandlersReturn(t *testing.T) {
	spec := servedSpec(t)
	for schema, typ := range map[string]reflect.Type{
		"RoomEntry":          reflect.TypeOf(models.RoomEntry{}),
		"Reply":              reflect.TypeOf(models.ReplyWithAuthor{}),
		"ReplyAuthor":        reflect.TypeOf(models.ReplyAuthor{}),
		"Room":               reflect.TypeOf(models.Room{}),
		"RoomSummary":        reflect.TypeOf(models.RoomWithStats{}),
		"CreateReplyRequest": reflect.TypeOf(models.CreateReplyRequest{}),
		"UpdateReplyRequest": reflect.TypeOf(models.UpdateReplyRequest{}),
		"UpdateRoomRequest":  reflect.TypeOf(models.UpdateRoomParams{}),
	} {
		want := jsonFields(typ)
		sort.Strings(want)
		sameSet(t, schema+" properties", propertyNames(t, spec, schema), want)
	}
}

// Structural rules an OpenAPI validator would enforce on the operations this slice publishes:
// every response says something, every path template variable is declared on each operation
// and no operation declares one parameter twice.
func TestOpenAPIOperations_PublishedOperationsAreStructurallySound(t *testing.T) {
	spec := servedSpec(t)
	for _, o := range roomEntryReplyOperations {
		key := o.method + " " + o.path
		op := operation(t, spec, o.method, o.path)

		for status, raw := range op["responses"].(map[string]interface{}) {
			resp := deref(t, spec, raw).(map[string]interface{})
			assert.NotEmpty(t, resp["description"], "%s: response %s has no description", key, status)
		}

		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			id := p["in"].(string) + ":" + p["name"].(string)
			assert.False(t, declared[id], "%s declares parameter %s twice", key, id)
			declared[id] = true
		}
		for _, variable := range regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(o.path, -1) {
			assert.True(t, declared["path:"+variable[1]], "%s does not declare path parameter %s", key, variable[1])
		}
	}
}

// The documented operations are served, at the documented methods and paths.
func TestOpenAPIOperations_EveryDocumentedOperationIsAServedRoute(t *testing.T) {
	served := servedRoutes(t)
	spec := servedSpec(t)
	for _, o := range roomEntryReplyOperations {
		operation(t, spec, o.method, o.path)
		route := strings.ToUpper(o.method) + " /v1" + o.path
		assert.True(t, served[route], "%s is documented but not served", route)
	}
}

// The contract holds against the running API: the documented entry-list parameters are the
// ones the handler accepts, an undocumented one is a 400, and the documented maximum is what a
// larger limit is clamped to.
func TestOpenAPIOperations_EntryListContractHoldsAgainstTheRunningAPI(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	spec := servedSpec(t)

	_, jwt := createRoomTestUser(t, pool)
	slug, tok := createTestRoomWithToken(t, ts, jwt)
	for i := 0; i < 3; i++ {
		postRoomMessage(t, ts.URL, slug, tok, "w1", "contract probe")
	}
	base := ts.URL + "/v1/rooms/" + slug + "/entries"

	status, page := doJSON(t, "GET", base+"?limit=1", tok, "")
	require.Equal(t, http.StatusOK, status, "%v", page)
	cursor, _ := page["meta"].(map[string]any)["next_cursor"].(string)
	require.NotEmpty(t, cursor)

	get := operation(t, spec, "get", "/rooms/{slug}/entries")
	samples := map[string]string{"cursor": cursor, "limit": "1", "kind": "message", "issue": "APP-X"}
	documented := map[string]bool{}
	for _, raw := range get["parameters"].([]interface{}) {
		p := deref(t, spec, raw).(map[string]interface{})
		if p["in"] != "query" {
			continue
		}
		name := p["name"].(string)
		documented[name] = true
		sample, ok := samples[name]
		require.True(t, ok, "documented query parameter %q has no probe value", name)
		status, out := doJSON(t, "GET", base+"?"+name+"="+sample, tok, "")
		assert.Equal(t, http.StatusOK, status, "documented parameter %s must be accepted: %v", name, out)
	}
	sameSet(t, "documented entry-list query parameters", mapKeys(documented), []string{"cursor", "limit", "kind", "issue"})

	status, out := doJSON(t, "GET", base+"?undocumented=1", tok, "")
	assert.Equal(t, http.StatusBadRequest, status, "%v", out)
	assert.Equal(t, "VALIDATION_ERROR", out["error"].(map[string]any)["code"])

	status, out = doJSON(t, "GET", base+"?limit=100000", tok, "")
	require.Equal(t, http.StatusOK, status, "%v", out)
	assert.Equal(t, float64(handlers.EntryPageMaxLimit), out["meta"].(map[string]any)["limit"],
		"a limit above the documented maximum is clamped to it")
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestOpenAPIOperations_StreamTicketMintIsPublishedWithItsRecoverableErrors(t *testing.T) {
	spec := servedSpec(t)
	mint := operation(t, spec, "post", "/rooms/{slug}/stream-ticket")

	assert.Equal(t, "createRoomStreamTicket", mint["operationId"])
	assert.Equal(t, "#/components/schemas/StreamTicketResponse", refName(at(t, mint, "responses", "201", "content", "application/json", "schema")))
	for _, code := range []string{"401", "403", "404", "429"} {
		at(t, mint, "responses", code)
	}
	assert.False(t, hasParamRef(mint, "StreamTicket"), "a ticket cannot mint the next ticket: the mint route is header-only")

	unauthorized, _ := at(t, spec, "components", "responses", "Unauthorized", "description").(string)
	for _, code := range []string{"STREAM_TICKET_INVALID", "STREAM_TICKET_EXPIRED"} {
		assert.Contains(t, unauthorized, code, "the shared 401 row names %s", code)
	}
}
