package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The traffic reports (SPEC.md 27.6): GET /admin/traffic/summary shows who the site's traffic
// came from, and POST /admin/traffic/reclassify runs the behavioural rules over a window the
// operator names, as a dry run first (REVIEW-r1 C4).

type fakeTrafficSummary struct {
	from, to time.Time
}

func (f *fakeTrafficSummary) Summary(_ context.Context, from, to time.Time) (*db.TrafficSummary, error) {
	f.from, f.to = from, to
	return &db.TrafficSummary{
		From: from, To: to,
		PageViews: []db.PageViewCount{{ContentGroup: "room", Class: "human", Count: 3}},
		Funnel:    []db.FunnelTrafficCount{}, Searches: []db.SearchTrafficCount{},
		Requests: []db.UAClassCount{}, Errors: []db.ErrorCodeCount{},
	}, nil
}

type fakeReclassifier struct {
	from, to time.Time
	calls    int
	busy     bool
	dryRun   bool
}

func (f *fakeReclassifier) TryLock(context.Context) (unlock func(), ok bool, err error) {
	if f.busy {
		return nil, false, nil
	}
	return func() {}, true, nil
}

func (f *fakeReclassifier) Apply(_ context.Context, from, to time.Time, dryRun bool) (db.TrafficRun, error) {
	f.calls++
	f.from, f.to, f.dryRun = from, to, dryRun
	return db.TrafficRun{From: from, To: to, DryRun: dryRun, Rules: map[string]int{db.RuleFetchedWithoutCopy: 2}}, nil
}

func TestTrafficSummary_ReadsTheWindowItIsAsked(t *testing.T) {
	summary := &fakeTrafficSummary{}
	h := NewTrafficReportHandler(summary, &fakeReclassifier{})

	w := httptest.NewRecorder()
	h.GetSummary(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/traffic/summary", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.InDelta(t, (7 * 24 * time.Hour).Seconds(), summary.to.Sub(summary.from).Seconds(), 1, "7d by default")
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
	var body struct {
		Data struct {
			Window    string            `json:"window"`
			PageViews []json.RawMessage `json:"pageviews"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "7d", body.Data.Window)
	assert.Len(t, body.Data.PageViews, 1)

	w = httptest.NewRecorder()
	h.GetSummary(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/traffic/summary?window=24h", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	assert.InDelta(t, (24 * time.Hour).Seconds(), summary.to.Sub(summary.from).Seconds(), 1)

	w = httptest.NewRecorder()
	h.GetSummary(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/admin/traffic/summary?window=90d", http.NoBody))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_WINDOW")
}

func reclassify(t *testing.T, h *TrafficReportHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.Reclassify(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/admin/traffic/reclassify", strings.NewReader(body)))
	return w
}

func TestTrafficReclassify_RunsTheRulesOverTheWindow(t *testing.T) {
	rules := &fakeReclassifier{}
	h := NewTrafficReportHandler(&fakeTrafficSummary{}, rules)

	w := reclassify(t, h, `{"from":"2026-10-01T00:00:00Z","to":"2026-10-17T02:00:00Z","dry_run":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, rules.dryRun, "a dry run is passed through")
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), rules.from.UTC())
	assert.Equal(t, time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC), rules.to.UTC())
	var body struct {
		Data db.TrafficRun `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Data.DryRun)
	assert.Equal(t, 2, body.Data.Rules[db.RuleFetchedWithoutCopy])

	w = reclassify(t, h, `{"from":"2026-10-01T00:00:00Z","to":"2026-10-17T02:00:00Z"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.False(t, rules.dryRun, "without dry_run the run writes")
	assert.Equal(t, 2, rules.calls)
}

func TestTrafficReclassify_RefusesAWindowItCannotJudge(t *testing.T) {
	rules := &fakeReclassifier{}
	h := NewTrafficReportHandler(&fakeTrafficSummary{}, rules)
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	for name, body := range map[string]string{
		"not JSON":           `{"from":`,
		"no from":            `{"to":"2026-10-17T02:00:00Z"}`,
		"no to":              `{"from":"2026-10-01T00:00:00Z"}`,
		"not RFC 3339":       `{"from":"2026-10-01","to":"2026-10-17"}`,
		"an empty window":    `{"from":"2026-10-17T02:00:00Z","to":"2026-10-17T02:00:00Z"}`,
		"a reversed window":  `{"from":"2026-10-17T02:00:00Z","to":"2026-10-01T00:00:00Z"}`,
		"a window in future": `{"from":"2026-10-01T00:00:00Z","to":"` + future + `"}`,
		"more than a year":   `{"from":"2025-01-01T00:00:00Z","to":"2026-10-17T02:00:00Z"}`,
	} {
		w := reclassify(t, h, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: %s", name, w.Body.String())
		assert.Contains(t, w.Body.String(), "VALIDATION_ERROR", name)
	}
	assert.Zero(t, rules.calls, "nothing ran")
}

// One run at a time: while the job (or another call) holds the lock, the call is refused.
func TestTrafficReclassify_RefusesWhileARunHoldsTheLock(t *testing.T) {
	rules := &fakeReclassifier{busy: true}
	w := reclassify(t, NewTrafficReportHandler(&fakeTrafficSummary{}, rules),
		`{"from":"2026-10-01T00:00:00Z","to":"2026-10-17T02:00:00Z","dry_run":true}`)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "CLASSIFICATION_RUNNING")
	assert.Zero(t, rules.calls)
}
