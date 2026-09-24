package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Contribution-migration exception kinds (task: migrate legacy contributions and their
// references without orphaning or silently reviving content). These are the per-record
// conditions the legacy->Reply conversion must surface rather than resolve silently.
const (
	// ContribExceptionOrphanComment: a comment whose target row (post/approach/answer/
	// response) no longer exists, so it cannot be attached to any canonical reply.
	ContribExceptionOrphanComment = "orphan_comment"
	// ContribExceptionContributionOnDeletedPost: a non-deleted approach/answer/response
	// attached to a soft-deleted ("non-live") post. It is still migrated so its original
	// visibility/deletion relationship is retained, but it is reported for review — this is
	// the research finding of 35 non-deleted approaches attached to non-live posts (step 5).
	ContribExceptionContributionOnDeletedPost = "contribution_on_deleted_post"
	// ContribExceptionOrphanContribution: a contribution whose parent post is absent
	// entirely (a genuine orphan; foreign keys normally prevent this).
	ContribExceptionOrphanContribution = "orphan_contribution"
)

// ContributionMigrationException records one legacy contribution that failed a migration
// invariant. LegacyID is the id in its source table so a test or operator can locate it.
type ContributionMigrationException struct {
	LegacyType string
	LegacyID   string
	Kind       string
	Detail     string
}

// ContributionMigrationReport is the pre-conversion inventory (step 1) plus the per-record
// exceptions (step 5) for the legacy-contribution -> Reply migration. Run it against a
// restored production snapshot before conversion; every genuine orphan must appear here
// rather than being dropped or silently revived.
type ContributionMigrationReport struct {
	ApproachesTotal, ApproachesLive, ApproachesDeleted int
	AnswersTotal, AnswersLive, AnswersDeleted          int
	AcceptedAnswers                                    int
	ResponsesTotal                                     int
	CommentsTotal, CommentsLive, CommentsDeleted       int
	ApproachesByStatus                                 map[string]int
	CommentsByTarget                                   map[string]int
	Exceptions                                         []ContributionMigrationException
}

// OK reports whether verification found zero exceptions.
func (r *ContributionMigrationReport) OK() bool { return len(r.Exceptions) == 0 }

// ExceptionsFor returns only the exceptions whose LegacyID is in ids, so a test can assert
// on its own seeded rows without being disturbed by unrelated rows in a shared database.
func (r *ContributionMigrationReport) ExceptionsFor(ids ...string) []ContributionMigrationException {
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []ContributionMigrationException
	for _, e := range r.Exceptions {
		if want[e.LegacyID] {
			out = append(out, e)
		}
	}
	return out
}

// VerifyContributionMigration inventories the four legacy contribution tables and reports
// genuine orphans and contributions attached to non-live posts (task steps 1, 5). It reads
// only; run it before and after MigrateContributions.
func VerifyContributionMigration(ctx context.Context, pool *Pool) (*ContributionMigrationReport, error) {
	rep := &ContributionMigrationReport{
		ApproachesByStatus: map[string]int{},
		CommentsByTarget:   map[string]int{},
	}

	// Existing post ids (live or soft-deleted) and which of them are non-live (deleted).
	postExists, err := loadIDSet(ctx, pool, `SELECT id::text FROM posts`)
	if err != nil {
		return nil, err
	}
	postDeleted, err := loadIDSet(ctx, pool, `SELECT id::text FROM posts WHERE deleted_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}

	// Approaches.
	if err := scanContribPosts(ctx, pool,
		`SELECT id::text, problem_id::text, status, (deleted_at IS NOT NULL) FROM approaches`,
		func(id, postID, status string, deleted bool) {
			rep.ApproachesTotal++
			if deleted {
				rep.ApproachesDeleted++
			} else {
				rep.ApproachesLive++
			}
			rep.ApproachesByStatus[status]++
			rep.addPostRelationException("approach", id, postID, deleted, postExists, postDeleted)
		}); err != nil {
		return nil, err
	}

	// Answers.
	if err := scanContribPosts(ctx, pool,
		`SELECT id::text, question_id::text, is_accepted::text, (deleted_at IS NOT NULL) FROM answers`,
		func(id, postID, isAccepted string, deleted bool) {
			rep.AnswersTotal++
			if deleted {
				rep.AnswersDeleted++
			} else {
				rep.AnswersLive++
			}
			if isAccepted == "true" {
				rep.AcceptedAnswers++
			}
			rep.addPostRelationException("answer", id, postID, deleted, postExists, postDeleted)
		}); err != nil {
		return nil, err
	}

	// Responses (no soft delete).
	if err := scanContribPosts(ctx, pool,
		`SELECT id::text, idea_id::text, ''::text, FALSE FROM responses`,
		func(id, postID, _ string, deleted bool) {
			rep.ResponsesTotal++
			rep.addPostRelationException("response", id, postID, deleted, postExists, postDeleted)
		}); err != nil {
		return nil, err
	}

	// Comments: detect orphans whose target row no longer exists.
	approachExists, err := loadIDSet(ctx, pool, `SELECT id::text FROM approaches`)
	if err != nil {
		return nil, err
	}
	answerExists, err := loadIDSet(ctx, pool, `SELECT id::text FROM answers`)
	if err != nil {
		return nil, err
	}
	responseExists, err := loadIDSet(ctx, pool, `SELECT id::text FROM responses`)
	if err != nil {
		return nil, err
	}
	targetSets := map[string]map[string]bool{
		"post":     postExists,
		"approach": approachExists,
		"answer":   answerExists,
		"response": responseExists,
	}
	if err := scanContribPosts(ctx, pool,
		`SELECT id::text, target_type, target_id::text, (deleted_at IS NOT NULL) FROM comments`,
		func(id, targetType, targetID string, deleted bool) {
			rep.CommentsTotal++
			if deleted {
				rep.CommentsDeleted++
			} else {
				rep.CommentsLive++
			}
			rep.CommentsByTarget[targetType]++
			set, known := targetSets[targetType]
			if !known || !set[targetID] {
				rep.Exceptions = append(rep.Exceptions, ContributionMigrationException{
					LegacyType: "comment", LegacyID: id, Kind: ContribExceptionOrphanComment,
					Detail: fmt.Sprintf("target %s %s not found", targetType, targetID),
				})
			}
		}); err != nil {
		return nil, err
	}

	return rep, nil
}

// addPostRelationException flags a contribution whose parent post is missing (genuine
// orphan) or is soft-deleted while the contribution itself is live (retained but reported).
func (r *ContributionMigrationReport) addPostRelationException(legacyType, id, postID string, deleted bool, postExists, postDeleted map[string]bool) {
	if !postExists[postID] {
		r.Exceptions = append(r.Exceptions, ContributionMigrationException{
			LegacyType: legacyType, LegacyID: id, Kind: ContribExceptionOrphanContribution,
			Detail: fmt.Sprintf("post %s not found", postID),
		})
		return
	}
	if !deleted && postDeleted[postID] {
		r.Exceptions = append(r.Exceptions, ContributionMigrationException{
			LegacyType: legacyType, LegacyID: id, Kind: ContribExceptionContributionOnDeletedPost,
			Detail: fmt.Sprintf("live %s on soft-deleted post %s", legacyType, postID),
		})
	}
}

// MigrateContributions converts every legacy approach, answer, response, and comment into a
// canonical Reply, preserving content and provenance (steps 2, 3). It is idempotent and
// resumable: the unique index on (legacy_type, legacy_id) makes ON CONFLICT DO NOTHING skip
// rows already migrated, so a second run creates nothing (step 6). Comments are converted
// after their targets so a comment on a contribution becomes a child of that contribution's
// reply and a comment on a post becomes a top-level reply. Genuine orphan comments are left
// for VerifyContributionMigration to report. Returns the number of replies created.
func MigrateContributions(ctx context.Context, pool *Pool) (int64, error) {
	var created int64

	// 1. Approaches -> replies (labeled Markdown sections).
	rows, err := pool.Query(ctx, `
		SELECT id::text, problem_id::text, author_type, author_id, angle, method,
		       assumptions, differs_from::text[], status, outcome, solution,
		       is_latest, archived_cid, created_at, updated_at, deleted_at
		FROM approaches`)
	if err != nil {
		return created, fmt.Errorf("migrate contributions: query approaches: %w", err)
	}
	type approachRow struct {
		id, postID, authorType, authorID, angle, status string
		method, outcome, solution, archivedCID          *string
		assumptions, differsFrom                         []string
		isLatest                                         bool
		createdAt                                        time.Time
		updatedAt, deletedAt                             *time.Time
	}
	var approaches []approachRow
	for rows.Next() {
		var a approachRow
		if err := rows.Scan(&a.id, &a.postID, &a.authorType, &a.authorID, &a.angle, &a.method,
			&a.assumptions, &a.differsFrom, &a.status, &a.outcome, &a.solution,
			&a.isLatest, &a.archivedCID, &a.createdAt, &a.updatedAt, &a.deletedAt); err != nil {
			rows.Close()
			return created, fmt.Errorf("migrate contributions: scan approach: %w", err)
		}
		approaches = append(approaches, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return created, fmt.Errorf("migrate contributions: approach rows: %w", err)
	}
	for _, a := range approaches {
		body := buildApproachBody(a.angle, ptr(a.method), a.assumptions, a.differsFrom, a.status, ptr(a.outcome), ptr(a.solution))
		prov := mustJSON(map[string]any{
			"legacy_table": "approaches", "angle": a.angle, "method": a.method,
			"assumptions": a.assumptions, "differs_from": a.differsFrom, "status": a.status,
			"outcome": a.outcome, "solution": a.solution, "is_latest": a.isLatest,
			"archived_cid": a.archivedCID,
		})
		ins, err := insertMigratedReply(ctx, pool, a.postID, nil, a.authorType, a.authorID, body,
			0, 0, "approach", a.id, prov, a.createdAt, coalesceTime(a.updatedAt, a.createdAt), a.deletedAt)
		if err != nil {
			return created, err
		}
		if ins {
			created++
		}
	}

	// 2. Answers -> replies (content body; is_accepted preserved in provenance).
	if n, err := migrateSimpleContributions(ctx, pool, `
		SELECT id::text, question_id::text, author_type, author_id, content, upvotes, downvotes,
		       is_accepted, created_at, deleted_at
		FROM answers`, "answer", func(extra map[string]any, isAccepted bool) {
		extra["legacy_table"] = "answers"
		extra["is_accepted"] = isAccepted
	}); err != nil {
		return created, err
	} else {
		created += n
	}

	// 3. Responses -> replies (content body; response_type preserved in provenance).
	if n, err := migrateResponses(ctx, pool); err != nil {
		return created, err
	} else {
		created += n
	}

	// 4. Post comments -> top-level replies; contribution comments -> child replies.
	if n, err := migrateComments(ctx, pool); err != nil {
		return created, err
	} else {
		created += n
	}

	return created, nil
}

// migrateSimpleContributions handles answers (content-only body) with a preserved boolean
// flag folded into provenance.
func migrateSimpleContributions(ctx context.Context, pool *Pool, query, legacyType string, setProv func(extra map[string]any, flag bool)) (int64, error) {
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("migrate %ss: query: %w", legacyType, err)
	}
	type row struct {
		id, postID, authorType, authorID, content string
		up, down                                  int
		flag                                      bool
		createdAt                                 time.Time
		deletedAt                                 *time.Time
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.postID, &r.authorType, &r.authorID, &r.content, &r.up, &r.down,
			&r.flag, &r.createdAt, &r.deletedAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("migrate %ss: scan: %w", legacyType, err)
		}
		all = append(all, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("migrate %ss: rows: %w", legacyType, err)
	}
	var created int64
	for _, r := range all {
		extra := map[string]any{}
		setProv(extra, r.flag)
		ins, err := insertMigratedReply(ctx, pool, r.postID, nil, r.authorType, r.authorID, r.content,
			r.up, r.down, legacyType, r.id, mustJSON(extra), r.createdAt, r.createdAt, r.deletedAt)
		if err != nil {
			return created, err
		}
		if ins {
			created++
		}
	}
	return created, nil
}

// migrateResponses converts idea responses (no soft delete) into replies, preserving the
// response_type in provenance.
func migrateResponses(ctx context.Context, pool *Pool) (int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, idea_id::text, author_type, author_id, content, response_type,
		       upvotes, downvotes, created_at
		FROM responses`)
	if err != nil {
		return 0, fmt.Errorf("migrate responses: query: %w", err)
	}
	type row struct {
		id, postID, authorType, authorID, content, responseType string
		up, down                                                int
		createdAt                                               time.Time
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.postID, &r.authorType, &r.authorID, &r.content, &r.responseType,
			&r.up, &r.down, &r.createdAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("migrate responses: scan: %w", err)
		}
		all = append(all, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("migrate responses: rows: %w", err)
	}
	var created int64
	for _, r := range all {
		prov := mustJSON(map[string]any{"legacy_table": "responses", "response_type": r.responseType})
		ins, err := insertMigratedReply(ctx, pool, r.postID, nil, r.authorType, r.authorID, r.content,
			r.up, r.down, "response", r.id, prov, r.createdAt, r.createdAt, nil)
		if err != nil {
			return created, err
		}
		if ins {
			created++
		}
	}
	return created, nil
}

// migrateComments converts comments into replies. A comment on a post becomes a top-level
// reply; a comment on a contribution becomes a child of that contribution's migrated reply,
// inheriting its post_id. Comments whose target was not migrated (a genuine orphan) are
// skipped and left for VerifyContributionMigration to report.
func migrateComments(ctx context.Context, pool *Pool) (int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, target_type, target_id::text, author_type, author_id, content,
		       created_at, deleted_at
		FROM comments`)
	if err != nil {
		return 0, fmt.Errorf("migrate comments: query: %w", err)
	}
	type row struct {
		id, targetType, targetID, authorType, authorID, content string
		createdAt                                               time.Time
		deletedAt                                               *time.Time
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.targetType, &r.targetID, &r.authorType, &r.authorID, &r.content,
			&r.createdAt, &r.deletedAt); err != nil {
			rows.Close()
			return 0, fmt.Errorf("migrate comments: scan: %w", err)
		}
		all = append(all, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("migrate comments: rows: %w", err)
	}

	postExists, err := loadIDSet(ctx, pool, `SELECT id::text FROM posts`)
	if err != nil {
		return 0, err
	}

	var created int64
	for _, r := range all {
		prov := mustJSON(map[string]any{
			"legacy_table": "comments", "target_type": r.targetType, "target_id": r.targetID,
		})
		var postID string
		var parent *string
		if r.targetType == "post" {
			if !postExists[r.targetID] {
				continue // genuine orphan; reported by Verify
			}
			postID = r.targetID
		} else {
			// Resolve the parent reply migrated from the target contribution.
			var pid, ppost string
			err := pool.QueryRow(ctx,
				`SELECT id::text, post_id::text FROM replies WHERE legacy_type=$1 AND legacy_id=$2`,
				r.targetType, r.targetID).Scan(&pid, &ppost)
			if err != nil {
				continue // target contribution not migrated (orphan); reported by Verify
			}
			parent = &pid
			postID = ppost
		}
		ins, err := insertMigratedReply(ctx, pool, postID, parent, r.authorType, r.authorID, r.content,
			0, 0, "comment", r.id, prov, r.createdAt, r.createdAt, r.deletedAt)
		if err != nil {
			return created, err
		}
		if ins {
			created++
		}
	}
	return created, nil
}

// insertMigratedReply inserts one migrated reply, skipping rows already present via the
// unique (legacy_type, legacy_id) index. It returns whether a new row was created.
func insertMigratedReply(ctx context.Context, pool *Pool, postID string, parentReplyID *string,
	authorType, authorID, body string, upvotes, downvotes int, legacyType, legacyID string,
	provenance json.RawMessage, createdAt, updatedAt time.Time, deletedAt *time.Time) (bool, error) {
	tag, err := pool.Exec(ctx, `
		INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body,
			upvotes, downvotes, legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12,$13)
		ON CONFLICT (legacy_type, legacy_id) WHERE legacy_id IS NOT NULL DO NOTHING`,
		postID, parentReplyID, authorType, authorID, body, upvotes, downvotes,
		legacyType, legacyID, string(provenance), createdAt, updatedAt, deletedAt)
	if err != nil {
		return false, fmt.Errorf("insert migrated reply (%s %s): %w", legacyType, legacyID, err)
	}
	return tag.RowsAffected() == 1, nil
}

// RemapAcceptedAnswerReferences rewrites posts.accepted_answer_id from a legacy answer id to
// the canonical reply id migrated from that answer, using the (legacy_type, legacy_id) map
// (step 4). It is idempotent: once rewritten the value is a reply id that matches no answer
// legacy_id, so a re-run is a no-op. Returns the number of posts updated.
func RemapAcceptedAnswerReferences(ctx context.Context, pool *Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE posts p SET accepted_answer_id = r.id
		FROM replies r
		WHERE r.legacy_type = 'answer' AND r.legacy_id = p.accepted_answer_id
		  AND p.accepted_answer_id IS NOT NULL`)
	if err != nil {
		return 0, fmt.Errorf("remap accepted answer references: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RemapContributionVotesAndReports retargets votes and reports from legacy contribution rows
// to their canonical replies through the (legacy_type, legacy_id) map (step 4). Both are
// idempotent: after retargeting, target_type is 'reply' and matches no legacy_type.
func RemapContributionVotesAndReports(ctx context.Context, pool *Pool) (votes int64, reports int64, err error) {
	vt, err := pool.Exec(ctx, `
		UPDATE votes SET target_type = 'reply', target_id = r.id
		FROM replies r
		WHERE votes.target_type IN ('approach','answer','response')
		  AND r.legacy_type = votes.target_type AND r.legacy_id = votes.target_id`)
	if err != nil {
		return 0, 0, fmt.Errorf("remap contribution votes: %w", err)
	}
	rt, err := pool.Exec(ctx, `
		UPDATE reports SET target_type = 'reply', target_id = r.id
		FROM replies r
		WHERE reports.target_type IN ('approach','answer','response','comment')
		  AND r.legacy_type = reports.target_type AND r.legacy_id = reports.target_id`)
	if err != nil {
		return vt.RowsAffected(), 0, fmt.Errorf("remap contribution reports: %w", err)
	}
	return vt.RowsAffected(), rt.RowsAffected(), nil
}

// buildApproachBody maps an approach's fields into labeled Markdown sections so none of the
// original text is discarded (step 2). Approach and Status are always present so a converted
// approach — including a failed one — stays recognizable (step 6); empty optional fields are
// omitted rather than emitting blank headings.
func buildApproachBody(angle, method string, assumptions, differsFrom []string, status, outcome, solution string) string {
	var secs []string
	secs = append(secs, "**Approach:** "+angle)
	if strings.TrimSpace(method) != "" {
		secs = append(secs, "**Method:** "+method)
	}
	if len(assumptions) > 0 {
		secs = append(secs, "**Assumptions:**\n"+bulletList(assumptions))
	}
	if len(differsFrom) > 0 {
		secs = append(secs, "**Differs from:**\n"+bulletList(differsFrom))
	}
	secs = append(secs, "**Status:** "+status)
	if strings.TrimSpace(outcome) != "" {
		secs = append(secs, "**Outcome:**\n\n"+outcome)
	}
	if strings.TrimSpace(solution) != "" {
		secs = append(secs, "**Solution:**\n\n"+solution)
	}
	return strings.Join(secs, "\n\n")
}

func bulletList(items []string) string {
	lines := make([]string, len(items))
	for i, it := range items {
		lines[i] = "- " + it
	}
	return strings.Join(lines, "\n")
}

// scanContribPosts runs a 4-column query (id, post/target id, a text field, deleted bool)
// and calls fn per row. It centralizes the scan boilerplate the inventory reuses.
func scanContribPosts(ctx context.Context, pool *Pool, query string, fn func(id, second, third string, deleted bool)) error {
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("scan contributions: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, second, third string
		var deleted bool
		if err := rows.Scan(&id, &second, &third, &deleted); err != nil {
			return fmt.Errorf("scan contributions: scan: %w", err)
		}
		fn(id, second, third, deleted)
	}
	return rows.Err()
}

// loadIDSet loads a single text-id column into a set for in-memory existence checks.
func loadIDSet(ctx context.Context, pool *Pool, query string) (map[string]bool, error) {
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("load id set: query: %w", err)
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("load id set: scan: %w", err)
		}
		set[id] = true
	}
	return set, rows.Err()
}

func ptr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func coalesceTime(v *time.Time, fallback time.Time) time.Time {
	if v == nil {
		return fallback
	}
	return *v
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		// The inputs are plain maps of strings/bools/slices; marshaling cannot fail.
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}
