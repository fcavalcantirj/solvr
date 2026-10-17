package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SPEC.md 22.7 and 22.8, through the real router and database: a pasted error message with a
// quoted word is a normal search (it answered 500 before the fix), and a query that is not
// UTF-8 is the client's 400, stored in search_failures under the id the response names.
func TestSearchRouter_AnswersPastedErrorsAndStoresFailuresUnderTheirRequestID(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/v1/search?q=Cannot%20find%20module%20%27express%27&page=1&per_page=20") //nolint:noctx // test client
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "a quoted word is plain search text")

	resp, err = http.Get(ts.URL + "/v1/search?q=c1-router-probe%FF&page=1&per_page=20") //nolint:noctx // test client
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	id := resp.Header.Get("X-Request-ID")
	require.NotEmpty(t, id)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM search_failures WHERE request_id = $1`, id) }) //nolint:errcheck

	var status int
	var code string
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = pool.QueryRow(context.Background(),
			`SELECT status, error_code FROM search_failures WHERE request_id = $1`, id).Scan(&status, &code)
		if err == nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err, "the failure is stored under the response's request id")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "VALIDATION_ERROR", code)
}
