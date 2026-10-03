package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Findings F1–F4 and F7 of the v1.3.1 live acceptance (docs/acceptance/v1.3.1-live-acceptance.md):
// what every handed-out prompt must teach an agent so it never misreads an edge block as an
// admission decision, sets the directive in force, reaches a private room's owner through the
// human, can be revoked, and joins an existing room through the canonical entries API.

// handedOutPrompts is every prompt an agent is given: the /v1/connect prompts, the room-bound
// join prompts (also served by /v1/rooms/{slug}/connect) and the add_agent role prompt.
func handedOutPrompts() []promptUnderTest {
	out := allConnectionPrompts()
	for _, sel := range everyConnectSelection() {
		out = append(out, promptUnderTest{"add_agent/" + sel.Preset + "/" + sel.Visibility, connectSlugPlaceholder, connectAddAgent(sel).RolePrompt})
	}
	return out
}

// ownerPrompts are the prompts of the agent that creates and owns the room.
func ownerPrompts(visibility string) map[string]string {
	out := map[string]string{}
	for _, tc := range connectPromptCases("ship the thing") {
		sel := tc.sel
		sel.Visibility = visibility
		out[tc.name] = buildConnectPrompt(sel).Text
	}
	return out
}

// callIn returns the call a prompt teaches to the given method and URL, read literally.
func callIn(t *testing.T, prompt, method, url string) promptCall {
	t.Helper()
	for _, c := range literalCalls(prompt) {
		if c.method == method && c.url == url {
			return c
		}
	}
	t.Fatalf("the prompt never teaches %s %s:\n%s", method, url, prompt)
	return promptCall{}
}

// F1: Solvr answers errors in JSON; a non-JSON 403/503 is the network edge. The prompt names
// the retry with an identifying User-Agent, keeps it, and escalates with the cf-ray.
func TestConnectPrompts_EveryPromptTellsAnEdgeBlockFromAnAdmissionDecision(t *testing.T) {
	for _, p := range handedOutPrompts() {
		t.Run(p.name, func(t *testing.T) {
			lower := strings.ToLower(p.text)
			require.Contains(t, p.text, "always answers an error in JSON")
			require.Contains(t, p.text, "403 or 503 whose body is NOT JSON")
			require.Contains(t, p.text, "error code: 1010")
			require.Contains(t, lower, "not an admission decision")
			require.Contains(t, p.text, "User-Agent: solvr-agent/1.0 (YOUR_CLIENT_NAME)")
			require.Contains(t, lower, "retry that call once")
			require.Contains(t, lower, "on every call after it")
			require.Contains(t, p.text, "cf-ray")
			// Only Solvr's own JSON 403 is an admission answer.
			require.Contains(t, p.text, "A JSON 403 from Solvr means you are not admitted")
			require.NotContains(t, p.text, "- A 403 means you are not admitted")
		})
	}
}

// F2: the owner pins its directive, so the room's latest_pinned is set from the start.
func TestConnectPrompts_OwnerPromptsPinTheirDirective(t *testing.T) {
	for _, vis := range []string{ConnectVisibilityPublic, ConnectVisibilityPrivate} {
		for name, prompt := range ownerPrompts(vis) {
			t.Run(name+"/"+vis, func(t *testing.T) {
				entries := connectEntriesURL(connectSlugPlaceholder)
				pin := entries + "/ENTRY_ID/pin"
				c := callIn(t, prompt, "POST", pin)
				require.Equal(t, "YOUR_ROOM_TOKEN", c.credential, "the pin uses the room token the post used")
				require.Greater(t, strings.Index(prompt, "POST "+pin), strings.Index(prompt, "POST "+entries+"\n"),
					"the pin comes after the post it pins")
				require.Contains(t, prompt, "latest_pinned")
				require.Contains(t, prompt, "data.id")
				require.Contains(t, strings.ToLower(prompt), "pin each newer directive")
			})
		}
	}
}

// F3: a joining agent cannot post before admission, so it gives its id to the human who
// handed it the prompt, never to the room.
func TestConnectPrompts_PrivateJoinerGivesItsIDToTheHuman(t *testing.T) {
	first := &models.Message{AgentName: "planner_bot", Content: "port the billing job"}
	room := &models.Room{Slug: "port-billing-20260924", IsPrivate: true}
	joiners := map[string]string{
		"executor": executorPromptText(room, first),
		"role":     roleSpecificPromptText(room, first, "reviewer"),
	}
	for _, sel := range everyConnectSelection() {
		if sel.Visibility == ConnectVisibilityPrivate {
			joiners["add_agent/"+sel.Preset] = connectAddAgent(sel).RolePrompt
		}
	}
	for name, prompt := range joiners {
		t.Run(name, func(t *testing.T) {
			lower := strings.ToLower(prompt)
			require.Contains(t, lower, "give your public agent id")
			require.Contains(t, lower, "the human who handed you this prompt")
			require.Contains(t, lower, "i relay it to the room owner")
			require.Contains(t, lower, "never post your id in the room")
			require.Contains(t, lower, "until then the handshake answers 403")
			require.NotContains(t, lower, "send the room owner your public agent id", "the old wording named no channel")
		})
	}
}

// F3 + F4: the owner learns the ids from the human, writes joiner prompts that say so, and
// can revoke an agent (DELETE the membership, which also ends its room tokens).
func TestConnectPrompts_PrivateOwnerRelaysIDsAndDocumentsRevocation(t *testing.T) {
	revoke := connectAPIBaseURL + "/v1/rooms/" + connectSlugPlaceholder + "/members/THEIR_PUBLIC_AGENT_ID"
	for name, prompt := range ownerPrompts(ConnectVisibilityPrivate) {
		t.Run(name, func(t *testing.T) {
			lower := strings.ToLower(prompt)
			require.Contains(t, lower, "i relay those ids to you")
			require.Contains(t, lower, "must tell it to give its id to me, never to post it in the room")
			c := callIn(t, prompt, "DELETE", revoke)
			require.Equal(t, "YOUR_AGENT_API_KEY", c.credential, "membership is managed with the owner's agent key")
			require.Contains(t, lower, "ends its room tokens")
		})
	}
	for name, prompt := range ownerPrompts(ConnectVisibilityPublic) {
		require.NotContains(t, prompt, "DELETE ", "%s: a public room has no admission to revoke", name)
	}
}

// F7: the add_agent role prompt joins through the canonical entries API, like every other
// prompt, with an idempotency key on each post and the step-by-step recovery.
func TestConnectAddAgent_RolePromptUsesCanonicalEntries(t *testing.T) {
	entries := connectEntriesURL(connectSlugPlaceholder)
	for _, sel := range everyConnectSelection() {
		t.Run(sel.Preset+"/"+sel.Visibility, func(t *testing.T) {
			prompt := connectAddAgent(sel).RolePrompt
			require.Contains(t, prompt, "POST "+entries)
			require.Contains(t, prompt, "GET "+entries)
			require.Contains(t, prompt, `"client_entry_id": "`)
			require.Contains(t, prompt, "next_cursor")
			require.NotContains(t, prompt, "/r/"+connectSlugPlaceholder+"/message")
			require.NotContains(t, prompt, `"agent_name": "your_agent_name", "content"`)
			last := -1
			for _, step := range []string{
				connectAPIBaseURL + "/v1/agents/register",
				connectAPIBaseURL + "/v1/rooms/" + connectSlugPlaceholder + "/handshake",
				connectAPIBaseURL + "/r/" + connectSlugPlaceholder + "/join",
				"POST " + entries,
			} {
				at := strings.Index(prompt, step)
				require.Greater(t, at, last, "%q must come after the previous bootstrap step", step)
				last = at
			}
			lower := strings.ToLower(prompt)
			require.Contains(t, lower, "if a step fails")
			require.Contains(t, lower, "same client_entry_id")
		})
	}
}
