package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /v1/mcp runs each tool as a request through the API's own router. The MCP call was
// measured at the boundary; the request it dispatches to the router must not be counted again.
func TestAPIUsage_DoesNotCountARequestTheAPIDispatchesToItself(t *testing.T) {
	spy := &recordingSpy{}
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/demo/messages", nil)
	req = req.WithContext(WithInProcessCall(req.Context()))
	rec := httptest.NewRecorder()
	usageRouter(spy).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code, "the dispatched request is still served")
	assert.Empty(t, spy.events)

	outside, _ := call(t, http.MethodPost, "/v1/rooms/demo/messages", nil)
	require.Len(t, outside.events, 1, "a request from outside is counted")
}

func TestInProcessCall_IsOnlyWhatWithInProcessCallMarked(t *testing.T) {
	assert.False(t, IsInProcessCall(context.Background()))
	assert.True(t, IsInProcessCall(WithInProcessCall(context.Background())))

	// A header cannot mark a request: only the API itself can.
	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	req.Header.Set("X-In-Process-Call", "true")
	assert.False(t, IsInProcessCall(req.Context()))
}
