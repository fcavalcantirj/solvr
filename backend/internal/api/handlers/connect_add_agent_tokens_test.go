package handlers

import (
	"strings"
	"testing"
)

// The "Add another agent" role prompt and the Customize API examples are default
// prompts too (task: "Keep room and content permissions authoritative on the server",
// step 5). Like every other copied prompt, they must send the agent's OWN API key to the
// handshake, keep the room token it returns to that agent alone, and never tell an agent
// to present a room token it cannot have yet. PURE: no DB, no HTTP, never SKIPs.

func everyConnectSelection() []ConnectSelection {
	var out []ConnectSelection
	for _, preset := range []string{ConnectPresetPlanAndBuild, ConnectPresetBuildAndReview, ConnectPresetCollaborate} {
		for _, vis := range []string{ConnectVisibilityPublic, ConnectVisibilityPrivate} {
			out = append(out, ConnectSelection{Preset: preset, Visibility: vis})
		}
	}
	return out
}

// handshakeInstruction returns the text from the handshake call up to the next numbered
// step, which is where the prompt must say which credential the call takes.
func handshakeInstruction(t *testing.T, prompt string) string {
	t.Helper()
	i := strings.Index(prompt, "/handshake")
	if i < 0 {
		t.Fatalf("prompt never calls the handshake:\n%s", prompt)
	}
	start := strings.LastIndex(prompt[:i], "\n\n") + 1
	end := strings.Index(prompt[i:], "\n\n")
	if end < 0 {
		return prompt[start:]
	}
	return prompt[start : i+end]
}

func TestConnectAddAgent_RolePromptTakesItsOwnRoomTokenWithItsOwnKey(t *testing.T) {
	for _, sel := range everyConnectSelection() {
		name := sel.Preset + "/" + sel.Visibility
		prompt := connectAddAgent(sel).RolePrompt
		lower := strings.ToLower(prompt)

		step := handshakeInstruction(t, prompt)
		if !strings.Contains(step, "YOUR_AGENT_API_KEY") {
			t.Errorf("%s: the handshake step does not say it takes the agent's own API key:\n%s", name, step)
		}
		if strings.Contains(step, "YOUR_ROOM_TOKEN") && strings.Index(step, "YOUR_ROOM_TOKEN") < strings.Index(step, "/handshake") {
			t.Errorf("%s: the handshake step presents a room token before the agent has one:\n%s", name, step)
		}
		for _, want := range []string{"yours alone", "never put it in another agent's prompt"} {
			if !strings.Contains(lower, want) {
				t.Errorf("%s: role prompt does not say %q about the room token", name, want)
			}
		}
		for _, want := range []string{"machine", "profile", "overwrite"} {
			if !strings.Contains(lower, want) {
				t.Errorf("%s: role prompt does not isolate the saved credential by agent profile (missing %q)", name, want)
			}
		}
		if !strings.Contains(lower, "never impersonate") {
			t.Errorf("%s: role prompt does not forbid impersonating another participant", name)
		}
	}
}

func TestConnectCustomize_HandshakeExampleUsesTheAgentKeyNotARoomToken(t *testing.T) {
	for _, sel := range everyConnectSelection() {
		name := sel.Preset + "/" + sel.Visibility
		found := false
		for _, ex := range connectCustomize(sel).ApiExamples {
			if !strings.Contains(ex, "/handshake") {
				continue
			}
			found = true
			if strings.Contains(ex, "YOUR_ROOM_TOKEN") {
				t.Errorf("%s: handshake example authenticates with a room token (the handshake is what issues it): %s", name, ex)
			}
			if !strings.Contains(ex, "Authorization: Bearer YOUR_AGENT_API_KEY") {
				t.Errorf("%s: handshake example does not authenticate with the agent's own API key: %s", name, ex)
			}
			if !strings.Contains(strings.ToLower(ex), "your own room token") {
				t.Errorf("%s: handshake example does not say it returns the agent's own room token: %s", name, ex)
			}
		}
		if !found {
			t.Errorf("%s: customize section has no handshake example", name)
		}
	}
}
