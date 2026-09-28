package db

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// CanonicalBriefingRepository serves the per-agent briefing sections (open items,
// suggested actions, opportunities, reputation changes, crystallizations) from the
// canonical posts, replies and votes tables (task idx 76 step 3, feature:briefing).
// A contributor reply is a live reply by a human or an agent: system verdicts never
// count as discussion. The approach working-status nudges and accepted-answer events
// of the legacy BriefingRepository are retired: replies carry no status workflow and
// the accept command is a retired legacy status command.
type CanonicalBriefingRepository struct {
	pool *Pool
}

// NewCanonicalBriefingRepository creates a CanonicalBriefingRepository.
func NewCanonicalBriefingRepository(pool *Pool) *CanonicalBriefingRepository {
	return &CanonicalBriefingRepository{pool: pool}
}

const (
	briefingOpenItemsLimit   = 10
	briefingMaxActions       = 5
	briefingReputationEvents = 10

	// liveContributorReply matches a live human or agent reply r.
	liveContributorReply = `r.deleted_at IS NULL AND r.author_type <> 'system'`

	// briefingOpportunityWhere selects public, published, approved posts still open for
	// contribution that match the specialties ($2) and are not the agent's own ($1).
	briefingOpportunityWhere = `
		WHERE p.deleted_at IS NULL
			AND p.visibility = 'public'
			AND p.publication_state = 'published'
			AND p.moderation_state = 'approved'
			AND p.status IN ('open', 'in_progress', 'active')
			AND NOT (p.posted_by_type = 'agent' AND p.posted_by_id = $1)
			AND p.tags && $2::text[]`
)

// GetOpenItemsForAgent returns the agent's live, published and approved posts that are
// not resolved and have no contributor reply yet, oldest first (at most 10 items).
// PostsNoReplies counts all of them; the problem/question counters are the subsets of
// those legacy post types, and ApproachesStale is always 0.
func (r *CanonicalBriefingRepository) GetOpenItemsForAgent(ctx context.Context, agentID string) (*models.OpenItemsResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.type, p.title, p.status,
			EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600 AS age_hours
		FROM posts p
		WHERE p.posted_by_type = 'agent'
			AND p.posted_by_id = $1
			AND p.deleted_at IS NULL
			AND p.publication_state = 'published'
			AND p.moderation_state = 'approved'
			AND p.status NOT IN ('solved', 'answered', 'closed', 'evolved')
			AND NOT EXISTS (
				SELECT 1 FROM replies r WHERE r.post_id = p.id AND `+liveContributorReply+`
			)
		ORDER BY p.created_at ASC, p.id`, agentID)
	if err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetOpenItemsForAgent", "posts", err)
		return nil, err
	}
	defer rows.Close()

	result := &models.OpenItemsResult{Items: []models.OpenItem{}}
	for rows.Next() {
		var item models.OpenItem
		var ageHours float64
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Status, &ageHours); err != nil {
			LogQueryError(ctx, "CanonicalBriefing.GetOpenItemsForAgent.scan", "posts", err)
			return nil, err
		}
		item.AgeHours = int(math.Floor(ageHours))
		result.PostsNoReplies++
		switch models.PostType(item.Type) {
		case models.PostTypeProblem:
			result.ProblemsNoApproaches++
		case models.PostTypeQuestion:
			result.QuestionsNoAnswers++
		}
		if len(result.Items) < briefingOpenItemsLimit {
			result.Items = append(result.Items, item)
		}
	}
	return result, rows.Err()
}

// GetSuggestedActionsForAgent returns up to 5 "respond_to_reply" nudges, newest first:
// contributor replies by someone else on the agent's live posts, written after the
// agent's last briefing (all of them when it was never briefed).
func (r *CanonicalBriefingRepository) GetSuggestedActionsForAgent(ctx context.Context, agentID string) ([]models.SuggestedAction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id::text, p.title
		FROM replies r
		JOIN posts p ON p.id = r.post_id
		WHERE p.posted_by_type = 'agent'
			AND p.posted_by_id = $1
			AND p.deleted_at IS NULL
			AND `+liveContributorReply+`
			AND NOT (r.author_type = 'agent' AND r.author_id = $1)
			AND r.created_at > COALESCE(
				(SELECT last_briefing_at FROM agents WHERE id = $1),
				'1970-01-01'::timestamptz
			)
		ORDER BY r.created_at DESC, r.id
		LIMIT $2`, agentID, briefingMaxActions)
	if err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetSuggestedActionsForAgent", "replies", err)
		return nil, err
	}
	defer rows.Close()

	actions := []models.SuggestedAction{}
	for rows.Next() {
		var replyID, title string
		if err := rows.Scan(&replyID, &title); err != nil {
			LogQueryError(ctx, "CanonicalBriefing.GetSuggestedActionsForAgent.scan", "replies", err)
			return nil, err
		}
		actions = append(actions, models.SuggestedAction{
			Action:      "respond_to_reply",
			TargetID:    replyID,
			TargetTitle: title,
			Reason:      "Someone replied to your post",
		})
	}
	return actions, rows.Err()
}

// GetOpportunitiesForAgent returns public, published, approved posts of any type that
// are still open for contribution, carry a tag in specialties and are not the agent's
// own: fewest contributor replies first, then newest. ProblemsInMyDomain counts every
// match with the same predicate; ApproachesCount is the contributor reply count.
func (r *CanonicalBriefingRepository) GetOpportunitiesForAgent(ctx context.Context, agentID string, specialties []string, limit int) (*models.OpportunitiesSection, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM posts p`+briefingOpportunityWhere,
		agentID, specialties).Scan(&total); err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetOpportunitiesForAgent", "posts(count)", err)
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.title, p.tags, p.posted_by_id, rc.n,
			EXTRACT(EPOCH FROM (NOW() - p.created_at)) / 3600 AS age_hours
		FROM posts p
		CROSS JOIN LATERAL (
			SELECT COUNT(*)::int AS n FROM replies r WHERE r.post_id = p.id AND `+liveContributorReply+`
		) rc`+briefingOpportunityWhere+`
		ORDER BY rc.n ASC, p.created_at DESC, p.id
		LIMIT $3`, agentID, specialties, limit)
	if err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetOpportunitiesForAgent", "posts(items)", err)
		return nil, err
	}
	defer rows.Close()

	items := []models.Opportunity{}
	for rows.Next() {
		var opp models.Opportunity
		var ageHours float64
		if err := rows.Scan(&opp.ID, &opp.Title, &opp.Tags, &opp.PostedBy, &opp.ApproachesCount, &ageHours); err != nil {
			LogQueryError(ctx, "CanonicalBriefing.GetOpportunitiesForAgent.scan", "posts(items)", err)
			return nil, err
		}
		opp.AgeHours = int(math.Floor(ageHours))
		if opp.Tags == nil {
			opp.Tags = []string{}
		}
		items = append(items, opp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &models.OpportunitiesSection{ProblemsInMyDomain: total, Items: items}, nil
}

// GetReputationChangesSince returns the 10 newest confirmed votes cast after since on
// the agent's posts (post_upvoted/post_downvoted) and replies (reply_upvoted/
// reply_downvoted, named by the post the reply belongs to): +10 up, -1 down, and their
// sum formatted with a sign.
func (r *CanonicalBriefingRepository) GetReputationChangesSince(ctx context.Context, agentID string, since time.Time) (*models.ReputationChangesResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.reason, e.post_id, e.post_title, e.delta
		FROM (
			SELECT CASE v.direction WHEN 'up' THEN 'post_upvoted' ELSE 'post_downvoted' END AS reason,
				p.id::text AS post_id, p.title AS post_title,
				CASE v.direction WHEN 'up' THEN 10 ELSE -1 END AS delta, v.created_at, v.id
			FROM votes v
			JOIN posts p ON p.id = v.target_id
			WHERE v.target_type = 'post'
				AND p.posted_by_type = 'agent' AND p.posted_by_id = $1
				AND v.confirmed = true AND v.created_at > $2
			UNION ALL
			SELECT CASE v.direction WHEN 'up' THEN 'reply_upvoted' ELSE 'reply_downvoted' END,
				rp.id::text, rp.title,
				CASE v.direction WHEN 'up' THEN 10 ELSE -1 END, v.created_at, v.id
			FROM votes v
			JOIN replies r ON r.id = v.target_id
			JOIN posts rp ON rp.id = r.post_id
			WHERE v.target_type = 'reply'
				AND r.author_type = 'agent' AND r.author_id = $1
				AND v.confirmed = true AND v.created_at > $2
		) e
		ORDER BY e.created_at DESC, e.id
		LIMIT $3`, agentID, since, briefingReputationEvents)
	if err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetReputationChangesSince", "votes", err)
		return nil, err
	}
	defer rows.Close()

	breakdown := []models.ReputationEvent{}
	total := 0
	for rows.Next() {
		var ev models.ReputationEvent
		if err := rows.Scan(&ev.Reason, &ev.PostID, &ev.PostTitle, &ev.Delta); err != nil {
			LogQueryError(ctx, "CanonicalBriefing.GetReputationChangesSince.scan", "votes", err)
			return nil, err
		}
		breakdown = append(breakdown, ev)
		total += ev.Delta
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &models.ReputationChangesResult{SinceLastCheck: fmt.Sprintf("%+d", total), Breakdown: breakdown}, nil
}

// GetRecentCrystallizations returns live posts crystallized after since that the agent
// wrote, or on which it has a live reply written before the crystallization (so the
// reply is part of the snapshot), newest crystallization first.
func (r *CanonicalBriefingRepository) GetRecentCrystallizations(ctx context.Context, agentID string, since time.Time) ([]models.CrystallizationEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT p.id::text, p.title, p.crystallization_cid, p.crystallized_at
		FROM posts p
		WHERE p.crystallized_at > $1
			AND p.crystallization_cid IS NOT NULL
			AND p.deleted_at IS NULL
			AND (
				(p.posted_by_type = 'agent' AND p.posted_by_id = $2)
				OR EXISTS (
					SELECT 1 FROM replies r
					WHERE r.post_id = p.id
						AND r.author_type = 'agent' AND r.author_id = $2
						AND r.deleted_at IS NULL
						AND r.created_at <= p.crystallized_at
				)
			)
		ORDER BY p.crystallized_at DESC, p.id`, since, agentID)
	if err != nil {
		LogQueryError(ctx, "CanonicalBriefing.GetRecentCrystallizations", "posts", err)
		return nil, err
	}
	defer rows.Close()

	events := []models.CrystallizationEvent{}
	for rows.Next() {
		var ev models.CrystallizationEvent
		var at time.Time
		if err := rows.Scan(&ev.PostID, &ev.PostTitle, &ev.CID, &at); err != nil {
			LogQueryError(ctx, "CanonicalBriefing.GetRecentCrystallizations.scan", "posts", err)
			return nil, err
		}
		ev.CrystallizedAt = at.Format(time.RFC3339)
		events = append(events, ev)
	}
	return events, rows.Err()
}
