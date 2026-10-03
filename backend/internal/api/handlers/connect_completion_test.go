package handlers

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Completion responses carry a clean room viewing link and only OFFER a shareable excerpt
// (idx 88 step 3): the agent hands the human the plain room page — no token, no query
// string, no flow id — and never posts anything to an external network itself.

var roomPageLink = regexp.MustCompile(`https://solvr\.dev/rooms/[A-Za-z0-9_-]+([^\s,)"]*)`)

func allPromptTexts() map[string]string {
	room := &models.Room{Slug: "port-billing-20260923", IsPrivate: false}
	first := &models.Message{AgentName: "planner_bot", Content: "port the billing job"}
	out := map[string]string{
		"executor":      executorPromptText(room, first),
		"role/reviewer": roleSpecificPromptText(room, first, "reviewer"),
	}
	for _, tc := range connectPromptCases("ship it") {
		sel := tc.sel
		sel.FlowID = "f_0123456789abcdef01234567"
		out["connect/"+tc.name] = buildConnectPrompt(sel).Text
	}
	return out
}

func TestConnectPrompts_CompletionReportCarriesTheCleanRoomLinkAndNeverAutoPosts(t *testing.T) {
	for name, text := range allPromptTexts() {
		t.Run(name, func(t *testing.T) {
			idx := strings.Index(text, "WHEN THE WORK IS DONE")
			require.GreaterOrEqual(t, idx, 0, "every prompt says how to report completion")
			section := text[idx:]
			if strings.HasPrefix(name, "connect/") {
				require.Contains(t, section, "https://solvr.dev/rooms/"+connectSlugPlaceholder)
			} else {
				require.Contains(t, section, "https://solvr.dev/rooms/port-billing-20260923")
			}
			require.Contains(t, section, "no token and no query string")
			require.Contains(t, section, "never post it anywhere yourself")
		})
	}
}

func TestConnectPrompts_EveryRoomPageLinkIsClean(t *testing.T) {
	for name, text := range allPromptTexts() {
		t.Run(name, func(t *testing.T) {
			links := roomPageLink.FindAllStringSubmatch(text, -1)
			require.NotEmpty(t, links)
			for _, l := range links {
				require.False(t, strings.HasPrefix(l[1], "?"), "room page link carries a query: %s", l[0])
				require.NotContains(t, l[0], "token")
				require.NotContains(t, l[0], "f_0123456789abcdef01234567")
			}
		})
	}
}
