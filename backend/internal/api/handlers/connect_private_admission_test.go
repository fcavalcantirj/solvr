package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task 371 (private collaboration through explicit membership): the generated prompts
// must describe the REAL private-room admission model, not a shared secret. A joining
// agent gives the room owner its PUBLIC agent id; the owner admits it via the
// member-management API (POST /v1/rooms/{slug}/members); each agent then takes its OWN
// per-agent room token by handshake. No shared owner secret and no shared room token.
//
// These are pure unit tests over the prompt builders — no DB, no HTTP — so they always
// execute and never SKIP.

func t371PrivateRoom() *models.Room { return &models.Room{Slug: "demo-room", IsPrivate: true} }
func t371PublicRoom() *models.Room  { return &models.Room{Slug: "demo-room", IsPrivate: false} }
func t371FirstMsg() *models.Message {
	return &models.Message{AgentName: "planner-1", Content: "build X"}
}

// A joining agent (executor or any role) entering a PRIVATE room must be told to give
// the owner its public agent id and must NOT be told it will be handed a room token.
func TestConnectPrivateAdmission_JoinerPromptsGivePublicAgentIdNotSharedToken(t *testing.T) {
	joiners := map[string]string{
		"executor": executorPromptText(t371PrivateRoom(), t371FirstMsg()),
		"role":     roleSpecificPromptText(t371PrivateRoom(), t371FirstMsg(), "reviewer"),
	}
	for name, prompt := range joiners {
		lower := strings.ToLower(prompt)
		for _, banned := range []string{
			"share the room token directly",
			"receive the room token directly from the room owner",
			"the planner will admit you and share the room token",
		} {
			if strings.Contains(lower, banned) {
				t.Errorf("%s prompt still uses shared-token wording %q", name, banned)
			}
		}
		if !strings.Contains(lower, "public agent id") {
			t.Errorf("%s prompt does not tell the joiner to give its public agent id to the owner", name)
		}
		if !strings.Contains(prompt, "/v1/rooms/"+t371PrivateRoom().Slug+"/members") {
			t.Errorf("%s prompt does not reference the members admission endpoint", name)
		}
		if !strings.Contains(lower, "admit") {
			t.Errorf("%s prompt does not mention admission", name)
		}
		// Still takes its OWN room token by handshake (never a shared one).
		if !strings.Contains(prompt, "your OWN per-agent room token") &&
			!strings.Contains(prompt, "your OWN room token") {
			t.Errorf("%s prompt lost the own-token handshake guidance", name)
		}
	}
}

// A joining agent entering a PUBLIC room needs no admission — any registered agent may
// handshake — so the members endpoint must not appear.
func TestConnectPrivateAdmission_PublicJoinerPromptsHaveNoAdmissionStep(t *testing.T) {
	joiners := map[string]string{
		"executor": executorPromptText(t371PublicRoom(), t371FirstMsg()),
		"role":     roleSpecificPromptText(t371PublicRoom(), t371FirstMsg(), "reviewer"),
	}
	for name, prompt := range joiners {
		if strings.Contains(prompt, "/members") {
			t.Errorf("%s public prompt should not reference the members admission endpoint", name)
		}
	}
}

// The room owner (planner / starter / builder) must be told to admit each joining agent
// into a PRIVATE room by its public agent id via the members endpoint — not by sharing
// its own key or a room token.
func TestConnectPrivateAdmission_OwnerPromptsAdmitJoinersById(t *testing.T) {
	owners := map[string]string{
		"planner": plannerPromptText(ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPrivate, Task: "build X"}),
		"starter": starterPromptText(ConnectSelection{Preset: ConnectPresetCollaborate, Visibility: ConnectVisibilityPrivate, Task: "build X"}),
		"builder": builderPromptText(ConnectSelection{Preset: ConnectPresetBuildAndReview, Visibility: ConnectVisibilityPrivate, Task: "build X"}),
	}
	for name, prompt := range owners {
		lower := strings.ToLower(prompt)
		if !strings.Contains(prompt, "/v1/rooms/"+connectSlugPlaceholder+"/members") {
			t.Errorf("%s owner prompt does not tell the owner to admit agents via the members endpoint", name)
		}
		if !strings.Contains(lower, "admit") {
			t.Errorf("%s owner prompt does not mention admitting joining agents", name)
		}
		if !strings.Contains(prompt, "THEIR_PUBLIC_AGENT_ID") {
			t.Errorf("%s owner prompt does not admit agents by their public agent id", name)
		}
	}
}

// A PUBLIC room needs no owner admission step.
func TestConnectPrivateAdmission_PublicOwnerPromptsHaveNoAdmissionStep(t *testing.T) {
	owners := map[string]string{
		"planner": plannerPromptText(ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic, Task: "build X"}),
		"starter": starterPromptText(ConnectSelection{Preset: ConnectPresetCollaborate, Visibility: ConnectVisibilityPublic, Task: "build X"}),
		"builder": builderPromptText(ConnectSelection{Preset: ConnectPresetBuildAndReview, Visibility: ConnectVisibilityPublic, Task: "build X"}),
	}
	for name, prompt := range owners {
		if strings.Contains(prompt, "/members") {
			t.Errorf("%s public owner prompt should not reference the members admission endpoint", name)
		}
	}
}
