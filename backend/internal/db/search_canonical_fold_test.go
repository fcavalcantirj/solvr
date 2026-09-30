package db

import (
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
)

func f64(v float64) *float64 { return &v }

func replyMatch(id, postID string, score float64, sim *float64) models.SearchReplyMatch {
	return models.SearchReplyMatch{ID: id, PostID: postID, Score: score, Similarity: sim}
}

// Task idx 53 step 2 (no database): reply matches fold into their post. A post matched by its
// own text and by replies appears once; it carries at most maxReplyMatchesPerPost anchors,
// best first; its score is the better of its own and its best reply's, and its similarity the
// best of its own and all its matches'.
func TestFoldReplyMatches_OneResultPerPostWithItsBestAnchors(t *testing.T) {
	posts := []models.SearchResult{
		{ID: "p1", Score: 0.5, Similarity: f64(0.2)},
		{ID: "p2", Score: 0.3},
		{ID: "p3", Score: 0.1},
	}
	matches := []models.SearchReplyMatch{
		replyMatch("r1", "p1", 0.2, nil),
		replyMatch("r2", "p2", 0.9, f64(0.6)),
		replyMatch("r3", "p1", 0.7, f64(0.1)),
		replyMatch("r4", "p1", 0.4, f64(0.8)),
		replyMatch("r5", "p1", 0.1, f64(0.9)), // the 4th match of p1: past the cap, still the best similarity
		replyMatch("r6", "gone", 0.9, f64(0.99)),
	}
	got := foldReplyMatches(posts, matches)

	assert.Len(t, got, 3, "one result per post; a match whose post is not a result is dropped")
	ids := func(ms []models.SearchReplyMatch) []string {
		var out []string
		for _, m := range ms {
			out = append(out, m.ID)
		}
		return out
	}
	assert.Equal(t, "p1", got[0].ID, "the order of the post results is kept")
	assert.Equal(t, []string{"r3", "r4", "r1"}, ids(got[0].MatchedReplies), "best first, capped at 3")
	assert.Equal(t, 0.7, got[0].Score, "the best reply outranks the post's own score")
	assert.Equal(t, 0.9, *got[0].Similarity, "the best similarity over the post and every match")

	assert.Equal(t, []string{"r2"}, ids(got[1].MatchedReplies))
	assert.Equal(t, 0.9, got[1].Score)
	assert.Equal(t, 0.6, *got[1].Similarity, "a post with no similarity takes its reply's")

	assert.Nil(t, got[2].MatchedReplies, "a post matched only by its own text has no anchors")
	assert.Equal(t, 0.1, got[2].Score)
	assert.Nil(t, got[2].Similarity)
}

// The posts that matched only through replies, each once, in the order of their best match;
// they are loaded and then folded like the others.
func TestReplyOnlyPostIDs_ListsPostsFoundOnlyThroughReplies(t *testing.T) {
	posts := []models.SearchResult{{ID: "p1"}}
	matches := []models.SearchReplyMatch{
		replyMatch("r1", "p2", 0.9, nil),
		replyMatch("r2", "p1", 0.8, nil),
		replyMatch("r3", "p3", 0.7, nil),
		replyMatch("r4", "p2", 0.1, nil),
	}
	assert.Equal(t, []string{"p2", "p3"}, replyOnlyPostIDs(posts, matches))
	assert.Empty(t, replyOnlyPostIDs(posts, nil))
}
