package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Since spec.json idx 74 step 5 an edit of a post, reply or room must carry
// If-Match (428 PRECONDITION_REQUIRED without it). These helpers edit the way a
// client does: read the resource as the same caller, then send its ETag back.

// currentETag GETs url as bearer and returns the ETag of that read ("" when the
// caller cannot read it, so the edit then answers what an unread edit answers).
func currentETag(t *testing.T, url, bearer string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Header.Get("ETag")
}

// doJSONAtCurrentVersion is doJSON for an edit, sent with If-Match set to the
// caller's own read of url.
func doJSONAtCurrentVersion(t *testing.T, method, url, bearer, body string) (int, map[string]any) {
	t.Helper()
	resp := doRoomRequestAtCurrentVersion(t, method, url, body, bearer)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

// doRoomRequestAtCurrentVersion is doRoomRequest for an edit, sent with
// If-Match set to the caller's own read of url.
func doRoomRequestAtCurrentVersion(t *testing.T, method, url, body, bearer string) *http.Response {
	t.Helper()
	etag := currentETag(t, url, bearer)
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}
