package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// KnowledgeCutoverOptions configures RunKnowledgeCutover.
type KnowledgeCutoverOptions struct {
	// DryRun only reads: it reports what the cutover would do and writes nothing, not even
	// the ledger.
	DryRun bool
}

// KnowledgeCutoverStep is one timed step of a run, as recorded in the ledger.
type KnowledgeCutoverStep struct {
	Name   string `json:"name"`
	Millis int64  `json:"ms"`
	Result any    `json:"result,omitempty"`
}

// KnowledgeCutoverReport is what one run found and changed (task idx 93). Counts of changes
// are this run's, so a repeated run over a finished cutover reports zeros.
type KnowledgeCutoverReport struct {
	RunID  string `json:"run_id"`
	DryRun bool   `json:"dry_run"`

	PostExceptions     int            `json:"post_exceptions"`
	PostExceptionKinds map[string]int `json:"post_exception_kinds"`
	PostStatesRemapped int64          `json:"post_states_remapped"`

	// PendingContributions counts legacy approaches, answers, responses and comments that
	// MigrateContributions would still convert (a comment whose target is gone never is).
	PendingContributions   int              `json:"pending_contributions"`
	PendingAfter           int              `json:"pending_after"`
	ContributionExceptions map[string]int   `json:"contribution_exceptions"`
	RepliesCreated         int64            `json:"replies_created"`
	ProgressNotes          int64            `json:"progress_notes"`
	Relations              map[string]int64 `json:"relations"`
	UnresolvedRelations    int              `json:"unresolved_relations"`

	VoteDriftBefore     int64 `json:"vote_drift_before"`
	VoteScoresRebuilt   int64 `json:"vote_scores_rebuilt"`
	VoteDriftAfter      int64 `json:"vote_drift_after"`
	RoomDriftBefore     int64 `json:"room_drift_before"`
	RoomActivityRebuilt int64 `json:"room_activity_rebuilt"`
	RoomDriftAfter      int64 `json:"room_drift_after"`

	Steps []KnowledgeCutoverStep `json:"steps"`

	// Every exception by record, for the operator's review.
	PostExceptionList         []PostMigrationException         `json:"post_exception_list,omitempty"`
	ContributionExceptionList []ContributionMigrationException `json:"contribution_exception_list,omitempty"`
	UnresolvedRelationList    []ContributionMigrationException `json:"unresolved_relation_list,omitempty"`
}

// cutoverLedgerDDL creates the run ledger on first use. Like rollback_archive it lives outside
// the migration chain on purpose: a rollback of the schema must not erase the record of the
// runs it undoes.
const cutoverLedgerDDL = `CREATE TABLE IF NOT EXISTS cutover_ledger (
	id          BIGSERIAL PRIMARY KEY,
	run_id      UUID        NOT NULL,
	step        TEXT        NOT NULL,
	started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	finished_at TIMESTAMPTZ,
	result      JSONB,
	error       TEXT
)`

// pendingContributionsSQL counts the legacy rows MigrateContributions would still convert.
const pendingContributionsSQL = `SELECT
	(SELECT count(*) FROM approaches a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.problem_id)
		AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = a.id)) +
	(SELECT count(*) FROM answers a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.question_id)
		AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'answer' AND r.legacy_id = a.id)) +
	(SELECT count(*) FROM responses a WHERE EXISTS (SELECT 1 FROM posts p WHERE p.id = a.idea_id)
		AND NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'response' AND r.legacy_id = a.id)) +
	(SELECT count(*) FROM comments c
		WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'comment' AND r.legacy_id = c.id)
		AND CASE c.target_type
			WHEN 'post'     THEN EXISTS (SELECT 1 FROM posts t WHERE t.id = c.target_id)
			WHEN 'approach' THEN EXISTS (SELECT 1 FROM approaches t WHERE t.id = c.target_id)
			WHEN 'answer'   THEN EXISTS (SELECT 1 FROM answers t WHERE t.id = c.target_id)
			WHEN 'response' THEN EXISTS (SELECT 1 FROM responses t WHERE t.id = c.target_id)
			ELSE false END)`

type knowledgeCutoverRun struct {
	pool   *Pool
	dryRun bool
	rep    *KnowledgeCutoverReport
}

// RunKnowledgeCutover converts the legacy knowledge model to posts and replies in the order
// the rehearsal on the restored production dump proved (task idx 93): post states, then
// contributions, then the relations that name them, then the vote-score and room-activity
// projections rebuilt from their authoritative records. Every step is idempotent, so a run
// interrupted anywhere is finished by running again, and each step is recorded in
// cutover_ledger under the run's id. It stops before converting anything when a post fails
// verification, and fails when a rebuilt projection still drifts. The schema must already be
// at the version that has replies (the caller checks it with SchemaVersion).
func RunKnowledgeCutover(ctx context.Context, pool *Pool, opts KnowledgeCutoverOptions) (*KnowledgeCutoverReport, error) {
	run := &knowledgeCutoverRun{pool: pool, dryRun: opts.DryRun, rep: &KnowledgeCutoverReport{
		RunID: uuid.NewString(), DryRun: opts.DryRun,
	}}
	if !opts.DryRun {
		if _, err := pool.Exec(ctx, cutoverLedgerDDL); err != nil {
			return run.rep, fmt.Errorf("knowledge cutover: ledger: %w", err)
		}
	}

	for _, s := range []struct {
		name  string
		apply bool // runs only when not a dry run
		fn    func(context.Context) (any, error)
	}{
		{"post_states", false, run.postStates},
		{"verify_contributions", false, run.verifyContributions},
		{"migrate_contributions", true, run.migrateContributions},
		{"remap_legacy_relations", true, run.remapLegacyRelations},
		{"vote_scores", false, run.voteScores},
		{"room_activity", false, run.roomActivity},
		{"verify_contributions_after", true, run.verifyContributionsAfter},
	} {
		if s.apply && opts.DryRun {
			continue
		}
		if err := run.step(ctx, s.name, s.fn); err != nil {
			return run.rep, fmt.Errorf("knowledge cutover: %s: %w", s.name, err)
		}
	}
	return run.rep, nil
}

// step times fn and records it in the report and, unless dry, in the ledger.
func (r *knowledgeCutoverRun) step(ctx context.Context, name string, fn func(context.Context) (any, error)) error {
	var ledgerID int64
	if !r.dryRun {
		if err := r.pool.QueryRow(ctx, `INSERT INTO cutover_ledger (run_id, step) VALUES ($1, $2) RETURNING id`,
			r.rep.RunID, name).Scan(&ledgerID); err != nil {
			return fmt.Errorf("ledger start: %w", err)
		}
	}
	start := time.Now()
	result, err := fn(ctx)
	r.rep.Steps = append(r.rep.Steps, KnowledgeCutoverStep{Name: name, Millis: time.Since(start).Milliseconds(), Result: result})
	if r.dryRun {
		return err
	}
	resultJSON, jerr := json.Marshal(result)
	if jerr != nil {
		return fmt.Errorf("ledger result: %w", jerr)
	}
	var errText *string
	if err != nil {
		s := err.Error()
		errText = &s
	}
	if _, lerr := r.pool.Exec(ctx, `UPDATE cutover_ledger SET finished_at = NOW(), result = $2, error = $3 WHERE id = $1`,
		ledgerID, resultJSON, errText); lerr != nil && err == nil {
		return fmt.Errorf("ledger finish: %w", lerr)
	}
	return err
}

func (r *knowledgeCutoverRun) postStates(ctx context.Context) (any, error) {
	if !r.dryRun {
		n, err := RemapPostStates(ctx, r.pool)
		if err != nil {
			return nil, err
		}
		r.rep.PostStatesRemapped = n
	}
	v, err := VerifyPostMigration(ctx, r.pool)
	if err != nil {
		return nil, err
	}
	r.rep.PostExceptions = len(v.Exceptions)
	r.rep.PostExceptionKinds = map[string]int{}
	for _, e := range v.Exceptions {
		r.rep.PostExceptionKinds[e.Kind]++
	}
	r.rep.PostExceptionList = v.Exceptions
	result := map[string]any{"total": v.Total, "remapped": r.rep.PostStatesRemapped, "exceptions": r.rep.PostExceptionKinds}
	if len(v.Exceptions) > 0 && !r.dryRun {
		return result, fmt.Errorf("%d post(s) fail verification (%s); nothing converted", len(v.Exceptions), kindList(r.rep.PostExceptionKinds))
	}
	return result, nil
}

func (r *knowledgeCutoverRun) verifyContributions(ctx context.Context) (any, error) {
	v, err := VerifyContributionMigration(ctx, r.pool)
	if err != nil {
		return nil, err
	}
	r.rep.ContributionExceptions = map[string]int{}
	for _, e := range v.Exceptions {
		r.rep.ContributionExceptions[e.LegacyType+":"+e.Kind]++
	}
	r.rep.ContributionExceptionList = v.Exceptions
	if err := r.pool.QueryRow(ctx, pendingContributionsSQL).Scan(&r.rep.PendingContributions); err != nil {
		return nil, fmt.Errorf("pending contributions: %w", err)
	}
	return map[string]any{"pending": r.rep.PendingContributions, "exceptions": r.rep.ContributionExceptions}, nil
}

// verifyContributionsAfter requires that nothing convertible is left unconverted.
func (r *knowledgeCutoverRun) verifyContributionsAfter(ctx context.Context) (any, error) {
	if err := r.pool.QueryRow(ctx, pendingContributionsSQL).Scan(&r.rep.PendingAfter); err != nil {
		return nil, fmt.Errorf("pending contributions: %w", err)
	}
	result := map[string]int{"pending": r.rep.PendingAfter}
	if r.rep.PendingAfter != 0 {
		return result, fmt.Errorf("%d legacy contribution(s) still have no reply", r.rep.PendingAfter)
	}
	return result, nil
}

func (r *knowledgeCutoverRun) migrateContributions(ctx context.Context) (any, error) {
	n, err := MigrateContributions(ctx, r.pool)
	r.rep.RepliesCreated = n
	return map[string]any{"replies_created": n}, err
}

func (r *knowledgeCutoverRun) remapLegacyRelations(ctx context.Context) (any, error) {
	rr, err := RemapLegacyRelations(ctx, r.pool)
	if rr != nil {
		r.rep.ProgressNotes = rr.ProgressNotes
		r.rep.Relations = map[string]int64{
			"reputation_history": rr.ReputationHistory, "accepted_answers": rr.AcceptedAnswers,
			"votes": rr.Votes, "reports": rr.Reports, "flags": rr.Flags,
			"approach_relationships": rr.ApproachRelationships, "progress_notes": rr.ProgressNotes,
			"notification_links": rr.NotificationLinks, "embeddings": rr.Embeddings,
		}
		r.rep.UnresolvedRelations = len(rr.Exceptions)
		r.rep.UnresolvedRelationList = rr.Exceptions
	}
	return map[string]any{"relations": r.rep.Relations, "unresolved": r.rep.UnresolvedRelations}, err
}

func (r *knowledgeCutoverRun) voteScores(ctx context.Context) (any, error) {
	return r.rebuild(ctx, "vote_score_drift()", "rebuild_vote_scores()",
		&r.rep.VoteDriftBefore, &r.rep.VoteScoresRebuilt, &r.rep.VoteDriftAfter)
}

func (r *knowledgeCutoverRun) roomActivity(ctx context.Context) (any, error) {
	return r.rebuild(ctx, "room_activity_drift()", "rebuild_room_activity()",
		&r.rep.RoomDriftBefore, &r.rep.RoomActivityRebuilt, &r.rep.RoomDriftAfter)
}

// rebuild measures a projection's drift, repairs it from its authoritative records unless
// dry, and requires no drift afterwards.
func (r *knowledgeCutoverRun) rebuild(ctx context.Context, drift, rebuildFn string, before, rebuilt, after *int64) (any, error) {
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+drift).Scan(before); err != nil {
		return nil, err
	}
	if r.dryRun {
		return map[string]int64{"drift": *before}, nil
	}
	var n int32
	if err := r.pool.QueryRow(ctx, `SELECT `+rebuildFn).Scan(&n); err != nil {
		return nil, err
	}
	*rebuilt = int64(n)
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+drift).Scan(after); err != nil {
		return nil, err
	}
	result := map[string]int64{"drift_before": *before, "rebuilt": *rebuilt, "drift_after": *after}
	if *after != 0 {
		return result, fmt.Errorf("%s still reports %d row(s) after %s", drift, *after, rebuildFn)
	}
	return result, nil
}

// SchemaVersion reads golang-migrate's schema_migrations row. It fails when the table is
// absent: production had none before the cutover (migrations there were applied by hand).
func SchemaVersion(ctx context.Context, pool *Pool) (version int64, dirty bool, err error) {
	err = pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
	return version, dirty, err
}

func kindList(kinds map[string]int) string {
	parts := make([]string, 0, len(kinds))
	for k, n := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", k, n))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
