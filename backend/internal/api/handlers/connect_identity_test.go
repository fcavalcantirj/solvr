package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// These tests pin the ledger task "Keep automatic agent registration and room
// identity separate for every participant". They are PURE (no DB, no HTTP) so
// they always execute and never SKIP.
//
// The task is about what every copied prompt PROMISES an agent regarding its own
// identity: register without a human account, reuse an existing key rather than
// re-registering, isolate credentials per agent profile when two agents share a
// machine, take a per-agent room token through the handshake, and never claim
// another agent's identity. The API-side guarantees (distinct identity + API key
// on register, 409 with suggestions on a name conflict, author_id stamped from
// the authenticated identity) are covered by the register, name-uniqueness,
// handshake and rooms_messages tests; here we pin the prompt contract.

// allTaskPrompts returns every prompt a visitor can copy, keyed for readable
// failure messages. It exercises the public /connect builders and the two
// room-bound builders with a representative room.
func allTaskPrompts() map[string]string {
	pub := ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic, Task: "ship a feature"}
	room := &models.Room{Slug: "demo-room-123", IsPrivate: false}
	first := &models.Message{AgentName: "planner-bot", Content: "the task"}
	return map[string]string{
		"planner":  plannerPromptText(pub),
		"starter":  starterPromptText(ConnectSelection{Preset: ConnectPresetCollaborate, Visibility: ConnectVisibilityPublic}),
		"builder":  builderPromptText(ConnectSelection{Preset: ConnectPresetBuildAndReview, Visibility: ConnectVisibilityPublic}),
		"executor": executorPromptText(room, first),
		"reviewer": roleSpecificPromptText(room, first, "reviewer"),
	}
}

// Step 2: reuse existing credentials before registering a new identity.
func TestConnectIdentity_EveryPromptReusesExistingKeyBeforeRegistering(t *testing.T) {
	for name, text := range allTaskPrompts() {
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "reuse") {
			t.Errorf("%s prompt never tells the agent to reuse its existing key", name)
		}
		if !strings.Contains(lower, "register yourself once") {
			t.Errorf("%s prompt does not scope registration to first-time (\"register yourself once\")", name)
		}
		reuseIdx := strings.Index(lower, "reuse")
		regIdx := strings.Index(lower, "register yourself once")
		if reuseIdx >= 0 && regIdx >= 0 && reuseIdx > regIdx {
			t.Errorf("%s prompt registers before it offers to reuse an existing identity", name)
		}
	}
}

// Step 3: saved credentials are isolated by agent profile and never overwrite
// the other participant's configuration when two agents share one machine.
func TestConnectIdentity_EveryPromptIsolatesCredentialsByProfile(t *testing.T) {
	for name, text := range allTaskPrompts() {
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "machine") {
			t.Errorf("%s prompt never mentions two agents sharing the same machine", name)
		}
		if !strings.Contains(lower, "profile") {
			t.Errorf("%s prompt does not tell the agent to keep its key under its own profile", name)
		}
		if !strings.Contains(lower, "overwrite") {
			t.Errorf("%s prompt does not forbid overwriting the other agent's saved credential", name)
		}
	}
}

// Step 1/5: registration needs no human account and never routes through a login.
func TestConnectIdentity_RegistrationNeedsNoHumanAccount(t *testing.T) {
	for name, text := range allTaskPrompts() {
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "no human account") {
			t.Errorf("%s prompt does not promise registration works with no human account", name)
		}
		if !strings.Contains(text, "POST "+connectAPIBaseURL+"/v1/agents/register") {
			t.Errorf("%s prompt does not point registration at POST %s/v1/agents/register", name, connectAPIBaseURL)
		}
		for _, forbidden := range []string{"log in", "sign in", "create a solvr account", "human login"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s prompt routes identity through %q, which requires a human", name, forbidden)
			}
		}
	}
}

// Step 4: every prompt takes its OWN per-agent room token through the handshake
// and never shares it into another agent's prompt.
func TestConnectIdentity_EveryPromptTakesItsOwnRoomTokenViaHandshake(t *testing.T) {
	for name, text := range allTaskPrompts() {
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "handshake") {
			t.Errorf("%s prompt does not obtain a room token through the handshake", name)
		}
		if !strings.Contains(lower, "yours alone") {
			t.Errorf("%s prompt does not state the room token is the agent's alone", name)
		}
		if !strings.Contains(lower, "never put it in another agent's prompt") &&
			!strings.Contains(lower, "never put your api key or your room token in that prompt") {
			t.Errorf("%s prompt does not forbid leaking the room token into another agent's prompt", name)
		}
	}
}

// Step 5: a second-agent prompt must never claim another agent's identity.
func TestConnectIdentity_SecondAgentPromptsForbidImpersonation(t *testing.T) {
	for _, name := range []string{"executor", "reviewer"} {
		text := allTaskPrompts()[name]
		if !strings.Contains(strings.ToLower(text), "never impersonate") {
			t.Errorf("%s prompt does not forbid impersonating another agent's identity", name)
		}
	}
}
