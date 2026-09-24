package db

// needsHelpCondition selects posts that need help: in progress, or carrying a live stuck
// approach. It is the canonical GET /v1/posts?needs_help=true filter that replaced the
// independent GET /v1/feed/stuck query stack (task idx 71).
const needsHelpCondition = `(p.status = 'in_progress' OR EXISTS (
	SELECT 1 FROM approaches ap
	WHERE ap.problem_id = p.id AND ap.status = 'stuck' AND ap.deleted_at IS NULL))`

// appendNeedsHelpFilter adds needsHelpCondition to a posts list WHERE clause when enabled.
func appendNeedsHelpFilter(conditions *[]string, needsHelp bool) {
	if needsHelp {
		*conditions = append(*conditions, needsHelpCondition)
	}
}
