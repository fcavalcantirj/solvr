package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 83: GET /v1/sitemap/counts names each type's newest material change, so the
// sitemap index dates its sub-sitemaps from the content, not from the clock.
func TestSitemapCounts_NameEachTypesLastmod(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()
	status, body := historyGet(t, ts.URL+"/v1/sitemap/counts")
	require.Equal(t, http.StatusOK, status)
	data, _ := body["data"].(map[string]any)
	require.Contains(t, data, "lastmod")
	lastmod, _ := data["lastmod"].(map[string]any)
	for _, key := range []string{"posts", "agents", "blog_posts", "rooms"} {
		assert.Contains(t, lastmod, key)
	}
}
