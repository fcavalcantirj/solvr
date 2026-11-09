package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// B4, the page-view burst (SEO audit 2026-11-08, Release A). On 2026-11-08, 41,016 of the
// month's 47,504 "human" page views sat in minutes with more than five views: headless
// browsers with real browser headers, which no single request could tell from a person. A
// source key (the day's HMAC of address and user agent, never stored in clear) lets a run
// count one source's views in a minute: more than PageViewBurstThreshold of them are
// automation. Rows from before the key existed carry none; the operator's legacy rule judges
// those by the minute's total, and only on request.
func (f *trafficFixture) view(key *string, class string, at time.Time) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO page_view_events (content_group, traffic_class, traffic_reason, source_key, occurred_at)
		VALUES ('post', $1, CASE WHEN $1 = 'human' THEN 'browser' ELSE 'declared_bot' END, $2, $3)`, class, key, at)
	require.NoError(f.t, err)
}

func (f *trafficFixture) viewLabels(key *string) map[string]int {
	f.t.Helper()
	rows, err := f.pool.Query(f.ctx, `
		SELECT traffic_class || '/' || COALESCE(traffic_reason, ''), count(*)::int FROM page_view_events
		 WHERE source_key IS NOT DISTINCT FROM $1 GROUP BY 1`, key)
	require.NoError(f.t, err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var label string
		var n int
		require.NoError(f.t, rows.Scan(&label, &n))
		out[label] = n
	}
	return out
}

func str(s string) *string { return &s }

func TestTrafficRules_PageViewBurstFlagsOneSourcesMinute(t *testing.T) {
	f := newTrafficFixture(t)
	minute := time.Date(2026, 11, 8, 13, 12, 0, 0, time.UTC)
	burst, calm, crawler := str("aaaaaaaaaaaaaaaa"), str("bbbbbbbbbbbbbbbb"), str("cccccccccccccccc")
	for i := 0; i < PageViewBurstThreshold+1; i++ { // six views in one minute: a burst
		f.view(burst, "human", minute.Add(time.Duration(i)*5*time.Second))
	}
	for i := 0; i < PageViewBurstThreshold; i++ { // five: not a burst
		f.view(calm, "human", minute.Add(time.Duration(i)*5*time.Second))
	}
	f.view(burst, "human", minute.Add(time.Minute)) // the next minute, alone: stays a person's
	for i := 0; i < PageViewBurstThreshold+2; i++ { // a declared crawler is left as it is
		f.view(crawler, "crawler", minute.Add(time.Duration(i)*time.Second))
	}
	f.view(nil, "human", minute) // a legacy row with no key is never judged by B4

	run, err := NewTrafficRules(f.pool).Apply(f.ctx, minute.Add(-time.Hour), minute.Add(2*time.Hour), false)
	require.NoError(t, err)
	assert.Equal(t, PageViewBurstThreshold+1, run.Rules[RulePageViewBurst])
	assert.Equal(t, map[string]int{"automation/page_view_burst": 6, "human/browser": 1}, f.viewLabels(burst))
	assert.Equal(t, map[string]int{"human/browser": 5}, f.viewLabels(calm))
	assert.Equal(t, map[string]int{"crawler/declared_bot": 7}, f.viewLabels(crawler))
	assert.Equal(t, map[string]int{"human/browser": 1}, f.viewLabels(nil))
	assert.NotEmpty(t, run.PageViews.Before)
	assert.NotEmpty(t, run.PageViews.After)

	// A second run is idempotent: the reset undoes B4 and applies it again.
	run, err = NewTrafficRules(f.pool).Apply(f.ctx, minute.Add(-time.Hour), minute.Add(2*time.Hour), false)
	require.NoError(t, err)
	assert.Equal(t, PageViewBurstThreshold+1, run.Rules[RulePageViewBurst])
	assert.Equal(t, map[string]int{"automation/page_view_burst": 6, "human/browser": 1}, f.viewLabels(burst))
}

func TestTrafficRules_LegacyPageViewBurstJudgesKeylessRowsByTheMinute(t *testing.T) {
	f := newTrafficFixture(t)
	minute := time.Date(2026, 10, 26, 13, 12, 0, 0, time.UTC)
	for i := 0; i < 8; i++ { // eight keyless human views in one minute: the month's typical burst
		f.view(nil, "human", minute.Add(time.Duration(i)*7*time.Second))
	}
	for i := 0; i < 3; i++ { // three in the next minute: a person could do that
		f.view(nil, "human", minute.Add(time.Minute+time.Duration(i)*10*time.Second))
	}
	f.view(str("dddddddddddddddd"), "human", minute) // a keyed row in the burst minute is B4's, not this rule's

	rules := NewTrafficRules(f.pool)
	dry, err := rules.ApplyLegacyPageViewBurst(f.ctx, minute.Add(-time.Hour), minute.Add(time.Hour), true)
	require.NoError(t, err)
	assert.Equal(t, 8, dry, "a dry run counts and writes nothing")
	assert.Equal(t, map[string]int{"human/browser": 11}, f.viewLabels(nil))

	n, err := rules.ApplyLegacyPageViewBurst(f.ctx, minute.Add(-time.Hour), minute.Add(time.Hour), false)
	require.NoError(t, err)
	assert.Equal(t, 8, n)
	assert.Equal(t, map[string]int{"automation/page_view_burst_legacy": 8, "human/browser": 3}, f.viewLabels(nil))
	assert.Equal(t, map[string]int{"human/browser": 1}, f.viewLabels(str("dddddddddddddddd")))

	n, err = rules.ApplyLegacyPageViewBurst(f.ctx, minute.Add(-time.Hour), minute.Add(time.Hour), false)
	require.NoError(t, err)
	assert.Equal(t, 0, n, "already judged rows are not counted again")
}
