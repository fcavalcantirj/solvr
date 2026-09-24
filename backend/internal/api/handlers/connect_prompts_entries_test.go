package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task "Offer one canonical room API with thin adapters for the existing transport
// routes", step 4: new connection prompts prefer the canonical entries contract
// (POST/GET /v1/rooms/{slug}/entries) over the /r/{slug}/message(s) transport adapters,
// while registration, handshake and presence join stay explicit steps an agent can
// recover one at a time.

// promptUnderTest is one generated prompt and the slug it must address.
type promptUnderTest struct {
	name string
	slug string
	text string
}

func allConnectionPrompts() []promptUnderTest {
	var out []promptUnderTest
	for _, tc := range connectPromptCases("ship the thing") {
		for _, vis := range []string{ConnectVisibilityPublic, ConnectVisibilityPrivate} {
			sel := tc.sel
			sel.Visibility = vis
			out = append(out, promptUnderTest{"connect/" + tc.name + "/" + vis, connectSlugPlaceholder, buildConnectPrompt(sel).Text})
		}
	}
	first := &models.Message{AgentName: "planner_bot", Content: "port the billing job"}
	for _, private := range []bool{false, true} {
		room := &models.Room{Slug: "port-billing-20260924", IsPrivate: private}
		out = append(out,
			promptUnderTest{"room/executor", room.Slug, executorPromptText(room, first)},
			promptUnderTest{"room/executor-empty", room.Slug, executorPromptText(room, nil)},
			promptUnderTest{"room/reviewer", room.Slug, roleSpecificPromptText(room, first, "reviewer")},
		)
	}
	return out
}

func TestConnectPrompts_PreferCanonicalEntries(t *testing.T) {
	for _, p := range allConnectionPrompts() {
		t.Run(p.name, func(t *testing.T) {
			entries := connectAPIBaseURL + "/v1/rooms/" + p.slug + "/entries"

			// Messages are posted and read through the canonical entries contract.
			require.Contains(t, p.text, "POST "+entries)
			require.Contains(t, p.text, "GET "+entries)
			require.Contains(t, p.text, `"body": "`, "the canonical entry body field")
			require.Contains(t, p.text, `"client_entry_id": "`, "every post carries an idempotency key")
			require.Contains(t, p.text, "next_cursor", "reads page forward with the opaque cursor")

			// The /r message adapters are no longer taught to new clients.
			require.NotContains(t, p.text, "/r/"+p.slug+"/message",
				"new prompts must not teach the /r/{slug}/message(s) adapters")

			// Bootstrap stays explicit: register, handshake, join are named steps in order,
			// and the first post comes after them.
			order := []string{
				connectAPIBaseURL + "/v1/agents/register",
				connectAPIBaseURL + "/v1/rooms/" + p.slug + "/handshake",
				connectAPIBaseURL + "/r/" + p.slug + "/join",
				"POST " + entries,
			}
			last := -1
			for _, step := range order {
				at := strings.Index(p.text, step)
				require.Greater(t, at, last, "%q must appear after the previous bootstrap step", step)
				last = at
			}
		})
	}
}

func TestConnectPrompts_EachFailedStepIsRecoverable(t *testing.T) {
	for _, p := range allConnectionPrompts() {
		t.Run(p.name, func(t *testing.T) {
			lower := strings.ToLower(p.text)
			require.Contains(t, lower, "if a step fails")
			// Registration: nothing to undo, retry it or reuse a key.
			require.Contains(t, lower, "registration fails")
			// Handshake: a lost/expired/revoked room token is recovered by a new handshake.
			require.Contains(t, lower, "handshake again")
			require.Contains(t, lower, "401")
			// Join: presence can be retried on its own.
			require.Contains(t, lower, "join fails")
			// Posting: resend with the same client_entry_id, stored once.
			require.Contains(t, lower, "same client_entry_id")
			require.Contains(t, lower, "idempotent_replay")

			// The recovery text is pasted-into-a-conversation prose like the rest.
			require.NotContains(t, p.text, "$")
			require.NotContains(t, p.text, "`")
		})
	}
}
