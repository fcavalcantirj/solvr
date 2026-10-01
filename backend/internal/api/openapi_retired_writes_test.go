package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 52 step 4: every retired legacy write shape is documented with its canonical
// replacement, in the served API document and in the SPEC.md migration notes. Both are
// derived from (or pinned to) LegacyWriteRetirements, the table the router mounts, so the
// docs cannot promise a 201 the server no longer gives.

func retiredOperation(t *testing.T, spec map[string]interface{}, ret LegacyRouteRetirement) (string, map[string]interface{}) {
	t.Helper()
	method, path, _ := strings.Cut(ret.Route, " ")
	return strings.ToLower(method) + " " + path, operation(t, spec, strings.ToLower(method), strings.TrimPrefix(path, "/v1"))
}

func TestOpenAPIRetiredWrites_EveryRetiredRouteIsDocumentedAsGoneWithItsReplacement(t *testing.T) {
	spec := servedSpec(t)
	require.Len(t, LegacyWriteRetirements, 19)
	for _, ret := range LegacyWriteRetirements {
		key, op := retiredOperation(t, spec, ret)

		assert.Equal(t, true, op["deprecated"], "%s must be marked deprecated", key)
		_, hasBody := op["requestBody"]
		assert.False(t, hasBody, "%s documents a request body the server never reads", key)

		responses := op["responses"].(map[string]interface{})
		statuses := make([]string, 0, len(responses))
		for status := range responses {
			statuses = append(statuses, status)
		}
		assert.Equal(t, []string{"410"}, statuses, "%s must document only 410, never a success", key)
		assert.Equal(t, "#/components/responses/EndpointRetired", refName(responses["410"]), key)

		sec, ok := op["security"].([]interface{})
		assert.True(t, ok && len(sec) == 0, "%s answers every caller the same way, so it requires no credential: %v", key, op["security"])

		retired := at(t, op, "x-solvr-retired").(map[string]interface{})
		if ret.Replacement == "" {
			assert.Nil(t, retired["replacement"], "%s has no canonical equivalent", key)
		} else {
			assert.Equal(t, ret.Replacement, retired["replacement"], key)
		}
		assert.Equal(t, ret.Instructions, retired["instructions"], key)
		assert.Contains(t, op["description"], ret.Instructions, key)

		declared := map[string]bool{}
		for _, raw := range op["parameters"].([]interface{}) {
			p := deref(t, spec, raw).(map[string]interface{})
			declared[p["in"].(string)+":"+p["name"].(string)] = true
		}
		for _, variable := range regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(ret.Route, -1) {
			assert.True(t, declared["path:"+variable[1]], "%s does not declare path parameter %s", key, variable[1])
		}
	}
}

// The documented answer is the served answer: the message in each description is the one
// the route's handler sends, and the documented details are the fields it sends. (A router
// without a database mounts no /v1 route, so the handler mountRetiredLegacyWrites mounts is
// called directly; the mount itself is pinned by TestLegacyWriteRoutes_AnswerTheMigrationErrorAndWriteNothing.)
func TestOpenAPIRetiredWrites_TheDocumentedAnswerIsTheServedAnswer(t *testing.T) {
	spec := servedSpec(t)

	gone := deref(t, spec, at(t, spec, "components", "responses", "EndpointRetired")).(map[string]interface{})
	assert.Contains(t, gone["description"], "410 ENDPOINT_RETIRED")
	schema := deref(t, spec, at(t, gone, "content", "application/json", "schema")).(map[string]interface{})
	errorProps := at(t, schema, "properties", "error", "properties").(map[string]interface{})
	assert.Equal(t, []interface{}{ErrCodeEndpointRetired}, at(t, errorProps, "code", "enum"))
	detailProps := at(t, errorProps, "details", "properties").(map[string]interface{})
	documented := make([]string, 0, len(detailProps))
	for name := range detailProps {
		documented = append(documented, name)
	}
	want := jsonFields(reflect.TypeOf(retiredRouteDetails{}))
	sort.Strings(want)
	sameSet(t, "EndpointRetired error.details properties", documented, want)
	assert.Equal(t, true, at(t, detailProps, "replacement", "nullable"))

	for _, ret := range LegacyWriteRetirements {
		key, op := retiredOperation(t, spec, ret)
		method, path, _ := strings.Cut(ret.Route, " ")
		path = regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(path, "00000000-0000-0000-0000-000000000001")
		w := httptest.NewRecorder()
		retiredLegacyRoute(ret).ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
		requireRetiredRecorder(t, w, ret.Route)

		var envelope struct {
			Error struct {
				Code    string                 `json:"code"`
				Message string                 `json:"message"`
				Details map[string]interface{} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		assert.True(t, strings.HasPrefix(op["description"].(string), envelope.Error.Message),
			"%s: description %q must open with the served message %q", key, op["description"], envelope.Error.Message)
		served := make([]string, 0, len(envelope.Error.Details))
		for name := range envelope.Error.Details {
			served = append(served, name)
		}
		sameSet(t, key+" served details", served, documented)
	}
}

// SPEC.md Part 26 carries the migration notes: every retired write with its canonical
// replacement (or "none") and the shape to send instead, and no sunset period.
func TestLegacyWriteRetirements_PublishedInSpecMigrationNotes(t *testing.T) {
	raw, err := os.ReadFile("../../../SPEC.md")
	require.NoError(t, err)
	spec := string(raw)
	start := strings.Index(spec, "# Part 26: Canonical Knowledge API and Route Dispositions")
	require.GreaterOrEqual(t, start, 0, "SPEC.md has no Part 26")
	part := spec[start:]
	if next := strings.Index(part[1:], "\n# Part "); next >= 0 {
		part = part[:next+1]
	}
	notesAt := strings.Index(part, "## 26.6 Retired Legacy Writes")
	require.GreaterOrEqual(t, notesAt, 0, "Part 26 has no 26.6 Retired Legacy Writes migration notes")
	notes := part[notesAt:]
	if next := strings.Index(notes[1:], "\n## "); next >= 0 {
		notes = notes[:next+1]
	}

	for _, fact := range []string{"410", "`ENDPOINT_RETIRED`", "no sunset period", "retired_route", "replacement", "instructions"} {
		assert.Contains(t, notes, fact, "the migration notes must state %s", fact)
	}
	for _, ret := range LegacyWriteRetirements {
		replacement := "none"
		if ret.Replacement != "" {
			replacement = "`" + ret.Replacement + "`"
		}
		assert.Contains(t, notes, "| `"+ret.Route+"` | "+replacement+" |",
			"the migration notes do not map %s to %s", ret.Route, replacement)
	}
	assert.NotContains(t, part, "During the transition a retired route is still served",
		"Part 26 still says every retired route is served; retired writes answer 410")
}
