package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// The acquisition model reads observed monthly flows (spec.json idx 90): for one calendar month,
// each population's active identities split EXACTLY into retained (also active last month), new
// (first qualifying action ever) and reactivated (back after a gap), so no identity is counted
// in two buckets; plus the monthly cohorts whose survival replaces the single retention term.
// Activity uses the same qualifying-action definition as the participant counter.

func TestAcquisitionModel_MonthlyFlowsPartitionEveryIdentityOnce(t *testing.T) {
	pool, dropLegacy := newMigratedScratchDatabase(t)
	dropLegacy()
	ctx := context.Background()
	f := &participantFixture{t: t, ctx: ctx, pool: pool, n: time.Now().UnixNano() % 1_000_000_000}

	june := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	july := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	august := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	search := func(kind, id string, at time.Time) { f.search(kind, id, "Mozilla/5.0", at) }

	h1, h2, h3, h4 := f.user("h1"), f.user("h2"), f.user("h3"), f.user("h4")
	search("human", h1, june)
	search("human", h1, july)
	search("human", h1, august) // retained
	search("human", h2, august) // new
	search("human", h3, june)
	search("human", h3, august) // reactivated
	search("human", h4, july)   // last month only

	a1, a2 := f.agent("a1"), f.agent("a2")
	search("agent", a1, july)
	search("agent", a1, august) // retained
	search("agent", a2, august) // new, claimed by h2 who is also active: known overlap
	search("agent", a2, august)
	f.exec(`UPDATE agents SET human_id = $1, human_claimed_at = NOW() WHERE id = $2`, h2, a2)

	got, err := NewAcquisitionModelRepository(pool).MonthlyFlows(ctx, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	assert.Equal(t, "2026-08", got.Month)
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), got.End)

	hu := got.Humans
	assert.Equal(t, 3, hu.Active)
	assert.Equal(t, 1, hu.Retained)
	assert.Equal(t, 1, hu.New)
	assert.Equal(t, 1, hu.Reactivated)
	assert.Equal(t, hu.Active, hu.Retained+hu.New+hu.Reactivated, "every active human is in exactly one bucket")
	assert.Equal(t, 2, hu.PreviousActive, "h1 and h4 were active in July")
	require.NotNil(t, hu.MeasuredRetention)
	assert.InDelta(t, 0.5, *hu.MeasuredRetention, 1e-9)
	assert.Equal(t, []growth.CohortObservation{
		{CohortMonth: "2026-06", ActiveByAge: []int{2, 1, 2}},
		{CohortMonth: "2026-07", ActiveByAge: []int{1, 0}},
		{CohortMonth: "2026-08", ActiveByAge: []int{1}},
	}, hu.Cohorts)

	ag := got.Agents
	assert.Equal(t, 2, ag.Active)
	assert.Equal(t, 1, ag.Retained)
	assert.Equal(t, 1, ag.New)
	assert.Equal(t, 0, ag.Reactivated)
	assert.Equal(t, 1, ag.PreviousActive)
	require.NotNil(t, ag.MeasuredRetention)
	assert.InDelta(t, 1.0, *ag.MeasuredRetention, 1e-9)

	assert.Equal(t, 1, got.KnownOverlap, "a2 belongs to h2, who is active the same month")
}
