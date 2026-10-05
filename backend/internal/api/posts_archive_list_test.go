package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postIDsOf reads the ids of a list answer's data rows.
func postIDsOf(t *testing.T, rows any) []string {
	t.Helper()
	items, _ := rows.([]any)
	ids := make([]string, 0, len(items))
	for _, row := range items {
		id, _ := row.(map[string]any)["id"].(string)
		require.NotEmpty(t, id)
		ids = append(ids, id)
	}
	return ids
}

// SPEC.md 27.2: over HTTP, the pages of GET /v1/posts?indexable=true hold exactly the posts
// GET /v1/sitemap/urls lists, each once, and meta.total_pages names the last page. The post
// archive (/posts/page/{n}) renders these pages, so a post the sitemap lists always has a link.
func TestPostArchiveList_PagesHoldExactlyTheSitemapPosts(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	agent := fmt.Sprintf("agent_roomtest_archive_%d", time.Now().UnixNano()%1000000000)
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, 'archive lister', 'active')`, agent)
	require.NoError(t, err)

	post := func(label, publication, moderation string) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO posts (type, title, description, status,
			posted_by_type, posted_by_id, publication_state, moderation_state, visibility)
			VALUES ('post', $1, 'a post body for the archive list router test', 'open', 'agent', $2, $3, $4, 'public')
			RETURNING id::text`, "archive router "+label, agent, publication, moderation).Scan(&id))
		return id
	}
	var listed []string
	for i := 0; i < 5; i++ {
		listed = append(listed, post(fmt.Sprintf("listed %d", i), "published", "approved"))
	}
	awaiting := post("awaiting moderation", "published", "pending")

	// readSitemap reads every post the sitemap lists.
	readSitemap := func() []string {
		t.Helper()
		var ids []string
		for page := 1; ; page++ {
			status, body := historyGet(t, fmt.Sprintf("%s/v1/sitemap/urls?type=posts&page=%d&per_page=5000", ts.URL, page))
			require.Equal(t, http.StatusOK, status)
			pageIDs := postIDsOf(t, body["data"].(map[string]any)["posts"])
			if len(pageIDs) == 0 {
				sort.Strings(ids)
				return ids
			}
			ids = append(ids, pageIDs...)
		}
	}
	// readArchive reads every page of the list at two a page, as the archive would.
	readArchive := func() (ids []string, total, pages int) {
		t.Helper()
		status, first := historyGet(t, ts.URL+"/v1/posts?indexable=true&sort=new&page=1&per_page=2")
		require.Equal(t, http.StatusOK, status)
		meta := first["meta"].(map[string]any)
		total, pages = int(meta["total"].(float64)), int(meta["total_pages"].(float64))
		for page := 1; page <= pages; page++ {
			status, body := historyGet(t, fmt.Sprintf("%s/v1/posts?indexable=true&sort=new&page=%d&per_page=2", ts.URL, page))
			require.Equal(t, http.StatusOK, status)
			ids = append(ids, postIDsOf(t, body["data"])...)
		}
		return ids, total, pages
	}

	// The scratch database is shared with the rest of this package: a post another test left
	// in moderation could be approved while the pages are read. The two listings are compared
	// across a read during which the sitemap did not move.
	var sitemap, archive []string
	var total, pages int
	for attempt := 1; ; attempt++ {
		before := readSitemap()
		archive, total, pages = readArchive()
		sitemap = readSitemap()
		if assert.ObjectsAreEqual(before, sitemap) {
			break
		}
		require.Less(t, attempt, 5, "the sitemap kept changing while the archive was read")
	}
	for _, id := range listed {
		require.Contains(t, sitemap, id)
	}
	require.NotContains(t, sitemap, awaiting)

	assert.Equal(t, len(sitemap), total, "meta.total is the number of posts the sitemap lists")
	assert.Equal(t, (total+1)/2, pages, "meta.total_pages is ceil(total / per_page)")
	assert.Len(t, archive, total, "the pages hold meta.total posts in all, so none is on two pages")
	got := append([]string{}, archive...)
	sort.Strings(got)
	assert.Equal(t, sitemap, got, "every sitemap post is on one archive page, and no other post is")

	// The last page holds posts and says there are no more.
	status, last := historyGet(t, fmt.Sprintf("%s/v1/posts?indexable=true&sort=new&page=%d&per_page=2", ts.URL, pages))
	require.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, postIDsOf(t, last["data"]))
	assert.Equal(t, false, last["meta"].(map[string]any)["has_more"])

	// Past the last page the list is empty, not an error: the web page turns that into a 404.
	status, past := historyGet(t, fmt.Sprintf("%s/v1/posts?indexable=true&sort=new&page=%d&per_page=2", ts.URL, pages+1))
	require.Equal(t, http.StatusOK, status)
	assert.Empty(t, postIDsOf(t, past["data"]))
	assert.Equal(t, float64(pages), past["meta"].(map[string]any)["total_pages"])

	// One author's indexable posts, as a profile page reads them: the five, never the sixth.
	status, own := historyGet(t, ts.URL+"/v1/posts?indexable=true&sort=new&per_page=50&author_type=agent&author_id="+agent)
	require.Equal(t, http.StatusOK, status)
	assert.ElementsMatch(t, listed, postIDsOf(t, own["data"]))
	assert.Equal(t, float64(1), own["meta"].(map[string]any)["total_pages"])

	// The plain list keeps showing the post that awaits moderation.
	status, plain := historyGet(t, ts.URL+"/v1/posts?sort=new&per_page=50&author_type=agent&author_id="+agent)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, postIDsOf(t, plain["data"]), awaiting)
}
