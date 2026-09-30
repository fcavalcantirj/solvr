package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 53 step 6: room search stays room discovery. The knowledge search (the default
// GET /v1/search, both paths) never returns a room or its raw messages, however much the room
// talks; only a room outcome someone saved as a post is knowledge. Room discovery (GET
// /v1/rooms?q=) finds a room by its name and description, not by its chatter.
func TestCanonicalSearch_RoomChatterStaysOutOfKnowledgeSearch(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	scan := func(sql string, args ...any) string {
		t.Helper()
		var id string
		require.NoError(t, pool.QueryRow(ctx, sql, args...).Scan(&id), sql)
		return id
	}
	agent := "agent_rsc_talker"
	insertRemapAgent(t, pool, ctx, agent)
	room := scan(`INSERT INTO rooms (slug, display_name, description)
		VALUES ('rsc-room', 'rscroomname planning room', 'rscroomdesc a public room') RETURNING id::text`)
	for range 30 {
		_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, author_type, author_id, agent_name, content)
			VALUES ($1, 'agent', $2, $2, 'rscchatter rscroomname status update, still working')`, room, agent)
		require.NoError(t, err)
	}
	outcome := scan(`INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, source_room_id)
		VALUES ('post', 'rscoutcome the plan we agreed', 'saved from the room', ARRAY['rsc'], 'open', 'agent', $1,
			'published', 'approved', $2) RETURNING id::text`, agent, room)

	fulltext := NewSearchRepository(pool)
	hybrid := NewSearchRepository(pool)
	// The query vector matches nothing stored: nothing in this fixture is embedded.
	hybrid.SetEmbeddingService(&fixedEmbeddingService{vec: cmpAxis(0)})
	for path, repo := range map[string]*SearchRepository{"fulltext_only": fulltext, "hybrid_rrf": hybrid} {
		for query, want := range map[string][]string{
			"rscchatter":             nil,
			"rscroomname":            nil,
			"rscroomdesc":            nil,
			"rscoutcome":             {outcome},
			"rscchatter rscoutcome":  {outcome},
			"rscroomname rscoutcome": {outcome},
		} {
			results, total, method, _, err := repo.Search(ctx, query, models.SearchOptions{Page: 1, PerPage: 50})
			require.NoError(t, err)
			require.Equal(t, path, method)
			var got []string
			for _, r := range results {
				got = append(got, r.ID)
				assert.Empty(t, r.MatchedReplies, "%s %q: a room message is never a reply anchor", path, query)
			}
			assert.Equal(t, want, got, "%s %q", path, query)
			assert.Equal(t, len(want), total, "%s %q", path, query)
		}
	}

	rooms := NewRoomRepository(pool)
	for query, want := range map[string]bool{"rscroomname": true, "rscroomdesc": true, "rscchatter": false, "rscoutcome": false} {
		listed, err := rooms.ListFiltered(ctx, RoomListParams{Limit: 20, Query: query})
		require.NoError(t, err)
		found := false
		for _, r := range listed {
			found = found || r.ID.String() == room
		}
		assert.Equal(t, want, found, "room discovery %q", query)
	}
}
