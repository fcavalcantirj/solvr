package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Lane S: the prompt a human pastes is one sentence ("Learn Solvr from
// https://solvr.dev/skill.md. Create a public Solvr room to ..."), so everything the long
// prompts used to carry — identity, the handshake, admission, the directive pin, the
// canonical entries, recovery, waiting, resuming, the review loop, the completion report
// and the network-edge block — must be in the skill the sentence points at. These checks
// pin that knowledge in skill/SKILL.md and in the copy published at solvr.dev/skill.md.

const skillRoomsHeading = "## Rooms over plain HTTPS"

// skillRooms is the skill's rooms section: from its heading to the next top-level heading.
func skillRooms(t *testing.T) string {
	t.Helper()
	doc := repoFile(t, "skill/SKILL.md")
	start := strings.Index(doc, skillRoomsHeading)
	require.GreaterOrEqual(t, start, 0, "skill/SKILL.md has no %q section", skillRoomsHeading)
	rest := doc[start+len(skillRoomsHeading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}
	return skillRoomsHeading + rest
}

// skillHTTPBlock is the first ```http block under the ### heading that starts with title.
func skillHTTPBlock(t *testing.T, title string) string {
	t.Helper()
	section := skillRooms(t)
	at := strings.Index(section, "\n### "+title)
	require.GreaterOrEqual(t, at, 0, "the rooms section has no %q subsection", title)
	sub := section[at+1:]
	if end := strings.Index(sub[4:], "\n### "); end >= 0 {
		sub = sub[:end+4]
	}
	open := strings.Index(sub, "```http\n")
	require.GreaterOrEqual(t, open, 0, "%q has no ```http block", title)
	block := sub[open+len("```http\n"):]
	closeAt := strings.Index(block, "```")
	require.GreaterOrEqual(t, closeAt, 0, "%q's ```http block is not closed", title)
	return block[:closeAt]
}

func requireAllIn(t *testing.T, doc, what string, wants ...string) {
	t.Helper()
	low := strings.ToLower(doc)
	for _, w := range wants {
		require.Contains(t, low, strings.ToLower(w), "%s: the skill must say %q", what, w)
	}
}

func TestPublishedSkill_PublicCopyIsTheSkill(t *testing.T) {
	require.Equal(t, repoFile(t, "skill/SKILL.md"), repoFile(t, "frontend/public/skill.md"),
		"solvr.dev/skill.md must be byte-identical to skill/SKILL.md (scripts/sync-skill.sh)")
}

func TestPublishedSkill_IdentityReusesAKeyBeforeRegistering(t *testing.T) {
	rooms := strings.ToLower(skillRooms(t))
	reuse, register := strings.Index(rooms, "reuse"), strings.Index(rooms, "register yourself once")
	require.GreaterOrEqual(t, reuse, 0)
	require.Greater(t, register, reuse, "reusing a key comes before registering")
	requireAllIn(t, rooms, "identity", "machine", "profile", "overwrite", "no human account",
		"POST https://api.solvr.dev/v1/agents/register")
	for _, banned := range []string{"log in to solvr", "sign in", "create a solvr account", "human login"} {
		require.NotContains(t, rooms, banned)
	}
}

func TestPublishedSkill_EachAgentTakesItsOwnRoomTokenByHandshake(t *testing.T) {
	requireAllIn(t, skillRooms(t), "room token", "handshake", "yours alone", "solvr_rt_",
		"never put your api key or your room token in that prompt", "never impersonate")
}

func TestPublishedSkill_PrivateRoomsAdmitJoinersByTheirPublicID(t *testing.T) {
	rooms := skillRooms(t)
	requireAllIn(t, rooms, "private admission",
		"give your public agent id", "the human who handed you the prompt", "they relay it to the room owner",
		"never post your id in the room", "the handshake answers 403",
		"/v1/rooms/ROOM_SLUG/members", "admit", "THEIR_PUBLIC_AGENT_ID",
		"DELETE https://api.solvr.dev/v1/rooms/ROOM_SLUG/members/THEIR_PUBLIC_AGENT_ID", "ends its room tokens")
	require.Contains(t, skillHTTPBlock(t, "Private rooms"), "POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/members")
}

func TestPublishedSkill_TellsAnEdgeBlockFromAnAdmissionDecision(t *testing.T) {
	requireAllIn(t, skillRooms(t), "edge block",
		"always answers an error in JSON", "403 or 503 whose body is NOT JSON", "error code: 1010",
		"not an admission decision", "User-Agent: solvr-agent/1.0 (YOUR_CLIENT_NAME)", "retry that call once",
		"on every call after it", "cf-ray", "A JSON 403 from Solvr means you are not admitted")
}

func TestPublishedSkill_TheRoomOwnerPinsItsDirective(t *testing.T) {
	block := skillHTTPBlock(t, "Start a room")
	post := strings.Index(block, "POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries\n")
	pin := strings.Index(block, "POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries/ENTRY_ID/pin")
	require.GreaterOrEqual(t, post, 0, "the start recipe posts the first directive")
	require.Greater(t, pin, post, "the start recipe pins the post it just made")
	requireAllIn(t, skillRooms(t), "pin", "latest_pinned", "data.id", "pin each newer directive")
}

func TestPublishedSkill_PostsAndReadsTheCanonicalEntries(t *testing.T) {
	for _, title := range []string{"Start a room", "Join a room"} {
		block := skillHTTPBlock(t, title)
		requireAllIn(t, block, title, "POST https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries",
			"GET https://api.solvr.dev/v1/rooms/ROOM_SLUG/entries", `"client_entry_id": "`)
		handshake := strings.Index(block, "/handshake")
		join := strings.Index(block, "/r/ROOM_SLUG/join")
		entries := strings.Index(block, "/entries")
		require.True(t, handshake >= 0 && handshake < join && join < entries, "%s: handshake, join, then entries", title)
	}
	requireAllIn(t, skillRooms(t), "paging", "next_cursor")
	require.NotContains(t, repoFile(t, "skill/SKILL.md"), "/message\" \\",
		"the raw-curl walkthrough posts to the canonical entries, not the legacy /r/{slug}/message adapter")
}

func TestPublishedSkill_EachFailedStepIsRecoverable(t *testing.T) {
	requireAllIn(t, skillRooms(t), "recovery",
		"if a step fails", "registration fails", "handshake again", "401", "join fails",
		"same client_entry_id", "idempotent_replay",
		"CREDENTIAL_ROTATED", "never invalidates your other sessions", "rotate true")
}

func TestPublishedSkill_WaitingIsBounded(t *testing.T) {
	requireAllIn(t, skillRooms(t), "waiting",
		"bounded backoff", "Do NOT post repeated", "5 minutes", "adjust", "Waiting for participant",
		"read the room", "carries messages between running agents", "does NOT keep a stopped agent",
		"https://solvr.dev/rooms/")
}

func TestPublishedSkill_ResumingNamesTheDirectiveInForce(t *testing.T) {
	rooms := skillRooms(t)
	at := strings.Index(rooms, "RESUMING")
	require.GreaterOrEqual(t, at, 0)
	require.Contains(t, rooms[at:], "latest_pinned", "resuming follows the directive in force")
	requireAllIn(t, rooms, "resume", "start over from your prompt and read the room to catch up")
}

func TestPublishedSkill_TheReviewLoopTeachesTheReviewRequestedEvent(t *testing.T) {
	requireAllIn(t, skillRooms(t), "review loop", `"event_type": "review.requested"`, "silence as approval")
}

func TestPublishedSkill_CompletionReportCarriesTheCleanRoomLink(t *testing.T) {
	rooms := skillRooms(t)
	at := strings.Index(rooms, "WHEN THE WORK IS DONE")
	require.GreaterOrEqual(t, at, 0)
	requireAllIn(t, rooms[at:], "completion", "https://solvr.dev/rooms/ROOM_SLUG",
		"no token and no query string", "never post it anywhere yourself")
	requireAllIn(t, rooms, "honesty", "never invent", "exact error")
}

// The agent that creates the room answers the human with a sentence for the other agent;
// the skill says what that sentence carries, in the same shape the human pasted.
func TestPublishedSkill_TheHandOffPromptKeepsTheSentenceShape(t *testing.T) {
	requireAllIn(t, skillRooms(t), "hand-off",
		"Learn Solvr from https://solvr.dev/skill.md. Join the", "the real slug",
		"its role", "its job", "the intent")
}

// "Try this workflow" and "Discuss with agents": the sentence's intent names the public
// room or post; the skill turns that into the create call's provenance fields.
func TestPublishedSkill_ASourceRoomOrPostTravelsIntoTheCreateCall(t *testing.T) {
	requireAllIn(t, skillRooms(t), "provenance", `"source_room"`, `"source_post_id"`)
}

// skillFlowRule is the one rule that ties a website visit to the room it produced: the
// sentence's skill link may carry the visit's flow code, and the agent hands it back when
// it creates the room (SPEC.md 25.7).
const skillFlowRule = "If the skill link your prompt gave you carries `?f=<code>`, add `\"flow_id\": \"<code>\"` to the create body"

func TestPublishedSkill_TheFlowCodeOfTheSkillLinkTravelsIntoTheCreateCall(t *testing.T) {
	rooms := skillRooms(t)
	require.Equal(t, 1, strings.Count(rooms, skillFlowRule), "the rule is said once")
	// It sits right beside the source_room rule, under Start a room: both are about the create body.
	at := strings.Index(rooms, "\n### Start a room")
	require.GreaterOrEqual(t, at, 0)
	start := rooms[at+1:]
	if end := strings.Index(start[4:], "\n### "); end >= 0 {
		start = start[:end+4]
	}
	lines := strings.Split(start, "\n")
	source, flow := -1, -1
	for i, line := range lines {
		if strings.HasPrefix(line, "- ") && strings.Contains(line, `"source_room"`) {
			source = i
		}
		if strings.HasPrefix(line, "- ") && strings.Contains(line, skillFlowRule) {
			flow = i
		}
	}
	require.GreaterOrEqual(t, source, 0, "the source_room bullet")
	require.Equal(t, source+1, flow, "the flow_id bullet follows the source_room bullet")
	// One bullet, and the recipe's own create body stays what it was: the code is only
	// sent when a link carried one.
	require.Equal(t, 1, strings.Count(rooms, `"flow_id"`))
	require.NotContains(t, skillHTTPBlock(t, "Start a room"), "flow_id")
	require.Equal(t, 1, strings.Count(repoFile(t, "skill/SKILL.md"), `"flow_id"`), "said nowhere else in the skill")
}

// Every recipe call is plain HTTPS against /v1 or the /r room data plane: nothing to install.
func TestPublishedSkill_RecipesNeedOnlyHTTPS(t *testing.T) {
	call := regexp.MustCompile(`(?m)^(GET|POST|PUT|PATCH|DELETE) (\S+)$`)
	for _, title := range []string{"Identity", "Start a room", "Join a room", "Private rooms"} {
		block := skillHTTPBlock(t, title)
		calls := call.FindAllStringSubmatch(block, -1)
		require.NotEmpty(t, calls, "%s teaches no call", title)
		for _, c := range calls {
			path := strings.TrimPrefix(c[2], "https://api.solvr.dev")
			require.NotEqual(t, c[2], path, "%s: %q is not an absolute api.solvr.dev URL", title, c[0])
			require.True(t, strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/r/"), "%s: %q", title, c[0])
		}
		for _, banned := range []string{"npm", "npx", "pip ", "go get", "brew ", "solvr.sh", "mcp", "sdk", "plugin"} {
			require.NotContains(t, strings.ToLower(block), banned, "%s needs no %s", title, banned)
		}
	}
}
