package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public status-code contract (idx 74 step 3) says 409 is a conflict and 429 a rate
// limit. These tests pin the two rows through the real router, in the standard error
// envelope: a stable code, a message, a request_id that equals the X-Request-Id header,
// and on 429 a retry_after_seconds that equals the Retry-After header.

// statusRowRequest sends one request as a browser page on the public origin would.
func statusRowRequest(t *testing.T, method, url, bearer, body string) (*http.Response, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Origin", "https://solvr.dev")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp, out
}

// requireErrorEnvelope asserts the status and the envelope fields every error carries and
// returns the error object for row-specific checks.
func requireErrorEnvelope(t *testing.T, resp *http.Response, out map[string]any, status int, code string) map[string]any {
	t.Helper()
	require.Equal(t, status, resp.StatusCode, "body: %v", out)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	errObj, ok := out["error"].(map[string]any)
	require.True(t, ok, "expected an error object, got %v", out)
	assert.Equal(t, code, errObj["code"])
	msg, _ := errObj["message"].(string)
	assert.NotEmpty(t, msg, "error.message")
	requestID, _ := errObj["request_id"].(string)
	assert.NotEmpty(t, requestID, "error.request_id")
	assert.Equal(t, resp.Header.Get("X-Request-Id"), requestID, "error.request_id must equal the X-Request-Id header")
	return errObj
}

// requireRetryAfter asserts the 429 retry contract: a positive Retry-After header, the same
// number in error.retry_after_seconds, and the header readable by a cross-origin page.
func requireRetryAfter(t *testing.T, resp *http.Response, errObj map[string]any) {
	t.Helper()
	header, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	require.NoError(t, err, "Retry-After header %q", resp.Header.Get("Retry-After"))
	assert.GreaterOrEqual(t, header, 1)
	seconds, ok := errObj["retry_after_seconds"].(float64)
	require.True(t, ok, "error.retry_after_seconds missing in %v", errObj)
	assert.Equal(t, header, int(seconds), "error.retry_after_seconds must equal Retry-After")
	assert.Contains(t, strings.ToLower(resp.Header.Get("Access-Control-Expose-Headers")), "retry-after",
		"a cross-origin page must be able to read Retry-After")
}

func TestStatusRows_ConflictsAnswerAStableCodeInTheEnvelope(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)

	t.Run("a second room with the same slug is DUPLICATE_ROOM", func(t *testing.T) {
		slug := entriesTestRoom(t, ts.URL, jwt, false)
		resp, out := statusRowRequest(t, "POST", ts.URL+"/v1/rooms", jwt,
			fmt.Sprintf(`{"display_name":"Second %s","slug":%q}`, slug, slug))
		requireErrorEnvelope(t, resp, out, http.StatusConflict, "DUPLICATE_ROOM")
	})

	t.Run("a second agent with the same name is DUPLICATE_NAME and keeps its suggestions", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"roomtest_status_%d","description":"status row conflict test agent"}`, time.Now().UnixNano()%100000000)
		first, firstOut := statusRowRequest(t, "POST", ts.URL+"/v1/agents/register", "", body)
		require.Equal(t, http.StatusCreated, first.StatusCode, "first registration: %v", firstOut)
		resp, out := statusRowRequest(t, "POST", ts.URL+"/v1/agents/register", "", body)
		errObj := requireErrorEnvelope(t, resp, out, http.StatusConflict, "DUPLICATE_NAME")
		suggestions, _ := errObj["suggestions"].([]any)
		assert.NotEmpty(t, suggestions, "the envelope must not drop the name suggestions")
	})
}

func TestStatusRows_RateLimitAnswers429WithRetryAfterInTheEnvelope(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug := entriesTestRoom(t, ts.URL, jwt, false)
	entriesURL := ts.URL + "/v1/rooms/" + slug + "/entries"

	t.Run("a human past the write budget", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			resp, out := statusRowRequest(t, "POST", entriesURL, jwt, fmt.Sprintf(`{"body":"human %d"}`, i))
			require.Equal(t, http.StatusCreated, resp.StatusCode, "write %d: %v", i, out)
		}
		resp, out := statusRowRequest(t, "POST", entriesURL, jwt, `{"body":"human over budget"}`)
		requireRetryAfter(t, resp, requireErrorEnvelope(t, resp, out, http.StatusTooManyRequests, "RATE_LIMITED"))
	})

	t.Run("an agent past the write budget", func(t *testing.T) {
		_, agentKey := registerRoomTestAgent(t, ts)
		roomTok := handshakeRoomToken(t, ts, slug, agentKey)
		for i := 0; i < 60; i++ {
			resp, out := statusRowRequest(t, "POST", entriesURL, roomTok, fmt.Sprintf(`{"body":"agent %d"}`, i))
			require.Equal(t, http.StatusCreated, resp.StatusCode, "write %d: %v", i, out)
		}
		resp, out := statusRowRequest(t, "POST", entriesURL, roomTok, `{"body":"agent over budget"}`)
		requireRetryAfter(t, resp, requireErrorEnvelope(t, resp, out, http.StatusTooManyRequests, "RATE_LIMITED"))
	})
}
