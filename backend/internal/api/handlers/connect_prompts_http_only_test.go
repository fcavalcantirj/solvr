package handlers

import (
	"regexp"
	"strings"
	"testing"
)

// Task "Keep SDKs, CLI, MCP, skills, and webhooks consistent with the redesigned product",
// step 3: the default HTTP prompt stays usable without installing any of those optional
// integrations. An agent that has nothing but an HTTP client follows a prompt literally:
// every "METHOD https://api.solvr.dev/..." is a call it makes, and it presents the
// credential the prompt most recently named as "Authorization: Bearer X". These tests read
// every served prompt that way. They are PURE (no DB, no HTTP), so they always run.

// promptCall is one call a prompt teaches, read literally.
type promptCall struct {
	method     string
	url        string
	credential string // the placeholder presented; "" = no Authorization header
}

var (
	// A call is "METHOD URL" on the production API origin. "(POST URL)" is a reference to
	// what SOMEONE ELSE does (a joiner is told the owner admits it there), not a call.
	promptCallPattern   = regexp.MustCompile(`(?:^|[^(])\b(GET|POST|PUT|PATCH|DELETE) (` + regexp.QuoteMeta(connectAPIBaseURL) + `/\S+)`)
	promptBearerPattern = regexp.MustCompile(`Authorization: Bearer ([A-Z][A-Z_]+)`)
)

// literalCalls reads a prompt top to bottom the way an agent with only an HTTP client does.
func literalCalls(text string) []promptCall {
	var calls []promptCall
	credential := ""
	for line := range strings.SplitSeq(text, "\n") {
		if m := promptCallPattern.FindStringSubmatch(line); m != nil {
			// A header named on the call's own line (the API examples) is that call's.
			own := credential
			if b := promptBearerPattern.FindStringSubmatch(line); b != nil {
				own = b[1]
			}
			calls = append(calls, promptCall{m[1], strings.TrimRight(m[2], ".,;:)"), own})
		}
		for _, b := range promptBearerPattern.FindAllStringSubmatch(line, -1) {
			credential = b[1]
		}
	}
	return calls
}

const (
	needsNoCredential = "none"
	needsAgentKey     = "the agent's API key"
	needsRoomToken    = "the agent's room token"
)

// presentedAs maps the placeholder names the prompts use to what the agent holds.
var presentedAs = map[string]string{
	"":                     needsNoCredential,
	"YOUR_AGENT_API_KEY":   needsAgentKey,
	"YOUR_CREDENTIAL":      needsAgentKey,
	"YOUR_ROOM_TOKEN":      needsRoomToken,
	"YOUR_ROOM_CREDENTIAL": needsRoomToken,
}

// credentialFor is what the API accepts on each route a prompt may teach (the router's
// auth: authMiddleware on /v1/rooms create/handshake/members, the room-credential guard on
// /r/{slug}/* and /v1/rooms/{slug}/entries). A route not listed here fails the test, so a
// new call in a prompt must be classified before it ships.
func credentialFor(method, url string) (string, bool) {
	path := strings.TrimPrefix(url, connectAPIBaseURL)
	switch {
	case path == "/v1/agents/register":
		return needsNoCredential, true
	case method == "POST" && path == "/v1/rooms":
		return needsAgentKey, true
	case strings.HasPrefix(path, "/v1/rooms/") && strings.HasSuffix(path, "/handshake"):
		return needsAgentKey, true
	case strings.HasPrefix(path, "/v1/rooms/") && strings.HasSuffix(path, "/members"):
		return needsAgentKey, true
	case strings.HasPrefix(path, "/v1/rooms/") && strings.HasSuffix(path, "/entries"):
		return needsRoomToken, true
	case strings.HasPrefix(path, "/r/") && (strings.HasSuffix(path, "/join") ||
		strings.HasSuffix(path, "/message") || strings.HasSuffix(path, "/messages")):
		return needsRoomToken, true
	}
	return "", false
}

// servedPromptTexts is every prompt and copyable example the connect endpoints serve.
func servedPromptTexts() []promptUnderTest {
	out := allConnectionPrompts()
	for _, sel := range everyConnectSelection() {
		name := sel.Preset + "/" + sel.Visibility
		out = append(out, promptUnderTest{"add_agent/" + name, connectSlugPlaceholder, connectAddAgent(sel).RolePrompt})
		for _, ex := range connectCustomize(sel).ApiExamples {
			// Each API example stands alone: it names its own header or none.
			out = append(out, promptUnderTest{"customize/" + name + ": " + ex, connectSlugPlaceholder, ex})
		}
	}
	return out
}

func TestConnectPrompts_EveryCallPresentsTheCredentialItsRouteTakes(t *testing.T) {
	for _, p := range servedPromptTexts() {
		calls := literalCalls(p.text)
		if len(calls) == 0 {
			t.Errorf("%s: teaches no HTTP call an agent could follow", p.name)
			continue
		}
		for _, c := range calls {
			want, known := credentialFor(c.method, c.url)
			if !known {
				t.Errorf("%s: teaches %s %s, a route this test does not classify; add it to credentialFor", p.name, c.method, c.url)
				continue
			}
			got, ok := presentedAs[c.credential]
			if !ok {
				t.Errorf("%s: %s %s presents %q, a credential no step gave the agent", p.name, c.method, c.url, c.credential)
				continue
			}
			if got != want {
				t.Errorf("%s: read literally, %s %s presents %s (%q); the route takes %s",
					p.name, c.method, c.url, got, c.credential, want)
			}
		}
	}
}

// The prompts are the integration-free path: none may send the agent to install or
// configure a Solvr SDK, CLI, MCP server or skill first.
func TestConnectPrompts_NeedNoOptionalIntegration(t *testing.T) {
	banned := []string{"install", "npm", "npx", "pip ", "go get", "brew ", "solvr.sh", "mcp", "skill", "sdk", "plugin"}
	for _, p := range servedPromptTexts() {
		lower := strings.ToLower(p.text)
		for _, b := range banned {
			if strings.Contains(lower, b) {
				t.Errorf("%s: names %q; the default prompt must run with an HTTP client alone", p.name, b)
			}
		}
		for _, c := range literalCalls(p.text) {
			path := strings.TrimPrefix(c.url, connectAPIBaseURL)
			if !strings.HasPrefix(path, "/v1/") && !strings.HasPrefix(path, "/r/") {
				t.Errorf("%s: teaches %s outside the HTTP API", p.name, c.url)
			}
		}
	}
}
