package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/hub"
)

// A fresh agent with nothing but net/http, the ONE sentence a human pasted, and the skill
// that sentence points at (v1.3.5, lane S). The sentence names the skill, the room to
// create or join, the visibility, the intent and the role; the skill's "Rooms over plain
// HTTPS" recipes (skill/SKILL.md, byte-identical to solvr.dev/skill.md) say how. These
// tests follow the recipes LITERALLY against the real router — a call line, then its
// Authorization header, then its JSON body — and fail on the first call that does not
// succeed. Nothing here imports a Solvr SDK, CLI, MCP or skill script.

var (
	recipeCall          = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) (https://api\.solvr\.dev/\S+)$`)
	recipeBearer        = regexp.MustCompile(`^Authorization: Bearer ([A-Z][A-Z_]+)$`)
	httpOnlyPlaceholder = regexp.MustCompile(`\b[A-Z][A-Z_]{3,}\b`)
	sentenceRoomLink    = regexp.MustCompile(`https://solvr\.dev/rooms/([a-z0-9][a-z0-9-]*)`)

	// Every sentence sends the agent to the skill first. A sentence that creates a room,
	// served by GET /v1/connect, carries the visit's flow code on that link (?f=<code>).
	skillSentenceStart = regexp.MustCompile(`^Learn Solvr from https://solvr\.dev/skill\.md(\?f=[a-hjkmnp-z2-9]{8})?\. `)
	skillLinkFlowCode  = regexp.MustCompile(`^https://solvr\.dev/skill\.md\?f=(.+)$`)
)

// httpOnlyAgent is one agent with an HTTP client and the values it has learned so far.
type httpOnlyAgent struct {
	name    string            // its own choice for "name"/"agent_name"
	title   string            // its room's display_name: the intent its sentence gave it
	message string            // what it posts
	private bool              // the visibility its sentence asked for
	flow    string            // the code its sentence's skill link carried (?f=<code>), if any
	id      string            // its public agent id, from registration
	vars    map[string]string // placeholder -> value (ROOM_SLUG, YOUR_AGENT_API_KEY, ...)
	calls   []string          // "METHOD path" of every call it made, in order
	seen    []string          // every entry body it has read
}

func newHTTPOnlyAgent(name, title string) *httpOnlyAgent {
	return &httpOnlyAgent{name: name, title: title, message: name + " reporting in", vars: map[string]string{}}
}

// sentence is a served prompt, decoded.
type sentence struct {
	Text     string `json:"text"`
	Segments []struct {
		Kind  string `json:"kind"`
		Text  string `json:"text"`
		Value string `json:"value"`
	} `json:"segments"`
}

func decodeSentence(t *testing.T, raw any) sentence {
	t.Helper()
	b, err := json.Marshal(raw)
	require.NoError(t, err)
	var s sentence
	require.NoError(t, json.Unmarshal(b, &s))
	require.Regexp(t, skillSentenceStart, s.Text, "the sentence sends the agent to the skill first")
	return s
}

// flowCode is the code the sentence's skill link carries (?f=<code>), or "" for a plain link.
func (s sentence) flowCode() string {
	if m := skillLinkFlowCode.FindStringSubmatch(s.segment("link")); m != nil {
		return m[1]
	}
	return ""
}

func (s sentence) segment(kind string) string {
	for _, seg := range s.Segments {
		if seg.Kind == kind {
			if kind == "visibility" {
				return seg.Value
			}
			return seg.Text
		}
	}
	return ""
}

// startRoom is the first agent: its sentence says "Create a ... room", so it follows the
// skill's Identity and Start a room recipes with the visibility and intent the sentence gave.
// When the skill link it was given carries ?f=<code>, it adds that code to the create body
// as flow_id, as the Start a room recipe says (skillFlowRule).
func (a *httpOnlyAgent) startRoom(t *testing.T, base string, s sentence) {
	t.Helper()
	require.Contains(t, s.Text, ". Create a ", "a first agent's sentence creates the room")
	a.private = s.segment("visibility") == "private"
	if a.flow = s.flowCode(); a.flow != "" {
		require.Contains(t, skillRooms(t), skillFlowRule, "the agent only sends flow_id because the skill tells it to")
	}
	if a.title == "" {
		a.title = s.segment("intent")
	}
	a.follow(t, base, skillHTTPBlock(t, "Identity"))
	a.follow(t, base, skillHTTPBlock(t, "Start a room"))
}

// register is the Identity recipe alone: a joining agent registers before a private
// room's owner can admit it.
func (a *httpOnlyAgent) register(t *testing.T, base string) {
	t.Helper()
	a.follow(t, base, skillHTTPBlock(t, "Identity"))
}

// joinRoom is a joining agent: its sentence names the room link, and it follows the skill's
// Join a room recipe (registering first unless it already has a key).
func (a *httpOnlyAgent) joinRoom(t *testing.T, base string, s sentence) {
	t.Helper()
	m := sentenceRoomLink.FindStringSubmatch(s.Text)
	require.NotNil(t, m, "a joining agent's sentence names the room link: %q", s.Text)
	a.vars["ROOM_SLUG"] = m[1]
	if _, ok := a.vars["YOUR_AGENT_API_KEY"]; !ok {
		a.register(t, base)
	}
	a.follow(t, base, skillHTTPBlock(t, "Join a room"))
}

// admit is the room owner admitting a joining agent by the public id the human relayed.
func (a *httpOnlyAgent) admit(t *testing.T, base, theirID string) {
	t.Helper()
	a.vars["THEIR_PUBLIC_AGENT_ID"] = theirID
	a.follow(t, base, skillHTTPBlock(t, "Private rooms"))
}

// read is the recipe's GET of the room timeline, made again later: what the agent sees now.
func (a *httpOnlyAgent) read(t *testing.T, base string) {
	t.Helper()
	a.follow(t, base, "GET https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries\nAuthorization: Bearer YOUR_ROOM_TOKEN\n")
}

// follow makes every call a recipe block teaches, in order, against base instead of
// production: a call line, then (until a blank line or the next call) its Authorization
// header and its JSON body.
func (a *httpOnlyAgent) follow(t *testing.T, base, block string) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	lines := strings.Split(block, "\n")
	for i := 0; i < len(lines); i++ {
		m := recipeCall.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			continue
		}
		credential, body := "", ""
		for j := i + 1; j < len(lines); j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" || recipeCall.MatchString(l) {
				break
			}
			if b := recipeBearer.FindStringSubmatch(l); b != nil {
				credential = b[1]
			} else if strings.HasPrefix(l, "{") {
				body = l
			}
		}
		a.call(t, client, base, lines[i], m[1], m[2], body, credential, len(a.calls))
	}
}

func (a *httpOnlyAgent) call(t *testing.T, client *http.Client, base, line, method, url, body, credential string, n int) {
	t.Helper()
	path := strings.TrimPrefix(url, "https://api.solvr.dev")
	for k, v := range a.vars {
		path = strings.ReplaceAll(path, k, v)
		body = strings.ReplaceAll(body, k, v)
	}
	require.Empty(t, httpOnlyPlaceholder.FindAllString(path, -1), "%s: the recipe asks for a value no earlier step gave the agent: %q", a.name, line)

	var rdr io.Reader
	if body != "" {
		var fields map[string]any
		require.NoError(t, json.Unmarshal([]byte(body), &fields), "%s: the body the recipe shows is not JSON: %s", a.name, body)
		// The agent fills in its own choices and what its sentence said; every other field
		// is sent as the recipe shows it.
		for k := range fields {
			switch k {
			case "name", "agent_name":
				fields[k] = a.name
			case "display_name":
				fields[k] = a.title
			case "is_private":
				fields[k] = a.private
			case "body", "content":
				fields[k] = a.message
			case "client_entry_id":
				fields[k] = fmt.Sprintf("%s-%d", a.name, n)
			}
		}
		// The skill's rule for a link that carried ?f=<code>: the create body gains flow_id.
		if a.flow != "" && method == http.MethodPost && path == "/v1/rooms" {
			fields["flow_id"] = a.flow
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
		require.True(t, ok, "%s: the recipe presents %s before any step gave the agent one: %q", a.name, credential, line)
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
		a.vars["YOUR_AGENT_API_KEY"] = key
	case method == http.MethodPost && path == "/v1/rooms":
		a.vars["ROOM_SLUG"], _ = data["slug"].(string)
	case strings.HasSuffix(path, "/handshake"):
		a.vars["YOUR_ROOM_TOKEN"], _ = data["room_token"].(string)
	case method == http.MethodPost && strings.HasSuffix(path, "/entries"):
		// The pin step names the entry the agent just posted (data.id).
		if id, ok := data["id"].(float64); ok {
			a.vars["ENTRY_ID"] = strconv.FormatInt(int64(id), 10)
		}
	case method == http.MethodGet && strings.HasSuffix(path, "/entries"):
		rows, _ := out["data"].([]any)
		for _, r := range rows {
			row, _ := r.(map[string]any)
			if b, ok := row["body"].(string); ok {
				a.seen = append(a.seen, b)
			}
		}
	}
}

// httpOnlyGet reads a JSON document, with an optional credential.
func httpOnlyGet(t *testing.T, url string) map[string]any {
	t.Helper()
	return httpOnlyGetAs(t, url, "")
}

func httpOnlyGetAs(t *testing.T, url, credential string) map[string]any {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, url, credential, "")
	require.Equal(t, http.StatusOK, status, "GET %s: %v", url, out)
	data, _ := out["data"].(map[string]any)
	require.NotNil(t, data, "GET %s: %v", url, out)
	return data
}

// roomSentence is the joining agent's sentence the room page serves for a role.
func roomSentence(t *testing.T, base, slug, role, credential string) sentence {
	t.Helper()
	return decodeSentence(t, httpOnlyGetAs(t, base+"/v1/rooms/"+slug+"/connect?role="+role, credential)["prompt"])
}

// requireAuthoredEntries checks that each agent's message is in the room under its own id.
func requireAuthoredEntries(t *testing.T, base, slug, credential string, agents ...*httpOnlyAgent) {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms/"+slug+"/entries", credential, "")
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

// startCalls is what the Start a room recipe makes, in order, after registering.
func startCalls(a *httpOnlyAgent) []string {
	slug := a.vars["ROOM_SLUG"]
	return []string{
		"POST /v1/agents/register", "POST /v1/rooms", "POST /v1/rooms/" + slug + "/handshake",
		"POST /r/" + slug + "/join", "POST /v1/rooms/" + slug + "/entries",
		"POST /v1/rooms/" + slug + "/entries/" + a.vars["ENTRY_ID"] + "/pin", "GET /v1/rooms/" + slug + "/entries",
	}
}

func TestConnectSentence_PlannerAndExecutorExchangeOverPlainHTTP(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	// The default contract, as a logged-out visitor copies it.
	start := httpOnlyGet(t, ts.URL+"/v1/connect?intent="+fmt.Sprintf("ship+the+signup+page+%d", n))
	selected, _ := start["selected"].(map[string]any)
	require.Equal(t, "plan-and-build", selected["preset"])
	require.Equal(t, "public", selected["visibility"])
	first := decodeSentence(t, start["prompt"])

	planner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hp%d", n), "")
	planner.startRoom(t, ts.URL, first)
	slug := planner.vars["ROOM_SLUG"]
	require.Equal(t, startCalls(planner), planner.calls, "the planner's calls, as the skill teaches them")
	room := httpOnlyGet(t, ts.URL+"/v1/rooms/"+slug)
	roomInfo, _ := room["room"].(map[string]any)
	require.Equal(t, fmt.Sprintf("ship the signup page %d", n), roomInfo["display_name"], "the intent named the room")
	pinned, _ := room["latest_pinned"].(map[string]any)
	require.NotNil(t, pinned, "the planner leaves latest_pinned unset")
	require.Equal(t, planner.message, pinned["content"], "latest_pinned is the planner's directive: %v", pinned)

	// The second paste: the executor's sentence for the real room.
	executor := newHTTPOnlyAgent(fmt.Sprintf("roomtest_he%d", n), "")
	executor.joinRoom(t, ts.URL, roomSentence(t, ts.URL, slug, "executor", ""))
	require.Equal(t, []string{
		"POST /v1/agents/register", "POST /v1/rooms/" + slug + "/handshake", "POST /r/" + slug + "/join",
		"GET /v1/rooms/" + slug + "/entries", "POST /v1/rooms/" + slug + "/entries",
	}, executor.calls, "the executor's calls, as the skill teaches them")
	require.Contains(t, executor.seen, planner.message, "the executor read the planner's directive before posting")

	// The exchange: the planner reads the executor's answer.
	planner.read(t, ts.URL)
	require.Contains(t, planner.seen, executor.message, "the planner reads the executor's post")

	// Any number of agents: a reviewer and a collaborator join the same room by role.
	reviewer := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hr%d", n), "")
	reviewer.joinRoom(t, ts.URL, roomSentence(t, ts.URL, slug, "reviewer", ""))
	third := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hc%d", n), "")
	third.joinRoom(t, ts.URL, roomSentence(t, ts.URL, slug, "collaborator", ""))

	requireAuthoredEntries(t, ts.URL, slug, "", planner, executor, reviewer, third)
}

func TestConnectSentence_EveryUseCaseStartsItsRoomOverPlainHTTP(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	for i, preset := range []string{"plan-and-build", "collaborate", "build-and-review"} {
		start := httpOnlyGet(t, ts.URL+"/v1/connect?preset="+preset+"&visibility=public")
		first := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hf%d_%d", i, n), fmt.Sprintf("test http first %d %d", i, n))
		first.startRoom(t, ts.URL, decodeSentence(t, start["prompt"]))
		require.Equal(t, startCalls(first), first.calls, preset)
		requireAuthoredEntries(t, ts.URL, first.vars["ROOM_SLUG"], "", first)
	}
}

// Share context is private by default: the expert gives its id to the human, the human
// relays it, the learner admits it, and only then does the expert's handshake succeed.
func TestConnectSentence_APrivateRoomAdmitsTheJoinerThroughTheHuman(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	n := time.Now().UnixNano() % 100000000

	start := httpOnlyGet(t, ts.URL+"/v1/connect?preset=collaborate")
	first := decodeSentence(t, start["prompt"])
	require.Equal(t, "private", first.segment("visibility"))
	require.Contains(t, first.Text, "It's private, so the EXPERT gives me its agent id for you to admit.")

	learner := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hl%d", n), fmt.Sprintf("test http learner %d", n))
	learner.startRoom(t, ts.URL, first)
	slug := learner.vars["ROOM_SLUG"]
	status, _ := doJSON(t, http.MethodGet, ts.URL+"/v1/rooms/"+slug, "", "")
	require.Equal(t, http.StatusForbidden, status, "the room the sentence asked for is private")

	// The owner reads the expert's sentence for its private room (its own key).
	expertSentence := roomSentence(t, ts.URL, slug, "expert", learner.vars["YOUR_AGENT_API_KEY"])
	require.Contains(t, expertSentence.Text, "give me your agent id first")

	expert := newHTTPOnlyAgent(fmt.Sprintf("roomtest_hx%d", n), "")
	expert.register(t, ts.URL)
	status, _ = handshake(t, ts.URL, slug, expert.vars["YOUR_AGENT_API_KEY"], "")
	require.Equal(t, http.StatusForbidden, status, "until admitted, the handshake answers 403")

	learner.admit(t, ts.URL, expert.id) // the id the human relayed
	expert.joinRoom(t, ts.URL, expertSentence)
	require.Contains(t, expert.seen, learner.message, "the expert read the learner's question")
	learner.read(t, ts.URL)
	require.Contains(t, learner.seen, expert.message, "the learner reads the expert's answer")
	requireAuthoredEntries(t, ts.URL, slug, learner.vars["YOUR_ROOM_TOKEN"], learner, expert)
}

// Every call the skill's recipes teach is a route this API actually serves.
func TestSkillRecipes_NameOnlyRoutesThisAPIServes(t *testing.T) {
	_, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	// Walk a router built exactly like the one under test, hub included: without the hub
	// the room transport routes are not mounted at all and the walk would prove nothing.
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)
	served := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		served[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	}))
	names := strings.NewReplacer("ROOM_SLUG", "{slug}", "ENTRY_ID", "{entry_id}")
	taught := 0
	for _, title := range []string{"Identity", "Start a room", "Join a room", "Private rooms"} {
		for _, line := range strings.Split(skillHTTPBlock(t, title), "\n") {
			m := recipeCall.FindStringSubmatch(strings.TrimSpace(line))
			if m == nil {
				continue
			}
			taught++
			path := names.Replace(strings.TrimPrefix(m[2], "https://api.solvr.dev"))
			require.True(t, served[m[1]+" "+strings.TrimSuffix(path, "/")], "%s teaches %s %s, which this API does not serve", title, m[1], path)
		}
	}
	require.GreaterOrEqual(t, taught, 11)
}
