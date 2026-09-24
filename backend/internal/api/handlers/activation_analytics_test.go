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

// fakeActivationReader records the window it was asked for and returns a canned
// report, so the handler can be tested without a database.
type fakeActivationReader struct {
	gotFrom time.Time
	gotTo   time.Time
	report  db.ActivationReport
	err     error
}

func (f *fakeActivationReader) Measure(_ context.Context, from, to time.Time) (db.ActivationReport, error) {
	f.gotFrom, f.gotTo = from, to
	if f.err != nil {
		return db.ActivationReport{}, f.err
	}
	return f.report, nil
}

func cannedActivationReport() db.ActivationReport {
	rooms := 3
	rate := 2.0 / 3.0
	median := 20000.0
	return db.ActivationReport{
		GeneratedAt:          time.Now().UTC(),
		ActivationDefinition: "two distinct authenticated agents",
		RoomsCreated:         3,
		ActivatedRooms:       2,
		RoomToActivation:     db.ActivationConversion{Denominator: &rooms, Numerator: 2, Rate: &rate},
		TimeToFirstExchangeMS: db.ActivationDurationStats{Count: 2, Median: &median, P90: &median},
		HistoricalMultiAuthorRooms: 40,
		HistoricalNote:             "historical, not activation",
	}
}

func activationRequest(t *testing.T, h *ActivationAnalyticsHandler, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.GetReport(rec, req)
	return rec
}

func TestActivationAnalyticsHandler_ServesReportToOperator(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeActivationReader{report: cannedActivationReport()}
	h := NewActivationAnalyticsHandler(reader)

	rec := activationRequest(t, h, "/admin/activation-analytics", map[string]string{"X-Admin-API-Key": "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Data struct {
			Window        string `json:"window"`
			WindowText    string `json:"window_text"`
			RoomsCreated  int    `json:"rooms_created"`
			ActivatedRooms int   `json:"activated_rooms"`
			RoomToActivation struct {
				Denominator *int     `json:"denominator"`
				Numerator   int      `json:"numerator"`
				Rate        *float64 `json:"rate"`
			} `json:"room_to_activation"`
			TimeToFirstExchange struct {
				Count  int      `json:"count"`
				Median *float64 `json:"median_ms"`
			} `json:"time_to_first_exchange_ms"`
			HistoricalMultiAuthorRooms int    `json:"historical_multi_author_rooms"`
			HistoricalNote             string `json:"historical_note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	assert.Equal(t, "30d", body.Data.Window, "activation defaults to a 30-day window")
	assert.NotEmpty(t, body.Data.WindowText)
	assert.Equal(t, 3, body.Data.RoomsCreated)
	assert.Equal(t, 2, body.Data.ActivatedRooms)
	require.NotNil(t, body.Data.RoomToActivation.Denominator)
	assert.Equal(t, 3, *body.Data.RoomToActivation.Denominator)
	assert.Equal(t, 2, body.Data.TimeToFirstExchange.Count)
	assert.Equal(t, 40, body.Data.HistoricalMultiAuthorRooms)
	assert.NotEmpty(t, body.Data.HistoricalNote)
}

func TestActivationAnalyticsHandler_PassesSelectedWindowToRepo(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeActivationReader{report: cannedActivationReport()}
	h := NewActivationAnalyticsHandler(reader)

	rec := activationRequest(t, h, "/admin/activation-analytics?window=7d", map[string]string{"X-Admin-API-Key": "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	span := reader.gotTo.Sub(reader.gotFrom)
	assert.InDelta(t, (7 * 24 * time.Hour).Seconds(), span.Seconds(), 5,
		"a 7-day window must ask the repository for a 7-day range")
}

func TestActivationAnalyticsHandler_RejectsUnknownWindow(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeActivationReader{report: cannedActivationReport()}
	h := NewActivationAnalyticsHandler(reader)

	rec := activationRequest(t, h, "/admin/activation-analytics?window=forever", map[string]string{"X-Admin-API-Key": "op-key"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"data"`)
}

func TestActivationAnalyticsHandler_RefusesWithoutOperatorKey(t *testing.T) {
	reader := &fakeActivationReader{report: cannedActivationReport()}
	h := NewActivationAnalyticsHandler(reader)

	t.Run("no key configured", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "")
		rec := activationRequest(t, h, "/admin/activation-analytics", nil)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("missing header", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "op-key")
		rec := activationRequest(t, h, "/admin/activation-analytics", nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("wrong key", func(t *testing.T) {
		t.Setenv("ADMIN_API_KEY", "op-key")
		rec := activationRequest(t, h, "/admin/activation-analytics", map[string]string{"X-Admin-API-Key": "nope"})
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}
