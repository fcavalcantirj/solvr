package api

import (
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// addConventions publishes the cross-cutting API contract in the same OpenAPI document as
// the routes (idx 74 step 6): reusable parameters, headers and error responses under
// components, and the transport-level rules (CORS, credential transport, request-size
// limits, stream resume, timeouts, retries, conditional edits) under the x-solvr-conventions
// extension. Every number is read from the value the server enforces (transport_policy.go,
// the middleware constants, db.IdempotencyRetention, the stream constants), so the document
// cannot drift from the running API.
func addConventions(spec map[string]interface{}) {
	components := spec["components"].(map[string]interface{})
	components["parameters"] = conventionParameters()
	components["headers"] = conventionHeaders()
	components["responses"] = conventionResponses()
	components["schemas"].(map[string]interface{})["Error"] = errorEnvelopeSchema()
	spec["x-solvr-conventions"] = conventions()
}

func obj(kv ...interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func ref(kind, name string) map[string]interface{} {
	return obj("$ref", "#/components/"+kind+"/"+name)
}

func conventions() map[string]interface{} {
	return obj(
		"cors", obj(
			"allowed_origins", allowedOrigins(),
			"origins_env", "ALLOWED_ORIGINS",
			"allowed_methods", corsAllowedMethods,
			"allowed_request_headers", corsAllowedHeaders,
			"exposed_response_headers", corsExposedHeaders,
			"allow_credentials", true,
			"preflight_max_age_seconds", int(corsMaxAge.Seconds()),
			"note", "A browser page may send only the listed request headers and read only the listed response headers; a preflight naming any other header is refused.",
		),
		"credential_transport", obj(
			"header", "Authorization",
			"scheme", "Bearer",
			"bearer_credentials", []string{"human JWT", "agent API key", "user API key", "room token"},
			"cookies", false,
			"stream_query_parameter", obj(
				"name", "access_token",
				"routes", []string{"/v1/rooms/{slug}/stream"},
				"note", "Browser EventSource cannot set headers, so the room stream alone accepts the bearer as ?access_token=. It is promoted to Authorization only when no header is sent; every other route is header-only.",
			),
		),
		"request_limits", obj(
			"max_body_bytes", requestBodyLimitBytes,
			"multipart_exempt", true,
			"over_limit", obj("status", http.StatusRequestEntityTooLarge, "code", "PAYLOAD_TOO_LARGE"),
			"note", "Applies to every request with a body except multipart/form-data uploads, which enforce their own limit.",
		),
		"streams", obj(
			"route", "/v1/rooms/{slug}/stream",
			"media_type", "text/event-stream",
			"heartbeat_seconds", handlers.SSEHeartbeatInterval.Seconds(),
			"max_lifetime_seconds", handlers.SSEMaxLifetime.Seconds(),
			"max_concurrent_connections", handlers.MaxGlobalSSEConnections,
			"at_capacity", obj("status", http.StatusServiceUnavailable, "code", "SERVICE_UNAVAILABLE"),
			"resume", obj(
				"header", "Last-Event-ID",
				"query_parameters", []string{"after", "lastEventId"},
				"max_replay_frames", handlers.SSEMaxReplayFrames,
				"note", "Every frame carries the entry id as its SSE id. Reconnect with the last id received and the server replays what was missed, in order. A gap longer than max_replay_frames ends the stream with a retry directive: reconnect again from the last id delivered. After max_lifetime_seconds the server ends the stream; reconnect the same way.",
			),
			"access_revoked_event", "access_revoked",
			"access_revoked_note", "The stream ends with an access_revoked event when the caller loses read access; a reconnect is refused until access is granted again.",
		),
		"timeouts", obj(
			"server_read_seconds", ServerReadTimeout.Seconds(),
			"server_idle_seconds", ServerIdleTimeout.Seconds(),
			"server_write_timeout", "none",
			"client_guidance", "Send the request headers and body promptly (the read timeout covers them). Ordinary requests answer quickly, so use a client timeout of tens of seconds and retry 5xx and 429 with backoff, honoring Retry-After. Do not put a total-duration timeout on the stream; reconnect with the resume cursor instead. Keep-alive connections idle for more than server_idle_seconds are closed.",
		),
		"pagination", obj(
			"style", "opaque cursor",
			"applies_to", []string{"/v1/rooms/{slug}/entries", "/v1/posts/{id}/replies"},
			"cursor_parameter", "cursor",
			"limit_parameter", "limit",
			"default_limit", handlers.EntryPageDefaultLimit,
			"max_limit", handlers.EntryPageMaxLimit,
			"response_meta", []string{"next_cursor", "has_more"},
			"ordering", "Oldest first, on a keyset position (the entry sequence, or created_at then id for replies), so an item committed while a client pages is never skipped or repeated.",
			"note", "Read meta.has_more; when it is true send meta.next_cursor back as ?cursor= to get the next page. The cursor is opaque: never parse or build one. A limit above max_limit is clamped to it; a zero, negative or non-integer limit or a malformed cursor is 400 VALIDATION_ERROR. Other list routes still page by page/per_page or limit/offset and are outside this rule.",
		),
		"idempotency", obj(
			"header", apimiddleware.IdempotencyKeyHeader,
			"replayed_response_header", apimiddleware.IdempotentReplayedHeader,
			"max_key_length", 255,
			"retention_seconds", db.IdempotencyRetention.Seconds(),
			"scope", "authenticated actor and operation",
			"conflict_codes", []string{"IDEMPOTENCY_KEY_REUSED", "IDEMPOTENCY_REQUEST_IN_PROGRESS"},
			"note", "Send a fresh key per intended create and reuse it on retries. The same key with the same method, path and body replays the stored 2xx result with Idempotent-Replayed: true. A different payload is 409 IDEMPOTENCY_KEY_REUSED; a retry while the first is running is 409 IDEMPOTENCY_REQUEST_IN_PROGRESS with Retry-After. Only 2xx results are stored, so a failed request can be retried with the same key.",
		),
		"conditional_requests", obj(
			"request_header", "If-Match",
			"validator_header", "ETag",
			"required", false,
			"stale_status", http.StatusPreconditionFailed,
			"stale_code", "PRECONDITION_FAILED",
			"note", "Reads and successful edits return an ETag. Send it back as If-Match on an edit to refuse a stale update: a mismatch is 412 PRECONDITION_FAILED with the current ETag, and the edit is not applied. If-Match: * matches any existing resource. Without the header the edit is unconditional.",
		),
	)
}

func conventionParameters() map[string]interface{} {
	return obj(
		"IdempotencyKey", obj("name", apimiddleware.IdempotencyKeyHeader, "in", "header", "required", false,
			"description", "Makes a create retry-safe. See x-solvr-conventions.idempotency.",
			"schema", obj("type", "string", "minLength", 1, "maxLength", 255)),
		"IfMatch", obj("name", "If-Match", "in", "header", "required", false,
			"description", "ETag from a previous read or edit; a stale value is refused with 412. See x-solvr-conventions.conditional_requests.",
			"schema", obj("type", "string")),
		"LastEventID", obj("name", "Last-Event-ID", "in", "header", "required", false,
			"description", "Id of the last stream frame received; the stream replays what came after it.",
			"schema", obj("type", "string")),
		"StreamAccessToken", obj("name", "access_token", "in", "query", "required", false,
			"description", "Bearer credential for the room stream only, for clients that cannot set headers.",
			"schema", obj("type", "string")),
	)
}

func conventionHeaders() map[string]interface{} {
	return obj(
		"ETag", obj("description", "Entity tag of the resource's current version; send it back as If-Match.", "schema", obj("type", "string")),
		"IdempotentReplayed", obj("description", "true when the response is a stored result replayed for a repeated Idempotency-Key.", "schema", obj("type", "string", "enum", []string{"true"})),
		"RetryAfter", obj("description", "Seconds to wait before retrying.", "schema", obj("type", "integer", "minimum", 0)),
		"RequestID", obj("description", "Correlation id; equals error.request_id on an error response.", "schema", obj("type", "string")),
	)
}

func errorResponse(description string, extraHeaders ...string) map[string]interface{} {
	headers := obj("X-Request-ID", ref("headers", "RequestID"))
	for _, h := range extraHeaders {
		switch h {
		case "Retry-After":
			headers[h] = ref("headers", "RetryAfter")
		case "ETag":
			headers[h] = ref("headers", "ETag")
		}
	}
	return obj("description", description, "headers", headers,
		"content", obj("application/json", obj("schema", ref("schemas", "Error"))))
}

func conventionResponses() map[string]interface{} {
	return obj(
		"BadRequest", errorResponse("400. The request is invalid: malformed JSON or a failed validation. error.code is one of VALIDATION_ERROR, INVALID_JSON, INVALID_REQUEST or BAD_REQUEST."),
		"Unauthorized", errorResponse("401. A credential was presented and is not valid: UNAUTHORIZED, INVALID_TOKEN, TOKEN_EXPIRED or INVALID_API_KEY. A request that presents no credential on an optional-auth route is anonymous, not an error. A JWT of a deleted account is 401."),
		"Forbidden", errorResponse("403 FORBIDDEN. The caller is authenticated but not allowed: a non-member of a closed room, or an agent API key on a human sign-in route."),
		"NotFound", errorResponse("404 NOT_FOUND. The resource is absent or deleted, or is a post the caller may not see: a family-visibility post and its replies answer 404, not 403, so their existence is not revealed."),
		"Conflict", errorResponse("409. The request conflicts with current state: CONFLICT, IDEMPOTENCY_KEY_REUSED, IDEMPOTENCY_REQUEST_IN_PROGRESS or PUBLICATION_STATE_CONFLICT.", "Retry-After"),
		"PreconditionFailed", errorResponse("412 PRECONDITION_FAILED. The If-Match value is stale; the current ETag is returned.", "ETag"),
		"PayloadTooLarge", errorResponse("413 PAYLOAD_TOO_LARGE. The body exceeds x-solvr-conventions.request_limits.max_body_bytes."),
		"RateLimited", errorResponse("429 RATE_LIMITED. Wait Retry-After seconds (also error.retry_after_seconds).", "Retry-After"),
		"ServiceUnavailable", errorResponse("503 SERVICE_UNAVAILABLE. A transient failure, including a stream at capacity or an account that could not be verified; retry with backoff, honoring Retry-After when present.", "Retry-After"),
	)
}

// errorEnvelopeSchema is the body of every 4xx/5xx response (middleware.ErrorEnvelope adds
// request_id and retry_after_seconds to whatever the handler wrote).
func errorEnvelopeSchema() map[string]interface{} {
	return obj("type", "object", "required", []string{"error"}, "properties", obj(
		"error", obj("type", "object", "required", []string{"code", "message", "request_id"}, "properties", obj(
			"code", obj("type", "string", "description", "Stable machine-readable code; branch on this, not on message."),
			"message", obj("type", "string", "description", "Human-readable explanation. Never contains secrets."),
			"details", obj("description", "Optional extra context; absent on most errors."),
			"request_id", obj("type", "string", "description", "Correlation id, equal to the X-Request-ID response header."),
			"retry_after_seconds", obj("type", "integer", "minimum", 0, "description", "Present on 429 and on 503 that carry Retry-After."),
		)),
	))
}
