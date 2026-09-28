package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LegacyRemapExceptionUnresolved: a dependent row points at a legacy contribution that has
// no migrated reply, so it is left exactly as stored and reported instead of guessed.
const LegacyRemapExceptionUnresolved = "unresolved_reference"

// LegacyRelationRemapReport counts what RemapLegacyRelations changed and lists every
// dependent row it could not resolve. Counts are changes made by this run, so a second run
// over the same data reports zeros.
type LegacyRelationRemapReport struct {
	AcceptedAnswers       int64
	Votes                 int64
	Reports               int64
	Flags                 int64
	ApproachRelationships int64
	ProgressNotes         int64
	NotificationLinks     int64
	Exceptions            []ContributionMigrationException
}

// ExceptionsFor returns only the exceptions whose LegacyID is in ids.
func (r *LegacyRelationRemapReport) ExceptionsFor(ids ...string) []ContributionMigrationException {
	return (&ContributionMigrationReport{Exceptions: r.Exceptions}).ExceptionsFor(ids...)
}

func (r *LegacyRelationRemapReport) unresolved(kind, id, detail string) {
	r.Exceptions = append(r.Exceptions, ContributionMigrationException{
		LegacyType: kind, LegacyID: id, Kind: LegacyRemapExceptionUnresolved, Detail: detail,
	})
}

// RemapLegacyRelations is the cutover step that follows MigrateContributions: it moves
// every relationship that names a legacy contribution onto the reply migrated from it,
// through the (legacy_type, legacy_id) map (task idx 76 step 2). Accepted answers, votes,
// reports and flags are retargeted; approach relationships are kept in the from-reply's
// provenance; progress notes become child replies; stored notification links are rewritten
// to canonical destinations. It only updates or inserts rows in replies, posts, votes,
// reports, flags and notifications.link: it never creates a notification or sends email.
// Every step is idempotent, so a partial run can simply be repeated.
func RemapLegacyRelations(ctx context.Context, pool *Pool) (*LegacyRelationRemapReport, error) {
	rep := &LegacyRelationRemapReport{}
	var err error
	if rep.AcceptedAnswers, err = RemapAcceptedAnswerReferences(ctx, pool); err != nil {
		return rep, err
	}
	if rep.Votes, rep.Reports, err = RemapContributionVotesAndReports(ctx, pool); err != nil {
		return rep, err
	}
	if rep.Flags, err = remapContributionFlags(ctx, pool); err != nil {
		return rep, err
	}
	for _, table := range []string{"votes", "reports", "flags"} {
		if err := reportUnmovedTargets(ctx, pool, rep, table); err != nil {
			return rep, err
		}
	}
	if err := remapApproachRelationships(ctx, pool, rep); err != nil {
		return rep, err
	}
	if err := migrateProgressNotes(ctx, pool, rep); err != nil {
		return rep, err
	}
	if err := rewriteNotificationLinks(ctx, pool, rep); err != nil {
		return rep, err
	}
	return rep, nil
}

// remapContributionFlags retargets flags like RemapContributionVotesAndReports does reports.
func remapContributionFlags(ctx context.Context, pool *Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE flags SET target_type = 'reply', target_id = r.id
		FROM replies r
		WHERE flags.target_type IN ('approach','answer','response','comment')
		  AND r.legacy_type = flags.target_type AND r.legacy_id = flags.target_id`)
	if err != nil {
		return 0, fmt.Errorf("remap contribution flags: %w", err)
	}
	return tag.RowsAffected(), nil
}

// reportUnmovedTargets lists rows of votes/reports/flags still naming a legacy contribution
// after the remap: their target has no migrated reply.
func reportUnmovedTargets(ctx context.Context, pool *Pool, rep *LegacyRelationRemapReport, table string) error {
	rows, err := pool.Query(ctx, `SELECT id::text, target_type, target_id::text FROM `+table+`
		WHERE target_type IN ('approach','answer','response','comment')`)
	if err != nil {
		return fmt.Errorf("remap legacy relations: unmoved %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, targetType, targetID string
		if err := rows.Scan(&id, &targetType, &targetID); err != nil {
			return fmt.Errorf("remap legacy relations: scan unmoved %s: %w", table, err)
		}
		rep.unresolved(strings.TrimSuffix(table, "s"), id, fmt.Sprintf("no reply migrated from %s %s", targetType, targetID))
	}
	return rows.Err()
}

// remapApproachRelationships records each approach relationship in the provenance of the
// reply migrated from its from-approach, naming both ends by reply id and by approach id.
// The relationship's own id guards against recording it twice.
func remapApproachRelationships(ctx context.Context, pool *Pool, rep *LegacyRelationRemapReport) error {
	rows, err := pool.Query(ctx, `
		SELECT ar.id::text, ar.relation_type, ar.created_at, ar.from_approach_id::text,
		       ar.to_approach_id::text, fr.id::text, tr.id::text
		FROM approach_relationships ar
		LEFT JOIN replies fr ON fr.legacy_type = 'approach' AND fr.legacy_id = ar.from_approach_id
		LEFT JOIN replies tr ON tr.legacy_type = 'approach' AND tr.legacy_id = ar.to_approach_id
		ORDER BY ar.created_at, ar.id`)
	if err != nil {
		return fmt.Errorf("remap approach relationships: query: %w", err)
	}
	type relRow struct {
		id, relationType, fromApproach, toApproach string
		createdAt                                  time.Time
		fromReply, toReply                         *string
	}
	var rels []relRow
	for rows.Next() {
		var r relRow
		if err := rows.Scan(&r.id, &r.relationType, &r.createdAt, &r.fromApproach, &r.toApproach, &r.fromReply, &r.toReply); err != nil {
			rows.Close()
			return fmt.Errorf("remap approach relationships: scan: %w", err)
		}
		rels = append(rels, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("remap approach relationships: rows: %w", err)
	}

	for _, r := range rels {
		if r.fromReply == nil || r.toReply == nil {
			rep.unresolved("approach_relationship", r.id,
				fmt.Sprintf("approach %s -> %s has no migrated reply on both ends", r.fromApproach, r.toApproach))
			continue
		}
		entry := mustJSON(map[string]string{
			"legacy_id": r.id, "relation_type": r.relationType,
			"to_reply_id": *r.toReply, "to_approach_id": r.toApproach,
			"created_at": r.createdAt.UTC().Format(time.RFC3339Nano),
		})
		tag, err := pool.Exec(ctx, `
			UPDATE replies SET provenance = jsonb_set(COALESCE(provenance, '{}'::jsonb),
				'{approach_relationships}',
				COALESCE(provenance->'approach_relationships', '[]'::jsonb) || jsonb_build_array($2::jsonb))
			WHERE id = $1
			  AND NOT COALESCE(provenance->'approach_relationships', '[]'::jsonb)
			      @> jsonb_build_array(jsonb_build_object('legacy_id', $3::text))`,
			*r.fromReply, string(entry), r.id)
		if err != nil {
			return fmt.Errorf("remap approach relationship %s: %w", r.id, err)
		}
		rep.ApproachRelationships += tag.RowsAffected()
	}
	return nil
}

// migrateProgressNotes turns each progress note into a child reply of the reply migrated
// from its approach. Only an approach's author could add notes to it, so the note keeps that
// author; it keeps its own time, and the approach reply's deletion so nothing is revived.
func migrateProgressNotes(ctx context.Context, pool *Pool, rep *LegacyRelationRemapReport) error {
	tag, err := pool.Exec(ctx, `
		INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body,
			legacy_type, legacy_id, provenance, created_at, updated_at, deleted_at)
		SELECT r.post_id, r.id, r.author_type, r.author_id, n.content, 'progress_note', n.id,
		       jsonb_build_object('legacy_table', 'progress_notes', 'approach_id', n.approach_id::text),
		       COALESCE(n.created_at, r.created_at), COALESCE(n.created_at, r.created_at), r.deleted_at
		FROM progress_notes n
		JOIN replies r ON r.legacy_type = 'approach' AND r.legacy_id = n.approach_id
		ON CONFLICT (legacy_type, legacy_id) WHERE legacy_id IS NOT NULL DO NOTHING`)
	if err != nil {
		return fmt.Errorf("migrate progress notes: %w", err)
	}
	rep.ProgressNotes = tag.RowsAffected()

	rows, err := pool.Query(ctx, `
		SELECT n.id::text, n.approach_id::text FROM progress_notes n
		WHERE NOT EXISTS (SELECT 1 FROM replies r WHERE r.legacy_type = 'approach' AND r.legacy_id = n.approach_id)`)
	if err != nil {
		return fmt.Errorf("migrate progress notes: unresolved: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, approachID string
		if err := rows.Scan(&id, &approachID); err != nil {
			return fmt.Errorf("migrate progress notes: scan unresolved: %w", err)
		}
		rep.unresolved("progress_note", id, fmt.Sprintf("approach %s has no migrated reply", approachID))
	}
	return rows.Err()
}

// LegacyReplyResolver returns the post and reply migrated from a legacy contribution.
type LegacyReplyResolver func(legacyType, legacyID string) (postID, replyID string, found bool)

var (
	legacyLinkAnchor  = regexp.MustCompile(`#(approach|answer|response|comment)-([0-9a-fA-F-]{36})$`)
	legacyLinkSegment = regexp.MustCompile(`^/(problems|questions|ideas)(/|$|[?#])`)
)

// CanonicalNotificationLink maps a stored notification link to its canonical destination.
// An anchor on a legacy contribution (#answer-<id>, #approach-<id>, #response-<id>,
// #comment-<id>) becomes /posts/<post>#<reply> of the reply migrated from it, whatever the
// path was; a /problems, /questions or /ideas page becomes the same path under /posts, like
// the frontend's permanent redirect. Any other link is returned unchanged. resolved is false
// only for a legacy anchor with no migrated reply, which is then returned as stored.
func CanonicalNotificationLink(link string, resolve LegacyReplyResolver) (canonical string, resolved bool) {
	if m := legacyLinkAnchor.FindStringSubmatch(link); m != nil {
		postID, replyID, found := resolve(m[1], strings.ToLower(m[2]))
		if !found {
			return link, false
		}
		return "/posts/" + postID + "#" + replyID, true
	}
	if m := legacyLinkSegment.FindStringSubmatch(link); m != nil {
		return "/posts" + link[len(m[1])+1:], true
	}
	return link, true
}

// rewriteNotificationLinks rewrites stored links in place. Only the link column changes:
// no row is added, and read state and timestamps stay as they were.
func rewriteNotificationLinks(ctx context.Context, pool *Pool, rep *LegacyRelationRemapReport) error {
	rows, err := pool.Query(ctx, `
		SELECT id::text, link FROM notifications
		WHERE link ~ '^/(problems|questions|ideas)(/|$|[?#])'
		   OR link ~ '#(approach|answer|response|comment)-'`)
	if err != nil {
		return fmt.Errorf("rewrite notification links: query: %w", err)
	}
	type notif struct{ id, link string }
	var all []notif
	for rows.Next() {
		var n notif
		if err := rows.Scan(&n.id, &n.link); err != nil {
			rows.Close()
			return fmt.Errorf("rewrite notification links: scan: %w", err)
		}
		all = append(all, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("rewrite notification links: rows: %w", err)
	}

	var lookupErr error
	resolve := func(legacyType, legacyID string) (string, string, bool) {
		if _, err := uuid.Parse(legacyID); err != nil {
			return "", "", false // not an id at all, so no reply can match it
		}
		var postID, replyID string
		err := pool.QueryRow(ctx, `SELECT post_id::text, id::text FROM replies
			WHERE legacy_type = $1 AND legacy_id = $2::uuid`, legacyType, legacyID).Scan(&postID, &replyID)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				lookupErr = err
			}
			return "", "", false
		}
		return postID, replyID, true
	}
	for _, n := range all {
		canonical, resolved := CanonicalNotificationLink(n.link, resolve)
		if lookupErr != nil {
			return fmt.Errorf("rewrite notification links: resolve %s: %w", n.id, lookupErr)
		}
		if !resolved {
			rep.unresolved("notification", n.id, "no reply migrated for link "+n.link)
			continue
		}
		if canonical == n.link {
			continue
		}
		tag, err := pool.Exec(ctx, `UPDATE notifications SET link = $2 WHERE id = $1 AND link = $3`, n.id, canonical, n.link)
		if err != nil {
			return fmt.Errorf("rewrite notification link %s: %w", n.id, err)
		}
		rep.NotificationLinks += tag.RowsAffected()
	}
	return nil
}
