package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 5: POST /v1/mcp 2.0.0 refuses each tool and argument of the legacy knowledge model
// that 1.x served, before any request, naming what replaces it — the same choices, with the same
// replacements, as the npm @solvr/mcp-server 2.0.0 (mcp-server/src/removed.ts).

const mcpMigrationNotes = `See "Migrating /v1/mcp from 1.x to 2.0.0" in SPEC.md 18.2.`

// mcp1xCalls are calls a 1.x client still makes, each with the choice it uses.
var mcp1xCalls = []struct {
	tool    string
	args    map[string]interface{}
	removed string // the start of the refusal: the removed tool or argument
	instead string
}{
	{"solvr_answer", map[string]interface{}{"post_id": "post-1", "content": "Raise max_conns."},
		"'solvr_answer'", "use solvr_reply with post_id and body: answers and approaches are replies"},
	{"solvr_answer", map[string]interface{}{"post_id": "post-1", "content": "Pool it.", "approach_angle": "pooling"},
		"'solvr_answer'", "use solvr_reply with post_id and body: answers and approaches are replies"},
	{"solvr_search", map[string]interface{}{"query": "pool", "type": "problem"},
		"The 'type' argument of solvr_search", `search covers every post (it was given "problem")`},
	{"solvr_search", map[string]interface{}{"query": "pool", "type": "question"},
		"The 'type' argument of solvr_search", `search covers every post (it was given "question")`},
	{"solvr_search", map[string]interface{}{"query": "pool", "type": "idea"},
		"The 'type' argument of solvr_search", `search covers every post (it was given "idea")`},
	{"solvr_search", map[string]interface{}{"query": "pool", "type": "all"},
		"The 'type' argument of solvr_search", `search covers every post (it was given "all")`},
	{"solvr_post", map[string]interface{}{"type": "problem", "title": "Pool", "description": "Exhausted."},
		"The 'type' argument of solvr_post", `a post has no type (it was given "problem")`},
	{"solvr_post", map[string]interface{}{"type": "question", "title": "Pool", "description": "Exhausted."},
		"The 'type' argument of solvr_post", `a post has no type (it was given "question")`},
	{"solvr_post", map[string]interface{}{"type": "idea", "title": "Pool", "description": "Exhausted."},
		"The 'type' argument of solvr_post", `a post has no type (it was given "idea")`},
	{"solvr_get", map[string]interface{}{"id": "post-1", "include": []interface{}{"approaches", "answers"}},
		"The 'include' argument of solvr_get", "solvr_get shows the post with its first replies and solvr_replies " +
			`pages through all of them: answers and approaches from before the change are replies there (it was given ["approaches","answers"])`},
}

func TestMCPRemoved_Refuses1xCallsBeforeAnyRequest(t *testing.T) {
	for _, call := range mcp1xCalls {
		for _, auth := range []string{mcpTestKey, ""} {
			h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
			text, isError := callMCP(t, h, call.tool, call.args, auth)
			assert.True(t, isError, "%s %v", call.tool, call.args)
			assert.Equal(t, call.removed+" was removed in /v1/mcp 2.0.0; "+call.instead+". "+mcpMigrationNotes, text)
			assert.Empty(t, rec.requests(), "%s %v: a refused call sends no request", call.tool, call.args)
		}
	}
}

// An argument that is missing or null is not given: the call runs.
func TestMCPRemoved_ANullArgumentIsNotAChoice(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{"id": "post-9", "title": "Pool"}})
		case strings.HasSuffix(r.URL.Path, "/replies"):
			writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}, "meta": map[string]interface{}{"total": 0}})
		case r.URL.Path == "/v1/search":
			writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}, "meta": map[string]interface{}{"total": 0}})
		default:
			writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"id": "post-1", "title": "Pool"}})
		}
	})
	for _, call := range []struct {
		tool string
		args map[string]interface{}
	}{
		{"solvr_search", map[string]interface{}{"query": "pool", "type": nil}},
		{"solvr_post", map[string]interface{}{"title": "Pool", "description": "Exhausted.", "type": nil}},
		{"solvr_get", map[string]interface{}{"id": "post-1", "include": nil}},
	} {
		text, isError := callMCP(t, h, call.tool, call.args, mcpTestKey)
		assert.False(t, isError, "%s: %s", call.tool, text)
		assert.NotContains(t, text, "was removed", call.tool)
	}
	sent := rec.requests()
	require.Len(t, sent, 4, "search, create, and the post read with its replies")
	assert.Equal(t, map[string][]string{"q": {"pool"}, "per_page": {"5"}}, sent[0].query)
	assert.JSONEq(t, `{"title":"Pool","description":"Exhausted."}`, sent[1].body)
	assert.Equal(t, "/v1/posts/post-1", sent[2].path)
	assert.Equal(t, "/v1/posts/post-1/replies", sent[3].path)
}

func TestMCPRemoved_AToolThatNeverExistedIsStillUnknown(t *testing.T) {
	text, isError := mcpToolText(t, "solvr_frobnicate", map[string]interface{}{"type": "problem"})
	assert.True(t, isError)
	assert.Equal(t, "Unknown tool: solvr_frobnicate", text)
}

func TestMCPRemoved_InitializeAnswersTheVersionOfTheNotes(t *testing.T) {
	assert.Equal(t, "2.0.0", MCPVersion)
	assert.Equal(t, MCPVersion, mcpRPC(t, "initialize", nil)["version"])
}

// tools/list offers none of them: no removed tool, and no tool takes a removed argument.
func TestMCPRemoved_ToolsListOffersNone(t *testing.T) {
	schemas, names := mcpToolSchemas(t)
	for tool := range mcpRemovedTools {
		assert.NotContains(t, names, tool)
	}
	for tool, arguments := range mcpRemovedArguments {
		require.Contains(t, names, tool)
		properties, _ := schemas[tool]["properties"].(map[string]interface{})
		for argument := range arguments {
			assert.NotContains(t, properties, argument, "%s offers the removed %s", tool, argument)
		}
	}
	for _, name := range names {
		properties, _ := schemas[name]["properties"].(map[string]interface{})
		for _, legacy := range []string{"type", "include", "approach_angle", "content"} {
			assert.NotContains(t, properties, legacy, "%s offers %s", name, legacy)
		}
	}
}

// Both first-party MCP servers removed the same tools and arguments, with the same replacements.
func TestMCPRemoved_MatchTheNpmServer(t *testing.T) {
	raw, err := os.ReadFile("../../../../mcp-server/src/removed.ts")
	require.NoError(t, err)
	src := string(raw)
	block := func(name string) string {
		m := regexp.MustCompile(`(?s)const ` + name + `[^=]*= \{\n(.*?)\n\};`).FindStringSubmatch(src)
		require.NotNil(t, m, "mcp-server/src/removed.ts has no %s", name)
		return m[1]
	}
	npmTools := map[string]string{}
	for _, m := range regexp.MustCompile(`(solvr_\w+): '([^']*)'`).FindAllStringSubmatch(block("REMOVED_TOOLS"), -1) {
		npmTools[m[1]] = m[2]
	}
	npmArguments := map[string]map[string]string{}
	for _, m := range regexp.MustCompile(`(solvr_\w+): \{\s*(\w+): '([^']*)',?\s*\}`).FindAllStringSubmatch(block("REMOVED_ARGUMENTS"), -1) {
		npmArguments[m[1]] = map[string]string{m[2]: m[3]}
	}
	require.NotEmpty(t, npmTools)
	require.Len(t, npmArguments, 3)
	assert.True(t, reflect.DeepEqual(npmTools, mcpRemovedTools), "npm %v, /v1/mcp %v", npmTools, mcpRemovedTools)
	assert.True(t, reflect.DeepEqual(npmArguments, mcpRemovedArguments), "npm %v, /v1/mcp %v", npmArguments, mcpRemovedArguments)
}

// Every tool a refusal points to is served.
func TestMCPRemoved_EveryReplacementIsServed(t *testing.T) {
	served := MCPToolNames()
	var pointed []string
	for _, call := range mcp1xCalls {
		for _, m := range regexp.MustCompile(`solvr_\w+`).FindAllString(call.instead, -1) {
			pointed = append(pointed, m)
		}
	}
	sort.Strings(pointed)
	require.Contains(t, pointed, "solvr_reply")
	require.Contains(t, pointed, "solvr_replies")
	for _, name := range pointed {
		assert.Contains(t, served, name)
	}
}

// SPEC.md 18.2 carries the notes the refusal points to: every removed tool and argument, what
// replaces it, the refusal itself and the version initialize answers.
func TestMCPRemoved_SpecCarriesTheMigrationNotes(t *testing.T) {
	raw, err := os.ReadFile("../../../../SPEC.md")
	require.NoError(t, err)
	spec := string(raw)
	start := strings.Index(spec, "## 18.2 Integration Methods")
	require.GreaterOrEqual(t, start, 0, "SPEC.md 18.2 not found")
	end := strings.Index(spec[start:], "### Method 2:")
	require.Greater(t, end, 0)
	section := spec[start : start+end]

	heading := "#### Migrating /v1/mcp from 1.x to " + MCPVersion
	require.Contains(t, section, heading, "the notes the refusal names are not in SPEC.md 18.2")
	notes := section[strings.Index(section, heading):]
	refusal, _ := mcpRemovedChoice("solvr_answer", map[string]interface{}{})
	for _, want := range []string{
		refusal, "`solvr_answer` (`post_id`, `content`, `approach_angle`)", "`solvr_reply` (`post_id`, `body`, `parent_reply_id`)",
		"`solvr_post` `type`", "`solvr_search` `type`", "`solvr_get` `include`", "`solvr_replies`",
		"`isError: true`", "before any request", "`null`", `"version": "` + MCPVersion + `"`, "tools/list",
	} {
		assert.Contains(t, notes, want)
	}
	assert.NotContains(t, section, "a legacy\n`type` argument is ignored")
	assert.NotContains(t, strings.Join(strings.Fields(section), " "), "`type` argument is ignored")
}

// The refusal quotes the value it was given as JSON, whatever its shape.
func TestMCPRemoved_RefusalIsValidJSONForEveryArgumentShape(t *testing.T) {
	for _, value := range []interface{}{"problem", 3.0, true, []interface{}{"answers"}, map[string]interface{}{"a": "b"}} {
		text, ok := mcpRemovedChoice("solvr_search", map[string]interface{}{"query": "x", "type": value})
		require.True(t, ok)
		given := regexp.MustCompile(`\(it was given (.*)\)\. See`).FindStringSubmatch(text)
		require.NotNil(t, given, text)
		var back interface{}
		assert.NoError(t, json.Unmarshal([]byte(given[1]), &back), text)
		assert.Equal(t, value, back)
	}
}
