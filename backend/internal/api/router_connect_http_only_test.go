package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 3: the default HTTP prompt stays usable without installing any of those optional
// integrations. These tests are the agent that has nothing else: net/http and the prompt
// text served by the real router. They follow each prompt LITERALLY — every
// "METHOD https://api.solvr.dev/..." is a call, a JSON line is its body, and the credential
// is the one the prompt most recently named as "Authorization: Bearer X" — and fail on the
// first call that does not succeed. Nothing here imports a Solvr SDK, CLI, MCP or skill.

var (
	httpOnlyCall        = regexp.MustCompile(`(?:^|[^(])\b(GET|POST|PUT|PATCH|DELETE) (https://api\.solvr\.dev/\S+)`)
	httpOnlyBearer      = regexp.MustCompile(`Authorization: Bearer ([A-Z][A-Z_]+)`)
	httpOnlyBody        = regexp.MustCompile(`\{.*\}`)
	httpOnlyPlaceholder = regexp.MustCompile(`\b[A-Z][A-Z_]{3,}\b`)
)

// httpOnlyAgent is one agent with an HTTP client and the values it has learned so far.
type httpOnlyAgent struct {
	name    string            // its own choice for "name"/"agent_name"
	title   string            // its own choice for a room's display_name
	message string            // what it posts
	id      string            // its agent id, from registration
	vars    map[string]string // placeholder -> value (ROOM_SLUG, YOUR_AGENT_API_KEY, ...)
	calls   []string          // "METHOD path" of every call it made, in order
}

func newHTTPOnlyAgent(name, title string) *httpOnlyAgent {
	return &httpOnlyAgent{name: name, title: title, message: name + " reporting in", vars: map[string]string{}}
}

// follow makes every call the text teaches, in order, against base instead of production.
func (a *httpOnlyAgent) follow(t *testing.T, base, text string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	lines := strings.Split(text, "\n")
	credential := ""
	for i, line := range lines {
		m := httpOnlyCall.FindStringSubmatch(line)
		if m != nil {
			own := credential
			if b := httpOnlyBearer.FindStringSubmatch(line); b != nil {
				own = b[1]
			}
			body := httpOnlyBody.FindString(line[strings.Index(line, m[2])+len(m[2]):])
			if body == "" && i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "{") {
				body = strings.TrimSpace(lines[i+1])
			}
			a.call(t, client, base, line, m[1], strings.TrimRight(m[2], ".,;:)"), body, own, len(a.calls))
		}
		for _, b := range httpOnlyBearer.FindAllStringSubmatch(line, -1) {
			credential = b[1]
		}
	}
}

func (a *httpOnlyAgent) call(t *testing.T, client *http.Client, base, line, method, url, body, credential string, n int) {
	t.Helper()
	path := strings.TrimPrefix(url, "https://api.solvr.dev")
	for k, v := range a.vars {
		path = strings.ReplaceAll(path, k, v)
	}
	require.Empty(t, httpOnlyPlaceholder.FindAllString(path, -1), "%s: the prompt asks for a value no earlier step gave the agent: %q", a.name, line)

	var rdr io.Reader
	if body != "" {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &fields), "%s: the body the prompt shows is not JSON: %s", a.name, body)
		// The agent fills in its own choices; every other field is sent as the prompt shows it.
		for k := range fields {
			switch k {
			case "name", "agent_name":
				fields[k] = a.name
			case "display_name":
				fields[k] = a.title
			case "body", "content":
				fields[k] = a.message
			case "client_entry_id":
				fields[k] = fmt.Sprintf("%s-%d", a.name, n)
			}
		}
		raw, _ := json.Marshal(fields)
		require.Empty(t, httpOnlyPlaceholder.FindAllString(string(raw), -1), "%s: the body asks for a value no earlier step gave the agent: %s", a.name, raw)
		rdr = strings.NewReader(string(raw))
	}
	req, err := http.NewRequest(method, base+path, rdr)
	require.NoError(t, err)
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		value, ok := a.vars[credential]
		require.True(t, ok, "%s: the prompt presents %s before any step gave the agent one: %q", a.name, credential, line)
		req.Header.Set("Authorization", "Bearer "+value)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
		"%s: followed literally, %q answered %d: %s", a.name, strings.TrimSpace(line), resp.StatusCode, raw)
	a.calls = append(a.calls, method+" "+strings.SplitN(path, "?", 2)[0])

	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	data, _ := out["data"].(map[string]any)
	switch {
	case path == "/v1/agents/register":
		key, _ := out["api_key"].(string)
		agent, _ := out["agent"].(map[string]any)
		a.id, _ = agent["id"].(string)
		a.vars["YOUR_AGENT_API_KEY"], a.vars["YOUR_CREDENTIAL"] = key, key
	case method == http.MethodPost && path == "/v1/rooms":
		a.vars["ROOM_SLUG"], _ = data["slug"].(string)
	case strings.HasSuffix(path, "/handshake"):
		tok, _ := data["room_token"].(string)
		a.vars["YOUR_ROOM_TOKEN"], a.vars["YOUR_ROOM_CREDENTIAL"] = tok, tok
	}
}

// httpOnlyGet reads a public JSON document with no credential at all.
func httpOnlyGet(t *testing.T, url string) map[string]any {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, url, "", "")
	require.Equal(t, http.StatusOK, status, "GET %s: %v", url, out)
	data, _ := out["data"].(map[string]any)
	require.NotNil(t, data, "GET %s: %v", url, out)
	return data
}

// requireAuthoredEntries checks that each agent's message is in the room under its own id.
func requireAuthoredEntries(t *testing.T, base, slug string, agents ...*httpOnlyAgent) {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms/"+slug+"/entries", "", "")
	require.Equal(t, http.StatusOK, status, "entries: %v", out)
	rows, _ := out["data"].([]any)
	for _, a := range agents {
		found := 0
		for _, raw := range rows {
			row, _ := raw.(map[string]any)
			if row["body"] == a.message {
				found++
				require.Equal(t, a.id, row["author_id"], "%s's message carries another identity", a.name)
			}
		}
		require.Equal(t, 1, found, "%s's message is not in the room exactly once", a.name)
	}
}

func TestConnectPrompts_DefaultFlowRunsOverPlainHTTP(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	// The default contract, as a logged-out visitor copies it.
	start := httpOnlyGet(t, ts.URL+"/v1/connect")
	selected, _ := start["selected"].(map[string]any)
	require.Equal(t, "plan-and-build", selected["preset"])
	require.Equal(t, "public", selected["visibility"])
	prompt, _ := start["prompt"].(map[string]any)
	text, _ := prompt["text"].(string)

	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hp%d", n), fmt.Sprintf("test http only %d", n))
	planner.follow(t, ts.URL, text)
	slug := planner.vars["ROOM_SLUG"]
	require.Equal(t, []string{
		"POST /v1/agents/register", "POST /v1/rooms", "POST /v1/rooms/" + slug + "/handshake",
		"POST /r/" + slug + "/join", "POST /v1/rooms/" + slug + "/entries", "GET /v1/rooms/" + slug + "/entries",
	}, planner.calls, "the planner prompt's calls")

	// The second paste: the executor prompt for the real room, then a reviewer by role.
	executor := newHTTPOnlyAgent(fmt.Sprintf("roomtest_he%d", n), "")
	executor.follow(t, ts.URL, httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug+"/connect")["prompt"].(string))
	reviewer := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hr%d", n), "")
	reviewer.follow(t, ts.URL, httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug+"/connect?role=reviewer")["prompt"].(string))
	for _, joiner := range []*httpOnlyAgent{executor, reviewer} {
		require.Contains(t, joiner.calls, "POST /v1/rooms/"+slug+"/handshake")
		require.Contains(t, joiner.calls, "POST /r/"+slug+"/join")
		require.Contains(t, joiner.calls, "POST /v1/rooms/"+slug+"/entries")
	}

	// A fourth agent from the contract's "Add another agent" prompt, told the room's slug.
	addAgent, _ := start["add_agent"].(map[string]any)
	fourth := newHTTPOnlyAgent(fmt.Sprintf("roomtest_ha%d", n), "")
	fourth.vars["ROOM_SLUG"] = slug
	fourth.follow(t, ts.URL, addAgent["role_prompt"].(string))
	require.Contains(t, fourth.calls, "POST /r/"+slug+"/join")

	requireAuthoredEntries(t, ts.URL, slug, planner, executor, reviewer, fourth)
}

func TestConnectPrompts_EveryPresetsFirstPromptRunsOverPlainHTTP(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	for i, preset := range []string{"plan-and-build", "build-and-review", "collaborate"} {
		start := httpOnlyGet(t, ts.URL+"/v1/connect?preset="+preset+"&visibility=public")
		prompt, _ := start["prompt"].(map[string]any)
		first := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hf%d_%d", i, n), fmt.Sprintf("test http first %d %d", i, n))
		first.follow(t, ts.URL, prompt["text"].(string))
		require.Len(t, first.calls, 6, "%s: the first prompt's calls: %v", preset, first.calls)
		requireAuthoredEntries(t, ts.URL, first.vars["ROOM_SLUG"], first)
	}
}

func TestConnectPrompts_CustomizeAPIExamplesRunOverPlainHTTP(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	start := httpOnlyGet(t, ts.URL+"/v1/connect")
	customize, _ := start["customize"].(map[string]any)
	examples, _ := customize["api_examples"].([]any)
	require.NotEmpty(t, examples)

	agent := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hx%d", n), fmt.Sprintf("test http examples %d", n))
	for _, ex := range examples {
		// Each example stands alone: it names its own header or none.
		agent.follow(t, ts.URL, ex.(string))
	}
	slug := agent.vars["ROOM_SLUG"]
	require.Equal(t, []string{
		"POST /v1/agents/register", "POST /v1/rooms", "POST /v1/rooms/" + slug + "/handshake",
		"GET /v1/rooms/" + slug + "/entries", "POST /v1/rooms/" + slug + "/entries",
	}, agent.calls, "the API examples' calls")
	requireAuthoredEntries(t, ts.URL, slug, agent)
}
