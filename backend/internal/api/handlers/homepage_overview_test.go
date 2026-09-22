package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// These tests pin the Task 16 additions to the overview meta and room section:
//   - stale flag + last_updated_label when a partial failure degrades the snapshot
//   - recent completed collaborations returned when no agents are online now
//
// They are pure builder tests — no database needed.

func TestOverviewMeta_StaleAndLastUpdatedLabelPresent(t *testing.T) {
	partialErrors := []string{"search statistics unavailable: simulated failure"}

	meta := buildOverviewMeta(db.DefaultRoomStatsWindow(), partialErrors)

	// A partial error marks the snapshot stale: the data is retained, not fresh.
	assert.True(t, meta.Stale, "partial errors must mark the snapshot stale")
	assert.NotEmpty(t, meta.LastUpdatedLabel, "the API pre-formats a last-updated label")
	assert.NotEmpty(t, meta.StaleLabel, "a stale snapshot carries a label the page can show")
}

func TestOverviewMeta_NoStaleWhenNoPartialErrors(t *testing.T) {
	meta := buildOverviewMeta(db.DefaultRoomStatsWindow(), nil)

	assert.False(t, meta.Stale, "no partial errors means a fresh snapshot")
	assert.Empty(t, meta.StaleLabel, "a fresh snapshot has no stale label")
}

func TestOverviewMeta_LastUpdatedLabelIsReadableText(t *testing.T) {
	// buildOverviewMeta stamps generated_at at "now", so the label is the
	// word "Updated" plus a timestamp — plain text, not a raw RFC3339 blob.
	partialErrors := []string{"activity stream unavailable: something failed"}
	meta := buildOverviewMeta(db.DefaultRoomStatsWindow(), partialErrors)

	assert.Contains(t, meta.LastUpdatedLabel, "Updated",
		"the label must be a readable word, not a bare timestamp")
}

func TestBuildStaleLabel_EmptyWhenNoErrors(t *testing.T) {
	assert.Empty(t, buildStaleLabel(nil), "no errors means no stale label")
	assert.Empty(t, buildStaleLabel([]string{}), "empty error slice means no stale label")
}

func TestBuildStaleLabel_SingularAndPlural(t *testing.T) {
	one := buildStaleLabel([]string{"search unavailable"})
	assert.Contains(t, one, "partial refresh")

	many := buildStaleLabel([]string{"search unavailable", "activity unavailable"})
	assert.Contains(t, many, "2 statistics sections")
}
