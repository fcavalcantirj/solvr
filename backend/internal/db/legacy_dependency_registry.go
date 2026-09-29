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
	"feature:forgetting":          done(LegacyActionRetire, "services.ForgettingService never had a production caller: nothing constructs it and nothing else calls MarkForForgetting, ArchiveApproach or ListStaleApproaches, so no production path writes an approach's forget_after or archived_at; replies get no forget/archive lifecycle; an approach's archived_cid stays in its reply's provenance (relation:archived-cids); the service and those columns drop with the tables"),
	"feature:moderation":          done(LegacyActionRefactor, "verdicts are system replies (ModerationReplyWriter); flags accept 'reply' and the API list is pinned to flags_target_type_check, so its legacy types narrow with check:flags at cleanup"),
	"feature:duplicate-detection": done(LegacyActionRefactor, "ModerationService.SetDuplicateFinder + db.ContentDuplicateRepository read the canonical tables: a post repeats a live post's title and description, a reply repeats a live non-system reply's body on the same post (24h window); legacy target types are not looked up"),
	"feature:embedding-workers":   done(LegacyActionRefactor, "reply create/edit embed the body into replies.embedding (an edit without a vector clears it) and backfill-embeddings embeds posts and replies only (its answers/approaches types are retired: the cutover copies their vectors onto the replies migrated from them); legacy answer/approach embedding stays only on the legacy routes that drop with their tables"),

	// Scheduled jobs (cmd/api/main.go) and LISTEN consumers.
	"job:CleanupJob":               keep("prunes claim tokens and idempotency keys; touches no legacy table"),
	"job:HealthCheckJob":           keep("checks API, database and IPFS health; touches no legacy table"),
	"job:PresenceReaperJob":        keep("reaps room presence; the room model is not the knowledge model"),
	"job:TranslationJob":           done(LegacyActionRefactor, "translation uses posts original_* columns; its moderation trigger now writes the verdict as a system reply (ModerationReplyWriter), not a legacy comment"),
	"job:CrystallizationJob":       done(LegacyActionRefactor, "see feature:crystallization; main.go wires the canonical lister and crystallizer and the job skips ErrNothingToCrystallize"),
	"consumer:RoomEntryChannel":    keep("room entry fan-out; independent of the knowledge model"),
	"consumer:RoomPresenceChannel": keep("room presence fan-out; independent of the knowledge model"),
	"consumer:RoomAccessChannel":   keep("room access revocation; independent of the knowledge model"),
	"consumer:OverviewChannel":     keep("homepage overview refresh signal; the query it triggers is listed separately"),

	// Application queries (non-test Go files whose SQL names a legacy table or type).
	"code:internal/db/approaches.go":               pending(LegacyActionRetire, "legacy approach repository incl. progress notes; replaced by replies; still wired to the legacy problem/approach routes, while PATCH /v1/posts checks a solved problem with CanonicalApproachCheckerRepository (approach_checker_canonical.go) and the contribution listings read migrated replies (contributions_canonical.go)"),
	"code:internal/db/answers.go":                  pending(LegacyActionRetire, "legacy answer repository; replaced by replies; still wired to the legacy question/answer routes, while the contribution listings read migrated replies (contributions_canonical.go)"),
	"code:internal/db/responses.go":                pending(LegacyActionRetire, "legacy response repository; replaced by replies; still wired to the legacy idea/response routes, while the contribution listings read migrated replies (contributions_canonical.go)"),
	"code:internal/db/comments.go":                 pending(LegacyActionRetire, "legacy comment repository; replaced by replies"),
	"code:internal/db/approach_relationships.go":   pending(LegacyActionRetire, "legacy approach relationship repository; see relation:approach-relationships"),
	"code:internal/db/problems.go":                 pending(LegacyActionRetire, "type-filtered problem listing; canonical listing is posts; also holds the unwired approach-based PostRepository.ListCrystallizationCandidates (feature:crystallization), moved out of posts.go"),
	"code:internal/db/questions.go":                pending(LegacyActionRetire, "type-filtered question listing; canonical listing is posts"),
	"code:internal/db/ideas.go":                    pending(LegacyActionRetire, "type-filtered idea listing and evolution; canonical listing is posts"),
	"code:internal/db/contribution_migration.go":   pending(LegacyActionRetire, "the migration reads legacy tables by design; remove with them"),
	"code:internal/db/legacy_relation_remap.go":    pending(LegacyActionRetire, "the cutover remap reads legacy tables by design; remove with them"),
	"code:internal/db/stats.go":                    pending(LegacyActionRefactor, "its methods that read answers/approaches/responses (GetAllStats, GetTotalContributionsCount, GetTrendingPosts, GetProblemsStats, GetRecentlySolvedProblems, GetTopProblemSolvers) are unwired: the stats routes and the overview serve them from CanonicalStatsRepository (stats_canonical.go: live human/agent replies on live public posts; active approaches retired; solvers from replies migrated from succeeded approaches) and they go with the tables; the /v1/stats/ideas sidebar moved to stats_ideas.go (posts only, the idea type passed as a parameter, so it reads zero once check:posts.posts_type_check narrows)"),
	"code:internal/db/stats_questions.go":          pending(LegacyActionRetire, "legacy question sidebar queries, unwired: CanonicalStatsRepository (stats_canonical.go) serves them from top-level contributor replies and the accepted reply; drops with the tables"),
	"code:internal/db/homepage_overview.go":        pending(LegacyActionRefactor, "the legacy ListReusablePosts (approach/answer/response counts) is unwired: CanonicalHomepageRepository serves the overview's reusable posts from live human and agent replies (homepage_reusable_canonical.go), so it goes with the tables"),
	"code:internal/db/agents.go":                   pending(LegacyActionRefactor, "the legacy GetActivity, GetAgentStats and legacyAgentListReputation are unwired: CanonicalReputationAgentRepository serves GET /v1/agents/{id}/activity from posts and replies (activity_canonical.go), the stats counts from posts, replies and votes (profile_stats_canonical.go) and the reputation canonically (feature:reputation), so they go with the tables"),
	"code:internal/db/users.go":                    pending(LegacyActionRefactor, "the legacy GetUserStats (answers/responses counts) and List's BuildReputationSQL are unwired: CanonicalReputationUserRepository serves the stats counts from posts, replies and votes (profile_stats_canonical.go) and the reputation canonically (feature:reputation); they go with the tables"),
	"code:internal/db/inferred_specialties.go":     pending(LegacyActionRetire, "legacy InferredSpecialtiesRepository, unwired: the briefing uses CanonicalInferredSpecialtiesRepository (briefing_recommendations_canonical.go); drops with the tables"),
	"code:internal/db/leaderboard.go":              pending(LegacyActionRetire, "legacy LeaderboardRepository, unwired: the router serves the leaderboard from CanonicalLeaderboardRepository (leaderboard_canonical.go); drops with the tables"),
	"code:internal/db/leaderboard_tags.go":         pending(LegacyActionRetire, "legacy per-tag LeaderboardRepository query, unwired (see code:internal/db/leaderboard.go); drops with the tables"),
	"code:internal/db/reputation_history.go":       pending(LegacyActionRetire, "the cutover freeze reads the legacy tables by design; remove with them (reputation_history itself stays)"),
	"code:internal/db/briefing.go":                 pending(LegacyActionRetire, "legacy per-agent BriefingRepository, unwired: the router serves those sections from CanonicalBriefingRepository (briefing_canonical.go); drops with the tables"),
	"code:internal/db/briefing_platform.go":        pending(LegacyActionRetire, "legacy PlatformBriefingRepository, unwired: the router serves the platform sections from CanonicalPlatformBriefingRepository (briefing_platform_canonical.go); drops with the tables"),
	"code:internal/db/briefing_recommendations.go": pending(LegacyActionRetire, "legacy RecommendationRepository, unwired: the router uses CanonicalRecommendationRepository (briefing_recommendations_canonical.go); drops with the tables"),
	"code:internal/db/auto_solve.go":               pending(LegacyActionRetire, "legacy AutoSolveRepository and jobs.AutoSolveJob, unscheduled: cmd/api/main.go no longer warns owners or auto-solves problems from succeeded approaches (replies have no status workflow); drops with the tables"),
	"code:internal/db/stale_content.go":            pending(LegacyActionRetire, "legacy StaleContentRepository and jobs.StaleContentJob, unscheduled: cmd/api/main.go no longer warns about or abandons idle approaches or marks unapproached problems dormant (replies have no status workflow); drops with the tables"),
	"code:internal/reputation/sql_builder.go":      pending(LegacyActionRetire, "legacy BuildReputationSQL, unwired: only the unwired legacy leaderboard and UserRepository.List build it (feature:reputation); drops with the tables"),

	// Go files naming a legacy post type through a PostType constant or a bare literal
	// (ScanLegacyPostTypeReferences). Each changes behavior once check:posts.posts_type_check
	// narrows posts.type to 'post'. A file that also holds legacy SQL is owned by its code:
	// entry. A type carried in a variable is found where the variable is declared.
	"typeref:internal/models/post.go":                      pending(LegacyActionRefactor, "ValidPostTypes/IsValidPostType/IsValidPostStatus accept the legacy types and hold their per-type status sets (solved, answered, evolved...); they narrow with check:posts.posts_type_check"),
	"typeref:internal/db/legacy_post_type_refs.go":         keep("the scanner's own table of the legacy post types it looks for; it names them to find them"),
	"typeref:internal/models/response.go":                  keep("ResponseTypeQuestion is the idea-response kind 'question', not a post type; the post type narrowing does not touch it"),
	"typeref:internal/api/handlers/homepage_activity.go":   keep("'question' is a room message action label (Asked a question), not a post type"),
	"typeref:internal/api/handlers/posts.go":               pending(LegacyActionRefactor, "POST /v1/posts validates weight and success criteria only for type problem, and PATCH /v1/posts runs the succeeded-approach check only when a problem is set to solved; once the check narrows no post takes either branch (idx 73 decides what solved means canonically)"),
	"typeref:internal/api/handlers/problems.go":            pending(LegacyActionRetire, "legacy problem route family (type check on GET, create with type problem); routed through idx 52's transition adapters, then retired"),
	"typeref:internal/api/handlers/questions.go":           pending(LegacyActionRetire, "legacy question route family (type check on GET, create with type question); routed through idx 52's transition adapters, then retired"),
	"typeref:internal/api/handlers/ideas.go":               pending(LegacyActionRetire, "legacy idea route family (type check on GET, create with type idea); routed through idx 52's transition adapters, then retired"),
	"typeref:internal/api/router.go":                       pending(LegacyActionRetire, "GET /v1/problems, /v1/questions and /v1/ideas are legacyTypedListAdapter over GET /v1/posts pinned to a legacy type; they list nothing once the check narrows; retired with idx 52's legacy routes"),
	"typeref:internal/api/handlers/legacy_feed.go":         pending(LegacyActionRetire, "deprecated GET /v1/feed/stuck and /v1/feed/unanswered pin type problem/question and count answers only on questions; they list nothing once the check narrows; retired with idx 52's legacy routes"),
	"typeref:internal/api/handlers/stats.go":               pending(LegacyActionRetire, "GET /v1/stats/problems|questions|ideas read the knowledge aggregate of one legacy type (knowledgeFor); type-specific statistics are retired by idx 73 step 3"),
	"typeref:internal/api/handlers/legacy_type_stats.go":   pending(LegacyActionRetire, "applyKnowledgeCounts maps the problem/question aggregates onto the legacy stats fields; retired with the type-specific statistics routes (idx 73 step 3)"),
	"typeref:internal/api/handlers/homepage_knowledge.go":  pending(LegacyActionRefactor, "the overview knowledge section labels one entry per legacy type beside 'post'; after the narrowing the legacy entries read 0 (idx 73 step 3 decides whether per-type figures survive)"),
	"typeref:internal/db/knowledge_totals.go":              pending(LegacyActionRefactor, "KnowledgeTypes publishes problem/question/idea aggregates beside post; after the narrowing every post counts under 'post' and the legacy entries read 0 (idx 73 step 3)"),
	"typeref:internal/api/handlers/homepage_overview.go":   pending(LegacyActionRefactor, "overviewPostURL links legacy types to /problems|/questions|/ideas pages; after the narrowing every post takes the /posts/<id> default; the legacy branches go with the legacy pages and their redirects (idx 83)"),
	"typeref:internal/api/handlers/users_contributions.go": pending(LegacyActionRefactor, "the contributions listing labels ParentType from the legacy contribution kind (answer->question, approach->problem, response->idea), not from the parent post, so it keeps naming legacy types after the narrowing"),
	"typeref:internal/api/openapi_schemas.go":              pending(LegacyActionRefactor, "the published post and create-post schemas enumerate only problem/question/idea, not 'post'; the 'question' in the response schemas is the idea-response kind (idx 78 keeps SDKs, MCP and docs consistent)"),
	"typeref:internal/api/handlers/mcp.go":                 pending(LegacyActionRefactor, "the MCP search and create tools advertise type enums of problem/question/idea only (idx 78 keeps MCP consistent with the canonical post)"),
	"typeref:internal/db/briefing_canonical.go":            pending(LegacyActionRefactor, "the per-agent open-items section counts ProblemsNoApproaches/QuestionsNoAnswers as posts.type subsets; they read 0 once the check narrows (idx 73 step 3)"),
	"typeref:internal/db/briefing_platform_canonical.go":   pending(LegacyActionRefactor, "the platform pulse counts open problems, open questions and active ideas as posts.type subsets; they read 0 once the check narrows (idx 73 step 3)"),
	"typeref:internal/db/stats_canonical.go":               pending(LegacyActionRefactor, "declares statsProblemType/statsQuestionType, the bound parameters of the canonical stats and profile problem/question figures; they read 0 once the check narrows (idx 73 step 3)"),
	"typeref:internal/db/profile_stats_canonical.go":       pending(LegacyActionRefactor, "declares statsIdeaType, the bound parameter of the profile idea count and every stats_ideas.go sidebar query; they read 0 once the check narrows (idx 73 step 3)"),
	"typeref:internal/services/cooldown.go":                pending(LegacyActionRetire, "per-type CooldownService (problem/question/idea/answer/comment) has no production caller: NewCooldownService, DefaultCooldownConfig and NewInMemoryCooldownStore are called only from tests; its keys have no canonical meaning"),
	"typeref:internal/services/crystallization.go":         pending(LegacyActionRetire, "the unwired problem/approach CrystallizationService (validateEligibility requires type problem); the canonical crystallizer is PostCrystallizationService (feature:crystallization); drops with its table"),
}
