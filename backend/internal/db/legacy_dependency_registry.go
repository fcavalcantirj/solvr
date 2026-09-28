package db

func pending(action LegacyDependencyAction, note string) LegacyDependencyDisposition {
	return LegacyDependencyDisposition{Action: action, Note: note}
}

func keep(note string) LegacyDependencyDisposition {
	return LegacyDependencyDisposition{Action: LegacyActionKeep, Note: note}
}

// LegacyDependencyDispositions is the decision for every discovered legacy dependency. The
// tests pin it both ways to the live catalog, the source tree and the declared lists: a new
// dependency without an entry fails, and so does an entry for one that is gone. Flip Done
// only in the change that implements and verifies the replacement or the retirement.
var LegacyDependencyDispositions = map[string]LegacyDependencyDisposition{
	// Legacy tables: dropped after the verified contribution migration (MigrateContributions).
	"table:approaches":             pending(LegacyActionRetire, "rows become replies (legacy_type approach); drop after verified migration"),
	"table:answers":                pending(LegacyActionRetire, "rows become replies (legacy_type answer); drop after verified migration"),
	"table:responses":              pending(LegacyActionRetire, "rows become replies (legacy_type response); drop after verified migration"),
	"table:comments":               pending(LegacyActionRetire, "rows become top-level or child replies; drop after verified migration"),
	"table:approach_relationships": pending(LegacyActionRetire, "drop once relation:approach-relationships is remapped onto replies"),
	"table:progress_notes":         pending(LegacyActionRetire, "drop once relation:progress-notes is remapped onto replies"),

	// Catalog objects outside the legacy tables' own indexes and constraints.
	"function:hybrid_search_approaches":         pending(LegacyActionRefactor, "replace with one replies search function over a replies embedding"),
	"function:hybrid_search_answers":            pending(LegacyActionRefactor, "replace with one replies search function over a replies embedding"),
	"index:approaches.idx_approaches_embedding": pending(LegacyActionRefactor, "replies need an embedding column and an HNSW index before this drops"),
	"index:answers.idx_answers_embedding":       pending(LegacyActionRefactor, "replies need an embedding column and an HNSW index before this drops"),
	"check:posts.posts_type_check":              pending(LegacyActionRefactor, "legacy post types stay readable in transition; narrow to 'post' at cleanup"),
	"check:votes.votes_target_type_check":       pending(LegacyActionRemap, "votes retarget to 'reply'; drop approach/answer/response after remap"),
	"check:reports.reports_target_type_check":   pending(LegacyActionRemap, "reports retarget to 'reply'; drop legacy target types after remap"),
	"check:flags.flags_target_type_check":       pending(LegacyActionRemap, "flags are not remapped yet; retarget to 'reply' like reports, then narrow"),
	"check:replies.replies_legacy_type_check":   keep("provenance of migrated replies; the compact legacy mapping outlives cleanup"),
	"column:posts.accepted_answer_id":           pending(LegacyActionRemap, "RemapAcceptedAnswerReferences points it at the reply migrated from the answer"),

	// Declared relationships (task step 2).
	"relation:votes":                      pending(LegacyActionRemap, "RemapContributionVotesAndReports retargets contribution votes to replies"),
	"relation:bookmarks":                  keep("bookmarks hold only post_id and posts keep their UUIDs, so no identity moves"),
	"relation:reports":                    pending(LegacyActionRemap, "RemapContributionVotesAndReports retargets contribution and comment reports"),
	"relation:notifications":              pending(LegacyActionRemap, "stored links may name legacy pages or anchors; rewrite, send nothing new"),
	"relation:accepted-answer-provenance": pending(LegacyActionRemap, "is_accepted kept in reply provenance; accepted_answer_id remapped to the reply"),
	"relation:approach-relationships":     pending(LegacyActionRemap, "from/to approach ids must resolve to replies through legacy_id; not built yet"),
	"relation:progress-notes":             pending(LegacyActionRemap, "notes hang off approach_id; carry them onto the migrated reply; not built yet"),
	"relation:verification-records":       pending(LegacyActionRemap, "approach status/outcome/solution are kept in reply body and provenance"),
	"relation:translations":               keep("translation state lives on posts (original_* columns) whose UUIDs stay"),
	"relation:archived-cids":              pending(LegacyActionRemap, "approach archived_cid is copied into reply provenance; post CIDs stay"),

	// Declared workers and features (task step 3).
	"feature:briefing":            pending(LegacyActionRefactor, "briefing reads approaches/answers/comments and legacy post types"),
	"feature:badges":              pending(LegacyActionRefactor, "milestones count solved problems and accepted answers; keep earned badges, award none from backfill"),
	"feature:leaderboards":        pending(LegacyActionRefactor, "leaderboards count answers and problems; move to posts and replies"),
	"feature:reputation":          pending(LegacyActionRefactor, "sql_builder scores answers/responses/comments; preserve earned totals as history"),
	"feature:crystallization":     pending(LegacyActionRefactor, "pins solved problems; redefine eligibility on canonical post states"),
	"feature:forgetting":          pending(LegacyActionRetire, "approach forget_after/archived_at lifecycle has no reply equivalent"),
	"feature:moderation":          pending(LegacyActionRefactor, "moderation and cmd/moderate-existing cover comments; must cover replies"),
	"feature:duplicate-detection": pending(LegacyActionRefactor, "similarity check in services/moderation.go must run on posts and replies"),
	"feature:embedding-workers":   pending(LegacyActionRefactor, "backfill embeds approaches/answers; replies have no embedding column yet"),

	// Scheduled jobs (cmd/api/main.go) and LISTEN consumers.
	"job:CleanupJob":               keep("prunes claim tokens and idempotency keys; touches no legacy table"),
	"job:HealthCheckJob":           keep("checks API, database and IPFS health; touches no legacy table"),
	"job:PresenceReaperJob":        keep("reaps room presence; the room model is not the knowledge model"),
	"job:TranslationJob":           keep("translates posts through original_* columns; posts keep their UUIDs"),
	"job:CrystallizationJob":       pending(LegacyActionRefactor, "see feature:crystallization; runs on posts.go legacy queries"),
	"job:StaleContentJob":          pending(LegacyActionRetire, "warns/abandons approaches and marks ideas dormant: legacy lifecycles"),
	"job:AutoSolveJob":             pending(LegacyActionRetire, "auto-solves problems from succeeded approaches; no status workflow remains"),
	"consumer:RoomEntryChannel":    keep("room entry fan-out; independent of the knowledge model"),
	"consumer:RoomPresenceChannel": keep("room presence fan-out; independent of the knowledge model"),
	"consumer:RoomAccessChannel":   keep("room access revocation; independent of the knowledge model"),
	"consumer:OverviewChannel":     keep("homepage overview refresh signal; the query it triggers is listed separately"),

	// Application queries (non-test Go files whose SQL names a legacy table or type).
	"code:internal/db/approaches.go":               pending(LegacyActionRetire, "legacy approach repository incl. progress notes; replaced by replies"),
	"code:internal/db/answers.go":                  pending(LegacyActionRetire, "legacy answer repository; replaced by replies"),
	"code:internal/db/responses.go":                pending(LegacyActionRetire, "legacy response repository; replaced by replies"),
	"code:internal/db/comments.go":                 pending(LegacyActionRetire, "legacy comment repository; replaced by replies"),
	"code:internal/db/approach_relationships.go":   pending(LegacyActionRetire, "legacy approach relationship repository; see relation:approach-relationships"),
	"code:internal/db/problems.go":                 pending(LegacyActionRetire, "type-filtered problem listing; canonical listing is posts"),
	"code:internal/db/questions.go":                pending(LegacyActionRetire, "type-filtered question listing; canonical listing is posts"),
	"code:internal/db/ideas.go":                    pending(LegacyActionRetire, "type-filtered idea listing and evolution; canonical listing is posts"),
	"code:internal/db/contribution_migration.go":   pending(LegacyActionRetire, "the migration reads legacy tables by design; remove with them"),
	"code:internal/db/posts.go":                    pending(LegacyActionRefactor, "post reads join approach/answer/comment counts; count replies"),
	"code:internal/db/posts_list_filters.go":       pending(LegacyActionRefactor, "list filter tests approaches; filter on replies"),
	"code:internal/db/search.go":                   pending(LegacyActionRefactor, "searches approaches/answers/comments; search posts and replies"),
	"code:internal/db/visibility.go":               pending(LegacyActionRefactor, "visibility predicate covers approaches; cover replies"),
	"code:internal/db/stats.go":                    pending(LegacyActionRefactor, "platform stats count legacy tables and types; count posts and replies"),
	"code:internal/db/stats_questions.go":          pending(LegacyActionRefactor, "question stats count answers; derive from posts and replies"),
	"code:internal/db/homepage_overview.go":        pending(LegacyActionRefactor, "overview counts approaches/answers/responses; count replies"),
	"code:internal/db/agents.go":                   pending(LegacyActionRefactor, "agent stats count legacy contributions and types; count replies"),
	"code:internal/db/users.go":                    pending(LegacyActionRefactor, "user stats count answers/responses and types; count replies"),
	"code:internal/db/inferred_specialties.go":     pending(LegacyActionRefactor, "specialties are inferred using answers/approaches; use replies"),
	"code:internal/db/leaderboard.go":              pending(LegacyActionRefactor, "see feature:leaderboards"),
	"code:internal/db/leaderboard_tags.go":         pending(LegacyActionRefactor, "see feature:leaderboards (per-tag)"),
	"code:internal/db/briefing.go":                 pending(LegacyActionRefactor, "see feature:briefing"),
	"code:internal/db/briefing_platform.go":        pending(LegacyActionRefactor, "see feature:briefing (platform section)"),
	"code:internal/db/briefing_recommendations.go": pending(LegacyActionRefactor, "see feature:briefing (recommendations)"),
	"code:internal/db/briefing_diff.go":            pending(LegacyActionRefactor, "see feature:briefing (diff counts legacy post types)"),
	"code:internal/db/resurrection.go":             pending(LegacyActionRefactor, "resurrection bundle reads approaches and legacy types"),
	"code:internal/db/sitemap.go":                  pending(LegacyActionRefactor, "sitemap emits legacy type URLs; emit /posts URLs, keep redirects"),
	"code:internal/db/auto_solve.go":               pending(LegacyActionRetire, "see job:AutoSolveJob"),
	"code:internal/db/stale_content.go":            pending(LegacyActionRetire, "see job:StaleContentJob and feature:forgetting"),
	"code:internal/reputation/sql_builder.go":      pending(LegacyActionRefactor, "see feature:reputation"),
	"code:cmd/backfill-embeddings/main.go":         pending(LegacyActionRefactor, "see feature:embedding-workers"),
	"code:cmd/moderate-existing/main.go":           pending(LegacyActionRefactor, "see feature:moderation"),
}
