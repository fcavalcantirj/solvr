package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/reputation"
)

const reputationHistoryInsert = `
	INSERT INTO reputation_history (source, source_id, owner_type, owner_id, post_id, points, earned_at)`

// legacyReputationEvents are the events the legacy reputation formula scores that the
// canonical model cannot score again: post types and statuses, the contribution tables, and
// votes on contributions (retargeted to replies by the cutover). Each statement records one
// row per event with the points passed as $1, keeping the legacy filters: live rows only
// where the formula skipped deleted ones, human and agent authors only, confirmed votes.
// Votes on approaches scored nothing; they are recorded with 0 points so that, once
// retargeted to replies, canonical readers never score them. Votes on posts are not frozen:
// they stay votes on the same post and canonical readers score them live.
var legacyReputationEvents = []struct {
	source string
	points int
	query  string
}{
	{"problem_solved", reputation.PointsProblemSolved, reputationHistoryInsert + `
		SELECT 'problem_solved', p.id, p.posted_by_type, p.posted_by_id, p.id, $1, p.created_at
		FROM posts p
		WHERE p.type = 'problem' AND p.status = 'solved' AND p.deleted_at IS NULL
		ON CONFLICT DO NOTHING`},
	{"problem_contributed", reputation.PointsProblemContributed, reputationHistoryInsert + `
		SELECT 'problem_contributed', p.id, p.posted_by_type, p.posted_by_id, p.id, $1, p.created_at
		FROM posts p
		WHERE p.type = 'problem' AND p.deleted_at IS NULL
		ON CONFLICT DO NOTHING`},
	{"idea_posted", reputation.PointsIdeaPosted, reputationHistoryInsert + `
		SELECT 'idea_posted', p.id, p.posted_by_type, p.posted_by_id, p.id, $1, p.created_at
		FROM posts p
		WHERE p.type = 'idea' AND p.deleted_at IS NULL
		ON CONFLICT DO NOTHING`},
	{"answer_given", reputation.PointsAnswerGiven, reputationHistoryInsert + `
		SELECT 'answer_given', a.id, a.author_type, a.author_id, a.question_id, $1, a.created_at
		FROM answers a
		WHERE a.deleted_at IS NULL AND a.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"answer_accepted", reputation.PointsAnswerAccepted, reputationHistoryInsert + `
		SELECT 'answer_accepted', a.id, a.author_type, a.author_id, a.question_id, $1, a.created_at
		FROM answers a
		WHERE a.is_accepted = true AND a.deleted_at IS NULL AND a.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"response_given", reputation.PointsResponseGiven, reputationHistoryInsert + `
		SELECT 'response_given', r.id, r.author_type, r.author_id, r.idea_id, $1, r.created_at
		FROM responses r
		WHERE r.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"comment_given", reputation.PointsCommentGiven, reputationHistoryInsert + `
		SELECT 'comment_given', c.id, c.author_type, c.author_id, NULL, $1, c.created_at
		FROM comments c
		WHERE c.deleted_at IS NULL AND c.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"answer_upvote", reputation.PointsUpvoteReceived, reputationHistoryInsert + `
		SELECT 'answer_upvote', v.id, a.author_type, a.author_id, a.question_id, $1, v.created_at
		FROM votes v JOIN answers a ON a.id = v.target_id
		WHERE v.target_type = 'answer' AND v.confirmed = true AND v.direction = 'up'
			AND a.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"answer_downvote", reputation.PointsDownvoteReceived, reputationHistoryInsert + `
		SELECT 'answer_downvote', v.id, a.author_type, a.author_id, a.question_id, $1, v.created_at
		FROM votes v JOIN answers a ON a.id = v.target_id
		WHERE v.target_type = 'answer' AND v.confirmed = true AND v.direction = 'down'
			AND a.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"response_upvote", reputation.PointsUpvoteReceived, reputationHistoryInsert + `
		SELECT 'response_upvote', v.id, r.author_type, r.author_id, r.idea_id, $1, v.created_at
		FROM votes v JOIN responses r ON r.id = v.target_id
		WHERE v.target_type = 'response' AND v.confirmed = true AND v.direction = 'up'
			AND r.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"response_downvote", reputation.PointsDownvoteReceived, reputationHistoryInsert + `
		SELECT 'response_downvote', v.id, r.author_type, r.author_id, r.idea_id, $1, v.created_at
		FROM votes v JOIN responses r ON r.id = v.target_id
		WHERE v.target_type = 'response' AND v.confirmed = true AND v.direction = 'down'
			AND r.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
	{"approach_vote", 0, reputationHistoryInsert + `
		SELECT 'approach_vote', v.id, ap.author_type, ap.author_id, ap.problem_id, $1, v.created_at
		FROM votes v JOIN approaches ap ON ap.id = v.target_id
		WHERE v.target_type = 'approach' AND ap.author_type IN ('agent', 'human')
		ON CONFLICT DO NOTHING`},
}

// FreezeLegacyReputation records the reputation earned under the legacy rules into
// reputation_history, in one transaction, and returns how many events it recorded (task
// idx 76 steps 3-4). It must run before the cutover retargets votes to replies;
// RemapLegacyRelations runs it first. It is idempotent: an event already recorded is kept
// as it was, so a later run records only legacy rows written since.
func FreezeLegacyReputation(ctx context.Context, pool *Pool) (int64, error) {
	var total int64
	err := pool.WithTx(ctx, func(tx Tx) error {
		for _, ev := range legacyReputationEvents {
			tag, err := tx.Exec(ctx, ev.query, ev.points)
			if err != nil {
				return fmt.Errorf("freeze %s reputation: %w", ev.source, err)
			}
			total += tag.RowsAffected()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}
