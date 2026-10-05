package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OpenAPI document lists every public route: each route the router serves outside /admin
// (RouteFamilies, pinned to the router by TestRouteFamilies_CoverEveryServedRoute) is an
// operation at its method and path. Routes outside /v1 are published with a path-level server.
func TestOpenAPI_DocumentsEveryPublicRoute(t *testing.T) {
	spec := servedSpec(t)
	paths := spec["paths"].(map[string]interface{})
	params := regexp.MustCompile(`\{[^}]+\}`)
	documented := map[string]map[string]interface{}{}
	for path, raw := range paths {
		documented[params.ReplaceAllString(path, "{}")] = raw.(map[string]interface{})
	}

	for _, family := range RouteFamilies {
		for _, route := range family.Routes {
			parts := strings.SplitN(route, " ", 2)
			method, path := strings.ToLower(parts[0]), parts[1]
			if strings.HasPrefix(path, "/admin") {
				continue
			}
			specPath, root := strings.TrimPrefix(path, "/v1"), !strings.HasPrefix(path, "/v1/")
			item, ok := documented[params.ReplaceAllString(specPath, "{}")]
			if !assert.True(t, ok, "%s is served but has no path in the OpenAPI document", route) {
				continue
			}
			op, ok := item[method].(map[string]interface{})
			if !assert.True(t, ok, "%s is served but the OpenAPI document has no %s operation for it", route, method) {
				continue
			}
			assert.NotEmpty(t, op["summary"], "%s has no summary", route)
			if family.Disposition != DispositionRetire { // a retired route is published as deprecated, without an id
				assert.NotEmpty(t, op["operationId"], "%s has no operationId", route)
			}
			assert.NotEmpty(t, op["responses"], "%s has no responses", route)
			if root {
				servers, _ := item["servers"].([]interface{})
				require.NotEmpty(t, servers, "%s is outside /v1: its path needs its own server", route)
				assert.Equal(t, "https://api.solvr.dev", servers[0].(map[string]interface{})["url"], route)
			}
		}
	}
}

// Operation ids are unique across the whole document.
func TestOpenAPI_OperationIDsAreUnique(t *testing.T) {
	spec := servedSpec(t)
	seen := map[string]string{}
	for path, raw := range spec["paths"].(map[string]interface{}) {
		for method, rawOp := range raw.(map[string]interface{}) {
			op, ok := rawOp.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := op["operationId"].(string)
			if id == "" {
				continue
			}
			where := method + " " + path
			if other, dup := seen[id]; dup {
				t.Errorf("operationId %s is used by %s and %s", id, other, where)
			}
			seen[id] = where
		}
	}
}
