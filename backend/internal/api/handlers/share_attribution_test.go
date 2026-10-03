package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /admin/share-attribution (idx 88 steps 5-6) is operator-only reporting about the
// share loop. The repository owns every definition; the handler picks the window, gates
// the caller and serves the answer, uncached.

type fakeShareReader struct {
	from, to, now time.Time
	err           error
}

func (f *fakeShareReader) Measure(_ context.Context, from, to, now time.Time) (db.ShareAttributionReport, error) {
	f.from, f.to, f.now = from, to, now
	if f.err != nil {
		return db.ShareAttributionReport{}, f.err
	}
	k := 0.5
	return db.ShareAttributionReport{ReferredVisits: 4, K: db.ShareK{Value: &k, ExperimentMetric: true}}, nil
}

func shareRequest(h *ShareAttributionHandler, target, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if key != "" {
		req.Header.Set(OperatorAccessHeader, key)
	}
	rec := httptest.NewRecorder()
	h.GetReport(rec, req)
	return rec
}

func TestShareAttributionHandler_IsGatedByTheOperatorKey(t *testing.T) {
	h := NewShareAttributionHandler(&fakeShareReader{})

	t.Setenv("ADMIN_API_KEY", "")
	require.Equal(t, http.StatusServiceUnavailable, shareRequest(h, "/admin/share-attribution", "x").Code)

	t.Setenv("ADMIN_API_KEY", "op-key")
	require.Equal(t, http.StatusUnauthorized, shareRequest(h, "/admin/share-attribution", "").Code)
	require.Equal(t, http.StatusForbidden, shareRequest(h, "/admin/share-attribution", "wrong").Code)
}

func TestShareAttributionHandler_ServesTheChosenWindowUncached(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeShareReader{}
	h := NewShareAttributionHandler(reader)

	rec := shareRequest(h, "/admin/share-attribution?window=7d", "op-key")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	require.InDelta(t, 7*24*time.Hour, reader.to.Sub(reader.from), float64(time.Second))
	require.False(t, reader.now.Before(reader.to), "returns are judged as of now")

	var out struct {
		Data struct {
			Window         string `json:"window"`
			ReferredVisits int    `json:"referred_visits"`
			K              struct {
				Value            float64 `json:"value"`
				ExperimentMetric bool    `json:"experiment_metric"`
			} `json:"k"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "7d", out.Data.Window)
	require.Equal(t, 4, out.Data.ReferredVisits)
	require.True(t, out.Data.K.ExperimentMetric)
}

func TestShareAttributionHandler_DefaultsTo30DaysAndRefusesAnUnknownWindow(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	reader := &fakeShareReader{}
	h := NewShareAttributionHandler(reader)

	require.Equal(t, http.StatusOK, shareRequest(h, "/admin/share-attribution", "op-key").Code)
	require.InDelta(t, 30*24*time.Hour, reader.to.Sub(reader.from), float64(time.Second))
	require.Equal(t, http.StatusBadRequest, shareRequest(h, "/admin/share-attribution?window=1y", "op-key").Code)
}

func TestShareAttributionHandler_AFailedMeasurementIsAServerError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "op-key")
	h := NewShareAttributionHandler(&fakeShareReader{err: errors.New("db down")})
	require.Equal(t, http.StatusInternalServerError, shareRequest(h, "/admin/share-attribution", "op-key").Code)
}
