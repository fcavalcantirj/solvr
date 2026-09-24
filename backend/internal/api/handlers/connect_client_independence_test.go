package handlers

import (
	"strings"
	"testing"
)

// These tests pin the ledger task "Demonstrate client independence with tested
// connection instructions". They are PURE (no DB, no HTTP) so they always run
// and never SKIP.
//
// The end-to-end demonstration with real clients (Claude Code, OpenClaw, Kimi
// Code, and a mixed-client pair) is an owner acceptance step that needs those
// clients installed against a deployment. What is verifiable here — and what
// these tests pin — are the API-owned guarantees that keep the flow independent
// of any one agent product no matter which client runs it.

func connectContract() ConnectStart {
	sel := ConnectSelection{Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}
	return buildConnectStart(sel, ConnectExample{Kind: "directory", URL: connectRoomsURL})
}

// Step 3: the only capability required is outbound HTTPS; no provider-specific
// SDK, model-vendor subscription, or shared filesystem is needed.
func TestConnectClientIndependence_RequiresOnlyHTTPS(t *testing.T) {
	req := connectContract().Requirements
	if !strings.Contains(strings.ToLower(req.Detail), "https") {
		t.Errorf("requirements.detail does not state the flow needs outbound HTTPS: %q", req.Detail)
	}
	joined := strings.ToLower(strings.Join(req.NotNeeded, " | "))
	for _, want := range []string{"sdk", "vendor", "filesystem", "account"} {
		if !strings.Contains(joined, want) {
			t.Errorf("requirements.not_needed does not disclaim %q: %v", want, req.NotNeeded)
		}
	}
}

// Step 6: client names are optional examples, never a required onboarding
// choice. The two required choices (preset and visibility) describe the room,
// not a client vendor.
func TestConnectClientIndependence_ClientNamesAreOptionalExamples(t *testing.T) {
	start := connectContract()
	if len(start.Requirements.ClientExamples) == 0 {
		t.Fatal("requirements.client_examples is empty; client examples must be offered")
	}
	clients := []string{"claude", "openclaw", "kimi"}
	for _, opt := range append(append([]ConnectOption{}, start.Presets...), start.VisibilityOptions...) {
		lower := strings.ToLower(opt.Value + " " + opt.Label + " " + opt.Description)
		for _, c := range clients {
			if strings.Contains(lower, c) {
				t.Errorf("required choice %q forces client product %q; client names must be optional examples, not onboarding choices", opt.Value, c)
			}
		}
	}
}

// Step 4: no client is presented as tested/verified-compatible. Unverified
// clients are examples, and the contract explicitly declines a compatibility
// badge claim rather than implying every named client was tested.
func TestConnectClientIndependence_NoTestedCompatibilityBadge(t *testing.T) {
	req := connectContract().Requirements
	note := strings.ToLower(req.ClientExamplesNote)
	if !strings.Contains(note, "not") || !strings.Contains(note, "tested compatibility") {
		t.Errorf("requirements.client_examples_note does not decline tested-compatibility claims: %q", req.ClientExamplesNote)
	}
}

// Step 5: an agent that cannot make HTTPS requests must report the missing
// capability and follow a help link, and must never claim it connected.
func TestConnectClientIndependence_MissingHTTPPointsToHelp(t *testing.T) {
	req := connectContract().Requirements
	lower := strings.ToLower(req.MissingCapability)
	if !strings.Contains(lower, "cannot make") && !strings.Contains(lower, "missing") {
		t.Errorf("requirements.missing_capability does not describe a missing HTTPS capability: %q", req.MissingCapability)
	}
	if !strings.Contains(lower, "never claim") {
		t.Errorf("requirements.missing_capability does not forbid falsely claiming a connection: %q", req.MissingCapability)
	}
	if req.HelpURL == "" || req.HelpLabel == "" {
		t.Errorf("requirements must carry a help link: url=%q label=%q", req.HelpURL, req.HelpLabel)
	}
}
