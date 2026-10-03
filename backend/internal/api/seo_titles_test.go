package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 82: every indexable post page gets a unique title. A title shared with
// another indexable post names its author; one shared with the same author's own post
// also names its date. Posts that are not indexable never count as twins.
func TestPostSEO_TitleIsUniqueAmongIndexablePosts(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	marker := fmt.Sprintf("twin%d", time.Now().UnixNano()%1000000000)
	agentA, _ := registerRoomTestAgent(t, ts)
	agentB, _ := registerRoomTestAgent(t, ts)
	name := func(id string) string {
		var n string
		require.NoError(t, pool.QueryRow(ctx, `SELECT display_name FROM agents WHERE id = $1`, id).Scan(&n))
		return n
	}
	day := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	insert := func(author, title, status string, created time.Time) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,
			   publication_state,moderation_state,created_at,updated_at)
			 VALUES ('post',$1,'a body long enough to describe the post','agent',$2,$3,'public','published','approved',$4,$4)
			 RETURNING id::text`, title, author, status, created).Scan(&id))
		return id
	}
	shared := "Shared title " + marker
	x1 := insert(agentA, shared, "open", day)
	x2 := insert(agentB, "  shared TITLE "+marker+" ", "open", day.Add(time.Hour))
	insert(agentA, shared, "open", day.Add(48*time.Hour))
	insert(agentB, shared, "rejected", day)
	unique := insert(agentA, "Unique title "+marker, "open", day)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title ILIKE '%"+marker+"%'") }) //nolint:errcheck

	title := func(id string) string {
		t.Helper()
		status, seo := seoGet(t, ts.URL+"/v1/posts/"+id+"/seo")
		require.Equal(t, http.StatusOK, status)
		s, _ := seo["title"].(string)
		return s
	}
	assert.Equal(t, "Unique title "+marker, title(unique))
	assert.Equal(t, "shared TITLE "+marker+" — "+name(agentB), title(x2), "a title shared with other authors names the author")
	assert.Equal(t, shared+" — "+name(agentA)+" (2026-09-14)", title(x1), "a title the same author reused also names the date")
}
