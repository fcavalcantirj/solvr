package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operations gates of spec.json idx 79 are documented where an operator looks first
// (docs/ops), and each document carries what keeps it honest: LAPTOP numbers labelled as
// such, planning inputs labelled hypothetical, prices left to the owner, and the runbook's
// links to the endpoint, the drills and the rollback.

var opsDocAnchors = map[string][]string{
	"docs/ops/README.md":        {"/admin/ops/slo", "UAT", "not_yet_measurable", "LAPTOP"},
	"docs/ops/load-test.md":     {"LAPTOP", "not production capacity", "Knee", "run-load-test.sh", "concurrency", "load average"},
	"docs/ops/capacity-plan.md": {"2,000,000", "hypothetical", "not a forecast", "burst"},
	"docs/ops/cost-model.md":    {"UAT", "per activated room", "egress", "observability", "per retained GB", "per search", "per returning participant"},
	"docs/ops/paid-options.md":  {"no implementation", "ops_billing_free_test.go"},
	"docs/ops/incident-runbook.md": {"/admin/ops/slo", "RUNBOOK-cutover-window.md", "restore-drill.sh", "migration-recovery-drill.sh",
		"ops alarm: webhook delivery queue lag", "/admin/incidents", "UAT"},
}

func TestOpsDocs_ExistAndCarryTheirAnchors(t *testing.T) {
	for rel, anchors := range opsDocAnchors {
		t.Run(rel, func(t *testing.T) {
			doc := repoFile(t, rel)
			for _, want := range anchors {
				assert.Contains(t, doc, want, "%s must mention %q", rel, want)
			}
		})
	}
}

// The recorded mix the load test replays is a distribution over the four request classes.
func TestOpsDocs_TheRecordedMixIsADistribution(t *testing.T) {
	var mix struct {
		Source  string             `json:"source"`
		Classes map[string]float64 `json:"classes"`
		Streams int                `json:"streams"`
	}
	require.NoError(t, json.Unmarshal([]byte(repoFile(t, "docs/ops/load-mix.json")), &mix))
	assert.True(t, strings.HasPrefix(mix.Source, "recorded"), "the mix says where it was recorded")
	sum := 0.0
	for _, class := range []string{"overview_read", "search", "agent_poll", "timeline_write"} {
		require.Contains(t, mix.Classes, class)
		sum += mix.Classes[class]
	}
	assert.InDelta(t, 1.0, sum, 0.001)
	assert.Positive(t, mix.Streams)
}
