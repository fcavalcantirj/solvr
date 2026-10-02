package db

import (
	"context"
	"fmt"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Keyword search pages its results in two phases (spec.json idx 77 step 1: search indexes from
// measured query plans). The first statement ranks every match from the stored search documents
// and returns the page's post ids, their scores and the total; the snippets, authors and reply
// counts are then read for the page's posts only, with the page's reply anchors. Before, the
// keyword search built every match's snippet, author and counts (and every matching reply's
// snippet and author) and paged in Go.

// keywordPaged reports whether a search's results all come from one keyword ranking, so that
// rankKeywordPage can page them: no query embedding, no min_similarity floor (a keyword match has
// no similarity, so the floor keeps none of them), and the default knowledge search or posts only.
func keywordPaged(embedding []float32, opts models.SearchOptions) bool {
	if embedding != nil || opts.MinSimilarity > 0 {
		return false
	}
	for _, ct := range opts.ContentTypes {
		if ct != "posts" {
			return false
		}
	}
	return true
}

// searchKeywordPage is the keyword search's page: the default knowledge search when knowledge is
// true (a post found by its own text or a reply's, with its reply anchors), else posts by their
// own text. Its results, their order and the total are those of the full search paged in Go
// (searchKnowledge or searchPosts, then Search's stable sort by score).
func (r *SearchRepository) searchKeywordPage(ctx context.Context, tsquery string, opts models.SearchOptions, knowledge bool) ([]models.SearchResult, int, error) {
	ranked, total, err := r.rankKeywordPage(ctx, tsquery, opts, knowledge)
	if err != nil || len(ranked) == 0 {
		return []models.SearchResult{}, total, err
	}
	ids := make([]string, len(ranked))
	score := make(map[string]float64, len(ranked))
	for i, p := range ranked {
		ids[i] = p.id
		score[p.id] = p.score
	}
	posts, err := r.loadSearchPosts(ctx, ids, tsquery, opts)
	if err != nil {
		return nil, 0, err
	}
	for i := range posts {
		posts[i].Score = score[posts[i].ID]
	}
	if !knowledge {
		return posts, total, nil
	}
	matches, err := r.searchReplyMatches(ctx, nil, tsquery, opts, ids)
	if err != nil {
		return nil, 0, err
	}
	return foldReplyMatches(posts, matches), total, nil
}

type rankedPost struct {
	id    string
	score float64
}

// rankKeywordPage ranks every keyword match and returns the page's posts, best first, and how
// many posts matched. A post's score is the better of its own rank and its best reply's
// (foldReplyMatches). Equal scores keep the order the full search gives them: posts found by
// their own text first, in keywordOwnOrder, then posts found only through replies, in the order
// of their best reply (searchReplyMatches orders matches by score, then created_at, then id).
func (r *SearchRepository) rankKeywordPage(ctx context.Context, tsquery string, opts models.SearchOptions, knowledge bool) ([]rankedPost, int, error) {
	limit, offset := searchPage(opts)
	args := []any{tsquery}
	argNum := 2
	own := `SELECT p.id, ts_rank(p.search_document, to_tsquery('english', $1)) AS score,
			p.created_at, p.updated_at, (p.upvotes - p.downvotes) AS vote_score
		  FROM posts p
		 WHERE p.search_document @@ to_tsquery('english', $1) AND ` + searchablePostRule +
		` AND ` + searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)
	filters, args, argNum := buildSearchFilters(opts, args, argNum)
	own += " " + filters

	with := "own AS (" + own + ")"
	id, score, from, tail := "o.id", "o.score", "own o", ""
	if knowledge {
		rm := `SELECT DISTINCT ON (r.post_id) r.post_id, ts_rank(r.search_document, to_tsquery('english', $1)) AS score,
				r.created_at, r.id
			  FROM replies r JOIN posts p ON p.id = r.post_id
			 WHERE r.search_document @@ to_tsquery('english', $1) AND r.deleted_at IS NULL
			   AND r.author_type <> 'system' AND ` + replyMatchPostRule +
			` AND ` + searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)
		filters, args, argNum = buildSearchFilters(opts, args, argNum)
		rm += " " + filters + " ORDER BY r.post_id, score DESC, r.created_at, r.id"
		with += ", rm AS (" + rm + ")"
		id, score, from, tail = "COALESCE(o.id, m.post_id)", "GREATEST(o.score, m.score)",
			"own o FULL JOIN rm m ON m.post_id = o.id", ", m.created_at, m.id"
	}
	args = append(args, offset, limit)
	query := `WITH ` + with + `,
		cand AS (
			SELECT ` + id + ` AS id, ` + score + ` AS score,
			       row_number() OVER (ORDER BY ` + score + ` DESC, o.id IS NULL, ` + keywordOwnOrder(opts.Sort) + tail + `) AS ord
			  FROM ` + from + `)
		SELECT t.total, c.id::text, c.score::float8
		  FROM (SELECT COUNT(*)::int AS total FROM cand) t
		  LEFT JOIN cand c ON c.ord > $` + strconv.Itoa(argNum) + ` AND c.ord <= $` + strconv.Itoa(argNum) + ` + $` + strconv.Itoa(argNum+1) + `
		 ORDER BY c.ord`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "Search.KeywordPage", "posts", err)
		return nil, 0, fmt.Errorf("keyword search page query failed: %w", err)
	}
	defer rows.Close()
	var (
		page  []rankedPost
		total int
	)
	for rows.Next() {
		var (
			id    *string
			score *float64
		)
		if err := rows.Scan(&total, &id, &score); err != nil {
			return nil, 0, fmt.Errorf("failed to scan keyword search page: %w", err)
		}
		if id != nil {
			page = append(page, rankedPost{id: *id, score: *score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating keyword search page: %w", err)
	}
	return page, total, nil
}

// keywordOwnOrder is getSearchOrderBy over the own matches o: the order searchPosts returns the
// posts found by their own text in, before Search's stable sort by score.
func keywordOwnOrder(sort string) string {
	switch sort {
	case "newest":
		return "o.created_at DESC, o.id"
	case "votes":
		return "o.vote_score DESC, o.created_at DESC, o.id"
	case "activity":
		return "o.updated_at DESC, o.created_at DESC, o.id"
	default:
		return "o.score DESC, o.created_at DESC, o.id"
	}
}
