package traffic

import "strings"

// ContentGroups is the kind of page a page view is counted under (SPEC.md 27.7, content_group),
// the closed list of 000146's page_view_events_content_group_check.
var ContentGroups = []string{"home", "connect", "post", "room", "transcript", "agent", "user", "blog", "docs",
	"guide", "collection", "account", "other"}

// accountSections are the pages only a signed-in person, or a person signing in, uses.
var accountSections = map[string]bool{
	"login": true, "join": true, "dashboard": true, "settings": true, "notifications": true, "pins": true,
	"referrals": true, "admin": true, "claim": true, "auth": true, "email": true,
}

// docsSections are the reference pages outside /docs itself.
var docsSections = map[string]bool{"api-docs": true, "skill": true, "mcp": true, "amcp": true, "ipfs": true}

// ContentGroupForPath is the kind of page a path is. It is the web client's contentGroupForPath
// (frontend/lib/analytics-paths.ts), held equal to it by contract/content-groups.json: the page
// views the API counts and the hits Google sees use one rule.
func ContentGroupForPath(path string) string {
	if path == "" {
		return "other"
	}
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	var segments []string
	for s := range strings.SplitSeq(path, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	if len(segments) == 0 {
		return "home"
	}
	section := segments[0]
	second, third := "", ""
	if len(segments) > 1 {
		second = segments[1]
	}
	if len(segments) > 2 {
		third = segments[2]
	}

	switch section {
	case "connect":
		return "connect"
	case "posts":
		switch {
		// The archive (/posts/page/{n}) lists posts, like the collection it pages through.
		case second == "" || second == "page":
			return "collection"
		// The composer and the editor are a signed-in person's pages, not a post being read.
		case second == "new" || third == "edit":
			return "account"
		default:
			return "post"
		}
	case "rooms":
		if second == "" {
			return "collection"
		}
		if third == "history" {
			return "transcript"
		}
		return "room"
	case "agents":
		if second == "" {
			return "collection"
		}
		return "agent"
	case "users":
		if second == "" {
			return "collection"
		}
		return "user"
	case "leaderboard":
		return "collection"
	case "blog":
		if second == "create" {
			return "account"
		}
		return "blog"
	case "docs":
		if second == "guides" {
			return "guide"
		}
		return "docs"
	}
	if docsSections[section] {
		return "docs"
	}
	if accountSections[section] {
		return "account"
	}
	return "other"
}
