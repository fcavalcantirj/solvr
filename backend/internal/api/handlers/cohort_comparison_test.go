package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// fakeCohortReader records the launch it was asked for and returns a canned
// report, so the handler can be tested without a database.
type fakeCohortReader struct {
	gotLaunch time.Time
	report    db.CohortComparisonReport
	err       error
}

func (f *fakeCohortReader) Compare(_ context.Context, launch, _ time.Time) (db.CohortComparisonReport, error) {
	f.gotLaunch = launch
	if f.err != nil {
		return db.CohortComparisonReport{}, f.err
	}
	return f.report, nil
}

func cannedCohortReport() db.CohortComparisonReport {
	rooms := 4
	rate := 0.5
	return db.CohortComparisonReport{
		GeneratedAt:          time.Now().UTC(),
		ActivationDefinition: "two distinct authenticated agents",
		ExclusionsNote:       "documented exclusions",
		Window7d:             db.CohortWindow{Days: 7, RoomsCreated: 3, ActivatedRooms: 1, Complete: true},
		Window28d: db.CohortWindow{
			Days: 28, RoomsCreated: 4, ActivatedRooms: 2, Complete: true,
			RoomToActivation: db.ActivationConversion{Denominator: &rooms, Numerator: 2, Rate: &rate},
		},
		Historical: db.CohortHistoricalContext{AllTimeMultiAuthorRooms: 40, Note: "historical context"},
		ExternalSources: db.CohortExternalSources{
			GoogleAnalytics: db.CohortExternalSource{Available: false, Note: "GA not connected"},
			SearchConsole:   db.CohortExternalSource{Available: false, Note: "GSC not connected"},
		},
	}
}

func cohortRequest(t *testing.T, h *CohortComparisonHandler, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.GetReport(rec, req)
	return rec
}

func TestCohortComparisonHandler_ServesReportToOperator(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeCohortReader{report: cannedCohortReport()}
	h := NewCohortComparisonHandler(reader)

	rec := cohortRequest(t, h, "/admin/cohort-comparison?launch=2026-09-01T00:00:00Z",
		map[string]string{"X-Admin-API-Key": "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The launch instant reaches the repository verbatim.
	assert.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), reader.gotLaunch.UTC())

	// The response is one caller's private answer, never cached and reused.
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")

	var body struct {
		Data struct {
			Window7d struct {
				Days         int  `json:"days"`
				RoomsCreated int  `json:"rooms_created"`
				Complete     bool `json:"complete"`
			} `json:"window_7d"`
			Window28d struct {
				RoomsCreated   int `json:"rooms_created"`
				ActivatedRooms int `json:"activated_rooms"`
			} `json:"window_28d"`
			ExternalSources struct {
				GoogleAnalytics struct {
					Available bool `json:"available"`
				} `json:"google_analytics"`
			} `json:"external_sources"`
			Historical struct {
				AllTimeMultiAuthorRooms int `json:"all_time_multi_author_rooms"`
			} `json:"historical_context"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 7, body.Data.Window7d.Days)
	assert.Equal(t, 3, body.Data.Window7d.RoomsCreated)
	assert.Equal(t, 4, body.Data.Window28d.RoomsCreated)
	assert.Equal(t, 2, body.Data.Window28d.ActivatedRooms)
	assert.False(t, body.Data.ExternalSources.GoogleAnalytics.Available, "GA is marked unavailable, not zero")
	assert.Equal(t, 40, body.Data.Historical.AllTimeMultiAuthorRooms)
}

func TestCohortComparisonHandler_RequiresLaunch(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewCohortComparisonHandler(&fakeCohortReader{report: cannedCohortReport()})

	rec := cohortRequest(t, h, "/admin/cohort-comparison", map[string]string{"X-Admin-API-Key": "op-key"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "MISSING_LAUNCH")
}

func TestCohortComparisonHandler_RejectsInvalidLaunch(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewCohortComparisonHandler(&fakeCohortReader{report: cannedCohortReport()})

	rec := cohortRequest(t, h, "/admin/cohort-comparison?launch=not-a-time",
		map[string]string{"X-Admin-API-Key": "op-key"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_LAUNCH")
}

func TestCohortComparisonHandler_RefusesWithoutOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewCohortComparisonHandler(&fakeCohortReader{report: cannedCohortReport()})

	rec := cohortRequest(t, h, "/admin/cohort-comparison?launch=2026-09-01T00:00:00Z", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
