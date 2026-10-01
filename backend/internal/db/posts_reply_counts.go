package db

// A post's contribution counts are read from its live replies (task idx 76), so the posts list
// and detail need no legacy table. Every live reply lands in exactly one bucket, so
// answers_count + approaches_count + comments_count = reply_count = the total of the post's
// reply list (GET /v1/posts/{id}/replies):
//   - approaches: the replies migrated as approaches;
//   - answers: top-level human/agent replies other than migrated approaches, comments and
//     progress notes, i.e. native replies and migrated answers and responses (has_answer);
//   - comments: every other live reply: child replies (incl. progress notes), migrated
//     comments and system replies such as moderation verdicts.
const (
	replyApproachBucket = `COALESCE(r.legacy_type, '') = 'approach'`
	replyAnswerBucket   = `(r.parent_reply_id IS NULL AND r.author_type <> 'system'
		AND COALESCE(r.legacy_type, '') NOT IN ('approach', 'comment', 'progress_note'))`

	// postReplyCountsJoin joins rc (ans, app, cmt) onto post p; a post without replies gets NULLs.
	postReplyCountsJoin = `
		LEFT JOIN (
			SELECT r.post_id,
				COUNT(*) FILTER (WHERE ` + replyAnswerBucket + `) AS ans,
				COUNT(*) FILTER (WHERE ` + replyApproachBucket + `) AS app,
				COUNT(*) FILTER (WHERE NOT ` + replyAnswerBucket + ` AND NOT (` + replyApproachBucket + `)) AS cmt
			FROM replies r
			WHERE r.deleted_at IS NULL
			GROUP BY r.post_id
		) rc ON rc.post_id = p.id`

	// postPageReplyCountsJoin joins the same rc onto each row of p from that post's own replies
	// (idx_replies_post), for a page whose rows are already chosen: its cost follows the page,
	// not the replies table. A post without replies gets zeros.
	postPageReplyCountsJoin = `
		LEFT JOIN LATERAL (
			SELECT
				COUNT(*) FILTER (WHERE ` + replyAnswerBucket + `) AS ans,
				COUNT(*) FILTER (WHERE ` + replyApproachBucket + `) AS app,
				COUNT(*) FILTER (WHERE NOT ` + replyAnswerBucket + ` AND NOT (` + replyApproachBucket + `)) AS cmt
			FROM replies r
			WHERE r.post_id = p.id AND r.deleted_at IS NULL
		) rc ON true`

	// postReplyCountColumns selects the buckets in the scan order of scanPostWithAuthorRows.
	postReplyCountColumns = `COALESCE(rc.ans, 0) as answers_count,
			COALESCE(rc.app, 0) as approaches_count,
			COALESCE(rc.cmt, 0) as comments_count`
)
