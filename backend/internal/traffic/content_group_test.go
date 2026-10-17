package traffic

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contract/content-groups.json is the one table of what kind of page each path is. The web
// client's contentGroupForPath (frontend/lib/analytics-paths.ts) answers every case of it in
// its own test, so a page view counted here and a hit Google sees use the same rule.
func TestContentGroupForPath_AnswersTheSharedFixture(t *testing.T) {
	raw, err := os.ReadFile("../../../contract/content-groups.json")
	require.NoError(t, err)
	var fixture struct {
		Cases []struct {
			Path  string `json:"path"`
			Group string `json:"group"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.Greater(t, len(fixture.Cases), 50)

	seen := map[string]bool{}
	for _, c := range fixture.Cases {
		assert.Equal(t, c.Group, ContentGroupForPath(c.Path), "%q", c.Path)
		seen[c.Group] = true
	}
	for _, group := range ContentGroups {
		assert.True(t, seen[group], "the fixture has no case for %s", group)
	}
}

// The thirteen groups are 000146's page_view_events_content_group_check, in its order.
func TestContentGroups_AreTheClosedList(t *testing.T) {
	assert.Equal(t, []string{"home", "connect", "post", "room", "transcript", "agent", "user", "blog", "docs",
		"guide", "collection", "account", "other"}, ContentGroups)
}
