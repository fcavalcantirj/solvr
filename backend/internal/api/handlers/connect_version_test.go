package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task 25: the connection contract is VERSIONED and served from ONE source of
// truth. Both the public bootstrap endpoint (GET /v1/connect) and the
// room-specific endpoint (GET /v1/rooms/{slug}/connect) must report the same
// instruction version, so a client can tell which contract it is following and a
// change to the connection contract forces a single version bump.
//
// These tests are pure (no database, no HTTP): they exercise the builders that
// GetConnect and GetRoomConnect return directly, so they always execute and
// never SKIP.

func TestConnect_InstructionVersionConstantIsNonEmpty(t *testing.T) {
	if strings.TrimSpace(ConnectInstructionVersion) == "" {
		t.Fatal("ConnectInstructionVersion must be a non-empty version string")
	}
}

func TestConnect_StartCarriesInstructionVersion(t *testing.T) {
	sel := ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}
	start := buildConnectStart(sel, ConnectExample{})

	if start.InstructionVersion != ConnectInstructionVersion {
		t.Fatalf("GET /v1/connect must carry instruction_version %q, got %q",
			ConnectInstructionVersion, start.InstructionVersion)
	}
}

func TestConnect_BothEndpointsShareOneInstructionVersion(t *testing.T) {
	sel := ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}
	start := buildConnectStart(sel, ConnectExample{})

	room := &models.Room{Slug: "shared-version-room", IsPrivate: false}
	env := buildRoomConnectEnvelope(room, nil, "executor")

	if start.InstructionVersion == "" {
		t.Fatal("public connect endpoint reported an empty instruction version")
	}
	if start.InstructionVersion != env.InstructionVersion {
		t.Fatalf("public and room-specific connect endpoints must report the SAME instruction version: /v1/connect=%q room=%q",
			start.InstructionVersion, env.InstructionVersion)
	}
}

// Step 2 for the public bootstrap endpoint: the response must include absolute
// production API URLs, the authentication sequence, the available roles, and
// concrete next actions. (Room visibility for /v1/connect is carried by the
// visibility_options; the room-specific test below covers per-room visibility.)
func TestConnect_StartExposesAbsoluteURLsAuthSequenceRolesAndNextActions(t *testing.T) {
	sel := ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}
	start := buildConnectStart(sel, ConnectExample{})

	// Absolute production API URL — the agent that reads the prompt is not the
	// browser that copied it, so a relative path would be useless to it.
	if !strings.Contains(start.Prompt.Text, "https://api.solvr.dev") {
		t.Error("planner prompt must carry the absolute production API URL https://api.solvr.dev")
	}

	// Authentication sequence: register an identity, send it as a bearer token,
	// then handshake for a per-agent room token.
	for _, want := range []string{"/v1/agents/register", "Authorization: Bearer", "/handshake"} {
		if !strings.Contains(start.Prompt.Text, want) {
			t.Errorf("planner prompt must describe the authentication sequence step %q", want)
		}
	}

	// Available roles: the presets are the collaboration shapes, and Add-another-agent
	// carries a role prompt for a third/Nth participant.
	if len(start.Presets) != 3 {
		t.Errorf("connect must expose the three available presets/roles, got %d", len(start.Presets))
	}
	if strings.TrimSpace(start.AddAgent.RolePrompt) == "" {
		t.Error("connect must expose an add-another-agent role prompt")
	}

	// Concrete next actions: the two ordered copy/paste steps.
	if len(start.Steps) != 2 {
		t.Errorf("connect must expose the two ordered next-action steps, got %d", len(start.Steps))
	}
}

// Step 2 (room visibility, roles) and step 4 (no leaked secrets) for the
// room-specific endpoint, exercised at the builder level.
func TestConnect_RoomEnvelopeCarriesVersionVisibilityRoleAndNoSecrets(t *testing.T) {
	room := &models.Room{Slug: "task25-room", IsPrivate: false}
	firstMsg := &models.Message{AgentName: "planner-x", Content: "Task: build it. Directive: start."}
	env := buildRoomConnectEnvelope(room, firstMsg, "reviewer")

	if env.InstructionVersion != ConnectInstructionVersion {
		t.Errorf("room envelope instruction_version = %q, want %q", env.InstructionVersion, ConnectInstructionVersion)
	}
	if env.RoomURL != "https://solvr.dev/rooms/task25-room" {
		t.Errorf("room URL must be the absolute production URL, got %q", env.RoomURL)
	}
	if env.Role != "reviewer" {
		t.Errorf("room envelope must carry the requested role, got %q", env.Role)
	}
	// Room visibility is reported explicitly.
	if env.Private {
		t.Error("a public room must report private=false")
	}

	// No owner credentials, private invitation secrets, or individual room tokens
	// may appear in the room-specific public prompt.
	lower := strings.ToLower(env.Prompt)
	for _, secret := range []string{"solvr_sk_", "solvr_rt_", "solvr_rm_", "room_token", "api_key", "token_hash", "owner_id"} {
		if strings.Contains(lower, strings.ToLower(secret)) {
			t.Errorf("room connect prompt leaked %q", secret)
		}
	}
}

func TestConnect_RoomEnvelopePrivateRoomReportsPrivateVisibility(t *testing.T) {
	room := &models.Room{Slug: "task25-private", IsPrivate: true}
	env := buildRoomConnectEnvelope(room, nil, "executor")

	if !env.Private {
		t.Error("a private room must report private=true")
	}
	if env.InstructionVersion != ConnectInstructionVersion {
		t.Errorf("room envelope instruction_version = %q, want %q", env.InstructionVersion, ConnectInstructionVersion)
	}
}
