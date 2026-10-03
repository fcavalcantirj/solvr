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

	"github.com/fcavalcantirj/solvr/internal/growth"
)

// fakeParticipantReader records the window end it was asked for and returns canned measures,
// so the growth report handler is tested without a database.
type fakeParticipantReader struct {
	gotEnd   time.Time
	measures growth.ParticipantMeasures
	err      error
}

func (f *fakeParticipantReader) Measure(_ context.Context, end time.Time) (growth.ParticipantMeasures, error) {
	f.gotEnd = end
	return f.measures, f.err
}

func growthRequest(t *testing.T, handler http.HandlerFunc, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestGrowthParticipants_RefusesWithoutTheOperatorKey(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{}})

	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = growthRequest(t, h.GetParticipants, "/admin/growth/participants",
		map[string]string{OperatorAccessHeader: "wrong"})
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
}

func TestGrowthParticipants_ServesTheReportForTheRequestedEnd(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeParticipantReader{measures: growth.ParticipantMeasures{Humans: 3, Agents: 5, KnownOverlap: 1}}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: reader})

	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants?end=2026-10-01T00:00:00Z",
		map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), reader.gotEnd)

	var body struct {
		Data growth.ParticipantReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 3, body.Data.Humans.Active)
	assert.Equal(t, 5, body.Data.AgentIdentities.Active)
	assert.Equal(t, 8, body.Data.Target.Measured)
	assert.Equal(t, growth.MonthlyActiveParticipantGoal, body.Data.Target.Goal)
	assert.Equal(t, growth.StatusUnmet, body.Data.Target.Status)
}

func TestGrowthParticipants_DefaultsToNowAndRejectsABadEnd(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeParticipantReader{}
	h := NewGrowthReportsHandler(GrowthReaders{Participants: reader})

	before := time.Now().UTC()
	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", map[string]string{OperatorAccessHeader: "op-key"})
	require.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, reader.gotEnd.Before(before.Truncate(time.Second)), "the window ends now by default")

	rec = growthRequest(t, h.GetParticipants, "/admin/growth/participants?end=yesterday", map[string]string{OperatorAccessHeader: "op-key"})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGrowthParticipants_AReadFailureIsAServerError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewGrowthReportsHandler(GrowthReaders{Participants: &fakeParticipantReader{err: errors.New("boom")}})
	rec := growthRequest(t, h.GetParticipants, "/admin/growth/participants", map[string]string{OperatorAccessHeader: "op-key"})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "boom")
}
