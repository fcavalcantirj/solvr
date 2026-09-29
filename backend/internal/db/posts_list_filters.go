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
