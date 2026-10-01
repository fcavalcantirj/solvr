package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// replySearchSource is one reply bucket that content_types can ask for (task idx 76): the
// buckets are the ones the post counts use (posts_reply_counts.go), so an "answer" hit is a
// reply the post counts as an answer and an "approach" hit one it counts as an approach.
type replySearchSource struct {
	contentType string // the content_types value, e.g. "answers"
	source      string // SearchResult.Source and Type, e.g. "answer"
	bucket      string // the reply bucket predicate over r
	status      string // the SQL status expression over r and p
}

var (
	answerReplySearch = replySearchSource{
		contentType: "answers", source: "answer", bucket: replyAnswerBucket,
		status: `CASE WHEN p.accepted_answer_id = r.id THEN 'accepted' ELSE '' END`,
	}
	approachReplySearch = replySearchSource{
		contentType: "approaches", source: "approach", bucket: replyApproachBucket,
		status: `COALESCE(r.provenance->>'status', '')`,
	}
)

// searchReplies full-text searches the live replies of one bucket. A reply is found only when
// its post passes the rule the post results of the same search use: not deleted, not a draft,
// pending or rejected, and public or of the viewer's family.
func (r *SearchRepository) searchReplies(ctx context.Context, src replySearchSource, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	args := []any{tsquery}
	argNum := 2
	visibility := searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)
	query := `
		SELECT
			r.id::text,
			'` + src.source + `' as type,
			ts_headline('english', r.body, to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as title,
			r.body as description,
			ts_headline('english', r.body, to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=80, MinWords=40, MaxFragments=1') as snippet,
			COALESCE(p.tags, ARRAY[]::text[]) as tags,
			` + src.status + ` as status,
			r.author_type,
			r.author_id,
			COALESCE(
				CASE WHEN r.author_type = 'human' THEN u.display_name
					 ELSE ag.display_name
				END,
				r.author_id
			) as author_name,
			ts_rank(r.search_document, to_tsquery('english', $1)) as score,
			(r.upvotes - r.downvotes) as vote_score,
			0 as answers_count,
			0 as approaches_count,
			0 as comments_count,
			0 as view_count,
			r.created_at,
			NULL::timestamptz as solved_at,
			NULL::float8 as similarity
		FROM replies r
		JOIN posts p ON p.id = r.post_id
		LEFT JOIN users u ON r.author_type = 'human' AND r.author_id = u.id::text
		LEFT JOIN agents ag ON r.author_type = 'agent' AND r.author_id = ag.id
		WHERE r.deleted_at IS NULL
		AND ` + src.bucket + `
		AND p.deleted_at IS NULL
		AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		AND ` + visibility + `
		AND r.search_document @@ to_tsquery('english', $1)
		ORDER BY score DESC
	`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, "Search.Replies", "replies", err)
		return nil, fmt.Errorf("search %s query failed: %w", src.contentType, err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Source = src.source
	}
	return results, nil
}
