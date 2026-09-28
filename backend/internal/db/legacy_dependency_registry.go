package db

func pending(action LegacyDependencyAction, note string) LegacyDependencyDisposition {
	return LegacyDependencyDisposition{Action: action, Note: note}
}

// done marks a disposition whose replacement is implemented and verified by a test.
func done(action LegacyDependencyAction, note string) LegacyDependencyDisposition {
	return LegacyDependencyDisposition{Action: action, Done: true, Note: note}
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
	"function:hybrid_search_approaches":         done(LegacyActionRefactor, "replaced by hybrid_search_replies (000110); no Go code calls it; drops with its table"),
	"function:hybrid_search_answers":            done(LegacyActionRefactor, "replaced by hybrid_search_replies (000110); no Go code calls it; drops with its table"),
	"index:approaches.idx_approaches_embedding": done(LegacyActionRefactor, "replies.embedding + HNSW idx_replies_embedding (000110); the cutover copies the vectors"),
	"index:answers.idx_answers_embedding":       done(LegacyActionRefactor, "replies.embedding + HNSW idx_replies_embedding (000110); the cutover copies the vectors"),
	"check:posts.posts_type_check":              pending(LegacyActionRefactor, "legacy post types stay readable in transition; narrow to 'post' at cleanup"),
	"check:votes.votes_target_type_check":       pending(LegacyActionRemap, "votes retarget to 'reply'; drop approach/answer/response after remap"),
	"check:reports.reports_target_type_check":   pending(LegacyActionRemap, "reports retarget to 'reply'; drop legacy target types after remap"),
	"check:flags.flags_target_type_check":       pending(LegacyActionRemap, "RemapLegacyRelations retargets flags to 'reply' (000109); drop legacy target types at cleanup"),
	"check:replies.replies_legacy_type_check":   keep("provenance of migrated replies; the compact legacy mapping outlives cleanup"),
	"column:posts.accepted_answer_id":           done(LegacyActionRemap, "RemapAcceptedAnswerReferences points it at the reply migrated from the answer"),

	// Declared relationships (task step 2).
	"relation:votes":                      done(LegacyActionRemap, "RemapContributionVotesAndReports retargets contribution votes to replies"),
	"relation:bookmarks":                  keep("bookmarks hold only post_id and posts keep their UUIDs, so no identity moves"),
	"relation:reports":                    done(LegacyActionRemap, "RemapContributionVotesAndReports retargets contribution and comment reports"),
	"relation:notifications":              done(LegacyActionRemap, "RemapLegacyRelations rewrites stored links to /posts/<post>#<reply>; sends nothing new"),
	"relation:accepted-answer-provenance": done(LegacyActionRemap, "is_accepted kept in reply provenance; accepted_answer_id remapped to the reply"),
	"relation:approach-relationships":     done(LegacyActionRemap, "kept in the from-reply's provenance with both ends as reply and approach ids"),
	"relation:progress-notes":             done(LegacyActionRemap, "each note becomes a child reply (legacy_type progress_note) of its approach's reply"),
	"relation:verification-records":       done(LegacyActionRemap, "approach status/outcome/solution are kept in reply body and provenance"),
	"relation:translations":               keep("translation state lives on posts (original_* columns) whose UUIDs stay"),
	"relation:archived-cids":              done(LegacyActionRemap, "approach archived_cid is copied into reply provenance; post CIDs stay"),

	// Declared workers and features (task step 3).
	"feature:briefing":            done(LegacyActionRefactor, "every section reads posts, replies, votes and room outcomes: per-agent sections (and /me/diff reputation) via CanonicalBriefingRepository, pulse/trending/rising/hardcore/victories via CanonicalPlatformBriefingRepository (public+published+approved posts, live human/agent replies, a victory is a published room outcome), you-might-like and inferred specialties via CanonicalRecommendationRepository/CanonicalInferredSpecialtiesRepository, /me/diff opportunities via briefingOpportunityWhere; retired: approach success/failure, weight, idea evolution; the problem/question/idea counters are type-column subsets that read 0 once check:posts.posts_type_check narrows"),
	"feature:badges":              done(LegacyActionRetire, "milestone awarding retired: services.BadgeService never had a production caller and its first_solve/ten_solves/first_answer_accepted rules count solved problems and accepted answers the canonical model does not have; earned badge rows stay as history, served unchanged by /v1/agents/{id}/badges, /v1/users/{id}/badges, /v1/me and /me/diff, and the cutover awards none"),
	"feature:leaderboards":        done(LegacyActionRefactor, "CanonicalLeaderboardRepository serves /v1/leaderboard and /v1/leaderboard/tags/{tag}: reputation earned under the legacy rules is frozen in reputation_history by the cutover (FreezeLegacyReputation, first step of RemapLegacyRelations, 000111) and served unchanged per timeframe and tag; confirmed votes on posts and replies score live, a frozen vote is never scored again (approach votes are frozen at 0); writing a post or reply scores nothing by itself"),
	"feature:reputation":          done(LegacyActionRefactor, "CanonicalReputationAgentRepository/CanonicalReputationUserRepository serve reputation on GET /v1/agents/{id}, /v1/users/{id}, /v1/me, /v1/users/{id}/agents and the /v1/agents and /v1/users lists (incl. sort=reputation) with the served leaderboard's all_time rules: reputation_history + live confirmed votes on posts and replies (+ the agent bonus); the legacy formulas (AgentRepository.GetAgentStats/List, UserRepository.GetUserStats/List, reputation.BuildReputationSQL) are unwired and drop with the tables"),
	"feature:crystallization":     done(LegacyActionRefactor, "services.PostCrystallizationService + db.PostCrystallizationRepository: a live public, published, approved post of any type, not yet crystallized, with a live human or agent reply, post and replies unchanged 7 days; snapshot v2.0 = post + those replies (system verdicts left out); v1.0 CIDs stay; the unwired problem/approach CrystallizationService drops with its table"),
	"feature:forgetting":          pending(LegacyActionRetire, "approach forget_after/archived_at lifecycle has no reply equivalent"),
	"feature:moderation":          done(LegacyActionRefactor, "verdicts are system replies (ModerationReplyWriter); flags accept 'reply' and the API list is pinned to flags_target_type_check, so its legacy types narrow with check:flags at cleanup"),
	"feature:duplicate-detection": done(LegacyActionRefactor, "ModerationService.SetDuplicateFinder + db.ContentDuplicateRepository read the canonical tables: a post repeats a live post's title and description, a reply repeats a live non-system reply's body on the same post (24h window); legacy target types are not looked up"),
	"feature:embedding-workers":   done(LegacyActionRefactor, "reply create/edit embed the body into replies.embedding (an edit without a vector clears it) and backfill-embeddings embeds replies (content type 'replies', in 'all'); legacy answer/approach embedding stays only on the legacy routes and backfill types that drop with their tables"),

	// Scheduled jobs (cmd/api/main.go) and LISTEN consumers.
	"job:CleanupJob":               keep("prunes claim tokens and idempotency keys; touches no legacy table"),
	"job:HealthCheckJob":           keep("checks API, database and IPFS health; touches no legacy table"),
	"job:PresenceReaperJob":        keep("reaps room presence; the room model is not the knowledge model"),
	"job:TranslationJob":           done(LegacyActionRefactor, "translation uses posts original_* columns; its moderation trigger now writes the verdict as a system reply (ModerationReplyWriter), not a legacy comment"),
	"job:CrystallizationJob":       done(LegacyActionRefactor, "see feature:crystallization; main.go wires the canonical lister and crystallizer and the job skips ErrNothingToCrystallize"),
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
	"code:internal/db/legacy_relation_remap.go":    pending(LegacyActionRetire, "the cutover remap reads legacy tables by design; remove with them"),
	"code:internal/db/posts.go":                    pending(LegacyActionRefactor, "post reads join approach/answer/comment counts; count replies; the approach-based ListCrystallizationCandidates is unwired (feature:crystallization) and goes with them"),
	"code:internal/db/posts_list_filters.go":       pending(LegacyActionRefactor, "list filter tests approaches; filter on replies"),
	"code:internal/db/search.go":                   pending(LegacyActionRefactor, "searches approaches/answers/comments; search posts and replies"),
	"code:internal/db/visibility.go":               pending(LegacyActionRefactor, "visibility predicate covers approaches; cover replies"),
	"code:internal/db/stats.go":                    pending(LegacyActionRefactor, "platform stats count legacy tables and types; count posts and replies"),
	"code:internal/db/stats_questions.go":          pending(LegacyActionRefactor, "question stats count answers; derive from posts and replies"),
	"code:internal/db/homepage_overview.go":        pending(LegacyActionRefactor, "overview counts approaches/answers/responses; count replies"),
	"code:internal/db/agents.go":                   pending(LegacyActionRefactor, "agent stats counts (served through CanonicalReputationAgentRepository) and activity count legacy contributions and types; count replies; the legacy reputation formulas (GetAgentStats' reputation, legacyAgentListReputation) are unwired (feature:reputation) and go with them"),
	"code:internal/db/users.go":                    pending(LegacyActionRefactor, "user stats counts (served through CanonicalReputationUserRepository) count answers/responses and types; count replies; the legacy reputation (GetUserStats' reputation, List's BuildReputationSQL) is unwired (feature:reputation) and goes with them"),
	"code:internal/db/inferred_specialties.go":     pending(LegacyActionRetire, "legacy InferredSpecialtiesRepository, unwired: the briefing uses CanonicalInferredSpecialtiesRepository (briefing_recommendations_canonical.go); drops with the tables"),
	"code:internal/db/leaderboard.go":              pending(LegacyActionRetire, "legacy LeaderboardRepository, unwired: the router serves the leaderboard from CanonicalLeaderboardRepository (leaderboard_canonical.go); drops with the tables"),
	"code:internal/db/leaderboard_tags.go":         pending(LegacyActionRetire, "legacy per-tag LeaderboardRepository query, unwired (see code:internal/db/leaderboard.go); drops with the tables"),
	"code:internal/db/reputation_history.go":       pending(LegacyActionRetire, "the cutover freeze reads the legacy tables by design; remove with them (reputation_history itself stays)"),
	"code:internal/db/briefing.go":                 pending(LegacyActionRetire, "legacy per-agent BriefingRepository, unwired: the router serves those sections from CanonicalBriefingRepository (briefing_canonical.go); drops with the tables"),
	"code:internal/db/briefing_platform.go":        pending(LegacyActionRetire, "legacy PlatformBriefingRepository, unwired: the router serves the platform sections from CanonicalPlatformBriefingRepository (briefing_platform_canonical.go); drops with the tables"),
	"code:internal/db/briefing_recommendations.go": pending(LegacyActionRetire, "legacy RecommendationRepository, unwired: the router uses CanonicalRecommendationRepository (briefing_recommendations_canonical.go); drops with the tables"),
	"code:internal/db/resurrection.go":             pending(LegacyActionRefactor, "resurrection bundle reads approaches and legacy types"),
	"code:internal/db/sitemap.go":                  pending(LegacyActionRefactor, "sitemap emits legacy type URLs; emit /posts URLs, keep redirects"),
	"code:internal/db/auto_solve.go":               pending(LegacyActionRetire, "see job:AutoSolveJob"),
	"code:internal/db/stale_content.go":            pending(LegacyActionRetire, "see job:StaleContentJob and feature:forgetting"),
	"code:internal/reputation/sql_builder.go":      pending(LegacyActionRetire, "legacy BuildReputationSQL, unwired: only the unwired legacy leaderboard and UserRepository.List build it (feature:reputation); drops with the tables"),
	"code:cmd/backfill-embeddings/main.go":         pending(LegacyActionRefactor, "answers/approaches content types still read the legacy tables; remove them with the tables (the cutover copies their vectors onto replies)"),
}
