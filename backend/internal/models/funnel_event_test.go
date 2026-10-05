package models

import (
	"reflect"
	"strings"
	"testing"
)

// skill_fetched is reported by the web server through the public ingest: a client may
// report it, but it is neither a browser step nor a confirmed server action.
func TestFunnelVocabulary_SkillFetchedIsReportedByTheWebServer(t *testing.T) {
	if !ValidFunnelEventName(FunnelSkillFetched) {
		t.Fatal("skill_fetched is not a recognized funnel step")
	}
	if !IsWebServerFunnelEvent(FunnelSkillFetched) {
		t.Error("skill_fetched is a web-server step")
	}
	if IsBrowserFunnelEvent(FunnelSkillFetched) || IsServerFunnelEvent(FunnelSkillFetched) {
		t.Error("skill_fetched is neither a browser step nor a server step")
	}
	if !IsClientReportedFunnelEvent(FunnelSkillFetched) {
		t.Error("the web server reports skill_fetched through the public ingest")
	}
}

// A skill fetch is one of three things, and the contract a reader is served says how the
// API tells them apart: a person's browser, a link preview or crawler, or an agent.
func TestFunnelVocabulary_ASkillFetchHasThreeSurfaces(t *testing.T) {
	surfaces := map[string]string{
		"agent_fetch":   FunnelSurfaceAgentFetch,
		"bot_fetch":     FunnelSurfaceBotFetch,
		"browser_visit": FunnelSurfaceBrowserVisit,
	}
	var description string
	for _, spec := range FunnelEventContract() {
		if spec.Name == FunnelSkillFetched {
			description = spec.Description
		}
	}
	for want, got := range surfaces {
		if got != want {
			t.Errorf("surface %q is spelled %q", want, got)
		}
		if !strings.Contains(description, want) {
			t.Errorf("the contract of skill_fetched does not name %s: %s", want, description)
		}
	}
	for _, word := range []string{"request_mode", "user_agent", "never stored"} {
		if !strings.Contains(description, word) {
			t.Errorf("the contract of skill_fetched does not say %q: %s", word, description)
		}
	}
}

// What a client may report is every browser step plus the web server's; never a step the
// API records from its own confirmed actions.
func TestFunnelVocabulary_ClientReportedSteps(t *testing.T) {
	want := []string{
		FunnelConnectionStarted, FunnelStarterPromptCopied, FunnelSkillFetched,
		FunnelRoomViewed, FunnelJoinPromptCopied, FunnelShareVisit, FunnelShareLinkCopied,
	}
	if got := ClientReportedFunnelEvents(); !reflect.DeepEqual(got, want) {
		t.Fatalf("ClientReportedFunnelEvents() = %v, want %v", got, want)
	}
	for _, name := range want {
		if !IsClientReportedFunnelEvent(name) {
			t.Errorf("%s must be reportable by a client", name)
		}
	}
	for _, name := range []string{FunnelRoomCreated, FunnelParticipantJoined, FunnelFirstTwoWayExchange, "", "not_a_step"} {
		if IsClientReportedFunnelEvent(name) {
			t.Errorf("%q must never be reportable by a client", name)
		}
	}
}

// The contract lists every step exactly once, each on the channel that records it, and
// its vocabulary is the same one the classification helpers use.
func TestFunnelEventContract_ListsEveryStepOnItsChannel(t *testing.T) {
	channels := map[string]string{}
	for _, spec := range FunnelEventContract() {
		if _, dup := channels[spec.Name]; dup {
			t.Errorf("the contract lists %s twice", spec.Name)
		}
		channels[spec.Name] = spec.SourceChannel
		if spec.Description == "" || len(spec.Attributes) == 0 {
			t.Errorf("%s is listed without a description or attributes", spec.Name)
		}
		if !ValidFunnelEventName(spec.Name) {
			t.Errorf("the contract lists %s, which is not a recognized step", spec.Name)
		}
		var want string
		switch {
		case IsBrowserFunnelEvent(spec.Name):
			want = FunnelSourceBrowser
		case IsWebServerFunnelEvent(spec.Name):
			want = FunnelSourceWebServer
		case IsServerFunnelEvent(spec.Name):
			want = FunnelSourceServer
		}
		if spec.SourceChannel != want {
			t.Errorf("%s is listed on channel %q, want %q", spec.Name, spec.SourceChannel, want)
		}
	}
	if len(channels) != 10 {
		t.Fatalf("the contract lists %d steps, want 10", len(channels))
	}
	if channels[FunnelSkillFetched] != "web_server" {
		t.Errorf("skill_fetched is on channel %q, want web_server", channels[FunnelSkillFetched])
	}
	skill := FunnelEventSpec{}
	for _, spec := range FunnelEventContract() {
		if spec.Name == FunnelSkillFetched {
			skill = spec
		}
	}
	if !reflect.DeepEqual(skill.Attributes, []string{"flow_id", "entry_surface"}) {
		t.Errorf("skill_fetched attributes = %v, want [flow_id entry_surface]", skill.Attributes)
	}
}
