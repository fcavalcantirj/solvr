package api

import "github.com/fcavalcantirj/solvr/internal/api/handlers"

// addOperations publishes the canonical room, timeline-entry and reply operations and wires
// the shared conventions (openapi_conventions.go) into them, so a client generator sees the
// retry, conditional-edit, cursor-page and stream-resume rules on the operations they govern
// (idx 74 step 6). The same wiring is applied to the post create and edit operations.
func addOperations(spec map[string]interface{}) {
	paths := spec["paths"].(map[string]interface{})
	for path, item := range roomPaths() {
		paths[path] = item
	}
	for path, item := range replyPaths() {
		paths[path] = item
	}
	wirePostConventions(paths)

	schemas := spec["components"].(map[string]interface{})["schemas"].(map[string]interface{})
	for name, schema := range operationSchemas() {
		schemas[name] = schema
	}
	for name, schema := range postSchemas() {
		schemas[name] = schema
	}
	spec["tags"] = append(spec["tags"].([]map[string]interface{}),
		obj("name", "Rooms", "description", "Rooms and their ordered message and event timeline"),
		obj("name", "Replies", "description", "Replies to a post"),
	)
}

// errorRows maps a status to the shared response (components.responses) that describes it.
var errorRows = map[string]string{
	"400": "BadRequest", "401": "Unauthorized", "403": "Forbidden", "404": "NotFound", "409": "Conflict",
	"412": "PreconditionFailed", "413": "PayloadTooLarge", "428": "PreconditionRequired", "429": "RateLimited",
	"503": "ServiceUnavailable",
}

// withErrors adds a reference to the shared error response of each status.
func withErrors(responses map[string]interface{}, statuses ...string) map[string]interface{} {
	for _, status := range statuses {
		responses[status] = ref("responses", errorRows[status])
	}
	return responses
}

// anonymousOrBearer marks a read a caller may make without a credential; a presented
// credential is still validated.
func anonymousOrBearer() []map[string]interface{} {
	return []map[string]interface{}{{}, {"bearerAuth": []interface{}{}}}
}

func jsonOK(description, schema string, headers map[string]interface{}) map[string]interface{} {
	resp := obj("description", description,
		"content", obj("application/json", obj("schema", ref("schemas", schema))))
	if headers != nil {
		resp["headers"] = headers
	}
	return resp
}

func etagHeader() map[string]interface{} { return obj("ETag", ref("headers", "ETag")) }

func replayedHeader() map[string]interface{} {
	return obj("Idempotent-Replayed", ref("headers", "IdempotentReplayed"))
}

func pathParam(name, description string, schema map[string]interface{}) map[string]interface{} {
	return obj("name", name, "in", "path", "required", true, "description", description, "schema", schema)
}

func queryParam(name, description string, schema map[string]interface{}) map[string]interface{} {
	return obj("name", name, "in", "query", "required", false, "description", description, "schema", schema)
}

func slugParam() map[string]interface{} {
	return pathParam("slug", "Room slug", obj("type", "string"))
}

func cursorParam(what string) map[string]interface{} {
	return queryParam("cursor", "The opaque cursor of the previous page: send its meta.next_cursor to read the "+what+
		" that follow. Do not parse or construct it; a malformed cursor is 400. See x-solvr-conventions.pagination.",
		obj("type", "string"))
}

func limitParam(what string) map[string]interface{} {
	return queryParam("limit", "Maximum "+what+" per page. A value above the maximum is clamped to it; zero, negative or non-integer is 400.",
		obj("type", "integer", "default", handlers.EntryPageDefaultLimit, "minimum", 1, "maximum", handlers.EntryPageMaxLimit))
}

func appendParam(op, param map[string]interface{}) {
	list, _ := op["parameters"].([]map[string]interface{})
	op["parameters"] = append(list, param)
}

// wirePostConventions attaches the shared retry and conditional-edit rules to the existing
// post operations: Idempotency-Key on create, If-Match and ETag on read and edit.
func wirePostConventions(paths map[string]interface{}) {
	create := paths["/posts"].(map[string]interface{})["post"].(map[string]interface{})
	appendParam(create, ref("parameters", "IdempotencyKey"))
	created := create["responses"].(map[string]interface{})["201"].(map[string]interface{})
	created["headers"] = replayedHeader()
	withErrors(create["responses"].(map[string]interface{}), "400", "401", "409", "413", "503")

	byID := paths["/posts/{id}"].(map[string]interface{})
	get := byID["get"].(map[string]interface{})
	get["responses"].(map[string]interface{})["200"].(map[string]interface{})["headers"] = etagHeader()

	patch := byID["patch"].(map[string]interface{})
	appendParam(patch, ref("parameters", "IfMatch"))
	patch["responses"].(map[string]interface{})["200"].(map[string]interface{})["headers"] = etagHeader()
	withErrors(patch["responses"].(map[string]interface{}), "400", "401", "403", "404", "412", "413", "428")
}
