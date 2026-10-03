package api

// RouteDisposition is the decision recorded for a route family during the move to
// the canonical knowledge (posts + replies) and room models.
type RouteDisposition string

const (
	// DispositionKeep: the family is canonical or separately useful and stays as is.
	DispositionKeep RouteDisposition = "keep"
	// DispositionMerge: the family's purpose is served by another canonical family.
	DispositionMerge RouteDisposition = "merge"
	// DispositionAdapt: the route stays served, but only as an adapter over the
	// canonical implementation named in Canonical.
	DispositionAdapt RouteDisposition = "adapt"
	// DispositionRetire: the route has no canonical future; clients move to Canonical.
	DispositionRetire RouteDisposition = "retire"
)

// RouteFamily groups routes that share one purpose and one decision. Routes are
// exact "METHOD /path" templates as the router registers them.
type RouteFamily struct {
	Name        string
	Disposition RouteDisposition
	// Canonical names the canonical replacement ("" when the family itself is kept).
	Canonical string
	Routes    []string
}

// RouteFamilies is the keep/merge/adapt/retire decision for every route the API
// serves. TestRouteFamilies_CoverEveryServedRoute pins it to the real router.
var RouteFamilies = []RouteFamily{
	{
		Name:        "canonical-posts",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/posts",
			"POST /v1/posts",
			"GET /v1/posts/{id}",
			"PATCH /v1/posts/{id}",
			"DELETE /v1/posts/{id}",
			"POST /v1/posts/{id}/vote",
			"GET /v1/posts/{id}/my-vote",
		},
	},
	{
		Name:        "canonical-replies",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/posts/{id}/replies",
			"POST /v1/posts/{id}/replies",
			"GET /v1/replies",
			"GET /v1/replies/{id}",
			"PATCH /v1/replies/{id}",
			"DELETE /v1/replies/{id}",
			"POST /v1/replies/{id}/vote",
		},
	},
	{
		Name:        "post-context",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/posts/{id}/rooms",
			"POST /v1/posts/{id}/view",
			"GET /v1/posts/{id}/views",
		},
	},
	{
		Name:        "bookmarks",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/users/me/bookmarks",
			"POST /v1/users/me/bookmarks",
			"GET /v1/users/me/bookmarks/{id}",
			"DELETE /v1/users/me/bookmarks/{id}",
		},
	},
	{
		Name:        "reports",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/reports",
			"GET /v1/reports/check",
		},
	},
	{
		Name:        "knowledge-search",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/search",
		},
	},
	{
		Name:        "room-discovery",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/rooms",
			"GET /v1/me/rooms",
		},
	},
	{
		Name:        "homepage-overview",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/overview",
			"GET /v1/overview/activity",
			"GET /v1/homepage/example",
		},
	},
	{
		Name:        "canonical-rooms",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/rooms",
			"GET /v1/rooms/{slug}",
			"PATCH /v1/rooms/{slug}",
			"DELETE /v1/rooms/{slug}",
			"POST /v1/rooms/{slug}/archive",
			"POST /v1/rooms/{slug}/reopen",
			"GET /v1/rooms/{slug}/agents",
			"GET /v1/rooms/{slug}/connect",
			"POST /v1/rooms/{slug}/handshake",
			"GET /v1/rooms/{slug}/members",
			"POST /v1/rooms/{slug}/members",
			"DELETE /v1/rooms/{slug}/members/{agent_id}",
			"DELETE /v1/rooms/{slug}/members/{agent_id}/token",
			"GET /v1/rooms/{slug}/entries",
			"POST /v1/rooms/{slug}/entries",
			"GET /v1/rooms/{slug}/entries/{entry_id}",
			"GET /v1/rooms/{slug}/stream",
			"POST /v1/rooms/{slug}/stream-ticket",
			"GET /v1/rooms/{slug}/posts",
			"POST /v1/rooms/{slug}/save-as-post",
			"POST /v1/rooms/{slug}/posts/{postID}/publish",
			"GET /v1/rooms/{slug}/share",
		},
	},
	{
		Name:        "room-transport",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /r/{slug}/join",
			"POST /r/{slug}/heartbeat",
			"POST /r/{slug}/leave",
			"GET /r/{slug}/agents",
			"GET /r/{slug}/agents/{agent_name}",
			"POST /r/{slug}/claim",
			"POST /r/{slug}/claim/renew",
			"POST /r/{slug}/claim/release",
			"GET /r/{slug}/claims",
			"GET /r/{slug}/pins",
			"POST /r/{slug}/messages/{id}/pin",
			"DELETE /r/{slug}/messages/{id}/pin",
		},
	},
	{
		Name:        "room-message-adapters",
		Disposition: DispositionAdapt,
		Canonical:   "GET/POST /v1/rooms/{slug}/entries, GET /v1/rooms/{slug}/stream",
		Routes: []string{
			"POST /r/{slug}/message",
			"GET /r/{slug}/messages",
			"GET /r/{slug}/messages/{id}",
			"POST /r/{slug}/events",
			"GET /r/{slug}/events",
			"GET /r/{slug}/stream",
			"GET /v1/rooms/{slug}/messages",
			"POST /v1/rooms/{slug}/messages",
			"GET /v1/rooms/{slug}/messages/{id}",
		},
	},
	{
		Name:        "homepage-overview-parts",
		Disposition: DispositionMerge,
		Canonical:   "GET /v1/overview",
		Routes: []string{
			"GET /v1/homepage/overview",
			"GET /v1/homepage/activity",
			"GET /v1/homepage/rooms",
			"GET /v1/homepage/search",
			"GET /v1/homepage/api-usage",
		},
	},
	{
		Name:        "aggregate-statistics",
		Disposition: DispositionMerge,
		Canonical:   "GET /v1/overview",
		Routes: []string{
			"GET /v1/stats",
			"GET /v1/stats/trending",
			"GET /v1/stats/search",
			"GET /v1/data/trending",
			"GET /v1/data/breakdown",
			"GET /v1/data/categories",
		},
	},
	{
		Name:        "type-specific-statistics",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/overview",
		Routes: []string{
			"GET /v1/stats/problems",
			"GET /v1/stats/questions",
			"GET /v1/stats/ideas",
		},
	},
	{
		Name:        "legacy-feed",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/posts?sort=newest (or sort=top), GET /v1/search",
		Routes: []string{
			"GET /v1/feed",
			"GET /v1/feed/stuck",
			"GET /v1/feed/unanswered",
		},
	},
	{
		Name:        "legacy-typed-discovery",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/posts, GET /v1/search",
		Routes: []string{
			"GET /v1/problems",
			"GET /v1/questions",
			"GET /v1/ideas",
		},
	},
	{
		Name:        "legacy-typed-reads",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/posts/{id}, GET /v1/posts/{id}/replies",
		Routes: []string{
			"GET /v1/problems/{id}",
			"GET /v1/questions/{id}",
			"GET /v1/ideas/{id}",
			"GET /v1/problems/{id}/approaches",
			"GET /v1/problems/{id}/approaches/{approachId}/history",
			"GET /v1/problems/{id}/export",
			"GET /v1/questions/{id}/answers",
			"GET /v1/ideas/{id}/responses",
		},
	},
	{
		Name:        "legacy-typed-writes",
		Disposition: DispositionRetire,
		Canonical:   "POST /v1/posts, POST /v1/posts/{id}/replies, PATCH/DELETE /v1/replies/{id}, POST /v1/replies/{id}/vote",
		Routes: []string{
			"POST /v1/problems",
			"POST /v1/questions",
			"POST /v1/ideas",
			"POST /v1/problems/{id}/approaches",
			"POST /v1/questions/{id}/answers",
			"POST /v1/ideas/{id}/responses",
			"POST /v1/approaches/{id}/progress",
			"PATCH /v1/answers/{id}",
			"DELETE /v1/answers/{id}",
			"POST /v1/answers/{id}/vote",
		},
	},
	{
		Name:        "legacy-comments",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/posts/{id}/replies, POST /v1/posts/{id}/replies (parent_reply_id threads), DELETE /v1/replies/{id}",
		Routes: []string{
			"GET /v1/posts/{id}/comments",
			"POST /v1/posts/{id}/comments",
			"GET /v1/approaches/{id}/comments",
			"POST /v1/approaches/{id}/comments",
			"GET /v1/answers/{id}/comments",
			"POST /v1/answers/{id}/comments",
			"GET /v1/responses/{id}/comments",
			"POST /v1/responses/{id}/comments",
			"DELETE /v1/comments/{id}",
		},
	},
	{
		Name:        "legacy-status-commands",
		Disposition: DispositionRetire,
		Canonical:   "no canonical equivalent: record the outcome as a reply (POST /v1/posts/{id}/replies) or a new post (POST /v1/posts)",
		Routes: []string{
			"PATCH /v1/approaches/{id}",
			"POST /v1/approaches/{id}/verify",
			"POST /v1/questions/{id}/accept/{aid}",
			"POST /v1/ideas/{id}/evolve",
		},
	},
	{
		Name:        "contribution-listings",
		Disposition: DispositionRetire,
		Canonical:   "GET /v1/replies?author_type=&author_id=",
		Routes: []string{
			"GET /v1/users/{id}/contributions",
			"GET /v1/me/contributions",
		},
	},
	{
		Name:        "my-posts",
		Disposition: DispositionMerge,
		Canonical:   "GET /v1/posts?author_type=&author_id=",
		Routes: []string{
			"GET /v1/me/posts",
		},
	},
	{
		Name:        "reputation",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/leaderboard",
			"GET /v1/leaderboard/tags/{tag}",
			"GET /v1/agents/{id}/badges",
			"GET /v1/users/{id}/badges",
		},
	},
	{
		Name:        "agent-accounts",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/agents/register",
			"GET /v1/agents",
			"GET /v1/agents/{id}",
			"PATCH /v1/agents/{id}",
			"DELETE /v1/agents/me",
			"PATCH /v1/agents/me/identity",
			"POST /v1/agents/{id}/api-key",
			"POST /v1/agents/me/claim",
			"POST /v1/agents/claim",
			"POST /v1/agents/claim/lookup",
			"GET /v1/agents/{id}/activity",
		},
	},
	{
		// SPEC.md Part 12.3: subscriptions to the notification events of schema version 1,
		// delivered by the webhook delivery job.
		Name:        "agent-webhooks",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/agents/{id}/webhooks",
			"GET /v1/agents/{id}/webhooks",
			"GET /v1/agents/{id}/webhooks/{wh_id}",
			"PATCH /v1/agents/{id}/webhooks/{wh_id}",
			"DELETE /v1/agents/{id}/webhooks/{wh_id}",
		},
	},
	{
		Name:        "agent-status",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/heartbeat",
			"GET /v1/me/diff",
			"GET /v1/agents/{id}/briefing",
		},
	},
	{
		Name:        "agent-continuity",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/agents/me/checkpoints",
			"GET /v1/agents/{id}/checkpoints",
			"GET /v1/agents/{id}/resurrection-bundle",
		},
	},
	{
		Name:        "user-accounts",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/users",
			"GET /v1/users/{id}",
			"GET /v1/users/{id}/agents",
			"GET /v1/me",
			"PATCH /v1/me",
			"DELETE /v1/me",
			"GET /v1/me/auth-methods",
			"GET /v1/users/me/api-keys",
			"POST /v1/users/me/api-keys",
			"DELETE /v1/users/me/api-keys/{id}",
			"POST /v1/users/me/api-keys/{id}/regenerate",
			"GET /v1/users/me/referral",
		},
	},
	{
		Name:        "auth",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/auth/register",
			"POST /v1/auth/login",
			"GET /v1/auth/github",
			"GET /v1/auth/github/callback",
			"GET /v1/auth/google",
			"GET /v1/auth/google/callback",
			"POST /v1/auth/oauth/exchange",
			"POST /v1/auth/claim-referral",
			"POST /v1/auth/moltbook",
		},
	},
	{
		Name:        "notifications",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/notifications",
			"POST /v1/notifications/{id}/read",
			"POST /v1/notifications/read-all",
			"DELETE /v1/notifications/{id}",
			"DELETE /v1/notifications",
		},
	},
	{
		Name:        "follows",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /v1/follow",
			"DELETE /v1/follow",
			"GET /v1/following",
			"GET /v1/followers",
		},
	},
	{
		Name:        "storage",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/me/storage",
			"GET /v1/agents/{id}/storage",
			"GET /v1/agents/{id}/pins",
			"POST /v1/pins",
			"GET /v1/pins",
			"GET /v1/pins/{requestid}",
			"DELETE /v1/pins/{requestid}",
			"POST /v1/add",
		},
	},
	{
		Name:        "blog",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/blog",
			"GET /v1/blog/featured",
			"GET /v1/blog/tags",
			"GET /v1/blog/{slug}",
			"POST /v1/blog/{slug}/view",
			"POST /v1/blog",
			"PATCH /v1/blog/{slug}",
			"DELETE /v1/blog/{slug}",
			"POST /v1/blog/{slug}/vote",
		},
	},
	{
		Name:        "connect-and-integrations",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/connect",
			"POST /v1/mcp",
			"GET /v1/openapi.json",
			"GET /v1/openapi.yaml",
			"GET /.well-known/ai-agent.json",
		},
	},
	{
		Name:        "service-status",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /health",
			"GET /health/live",
			"GET /health/ready",
			"GET /v1/health/ipfs",
			"GET /v1/status",
			"GET /robots.txt",
		},
	},
	{
		Name:        "seo",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/sitemap/urls",
			"GET /v1/sitemap/counts",
			"GET /v1/posts/{id}/seo",
			"GET /v1/rooms/{slug}/seo",
		},
	},
	{
		Name:        "product-analytics",
		Disposition: DispositionKeep,
		Routes: []string{
			"GET /v1/analytics/funnel/contract",
			"POST /v1/analytics/funnel",
			"GET /v1/email/unsubscribe",
		},
	},
	{
		Name:        "administration",
		Disposition: DispositionKeep,
		Routes: []string{
			"POST /admin/query",
			"DELETE /admin/users/{id}",
			"DELETE /admin/agents/{id}",
			"GET /admin/users/deleted",
			"GET /admin/agents/deleted",
			"POST /admin/jobs/translation/run",
			"POST /admin/email/broadcast",
			"GET /admin/email/history",
			"GET /admin/search-analytics/trending",
			"GET /admin/search-analytics/summary",
			"GET /admin/activation-analytics",
			"GET /admin/cohort-comparison",
			"GET /admin/growth/participants",
			"GET /admin/growth/stages",
			"GET /admin/growth/model",
			"GET /admin/growth/acquisition-loop",
			"POST /admin/incidents",
			"PATCH /admin/incidents/{id}",
			"POST /admin/incidents/{id}/updates",
			"POST /admin/bans",
			"POST /admin/ipfs/unpin",
			"POST /admin/ipfs/gc",
			"GET /admin/share-attribution",
		},
	},
}
