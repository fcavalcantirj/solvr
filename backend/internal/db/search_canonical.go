package db

import (
	"context"
	"fmt"
	"sort"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	pgvector "github.com/pgvector/pgvector-go"
)

// maxReplyMatchesPerPost caps the reply anchors one post result carries.
const maxReplyMatchesPerPost = 3

// searchablePostRule is the rule a post passes to be found, by its own text or through a reply:
// not deleted, and not a draft, pending review or rejected. Visibility is added per viewer.
const searchablePostRule = `p.deleted_at IS NULL AND p.status NOT IN ('pending_review', 'rejected', 'draft')`

// replyMatchPostRule is the rule a reply's post passes for the reply to be found, on both
// paths: searchablePostRule plus the canonical read rule hybrid_search_replies applies
// (published and moderation-approved), so the full-text path finds exactly the replies the
// hybrid path can. A closed (archived) post is still found by its own text; its replies are
// not matched.
const replyMatchPostRule = searchablePostRule + ` AND p.publication_state = 'published' AND p.moderation_state = 'approved'`

// replyAnchorURL is the post page scrolled to the reply (the post page renders each reply
// with its id as the element id).
func replyAnchorURL(postID, replyID string) string {
	return "/posts/" + postID + "#" + replyID
}

// searchKnowledge is the default search over the canonical knowledge model (task idx 53): what
// used to be a problem, question or idea is a post, and what used to be an answer, response or
// approach is a reply. A post is found by its own text or by the text of its live, non-system
// replies, and is returned once, with its best matching replies as anchors (foldReplyMatches).
func (r *SearchRepository) searchKnowledge(ctx context.Context, embedding []float32, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	posts, err := r.searchPostResults(ctx, embedding, tsquery, opts)
	if err != nil {
		return nil, err
	}
	matches, err := r.searchReplyMatches(ctx, embedding, tsquery, opts, nil)
	if err != nil {
		return nil, err
	}
	if ids := replyOnlyPostIDs(posts, matches); len(ids) > 0 {
		extra, err := r.loadSearchPosts(ctx, ids, tsquery, opts)
		if err != nil {
			return nil, err
		}
		posts = append(posts, extra...)
	}
	return foldReplyMatches(posts, matches), nil
}

// searchPostResults searches the posts' own text: hybrid when a query embedding is available,
// full text otherwise.
func (r *SearchRepository) searchPostResults(ctx context.Context, embedding []float32, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	if embedding != nil {
		return r.searchPostsHybrid(ctx, embedding, tsquery, opts)
	}
	return r.searchPosts(ctx, tsquery, opts)
}

// searchReplyMatches finds the replies whose body matches the query, best first. A reply is
// found only when it is live and not a system reply, and its post passes replyMatchPostRule, the
// viewer's visibility and the search's post filters. With a query embedding it uses
// hybrid_search_replies and falls back to full text if that query fails, like the post search.
// postIDs, when not nil, keeps the full-text matches of those posts only (a keyword page's).
func (r *SearchRepository) searchReplyMatches(ctx context.Context, embedding []float32, tsquery string, opts models.SearchOptions, postIDs []string) ([]models.SearchReplyMatch, error) {
	var (
		query string
		args  []any
	)
	if embedding != nil {
		args = []any{tsquery, pgvector.NewVector(embedding), hybridMatchCount(opts), tsquery, nullableViewer(opts.ViewerHuman)}
		query = replyMatchSelect("$4", "hs.rrf_score",
			"CASE WHEN r.embedding IS NOT NULL THEN 1 - (r.embedding <=> $2::vector) END",
			"hybrid_search_replies($1, $2, $3, 2.0, 1.0, 60, $5::uuid) hs JOIN replies r ON r.id = hs.reply_id") +
			" WHERE r.deleted_at IS NULL"
	} else {
		args = []any{tsquery}
		query = replyMatchSelect("$1", "ts_rank(r.search_document, to_tsquery('english', $1))",
			"NULL::float8", "replies r") +
			" WHERE r.deleted_at IS NULL AND r.search_document @@ to_tsquery('english', $1)"
		if postIDs != nil {
			args = append(args, postIDs)
			query += " AND r.post_id = ANY($2::uuid[])"
		}
	}
	argNum := len(args) + 1
	query += " AND r.author_type <> 'system' AND " + replyMatchPostRule +
		" AND " + searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)
	filters, args, _ := buildSearchFilters(opts, args, argNum)
	query += " " + filters + " ORDER BY score DESC, r.created_at, r.id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		if embedding != nil {
			LogSearchEmbeddingFailed(ctx, fmt.Sprintf("hybrid_search_replies query failed: %v", err))
			return r.searchReplyMatches(ctx, nil, tsquery, opts, nil)
		}
		LogQueryError(ctx, "Search.ReplyMatches", "replies", err)
		return nil, fmt.Errorf("search reply matches query failed: %w", err)
	}
	defer rows.Close()
	return scanReplyMatches(rows)
}

// replyMatchSelect is the SELECT list and joins of a reply match query, in the scan order of
// scanReplyMatches. from names the reply row r; the post p and the author joins are added.
func replyMatchSelect(tsArg, score, similarity, from string) string {
	return `
		SELECT
			r.id::text,
			r.post_id::text,
			ts_headline('english', r.body, to_tsquery('english', ` + tsArg + `),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as snippet,
			r.author_type,
			r.author_id,
			COALESCE(
				CASE WHEN r.author_type = 'human' THEN u.display_name
					 ELSE ag.display_name
				END,
				r.author_id
			) as author_name,
			r.legacy_type,
			CASE WHEN r.legacy_type = 'approach' THEN NULLIF(r.provenance->>'status', '') END as legacy_status,
			` + score + ` as score,
			` + similarity + ` as similarity,
			r.created_at
		FROM ` + from + `
		JOIN posts p ON p.id = r.post_id
		LEFT JOIN users u ON r.author_type = 'human' AND r.author_id = u.id::text
		LEFT JOIN agents ag ON r.author_type = 'agent' AND r.author_id = ag.id`
}

func scanReplyMatches(rows pgx.Rows) ([]models.SearchReplyMatch, error) {
	var matches []models.SearchReplyMatch
	for rows.Next() {
		var m models.SearchReplyMatch
		if err := rows.Scan(&m.ID, &m.PostID, &m.Snippet, &m.Author.Type, &m.Author.ID, &m.Author.DisplayName,
			&m.LegacyType, &m.LegacyStatus, &m.Score, &m.Similarity, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan reply match: %w", err)
		}
		m.URL = replyAnchorURL(m.PostID, m.ID)
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reply matches: %w", err)
	}
	return matches, nil
}

// loadSearchPosts loads posts by id, in the order of the ids, as post results with score 0 and
// no similarity: the posts found only through their replies (foldReplyMatches gives them their
// best reply's score), or a keyword page's posts (searchKeywordPage sets their scores). The post
// rule, the viewer's visibility and the post filters are applied again. At most hybridMatchCount
// posts (the hybrid reply search's matches, a keyword page) have their reply counts read per
// post; more keep the one aggregate (searchPostSelect).
func (r *SearchRepository) loadSearchPosts(ctx context.Context, ids []string, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	args := []any{tsquery, ids}
	argNum := 3
	counts := postReplyCountsJoin
	if len(ids) <= hybridMatchCount(opts) {
		counts = postPageReplyCountsJoin
	}
	query := searchPostSelect("$1", "0::float8", "NULL::float8", "posts p", counts) +
		" WHERE p.id = ANY($2::uuid[]) AND " + searchablePostRule +
		" AND " + searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)
	filters, args, _ := buildSearchFilters(opts, args, argNum)
	query += " " + filters

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "Search.LoadPosts", "posts", err)
		return nil, fmt.Errorf("load posts of reply matches failed: %w", err)
	}
	defer rows.Close()
	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}
	// Keep the order of the ids: the order of each post's best reply match.
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	sort.SliceStable(results, func(i, j int) bool { return pos[results[i].ID] < pos[results[j].ID] })
	for i := range results {
		results[i].Source = "post"
	}
	return results, nil
}

// replyOnlyPostIDs lists the posts that matched only through replies, once each, in the order
// of their best match (matches come best first).
func replyOnlyPostIDs(posts []models.SearchResult, matches []models.SearchReplyMatch) []string {
	seen := make(map[string]bool, len(posts))
	for _, p := range posts {
		seen[p.ID] = true
	}
	var ids []string
	for _, m := range matches {
		if !seen[m.PostID] {
			seen[m.PostID] = true
			ids = append(ids, m.PostID)
		}
	}
	return ids
}

// foldReplyMatches attaches each post's reply matches to its result, best first and at most
// maxReplyMatchesPerPost. A post's score becomes the better of its own and its best reply's,
// and its similarity the best of its own and all its matches'. A match whose post is not a
// result is dropped; the order of the results is kept.
func foldReplyMatches(posts []models.SearchResult, matches []models.SearchReplyMatch) []models.SearchResult {
	byPost := make(map[string][]models.SearchReplyMatch)
	for _, m := range matches {
		byPost[m.PostID] = append(byPost[m.PostID], m)
	}
	for i := range posts {
		ms := byPost[posts[i].ID]
		if len(ms) == 0 {
			continue
		}
		sort.SliceStable(ms, func(a, b int) bool { return ms[a].Score > ms[b].Score })
		for _, m := range ms {
			if m.Similarity != nil && (posts[i].Similarity == nil || *m.Similarity > *posts[i].Similarity) {
				v := *m.Similarity
				posts[i].Similarity = &v
			}
		}
		if ms[0].Score > posts[i].Score {
			posts[i].Score = ms[0].Score
		}
		if len(ms) > maxReplyMatchesPerPost {
			ms = ms[:maxReplyMatchesPerPost]
		}
		posts[i].MatchedReplies = ms
	}
	return posts
}
