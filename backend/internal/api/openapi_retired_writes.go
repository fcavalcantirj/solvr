package api

import (
	"regexp"
	"strings"
)

// documentRetiredRoutes publishes every retired legacy route — the writes
// (LegacyWriteRetirements, task idx 52 step 4) and the reads (LegacyReadRetirements, task idx
// 73 step 5) — as a deprecated operation whose only answer is 410 ENDPOINT_RETIRED, naming the
// canonical replacement and how to move the call. It replaces whatever the legacy path
// functions documented for that method, so the document never promises a 2xx the server no
// longer gives.
func documentRetiredRoutes(spec map[string]interface{}) {
	paths := spec["paths"].(map[string]interface{})
	for _, table := range [][]LegacyRouteRetirement{LegacyWriteRetirements, LegacyReadRetirements} {
		for _, ret := range table {
			method, route, _ := strings.Cut(ret.Route, " ")
			path := strings.TrimPrefix(route, "/v1")
			item, _ := paths[path].(map[string]interface{})
			if item == nil {
				item = map[string]interface{}{}
				paths[path] = item
			}
			key := strings.ToLower(method)
			previous, _ := item[key].(map[string]interface{})
			item[key] = retiredRouteOperation(ret, previous)
		}
	}

	components := spec["components"].(map[string]interface{})
	components["responses"].(map[string]interface{})["EndpointRetired"] = obj(
		"description", "410 ENDPOINT_RETIRED. This legacy route was retired with the canonical knowledge model; "+
			"there is no sunset period. Nothing is read, checked or written, so every caller gets the same answer. "+
			"error.details.replacement names the canonical route to call instead (null when the command has no "+
			"canonical equivalent) and error.details.instructions says how to move the call.",
		"headers", obj("X-Request-ID", ref("headers", "RequestID")),
		"content", obj("application/json", obj("schema", ref("schemas", "RetiredEndpointError"))),
	)
	components["schemas"].(map[string]interface{})["RetiredEndpointError"] = retiredEndpointErrorSchema()
}

var pathVariable = regexp.MustCompile(`\{([^}]+)\}`)

func retiredRouteOperation(ret LegacyRouteRetirement, previous map[string]interface{}) map[string]interface{} {
	message, details := retirementAnswer(ret)
	summary := ret.Route
	if s, ok := previous["summary"].(string); ok {
		summary = s
	}
	params := []map[string]interface{}{}
	for _, v := range pathVariable.FindAllStringSubmatch(ret.Route, -1) {
		params = append(params, pathParam(v[1], "The legacy id the retired route named; it is not read.", obj("type", "string")))
	}
	op := obj(
		"summary", "Retired: "+summary,
		"description", message+" "+ret.Instructions,
		"deprecated", true,
		"security", []map[string]interface{}{},
		"parameters", params,
		"responses", obj("410", ref("responses", "EndpointRetired")),
		"x-solvr-retired", obj("replacement", details.Replacement, "instructions", ret.Instructions),
	)
	for _, field := range []string{"operationId", "tags"} {
		if v, ok := previous[field]; ok {
			op[field] = v
		}
	}
	return op
}

func retiredEndpointErrorSchema() map[string]interface{} {
	return obj("type", "object", "required", []string{"error"}, "properties", obj(
		"error", obj("type", "object", "required", []string{"code", "message", "details", "request_id"}, "properties", obj(
			"code", obj("type", "string", "enum", []string{ErrCodeEndpointRetired}),
			"message", obj("type", "string", "description", "Names the retired route and the canonical route to use instead."),
			"details", obj("type", "object", "required", []string{"retired_route", "replacement", "instructions"}, "properties", obj(
				"retired_route", obj("type", "string", "description", "The retired \"METHOD /path\" template."),
				"replacement", obj("type", "string", "nullable", true,
					"description", "The canonical \"METHOD /path\" to call instead; null when the command has no canonical equivalent."),
				"instructions", obj("type", "string", "description", "How to move the call to the canonical model."),
			)),
			"request_id", obj("type", "string", "description", "Correlation id, equal to the X-Request-ID response header."),
		)),
	))
}
