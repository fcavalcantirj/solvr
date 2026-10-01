package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// idx 74 step 6: the cross-cutting API contract (CORS, credential transport, request-size
// limits, stream resume, timeouts, retries, conditional edits, errors) is published in the
// same OpenAPI document as the routes, and every number in it is read from the value the
// server enforces, so the document cannot drift from the running API.

func servedSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	t.Setenv("ALLOWED_ORIGINS", "")
	router := NewRouter(nil, nil, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/openapi.json = %d", w.Code)
	}
	var spec map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatalf("openapi.json is not JSON: %v", err)
	}
	return spec
}

func at(t *testing.T, node interface{}, path ...string) interface{} {
	t.Helper()
	for _, key := range path {
		m, ok := node.(map[string]interface{})
		if !ok {
			t.Fatalf("%s: parent is not an object", strings.Join(path, "."))
		}
		if node, ok = m[key]; !ok {
			t.Fatalf("spec has no %s", strings.Join(path, "."))
		}
	}
	return node
}

func strings_(t *testing.T, node interface{}) []string {
	t.Helper()
	items, ok := node.([]interface{})
	if !ok {
		t.Fatalf("want a JSON array, got %T", node)
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[i], _ = it.(string)
	}
	return out
}

func sameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	g, w := append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if strings.Join(g, "|") != strings.Join(w, "|") {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestOpenAPIConventions_CORSMatchesThePolicyTheServerEnforces(t *testing.T) {
	spec := servedSpec(t)
	cors := at(t, spec, "x-solvr-conventions", "cors")

	sameSet(t, "allowed_origins", strings_(t, at(t, cors, "allowed_origins")), defaultAllowedOrigins)
	sameSet(t, "allowed_methods", strings_(t, at(t, cors, "allowed_methods")), corsAllowedMethods)
	sameSet(t, "allowed_request_headers", strings_(t, at(t, cors, "allowed_request_headers")), corsAllowedHeaders)
	sameSet(t, "exposed_response_headers", strings_(t, at(t, cors, "exposed_response_headers")), corsExposedHeaders)
	if at(t, cors, "allow_credentials") != true {
		t.Error("allow_credentials must be true")
	}
	if got := at(t, cors, "preflight_max_age_seconds"); got != float64(12*60*60) {
		t.Errorf("preflight_max_age_seconds = %v, want 43200", got)
	}
	if got := at(t, cors, "origins_env"); got != "ALLOWED_ORIGINS" {
		t.Errorf("origins_env = %v", got)
	}
}

func TestOpenAPIConventions_DocumentsTheOriginsAConfiguredServerAllows(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", " https://a.example , https://b.example ")
	router := NewRouter(nil, nil, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil))
	var spec map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	sameSet(t, "allowed_origins", strings_(t, at(t, spec, "x-solvr-conventions", "cors", "allowed_origins")),
		[]string{"https://a.example", "https://b.example"})
}

func TestOpenAPIConventions_CredentialTransportAndRequestLimits(t *testing.T) {
	spec := servedSpec(t)
	conv := at(t, spec, "x-solvr-conventions")

	if at(t, conv, "credential_transport", "header") != "Authorization" ||
		at(t, conv, "credential_transport", "scheme") != "Bearer" {
		t.Error("credential_transport must name the Authorization: Bearer header")
	}
	if at(t, conv, "credential_transport", "cookies") != false {
		t.Error("the API reads no cookies; credential_transport.cookies must be false")
	}
	if at(t, conv, "credential_transport", "stream_query_parameter", "name") != "ticket" {
		t.Error("the stream-only ticket query parameter is not documented")
	}
	if at(t, conv, "credential_transport", "stream_query_parameter", "mint_route") != "POST /v1/rooms/{slug}/stream-ticket" {
		t.Error("the route that mints a stream ticket is not documented")
	}
	if at(t, conv, "credential_transport", "stream_query_parameter", "ttl_seconds") != float64(auth.StreamTicketTTL/time.Second) {
		t.Error("the stream ticket lifetime is not documented")
	}
	sameSet(t, "stream_query_parameter.routes", strings_(t, at(t, conv, "credential_transport", "stream_query_parameter", "routes")),
		[]string{"/v1/rooms/{slug}/stream"})

	if got := at(t, conv, "request_limits", "max_body_bytes"); got != float64(requestBodyLimitBytes) {
		t.Errorf("max_body_bytes = %v, want %d", got, requestBodyLimitBytes)
	}
	if at(t, conv, "request_limits", "multipart_exempt") != true {
		t.Error("multipart uploads are exempt from the body limit")
	}
	if at(t, conv, "request_limits", "over_limit", "status") != float64(http.StatusRequestEntityTooLarge) ||
		at(t, conv, "request_limits", "over_limit", "code") != "PAYLOAD_TOO_LARGE" {
		t.Error("over_limit must be 413 PAYLOAD_TOO_LARGE")
	}
}

func TestOpenAPIConventions_StreamResumeAndTimeoutsMatchTheServer(t *testing.T) {
	spec := servedSpec(t)
	streams := at(t, spec, "x-solvr-conventions", "streams")

	if at(t, streams, "route") != "/v1/rooms/{slug}/stream" || at(t, streams, "media_type") != "text/event-stream" {
		t.Error("streams must name the room stream route and text/event-stream")
	}
	if got := at(t, streams, "heartbeat_seconds"); got != handlers.SSEHeartbeatInterval.Seconds() {
		t.Errorf("heartbeat_seconds = %v, want %v", got, handlers.SSEHeartbeatInterval.Seconds())
	}
	if got := at(t, streams, "max_lifetime_seconds"); got != handlers.SSEMaxLifetime.Seconds() {
		t.Errorf("max_lifetime_seconds = %v, want %v", got, handlers.SSEMaxLifetime.Seconds())
	}
	if got := at(t, streams, "max_concurrent_connections"); got != float64(handlers.MaxGlobalSSEConnections) {
		t.Errorf("max_concurrent_connections = %v", got)
	}
	if got := at(t, streams, "resume", "max_replay_frames"); got != float64(handlers.SSEMaxReplayFrames) {
		t.Errorf("max_replay_frames = %v, want %d", got, handlers.SSEMaxReplayFrames)
	}
	if at(t, streams, "resume", "header") != "Last-Event-ID" {
		t.Error("resume must name the Last-Event-ID header")
	}
	sameSet(t, "resume.query_parameters", strings_(t, at(t, streams, "resume", "query_parameters")), []string{"after", "lastEventId"})

	timeouts := at(t, spec, "x-solvr-conventions", "timeouts")
	if got := at(t, timeouts, "server_read_seconds"); got != ServerReadTimeout.Seconds() {
		t.Errorf("server_read_seconds = %v, want %v", got, ServerReadTimeout.Seconds())
	}
	if got := at(t, timeouts, "server_idle_seconds"); got != ServerIdleTimeout.Seconds() {
		t.Errorf("server_idle_seconds = %v, want %v", got, ServerIdleTimeout.Seconds())
	}
	if at(t, timeouts, "server_write_timeout") != "none" {
		t.Error("the server sets no write timeout (streams are long-lived); the contract must say so")
	}
}

func TestOpenAPIConventions_RetryAndConditionalEditContracts(t *testing.T) {
	spec := servedSpec(t)
	idem := at(t, spec, "x-solvr-conventions", "idempotency")

	if at(t, idem, "header") != apimiddleware.IdempotencyKeyHeader ||
		at(t, idem, "replayed_response_header") != apimiddleware.IdempotentReplayedHeader {
		t.Error("idempotency must name the request and replay headers the middleware uses")
	}
	if got := at(t, idem, "retention_seconds"); got != db.IdempotencyRetention.Seconds() {
		t.Errorf("retention_seconds = %v, want %v", got, db.IdempotencyRetention.Seconds())
	}
	if got := at(t, idem, "max_key_length"); got != float64(255) {
		t.Errorf("max_key_length = %v, want 255", got)
	}
	sameSet(t, "idempotency.conflict_codes", strings_(t, at(t, idem, "conflict_codes")),
		[]string{"IDEMPOTENCY_KEY_REUSED", "IDEMPOTENCY_REQUEST_IN_PROGRESS"})

	cond := at(t, spec, "x-solvr-conventions", "conditional_requests")
	if at(t, cond, "request_header") != "If-Match" || at(t, cond, "validator_header") != "ETag" {
		t.Error("conditional_requests must name If-Match and ETag")
	}
	if at(t, cond, "stale_status") != float64(http.StatusPreconditionFailed) || at(t, cond, "stale_code") != "PRECONDITION_FAILED" {
		t.Error("a stale If-Match is 412 PRECONDITION_FAILED")
	}
	// Required since idx 74 step 5 (owner decision 8 of 2026-09-30): an edit without
	// If-Match is 428 PRECONDITION_REQUIRED in the running API.
	if at(t, cond, "required") != true {
		t.Error("If-Match is required on edits in the running API; the contract must say so")
	}
	if at(t, cond, "missing_status") != float64(http.StatusPreconditionRequired) || at(t, cond, "missing_code") != "PRECONDITION_REQUIRED" {
		t.Error("an edit without If-Match is 428 PRECONDITION_REQUIRED")
	}
	sameSet(t, "conditional_requests.applies_to", strings_(t, at(t, cond, "applies_to")),
		[]string{"PATCH /v1/posts/{id}", "PATCH /v1/replies/{id}", "PATCH /v1/rooms/{slug}"})
	if note, _ := at(t, cond, "note").(string); !strings.Contains(note, "428") || !strings.Contains(note, "same version") {
		t.Errorf("the note must explain the 428 and that one of several edits at the same version wins: %q", note)
	}
}

func TestOpenAPIConventions_ReusableComponentsAndErrorEnvelope(t *testing.T) {
	spec := servedSpec(t)

	props := at(t, spec, "components", "schemas", "Error", "properties", "error", "properties")
	for _, f := range []string{"code", "message", "details", "request_id", "retry_after_seconds"} {
		at(t, props, f)
	}
	sameSet(t, "Error.error.required", strings_(t, at(t, spec, "components", "schemas", "Error", "properties", "error", "required")),
		[]string{"code", "message", "request_id"})

	for _, p := range []string{"IdempotencyKey", "IfMatch", "LastEventID"} {
		at(t, spec, "components", "parameters", p, "in")
	}
	for _, h := range []string{"ETag", "IdempotentReplayed", "RetryAfter", "RequestID"} {
		at(t, spec, "components", "headers", h, "schema")
	}
	for _, r := range []string{"BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict",
		"PreconditionFailed", "PreconditionRequired", "PayloadTooLarge", "RateLimited", "ServiceUnavailable"} {
		at(t, spec, "components", "responses", r, "content", "application/json", "schema", "$ref")
	}
}

// The 400 row tells a client what to branch on for a body that is not JSON: one code, the
// one the API answers on every public route (pinned against the running router by
// TestStatusContract_MalformedJSONBodyIsOneValidationError), not a list of alternatives.
func TestOpenAPIConventions_BadRequestNamesOneCodeForMalformedJSON(t *testing.T) {
	spec := servedSpec(t)
	desc, _ := at(t, spec, "components", "responses", "BadRequest", "description").(string)
	if !strings.Contains(desc, "malformed JSON is always VALIDATION_ERROR") {
		t.Errorf("BadRequest must say malformed JSON is always VALIDATION_ERROR, got %q", desc)
	}
	for _, retired := range []string{"INVALID_JSON", "INVALID_REQUEST"} {
		if strings.Contains(desc, retired) {
			t.Errorf("BadRequest still offers %s as a malformed-body code: %q", retired, desc)
		}
	}
}

// Every $ref in the served document must resolve, so a typo in the new components cannot
// ship a spec that a client generator rejects.
func TestOpenAPIConventions_EveryReferenceResolves(t *testing.T) {
	spec := servedSpec(t)
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch v := node.(type) {
		case map[string]interface{}:
			if ref, ok := v["$ref"].(string); ok {
				parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
				var cur interface{} = spec
				for _, p := range parts {
					m, ok := cur.(map[string]interface{})
					if !ok || m[p] == nil {
						t.Errorf("unresolved $ref %s", ref)
						return
					}
					cur = m[p]
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []interface{}:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(spec)
}

func TestOpenAPIConventions_YAMLCarriesTheSameContract(t *testing.T) {
	router := NewRouter(nil, nil, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil))
	if !strings.Contains(w.Body.String(), "x-solvr-conventions:") {
		t.Error("openapi.yaml does not carry x-solvr-conventions")
	}
}
