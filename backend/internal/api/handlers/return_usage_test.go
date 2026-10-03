package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /admin/return-usage (idx 92): operator-only, the window chosen here, uncached.
type fakeReturnReader struct {
	from, to, now time.Time
	err           error
}

func (f *fakeReturnReader) Measure(_ context.Context, from, to, now time.Time) (db.ReturnUsageReport, error) {
	f.from, f.to, f.now = from, to, now
	return db.ReturnUsageReport{Notifications: db.ReturnNotifications{NotAGrowthMetric: true}}, f.err
}

func returnRequest(h *ReturnUsageHandler, target, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if key != "" {
		req.Header.Set(OperatorAccessHeader, key)
	}
	rec := httptest.NewRecorder()
	h.GetReport(rec, req)
	return rec
}

func TestReturnUsageHandler_GateWindowAndCachePolicy(t *testing.T) {
	reader := &fakeReturnReader{}
	h := NewReturnUsageHandler(reader)

	t.Setenv("ADMIN_API_KEY", "")
	require.Equal(t, http.StatusServiceUnavailable, returnRequest(h, "/admin/return-usage", "x").Code)
	t.Setenv("ADMIN_API_KEY", "op-key")
	require.Equal(t, http.StatusUnauthorized, returnRequest(h, "/admin/return-usage", "").Code)
	require.Equal(t, http.StatusForbidden, returnRequest(h, "/admin/return-usage", "nope").Code)
	require.Equal(t, http.StatusBadRequest, returnRequest(h, "/admin/return-usage?window=2y", "op-key").Code)

	rec := returnRequest(h, "/admin/return-usage?window=7d", "op-key")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	require.InDelta(t, 7*24*time.Hour, reader.to.Sub(reader.from), float64(time.Second))
	require.Contains(t, rec.Body.String(), `"window":"7d"`)
	require.Contains(t, rec.Body.String(), `"not_a_growth_metric":true`)

	h = NewReturnUsageHandler(&fakeReturnReader{err: errors.New("down")})
	require.Equal(t, http.StatusInternalServerError, returnRequest(h, "/admin/return-usage", "op-key").Code)
}
