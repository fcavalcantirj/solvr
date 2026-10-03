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
	// Catalog objects. The legacy tables (with their indexes and foreign keys), the functions
	// typed by their rows (hybrid_search_answers/approaches), posts.accepted_answer_id and the
	// legacy type and target-type checks on posts, votes, reports and flags left the public
	// schema in 000138_legacy_archive (idx 68): their rows are kept in legacy_archive and the
	// down migration restores them. What the live catalog still ties to a legacy type is
	// replies' provenance.
	"check:replies.replies_legacy_type_check":  keep("provenance of migrated replies; the compact legacy mapping outlives cleanup"),
	"check:replies.replies_provenance_bounded": keep("bounds migrated replies' provenance to the keys each legacy type's migration writes (000118); outlives cleanup with that provenance"),

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
	"feature:duplicate-detection": done(LegacyActionRefactor, "contentgate.Gate + db.ContentDuplicateRepository, wired into every create handler (anti-abuse W1): a same-author repeat is refused with 409 DUPLICATE_CONTENT before the insert — a post title (digits normalized) against the author's live posts, a contribution body against the author's live replies, answers, approaches, responses and comments on any post; the in-memory hash store (services/duplicate.go) is deleted"),
	"feature:embedding-workers":   done(LegacyActionRefactor, "reply create/edit embed the body into replies.embedding (an edit without a vector clears it) and backfill-embeddings embeds posts and replies only (its answers/approaches types are retired: the cutover copies their vectors onto the replies migrated from them); legacy answer/approach embedding stays only on the legacy routes that drop with their tables"),

	// Scheduled jobs (cmd/api/main.go) and LISTEN consumers.
	"job:CleanupJob":               keep("prunes claim tokens and idempotency keys; touches no legacy table"),
	"job:HealthCheckJob":           keep("checks API, database and IPFS health; touches no legacy table"),
	"job:PresenceReaperJob":        keep("reaps room presence; the room model is not the knowledge model"),
	"job:SearchDocumentJob":        keep("embeds the posts and replies search_document_drift() lists; touches no legacy table"),
	"job:WebhookDeliveryJob":       keep("sends the webhook deliveries queued with notification events (webhooks, webhook_deliveries, notifications); touches no legacy table"),
	"job:CounterReconcileJob":      keep("repairs the posts, replies, blog posts, rooms and agents counters the *_drift() functions list, row by row; touches no legacy table"),
	"job:TranslationJob":           done(LegacyActionRefactor, "translation uses posts original_* columns; its moderation trigger now writes the verdict as a system reply (ModerationReplyWriter), not a legacy comment"),
	"job:CrystallizationJob":       done(LegacyActionRefactor, "see feature:crystallization; main.go wires the canonical lister and crystallizer and the job skips ErrNothingToCrystallize"),
	"consumer:RoomEntryChannel":    keep("room entry fan-out; independent of the knowledge model"),
	"consumer:RoomPresenceChannel": keep("room presence fan-out; independent of the knowledge model"),
	"consumer:RoomAccessChannel":   keep("room access revocation; independent of the knowledge model"),
	"consumer:OverviewChannel":     keep("homepage overview refresh signal; the query it triggers is listed separately"),

	// Application queries (non-test Go files whose SQL names a legacy table or type).
	"code:internal/db/contribution_migration.go":   keep("the cutover tool (cmd/cutover) converts legacy rows into replies by design; owner decision 2026-10-02 (idx 68): it stays as the pre-archive operator and rehearsal tool, never reached from cmd/api, and refuses once the legacy tables are archived"),
	"code:internal/db/knowledge_cutover.go":        keep("the cutover runner (cmd/cutover) reads the legacy tables by design; owner decision 2026-10-02 (idx 68): it stays as the pre-archive operator and rehearsal tool, never reached from cmd/api, and refuses once the legacy tables are archived"),
	"code:internal/db/knowledge_cutover_search.go": keep("the cutover search sample compares the legacy contribution search over answers and approaches with replies by design; it stays with the pre-archive cutover tool (owner decision 2026-10-02, idx 68)"),
	"code:internal/db/legacy_relation_remap.go":    keep("the cutover remap reads the legacy tables by design; it stays with the pre-archive cutover tool (owner decision 2026-10-02, idx 68)"),
	"code:internal/db/reputation_history.go":       keep("the cutover freeze (FreezeLegacyReputation) reads the legacy tables by design and stays with the pre-archive cutover tool (owner decision 2026-10-02, idx 68); reputation_history itself is canonical history"),

	// Go files naming a legacy post type through a PostType constant or a bare literal
	// (ScanLegacyPostTypeReferences). Each changes behavior once check:posts.posts_type_check
	// narrows posts.type to 'post'. A file that also holds legacy SQL is owned by its code:
	// entry. A type carried in a variable is found where the variable is declared.
	"typeref:internal/db/legacy_post_type_refs.go":       keep("the scanner's own table of the legacy post types it looks for; it names them to find them"),
	"typeref:internal/api/handlers/homepage_activity.go": keep("'question' is a room message action label (Asked a question), not a post type"),
	"typeref:internal/models/post.go":                    keep("RetiredPostTypes names the legacy problem, question and idea types only so POST /v1/posts and the list and search filters refuse them with LEGACY_FIELD_RETIRED (idx 68); no stored post carries one after the legacy archive migration"),
	"typeref:internal/api/legacy_read_retirement.go":     keep("the migration errors of the retired single-post reads (GET /v1/problems|questions|ideas/{id}, idx 73 step 3) name the legacy type each route pinned; it is text served with 410, and no request takes a branch on it"),
	"typeref:internal/api/openapi_schemas.go":            keep("the 'question' in the IdeaResponse and CreateIdeaResponseRequest schemas is the idea-response kind, not a post type; the post schemas moved to openapi_post_schemas.go (idx 78)"),
}
