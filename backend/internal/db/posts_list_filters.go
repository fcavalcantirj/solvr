package db

// needsHelpCondition selects posts that need help: in progress, or carrying a live reply
// migrated from a stuck approach (the contribution cutover keeps the approach status in the
// reply's provenance; native replies have no status). It is the canonical GET
// /v1/posts?needs_help=true filter that replaced the independent GET /v1/feed/stuck query
// stack (task idx 71) and reads no legacy table (task idx 76).
const needsHelpCondition = `(p.status = 'in_progress' OR EXISTS (
	SELECT 1 FROM replies nh
	WHERE nh.post_id = p.id AND nh.deleted_at IS NULL
	  AND nh.legacy_type = 'approach' AND nh.provenance->>'status' = 'stuck'))`

// appendNeedsHelpFilter adds needsHelpCondition to a posts list WHERE clause when enabled.
func appendNeedsHelpFilter(conditions *[]string, needsHelp bool) {
	if needsHelp {
		*conditions = append(*conditions, needsHelpCondition)
	}
}

// appendHasAnswerFilter adds the has_answer filter: a post has an answer when one of its live
// replies is in the answer bucket (posts_reply_counts.go), i.e. exactly when its answers_count is
// above zero. As EXISTS it is probed per candidate through idx_replies_post, so a page stops at
// its rows and the total semi-joins the replies once, where reading answers_count needed every
// post's reply counts in both (idx 77 slice 13: unanswered page 278 ms -> 0.7 ms, total 333 ->
// 169 ms, same 128,290 posts, at 200k posts and 1M replies).
func appendHasAnswerFilter(conditions *[]string, hasAnswer *bool) {
	if hasAnswer == nil {
		return
	}
	exists := "EXISTS (SELECT 1 FROM replies r WHERE r.post_id = p.id AND r.deleted_at IS NULL AND " + replyAnswerBucket + ")"
	if !*hasAnswer {
		exists = "NOT " + exists
	}
	*conditions = append(*conditions, exists)
}
