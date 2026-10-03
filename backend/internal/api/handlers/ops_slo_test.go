package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

// fakeSLOReader records the window it was asked for and returns canned observations.
type fakeSLOReader struct {
	gotStart, gotEnd, gotNow time.Time
	obs                      ops.SLOObservations
	err                      error
}

func (f *fakeSLOReader) ObserveSLO(_ context.Context, start, end, now time.Time) (ops.SLOObservations, error) {
	f.gotStart, f.gotEnd, f.gotNow = start, end, now
	obs := f.obs
	obs.WindowStart, obs.WindowEnd, obs.Now = start, end, now
	return obs, f.err
}

func sloRequest(h *OpsSLOHandler, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.GetSLO(rec, req)
	return rec
}

var operatorHeader = map[string]string{"X-Admin-API-Key": "op-key"}

func TestOpsSLOHandler_ServesTheFourTargetsToTheOperator(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	p95 := 42.0
	reader := &fakeSLOReader{obs: ops.SLOObservations{Read: ops.LatencyObservation{P95Ms: &p95, Samples: 500}}}
	rec := sloRequest(NewOpsSLOHandler(reader), "/admin/ops/slo", operatorHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")

	// The window is the 30 days ending now.
	assert.WithinDuration(t, time.Now(), reader.gotEnd, 5*time.Second)
	assert.Equal(t, ops.AvailabilityWindow, reader.gotEnd.Sub(reader.gotStart))

	var body struct {
		Data struct {
			Targets []struct {
				Key      string   `json:"key"`
				Status   string   `json:"status"`
				Measured *float64 `json:"measured"`
				Missing  string   `json:"missing"`
			} `json:"targets"`
			ExternalModel struct {
				Key string `json:"key"`
			} `json:"external_model"`
			Queues []struct {
				Queue  string `json:"queue"`
				Status string `json:"status"`
			} `json:"queues"`
			Missing []string `json:"missing"`
			UAT     []string `json:"uat"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Targets, 4)
	assert.Equal(t, "read_p95", body.Data.Targets[1].Key)
	assert.Equal(t, "met", body.Data.Targets[1].Status)
	assert.Equal(t, "not_yet_measurable", body.Data.Targets[0].Status, "no checks were observed")
	assert.Equal(t, "not_yet_measurable", body.Data.Targets[3].Status)
	assert.NotEmpty(t, body.Data.Targets[3].Missing)
	assert.Equal(t, "search_p95", body.Data.ExternalModel.Key)
	require.Len(t, body.Data.Queues, 1)
	assert.Equal(t, "ok", body.Data.Queues[0].Status)
	assert.NotEmpty(t, body.Data.Missing)
	assert.NotEmpty(t, body.Data.UAT)
}

// ?end= evaluates the window ending at a past instant: a restored copy of production
// is judged at the moment its data stops, not across the silence since.
func TestOpsSLOHandler_EvaluatesAtAGivenEnd(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeSLOReader{}
	rec := sloRequest(NewOpsSLOHandler(reader), "/admin/ops/slo?end=2026-10-02T23:52:00Z", operatorHeader)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	end := time.Date(2026, 10, 2, 23, 52, 0, 0, time.UTC)
	assert.Equal(t, end, reader.gotEnd.UTC())
	assert.Equal(t, end.Add(-ops.AvailabilityWindow), reader.gotStart.UTC())
	assert.Equal(t, end, reader.gotNow.UTC(), "the queue is read as of the end too")

	bad := sloRequest(NewOpsSLOHandler(reader), "/admin/ops/slo?end=yesterday", operatorHeader)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Contains(t, bad.Body.String(), "INVALID_END")
}

func TestOpsSLOHandler_RefusesWithoutTheOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewOpsSLOHandler(&fakeSLOReader{})
	assert.Equal(t, http.StatusUnauthorized, sloRequest(h, "/admin/ops/slo", nil).Code)
	assert.Equal(t, http.StatusForbidden, sloRequest(h, "/admin/ops/slo", map[string]string{"X-Admin-API-Key": "nope"}).Code)
}

func TestOpsSLOHandler_ReadFailureIsAnInternalError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	rec := sloRequest(NewOpsSLOHandler(&fakeSLOReader{err: errors.New("boom")}), "/admin/ops/slo", operatorHeader)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom")
}
