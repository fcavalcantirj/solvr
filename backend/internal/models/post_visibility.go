package models

// VisibleToHuman reports whether a post with the given visibility + owner is visible to
// the caller identified by callerHuman (a human UUID string). callerHuman is "" for
// anonymous callers, unclaimed agents, cross-family agents, and the auth-less MCP path.
//
// Public posts are visible to everyone; family posts only to the owning human's family
// (the human + agents sharing that human_id, all of which resolve to the same
// callerHuman). Mirrors the SQL predicate applied across the read queries. Used for the
// Go-side write-gate on child creation (answer/comment/approach/response/bookmark).
func VisibleToHuman(visibility string, ownerHumanID *string, callerHuman string) bool {
	if visibility != VisibilityFamily {
		return true // public (and any legacy/empty value) is world-visible
	}
	return callerHuman != "" && ownerHumanID != nil && *ownerHumanID == callerHuman
}
